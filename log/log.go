package log

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/bitnami/gonit/utils"
)

type Logger struct {
	logrus.Logger
}

const DebugLevel = logrus.DebugLevel

func (l *Logger) MLogf(level int, format string, args ...interface{}) {
	var printer func(args ...interface{})
	switch logrus.Level(level) {
	case DebugLevel:
		printer = l.Debug
	default:
		return
	}
	text := fmt.Sprintf(format, args...)
	for _, line := range strings.Split(text, "\n") {
		printer(line)
	}
}
func (l *Logger) MDebugf(format string, args ...interface{}) {
	l.MLogf(int(DebugLevel), format, args...)
}

func New() *logrus.Logger {
	return logrus.New()
}

func DummyLogger() *Logger {
	return StreamLogger(ioutil.Discard)
}

func StreamLogger(w interface {
	io.Writer
}) *Logger {
	return &Logger{Logger: logrus.Logger{
		Out: w, Formatter: &logrus.TextFormatter{FullTimestamp: true},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.InfoLevel,
	}}
}
func FileLogger(file string) *Logger {
	parentDir := filepath.Dir(file)
	if !utils.FileExists(parentDir) {
		if err := os.MkdirAll(parentDir, os.FileMode(0755)); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating parent directory for log file: %s\n", err.Error())
			return DummyLogger()
		}
	}
	fh, err := os.OpenFile(file, syscall.O_APPEND|syscall.O_RDWR|syscall.O_CREAT, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %s\n", err.Error())
		return DummyLogger()
	} else {
		return StreamLogger(fh)
	}
}
func init() {

}
