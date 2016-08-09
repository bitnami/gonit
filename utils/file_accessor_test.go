package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"

	tu "github.com/bitnami/gonit/testutils"
	"github.com/stretchr/testify/assert"
)

var (
	unsafePermissions = [](os.FileMode){0777, 0755, 0722, 0677, 0044, 0004, 0020, 0002}
	safePermissions   = [](os.FileMode){0700, 0600, 0400}
)

func TestReadSecure(t *testing.T) {
	data := "sample-data"
	for _, p := range safePermissions {
		file, _ := sb.WriteFile(sb.TempFile(), []byte(data), p)
		contents, _ := ReadSecure(file)
		assert.Equal(t, contents, data)
	}
}

func TestReadSecureFailed(t *testing.T) {
	for _, p := range unsafePermissions {
		file, _ := sb.WriteFile(sb.TempFile(), []byte{}, p)
		os.Chmod(file, p)

		tu.AssertPanicsMatch(t,
			func() {
				ReadSecure(file)
			},
			regexp.MustCompile("must have permissions no more than"))
	}
}

func TestWriteSecure(t *testing.T) {
	data := "sample-data"
	f, _ := sb.WriteFile(sb.TempFile(), []byte(data), os.FileMode(0777))
	for _, file := range []string{sb.TempFile(), f} {
		WriteSecure(file, data)
		fileInfo, _ := os.Stat(file)
		expectedPerm := os.FileMode(0644)
		assert.Equal(t, fileInfo.Mode().Perm(), expectedPerm.Perm())
		b, _ := ioutil.ReadFile(file)
		assert.Equal(t, string(b), data)
	}
}

func TestEnsureSafePermissions(t *testing.T) {

	for _, p := range unsafePermissions {
		file, _ := sb.WriteFile(sb.TempFile(), []byte{}, p)
		os.Chmod(file, p)
		tu.AssertPanicsMatch(t,
			func() {
				EnsureSafePermissions(file)
			},
			regexp.MustCompile("must have permissions no more than"))
	}
}

func TestEnsurePermissions(t *testing.T) {

	for _, p := range unsafePermissions {
		file, _ := sb.WriteFile(sb.TempFile(), []byte{}, p)
		os.Chmod(file, p)
		assert.NotPanics(t,
			func() {
				EnsurePermissions(file, int(p))
			})

		tu.AssertPanicsMatch(t,
			func() {
				EnsurePermissions(file, 0700)
			},
			regexp.MustCompile("must have permissions no more than"))
	}
}

func TestOpenFileSecureNonExisting(t *testing.T) {
	nonExistentFile := sb.TempFile("a/b/c/d.txt")
	_, err := OpenFileSecure(nonExistentFile, syscall.O_RDONLY, 0700)
	assert.Error(t, err)

	for _, p := range []int{syscall.O_RDONLY, syscall.O_CREAT} {
		tu.AssertPanicsMatch(t,
			func() {
				OpenFileSecure(nonExistentFile, p, 0740)
			},
			regexp.MustCompile("Requested opening file in a too permissive mode"))
	}

	fh, err := OpenFileSecure(nonExistentFile, syscall.O_CREAT, 0700)
	assert.NoError(t, err)
	fh.Close()
	_, err = os.Stat(nonExistentFile)
	assert.NoError(t, err, "Expected file to exists")
	dir, _ := filepath.Split(nonExistentFile)
	stat, _ := os.Stat(dir)
	expectedPermissions := os.FileMode(0700).Perm()
	assert.Equal(t, stat.Mode().Perm(), expectedPermissions)
}

func init() {
	// Mockup the exit with panic
	doNotExit = true
}
