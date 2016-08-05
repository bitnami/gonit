package utils

import (
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// AbsFile returns an absolute representation of path. Works like filepath.Abs
// but do not return a second error return value
func AbsFile(path string) string {
	fullPath := ""
	if filepath.IsAbs(path) {
		return path
	}

	res, err := filepath.Abs(path)
	if err != nil {
		fullPath = filepath.Join("/", path)
	} else {
		fullPath = res
	}
	return fullPath
}

// AbsFileFromRoot returns an absolute representation of path, using root
// as the cwd. Setting root to the empty string, uses the CWD
func AbsFileFromRoot(path string, root string) string {
	fullPath := ""
	if filepath.IsAbs(path) {
		fullPath = path
	} else if root == "" {
		fullPath = AbsFile(path)
	} else {
		fullPath = filepath.Join(AbsFile(root), path)
	}
	return fullPath
}

// IsFile returns true if path exists and is a file (or a link to a file) and false otherwise
func IsFile(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		// TODO We should return an error here
		return false
	}
	return stat.Mode().IsRegular()
}

// IsWritable returns true if the path exists and is writable or if it can be
// created, and false otherwise
func IsWritable(path string) bool {
	if !FileExists(path) {
		dir, _ := filepath.Split(filepath.Clean(path))
		return IsWritable(dir)
	}
	return syscall.Access(path, unix.W_OK) == nil
}

// IsWritableFile returns true if path is a file and writable or does not exists
// but could be created, and false otherwise
func IsWritableFile(path string) bool {
	if !FileExists(path) {
		return IsWritable(path)
	}
	return IsFile(path) && IsWritable(path)
}

// FileExists returns true if path exists or false otherwise
func FileExists(path string) bool {
	if _, err := os.Stat(AbsFile(path)); err == nil {
		return true
	}
	return false
}
