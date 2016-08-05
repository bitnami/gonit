package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tu "github.com/bitnami/gonit/testutils"
	"github.com/stretchr/testify/assert"
)

var (
	sb *tu.Sandbox
)

func TestMain(m *testing.M) {

	sb = tu.NewSandbox()
	c := m.Run()

	sb.Cleanup()
	os.Exit(c)
}

func TestWaitUntilReturn(t *testing.T) {
	done := false
	precision := 10 * time.Millisecond
	triggerTime := 100 * time.Millisecond
	//	maxAcceptableTime := 100*time.Millisecond + 2*precision

	var start, end time.Time
	go func() {
		start = time.Now()
		time.Sleep(triggerTime)
		done = true
	}()

	ellapsed := tu.Measure(func() {
		WaitUntil(func() bool {
			return done
		}, 2*time.Second, precision)
	})
	end = time.Now()
	assert.WithinDuration(t, end, start.Add(triggerTime), 2*precision, "Measured time %v is out of the acceptable ranges", ellapsed)
}

func TestWaitUntilTimedOut(t *testing.T) {
	var start, end time.Time
	precision := 10 * time.Millisecond
	timeout := 300 * time.Millisecond

	start = time.Now()
	ellapsed := tu.Measure(func() {
		WaitUntil(func() bool {
			return false
		}, timeout, precision)
	})
	end = time.Now()
	assert.WithinDuration(t, end, start.Add(timeout), precision, "Measured time is out of the acceptable ranges: %v", ellapsed)
}

func TestWaitUntilDefaultPrecision(t *testing.T) {
	done := false
	triggerTime := 100 * time.Millisecond

	var start, end time.Time
	go func() {
		start = time.Now()
		time.Sleep(triggerTime)
		done = true
	}()

	ellapsed := tu.Measure(func() {
		WaitUntil(func() bool {
			return done
		}, 2*time.Second)
	})
	end = time.Now()
	// This is the function default precission
	defaultPrecision := 500 * time.Millisecond
	assert.WithinDuration(t, end, start.Add(defaultPrecision), 10*time.Millisecond, "Measured time is out of the acceptable ranges: %v", ellapsed)
}

func compareWithFilepathAbs(t *testing.T, fn func(string) string) {
	for _, p := range []string{"a", "b/c", "../a/b", "", "/", "/a/b/c"} {
		expectedPath, _ := filepath.Abs(p)
		fullPath := fn(p)
		assert.Equal(t, fullPath, expectedPath)
	}
}
func TestAbsFileBehavesAsAbs(t *testing.T) {
	compareWithFilepathAbs(t, AbsFile)
}

func TestAbsFileWithWrongCWD(t *testing.T) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	dir, _ := sb.Mkdir("cwd_test", os.FileMode(0766))
	// This should make Abs fail because of the invalid dir
	os.Chdir(dir)
	os.Remove(dir)

	assert.Equal(t, AbsFile("cwd_test"), "/cwd_test")
}

func TestAbsFileFromRoot(t *testing.T) {
	rootDir := "/file/root/dir/a"
	for tail, expectedPath := range map[string]string{
		"a":     filepath.Join(rootDir, "a"),
		"/b":    "/b",
		"c/d/e": filepath.Join(rootDir, "c/d/e"),
		"":      rootDir,
	} {
		assert.Equal(t, AbsFileFromRoot(tail, rootDir), expectedPath)
	}
}

func TestAbsFileFromRootWithEmptyRoot(t *testing.T) {
	compareWithFilepathAbs(t, func(p string) string {
		return AbsFileFromRoot(p, "")
	})
}

func TestAbsFileFromRootWithCwdRoot(t *testing.T) {
	cwd, _ := os.Getwd()
	compareWithFilepathAbs(t, func(p string) string {
		return AbsFileFromRoot(p, cwd)
	})
}
