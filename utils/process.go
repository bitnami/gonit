package utils

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ReadPid returns a parsed numeric pid from the given file.
// If the file does not exists, its contents do not contain a valid PID in the
// first line or any other error occurs, it will return -1 as the pid and the
// detailed error
func ReadPid(file string) (int, error) {
	if !FileExists(file) {
		return -1, fmt.Errorf("Pid file '%s' does not exist", file)
	}

	res, err := os.ReadFile(file)
	if err != nil {
		return -1, fmt.Errorf("Error reading pid file '%s': %s", file, err.Error())
	}
	firstLine := strings.Split(strings.TrimSpace(string(res)), "\n")[0]
	pidData := strings.TrimSpace(firstLine)
	pid, err := strconv.ParseUint(pidData, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("Malformed pid file '%s': First line must contain a positive integer number", file)
	}
	return int(pid), nil
}

// ValidatePidFilePath returns an error unless path is valid to write a PID file.
// Specifically, it will ensure it is writable and, if it already exists,
// point to a file.
func ValidatePidFilePath(path string) (err error) {
	if FileExists(path) && !IsFile(path) {
		err = fmt.Errorf("Invalid pid file: '%s' is not a file", path)
	} else if !IsWritable(path) {
		err = fmt.Errorf("Invalid pid file: '%s' is not writable", path)
	}
	return err
}

// IsProcessRunning returns true if a process is running in  the specified pid
// or false otherwhise
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err = process.Signal(syscall.Signal(0)); err == nil || err == syscall.EPERM {
		return true
	}
	return false
}

// WritePid saves the provided pid into file with safe permissions (644)
// Returns an error in case of failure
func WritePid(file string, pid int) error {
	if err := ValidatePidFilePath(file); err != nil {
		return err
	}
	if err := WriteSecure(file, fmt.Sprintf("%d", pid)); err != nil {
		return fmt.Errorf("Failed to serialize pid file '%s': %s", file, err.Error())
	}
	return nil
}
