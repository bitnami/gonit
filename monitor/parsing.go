package monitor

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitnami/gonit/utils"
)

type configParser struct {
}

func (cp *configParser) cleanLines(data string) string {
	startWithCommentRe := regexp.MustCompile("^\\s*\\#.*")
	endsWithCommentRe := regexp.MustCompile("^\\s*([^\\s\\#].*?)\\#")
	result := []string{}
	for _, l := range strings.Split(data, "\n") {
		if startWithCommentRe.MatchString(l) {
			continue
		}

		if match := endsWithCommentRe.FindStringSubmatch(l); match != nil {
			result = append(result, match[1])
		} else {
			result = append(result, l)
		}
	}
	return strings.Join(result, "\n")
}

func (cp *configParser) parseVarSet(data string) (key, value string) {
	re := regexp.MustCompile("\\s*set\\s+([^\\s]+)(.*)")
	if match := re.FindStringSubmatch(data); match != nil {
		key = strings.TrimSpace(match[1])
		value = strings.TrimSpace(match[2])
	}
	return key, value
}

func (cp *configParser) parseObjSet(data string) (what string, result map[string]string) {
	result = make(map[string]string)
	re := regexp.MustCompile("^\\s*set\\s+(daemon|ssl|tls|httpd|alert|mail-format|mailserver|eventqueue|limits)\\s+(.*)")

	if match := re.FindStringSubmatch(data); match != nil {
		what = match[1]
		settingsRe := regexp.MustCompile("^\\s*([^\\s]+)\\s+([^\\s]+)(.*$)")
		toParse := match[2]
		for {
			if match = settingsRe.FindStringSubmatch(toParse); match == nil {
				break
			}
			toParse = match[3]
			key := match[1]
			value := match[2]
			result[key] = value
		}
	}
	return what, result
}

func (cp *configParser) parseInclude(data string) map[string]string {
	result := make(map[string]string)
	result["pattern"] = ""
	re := regexp.MustCompile("\\s*include\\s+(.*)")
	if match := re.FindStringSubmatch(data); match != nil {
		result["pattern"] = match[1]
	}
	return result
}

// TODO: ParseConfigFile and ParseConfig should be methods of monitor.Config
// and store everything there
func (cp *configParser) ParseConfigFile(f string, cw interface {
	configWalker
}, logger Logger) error {
	logger.Debugf("Parsing file %s", f)
	// TODO: Not sure if bug or feature, but monit only validates the first level
	// config file. Included files permissions and ownership are not validated
	// data, err := utils.ReadSecure(f)
	bytes, err := ioutil.ReadFile(utils.AbsFile(f))
	if err != nil {
		return err
	}
	return cp.ParseConfig(string(bytes), cw, logger)
}

func (cp *configParser) ParseConfig(config string, walker interface {
	configWalker
}, logger Logger) error {

	toParse := config

	// Cleanup comments
	toParse = cp.cleanLines(toParse)
	directivePattern := "(\n|^)\\s*(check|include|set)"

	checkPattern := fmt.Sprintf("(%s\\s+((.|\n)*?))((%s (.|\n)*)|$)", directivePattern, directivePattern)

	re := regexp.MustCompile(
		"(\\s*\\n)*" +
			checkPattern)

	var match []string
	for {
		if match = re.FindStringSubmatch(toParse); match == nil {
			break
		}
		directive := strings.TrimSpace(match[4])
		directiveConfig := strings.TrimSpace(match[2])
		toParse = match[7]
		switch directive {
		case "include":
			cfg := cp.parseInclude(directiveConfig)

			if matches, err := filepath.Glob(cfg["pattern"]); err == nil {
				for _, f := range matches {
					cp.ParseConfigFile(f, walker, logger)
				}
			} else {
				return err
			}
		case "check":
			c, err := newCheckFromData(directiveConfig)
			if err != nil {
				return err
			}
			if err := walker.AddCheck(c); err != nil {
				return err
			}
		case "set":
			what, data := cp.parseObjSet(directiveConfig)
			if what != "" {
				walker.SetNamespacedConfig(what, data)
			} else {
				walker.SetAttribute(cp.parseVarSet(directiveConfig))
			}
		default:
			logger.Debugf("Ignoring %s", directive)
		}
	}
	return nil
}
