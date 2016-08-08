package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAbsFileWithWrongCWD(t *testing.T) {
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	dir, _ := sb.Mkdir("cwd_test", os.FileMode(0766))
	// This should make Abs fail because of the invalid dir
	os.Chdir(dir)
	os.Remove(dir)

	assert.Equal(t, AbsFile("cwd_test"), "/cwd_test")
}
