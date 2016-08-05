package testutils

import (
	"os"
	"path/filepath"
	"strings"
)

func fileSplit(p string) []string {
	return strings.Split(filepath.Clean(p), "/")
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err == nil {
		return true
	}
	return false
}
