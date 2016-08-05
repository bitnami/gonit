package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"

	tu "github.com/bitnami/gonit/testutils"
	"github.com/stretchr/testify/assert"
)

func assertPidFileError(t *testing.T, pid int, err error, expectedError string) {
	tu.AssertErrorMatch(t, err, regexp.MustCompile(expectedError))
	assert.Equal(t, pid, -1, "Expected PID to be -1 on error")
}
func TestReadPidNonExistent(t *testing.T) {
	p, err := ReadPid(sb.TempFile())
	assertPidFileError(t, p, err, "^Pid file.*does not exist$")
}

func TestReadPidNonReadable(t *testing.T) {
	file, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0000))
	p, err := ReadPid(file)

	assertPidFileError(t, p, err, "^Error reading pid file")
}

func TestReadPidMalformed(t *testing.T) {
	for _, data := range []string{"", "asdf", "-1", "0.56"} {
		file, _ := sb.WriteFile(sb.TempFile(), []byte(data), os.FileMode(0700))
		p, err := ReadPid(file)

		assertPidFileError(t, p, err, "^Malformed pid file.*: First line must contain a positive integer number$")
	}
}

func TestReadPidSuccess(t *testing.T) {
	pid := 1234
	for _, data := range []string{"%d", "  %d", "   %d   ", "%d  ", "\n%d", "\n%d\nfoobar\nmore text"} {
		file, _ := sb.WriteFile(
			sb.TempFile(),
			[]byte(fmt.Sprintf(data, pid)),
			os.FileMode(0700),
		)
		p, err := ReadPid(file)
		assert.Equal(t, p, pid)
		assert.NoError(t, err)
	}
}

func TestValidatePidFileSuceeds(t *testing.T) {
	writableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	writableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0777))
	for _, f := range []string{
		filepath.Join(writableDir, "sample.pid"),
		writableFile,
	} {
		assert.NoError(t, ValidatePidFilePath(f))
	}
}
func TestValidatePidFilePathInvalid(t *testing.T) {
	nonWritableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0555))
	dir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0777))
	nonWritableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0555))

	for _, f := range []string{
		filepath.Join(nonWritableDir, "sample.pid"),
		nonWritableFile,
	} {
		tu.AssertErrorMatch(t, ValidatePidFilePath(f),
			regexp.MustCompile("Invalid pid file:.*is not writable"))
	}
	tu.AssertErrorMatch(t, ValidatePidFilePath(dir),
		regexp.MustCompile("Invalid pid file:.*is not a file"))
}

func TestIsProcessRunning(t *testing.T) {
	// Non running pids
	// 1234567 seems to be weird enough to assume won't be actually running
	for _, pid := range []int{1234567, -4} {
		assert.Equal(t, IsProcessRunning(pid), false,
			"Expected %d to not be running", pid)
	}
	// Running PIDs
	for _, pid := range []int{syscall.Getppid(), syscall.Getpid(), 1} {
		assert.Equal(t, IsProcessRunning(pid), true,
			"Expected %d to be running", pid)
	}
}

func TestWritePidErrors(t *testing.T) {
	pid := 12345
	nonWritableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0555))
	dir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0777))
	nonWritableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0555))

	for _, f := range []string{
		filepath.Join(nonWritableDir, "sample.pid"),
		nonWritableFile,
	} {
		tu.AssertErrorMatch(t, WritePid(f, pid),
			regexp.MustCompile("Invalid pid file:.*is not writable"))
	}
	tu.AssertErrorMatch(t, ValidatePidFilePath(dir),
		regexp.MustCompile("Invalid pid file:.*is not a file"))
}

func TestWritePidSucceeds(t *testing.T) {
	pid := 12345
	expectedPerm := os.FileMode(0644)
	writableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	writableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0777))
	for _, f := range []string{
		filepath.Join(writableDir, "sample.pid"),
		writableFile,
	} {
		assert.NoError(t, WritePid(f, pid))
		fileInfo, _ := os.Stat(f)
		assert.Equal(t, expectedPerm.Perm(), fileInfo.Mode().Perm(),
			"Expected %s permissions", expectedPerm)

		d, _ := ioutil.ReadFile(f)
		readPid, _ := strconv.Atoi(string(d))
		assert.Equal(t, readPid, pid, "Expected %d PID", pid)
	}
}
