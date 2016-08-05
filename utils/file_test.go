package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFile(t *testing.T) {
	dir, _ := sb.Mkdir("test_dir", os.FileMode(0755))
	file := sb.Touch("sample.txt")
	linkToFile, _ := sb.Symlink(file, "link-to-file")
	linkToDir, _ := sb.Symlink(dir, "link-to-dir")
	brokenLink, _ := sb.Symlink("broken-link-dest", "broken-link")
	for path, expected := range map[string]bool{
		dir:                               false,
		linkToFile:                        true,
		linkToDir:                         false,
		brokenLink:                        false,
		sb.Normalize("non-existent-path"): false,
		file: true,
	} {
		assert.Equal(t, IsFile(path), expected,
			"Expected IsFile('%s') to be '%t'", path, expected)
	}
}

func TestFileExists(t *testing.T) {
	existingFile := sb.Touch(sb.TempFile())
	existingDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	nonTraversableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0000))

	nonExistingFile := sb.TempFile()

	for path, expected := range map[string]bool{
		existingFile:                                  true,
		existingDir:                                   true,
		nonTraversableDir:                             true,
		filepath.Join(nonTraversableDir, "dummy.txt"): false,
		nonExistingFile:                               false,
	} {
		assert.Equal(t, FileExists(path), expected,
			"Expected FileExists('%s') to be '%t'", path, expected)
	}
}

func TestIsWritable(t *testing.T) {
	writableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0744))
	nonWritableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0444))
	writableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	nonWritableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0555))

	nonExistingFile := sb.TempFile()

	for path, expected := range map[string]bool{
		writableFile:                                true,
		nonWritableFile:                             false,
		writableDir:                                 true,
		filepath.Join(writableDir, "sample.txt"):    true,
		filepath.Join(writableDir, "a/b/c/d"):       true,
		filepath.Join(nonWritableDir, "sample.txt"): false,
		nonWritableDir:                              false,
		nonExistingFile:                             true,
	} {
		assert.Equal(t, IsWritable(path), expected,
			"Expected IsWritable('%s') to be '%t'", path, expected)
	}
}

func TestIsWritableFile(t *testing.T) {
	writableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0744))
	nonWritableFile, _ := sb.WriteFile(sb.TempFile(), []byte{}, os.FileMode(0444))
	writableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	nonWritableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0555))

	nonExistingFile := sb.TempFile()

	for path, expected := range map[string]bool{
		writableFile:                                true,
		nonWritableFile:                             false,
		writableDir:                                 false,
		filepath.Join(writableDir, "sample.txt"):    true,
		filepath.Join(nonWritableDir, "sample.txt"): false,
		nonWritableDir:                              false,
		nonExistingFile:                             true,
	} {
		assert.Equal(t, IsWritableFile(path), expected,
			"Expected IsWritableFile('%s') to be '%t'", path, expected)
	}
}
