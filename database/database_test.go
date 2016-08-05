package database

import (
	"os"
	"regexp"
	"sort"
	"testing"

	tu "github.com/bitnami/gonit/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setDbData(db Storer, data map[string]interface{}) []string {
	keys := []string{}
	for k, v := range data {
		db.Set(k, v)
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var (
	sb         *tu.Sandbox
	sampleData = map[string]interface{}{
		"ping": "pong",
		"i":    125,
	}
)

func fileExists(f string) bool {
	if _, err := os.Stat(f); err == nil {
		return true
	}
	return false
}

func TestMain(m *testing.M) {

	sb = tu.NewSandbox()
	defer sb.Cleanup()

	os.Exit(m.Run())
}

func TestNewFileDatabaseNoFile(t *testing.T) {
	dbFile := sb.TempFile()

	assert.False(t, fileExists(dbFile), "File %s should not exists", dbFile)

	db, err := NewFileDatabase(dbFile)
	require.NoError(t, err)

	assert.Len(t, db.Keys(), 0)

	assert.False(t, fileExists(dbFile), "File %s should not exists", dbFile)

	assert.Equal(t, setDbData(db, sampleData), db.Keys())

	db.Serialize()

	assert.True(t, fileExists(dbFile),
		"File %s should exists after serialization", dbFile)
}

func TestNewFileDatabaseWithFile(t *testing.T) {
	dbFile := sb.Normalize("dbfile")
	db, err := NewFileDatabase(dbFile)
	require.NoError(t, err)

	assert.Equal(t, setDbData(db, sampleData), db.Keys())

	db.Serialize()

	db2, err := NewFileDatabase(dbFile)
	require.NoError(t, err)
	assert.Equal(t, db.Keys(), db2.Keys())
}

func TestFileDatabaseGet(t *testing.T) {
	db, err := NewFileDatabase(sb.TempFile())
	require.NoError(t, err)
	assert.Equal(t, db.Get("missing_key"), nil,
		"Expected missing key 'missing_key' to return nil value")
}

func TestFileDatabaseDelete(t *testing.T) {
	dbFile := sb.TempFile()

	db, err := NewFileDatabase(dbFile)
	require.NoError(t, err)
	db2, err := NewFileDatabase(dbFile)
	require.NoError(t, err)

	db.Set("foo", "bar")
	db.Serialize()

	db2.Deserialize()
	assert.Equal(t, db2.Get("foo"), "bar",
		"Expected missing key 'foo' to return 'bar' value")

	assert.False(t, db.Delete("nonexistent"), "Expected Delete to return false on non-present values but got true")
	assert.True(t, db.Delete("foo"),
		"Expected Delete to return true on present values but got false")

	db.Serialize()
	db2.Deserialize()

	assert.Equal(t, db2.Get("foo"), nil,
		"Expected 'foo' to be deleted but equals %s", db2.Get("foo"))
}

func TestFileDatabaseDeserialize(t *testing.T) {
	db, err := NewFileDatabase(sb.TempFile())
	require.NoError(t, err)

	db.Set("foo", "bar")

	assert.Equal(t, db.Get("foo"), "bar")

	db.Serialize()
	db.Set("foo", "bar2")

	assert.Equal(t, db.Get("foo"), "bar2")

	db.Deserialize()
	assert.Equal(t, db.Get("foo"), "bar")
}

func TestFileDatabaseDeserializeFromEmptyFile(t *testing.T) {
	dbFile, err := sb.WriteFile("dbfile", []byte{}, os.FileMode(0777))
	require.NoError(t, err)

	db, err := NewFileDatabase(dbFile)
	require.NoError(t, err)

	assert.NoError(t, db.Deserialize())

	assert.Len(t, db.Keys(), 0, "Storage should be empty")

	db.Set("foo", "bar")
	assert.Equal(t, db.Get("foo"), "bar")

	db.Serialize()
	db.Deserialize()

	assert.Equal(t, db.Get("foo"), "bar")
}

func TestFileDatabaseDeserializeFromMalformedFile(t *testing.T) {
	dbFile, err := sb.WriteFile("dbFile", []byte("asdf"), os.FileMode(0777))
	require.NoError(t, err)

	db, err := NewFileDatabase(dbFile)

	tu.AssertErrorMatch(t, db.Deserialize(),
		regexp.MustCompile("Error reading possible malformed database file"))

	assert.Len(t, db.Keys(), 0)

	db.Set("foo", "bar")
	assert.Equal(t, db.Get("foo"), "bar")
	db.Serialize()
	db.Deserialize()
	assert.Equal(t, db.Get("foo"), "bar")
}

func TestFileDatabaseOrderedKeys(t *testing.T) {
	db, err := NewFileDatabase(sb.TempFile())
	require.NoError(t, err)

	strs := []string{"X", "B", "C", "A", "Y"}
	for i, k := range strs {
		db.Set(k, i)
	}
	sort.Strings(strs)
	assert.Equal(t, strs, db.Keys())
}

func TestFileDatabaseErrors(t *testing.T) {
	f := sb.TempFile()
	db, err := NewFileDatabase(f)
	require.NoError(t, err)

	tu.AssertErrorMatch(t, db.Deserialize(),
		regexp.MustCompile("no such file or directory"))

	sb.WriteFile(f, []byte{}, os.FileMode(0000))

	tu.AssertErrorMatch(t, db.Serialize(),
		regexp.MustCompile("permission denied"))
}
