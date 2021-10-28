package log

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/bitnami/gonit/utils"
	"github.com/sirupsen/logrus"
)

// Logger defines a logger object
type Logger struct {
	logrus.Logger
}

// DebugLevel defines the dubug verbosity of the logger
const DebugLevel = logrus.DebugLevel

func (l *Logger) mLogf(level int, format string, args ...interface{}) {
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

// MDebugf prints a multiline debug message.
// Multiple lines are splitted into multiple log entries
func (l *Logger) MDebugf(format string, args ...interface{}) {
	l.mLogf(int(DebugLevel), format, args...)
}

// New returns a new logger
func New() *logrus.Logger {
	return logrus.New()
}

// DummyLogger provides a dummy logger object
func DummyLogger() *Logger {
	return StreamLogger(ioutil.Discard)
}

// StreamLogger returns a logger backed by a provided io.Writter
func StreamLogger(w interface {
	io.Writer
}) *Logger {
	return &Logger{Logger: logrus.Logger{
		Out: w, Formatter: &logrus.TextFormatter{FullTimestamp: true},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.InfoLevel,
	}}
}

// FileLogger returns a logger backed by file
func FileLogger(file string) *Logger {
	var l *Logger

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
		l = DummyLogger()
	} else {
		l = StreamLogger(fh)
	}
	return l
}
