package gonittest

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/bitnami/gonit/utils"
)

var (
	tmplExtension = ".tmpl"
	tmplData      = make(map[string]string)
	searchPath    = []string{}
)

// CfgOpts defines the set of available configuration templates options
type CfgOpts struct {
	Name         string
	RootDir      string
	Timeout      string
	ConfFile     string
	PidFile      string
	ScriptsDir   string
	ScriptFile   string
	TempDir      string
	ConfDir      string
	TimeoutUnits string
	StopCmd      string
	StartCmd     string
}

type tmplResolver struct {
	Data  string
	Error error
}

func (r *tmplResolver) LoadFile(path string) error {
	if r.Error == nil {
		data, err := os.ReadFile(path)
		r.Data, r.Error = string(data), err
	}
	return r.Error
}

func (r *tmplResolver) Load(data string) {
	r.Data = data
}

func (r *tmplResolver) WriteToFile(path string) error {
	if r.Error == nil {
		r.Error = os.WriteFile(path, []byte(r.Data), os.FileMode(0755))
	}
	return r.Error
}

func (r *tmplResolver) Subst(opts *CfgOpts) (string, error) {
	r.do(opts)
	return r.Data, r.Error
}

func (r *tmplResolver) do(opts *CfgOpts) {
	if r.Error != nil {
		return
	}
	buff := bytes.NewBufferString("")
	tmpl, err := template.New("tmpl").Parse(r.Data)
	if err != nil {
		r.Error = err
		return
	}
	err = tmpl.Execute(buff, opts)
	if err != nil {
		r.Error = err
		return
	}
	if buff.String() != r.Data {
		r.Data = buff.String()
		r.do(opts)
	}
}

// SubstText renders a template text into a file with the provided opts settings
func SubstText(text string, file string, opts CfgOpts) error {
	r := new(tmplResolver)
	r.Load(text)
	r.Subst(fillupCfgOpts(&opts))
	return r.WriteToFile(file)
}

// Subst copies the template file at origint into the destination dest with the provided opts settings
func Subst(origin string, dest string, opts CfgOpts) error {
	r := new(tmplResolver)
	r.LoadFile(origin)
	r.Subst(fillupCfgOpts(&opts))
	return r.WriteToFile(dest)
}

// AddDirToTemplateSearchPath allows adding an extra dir to look for templates
func AddDirToTemplateSearchPath(dir string) {
	searchPath = append(searchPath, dir)
}

func findTemplate(id string) string {
	for _, p := range searchPath {
		tplFile := filepath.Join(p, id)
		if utils.FileExists(tplFile) {
			return tplFile
		}

		if !strings.HasSuffix(tplFile, tmplExtension) {
			tplFile += tmplExtension
			if utils.FileExists(tplFile) {
				return tplFile
			}
		}
	}
	return ""
}

// RenderTemplate renders a built-in template into the destination dest with the provided opts settings
func RenderTemplate(id string, dest string, opts CfgOpts) error {
	if data, ok := tmplData[id]; ok {
		return SubstText(data, dest, opts)
	}
	tplFile := findTemplate(id)
	if tplFile == "" {
		return fmt.Errorf("Cannot find template file for id %s", id)
	}
	return Subst(tplFile, dest, opts)
}

// TODO: This deserves its own function in utils, but it would require
// some more work to get a final version

func copyFile(src string, dest string) error {
	var srcFh, destFh *os.File
	var err error

	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if srcFh, err = os.Open(src); err == nil {
		defer srcFh.Close()
	} else {
		return err
	}

	if destFh, err = os.OpenFile(dest, syscall.O_TRUNC|syscall.O_RDWR|syscall.O_CREAT, info.Mode()); err == nil {
		defer destFh.Close()
	} else {
		return err
	}

	len, err := io.Copy(destFh, srcFh)
	if err != nil {
		return err
	}
	if len != info.Size() {
		return fmt.Errorf("Size mismatch. Got %d bytes but expected %d", len, info.Size())
	}
	return nil
}

// RenderSampleProcessCheck writes a sample process check configuration file into file with the provided opts settings
func RenderSampleProcessCheck(file string, opts CfgOpts) error {
	return RenderTemplate("service-check", file, opts)
}

// RenderConfiguration copies the files at origin into destDir, replacing templates in the process
func RenderConfiguration(origin string, destDir string, opts CfgOpts) error {
	matches, err := filepath.Glob(origin)
	if err != nil {
		return err
	}
	for _, p := range matches {
		rootDir := filepath.Dir(filepath.Clean(p))
		err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			relative, _ := filepath.Rel(rootDir, path)
			destFile := filepath.Join(destDir, relative)

			if info.Mode().IsRegular() {
				if strings.HasSuffix(path, tmplExtension) {
					destFile = strings.TrimSuffix(destFile, tmplExtension)
					if err := Subst(
						path,
						destFile,
						opts,
					); err != nil {
						return err
					}
				} else {
					err := copyFile(path, destFile)
					if err != nil {
						return err
					}
				}
			} else if info.IsDir() {
				os.MkdirAll(destFile, info.Mode())
			} else {
				return fmt.Errorf("Unknown file type (%s)", path)
			}
			os.Chmod(destFile, info.Mode().Perm())
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func fillupCfgOpts(cfg *CfgOpts) *CfgOpts {
	defautlOpts := CfgOpts{
		RootDir:      "",
		Name:         "sample",
		TimeoutUnits: "seconds",
		ScriptsDir:   "{{.RootDir}}/scripts",
		ScriptFile:   "{{.ScriptsDir}}/ctl.sh",
		ConfDir:      "{{.RootDir}}/conf",
		TempDir:      "{{.RootDir}}/temp",
		PidFile:      "{{.TempDir}}/{{.Name}}.pid",
		ConfFile:     "{{.ConfDir}}/{{.Name}}.cnf",
		StartCmd:     "{{.ScriptFile}} start",
		StopCmd:      "{{.ScriptFile}} stop",
	}

	defaultValue := reflect.ValueOf(defautlOpts)

	v := reflect.ValueOf(cfg)
	for i := 0; i < v.Elem().NumField(); i++ {
		if v.Elem().Field(i).Interface().(string) == "" {
			v.Elem().Field(i).Set(defaultValue.Field(i))
		}
	}
	return cfg
}

func init() {
	if cwd, err := os.Getwd(); err == nil {
		searchPath = append(searchPath, filepath.Join(cwd, "testdata"))
	}
	// Built-in templates
	tmplData["sample-ctl-script"] = `#!/bin/bash
PID=$$
PID_FILE="{{.PidFile}}"
STOP_FILE="{{.RootDir}}/{{.Name}}.stop"
ERROR_FILE="{{.RootDir}}/{{.Name}}.doerror"

function kill_self {
    kill -0 $PID
    if [[ $? == 0 ]]; then
        kill $PID
    fi
}

if [[ -f $ERROR_FILE ]]; then
    exit 1
fi
echo $PID > $PID_FILE

rm $STOP_FILE
(sleep 30; kill_self)&
while [[ ! -f $STOP_FILE ]]; do
  sleep 0.1
done
rm $PID_FILE
rm $STOP_FILE
echo TERMINATED
`
	tmplData["service-check"] = `
check process {{.Name}}
  with pidfile "{{.PidFile}}"
  start program = "{{.StartCmd}}" with timeout {{.Timeout}} {{.TimeoutUnits}}
  stop program = "{{.StopCmd}}" with timeout {{.Timeout}} {{.TimeoutUnits}}
`
}
