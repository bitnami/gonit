// Package database provides a key/value storage.
// It allows binary serialization and deserialization to a reader/writer of to file
package database

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sort"
	"syscall"
	"time"

	"github.com/bitnami/gonit/utils"
)

// NewFileDatabase returns a file-backed database.
// If 'file' exists, the function will try to load its contents to populate the database.
func NewFileDatabase(file string) (Storer, error) {
	db := &FileDB{file: file}
	var err error
	if utils.FileExists(file) {
		err = db.Deserialize()
	} else {
		db.initialize()
		err = nil
	}
	return db, err
}

// Serializable defines the interface that all serializable objects should implement
type Serializable interface {
	// Serialize allows saving the database storage to a file on disk
	Serialize() error
	// DeserializeFromFile allows populating the database from a file on disk
	Deserialize() error
}

// Storer defines the interface that all database kinds should implement
type Storer interface {
	Serializable
	// Get allows retrieving the value associated with the provided key
	// If the key is not present, it returns nil
	Get(key string) interface{}
	// Set allows setting a value identified by the provided key
	Set(key string, value interface{})
	// Delete allows deleting a key.
	// It returns true if the key was present or false otherwise
	Delete(key string) bool
	//Exists returns true if the key exists and false otherwise
	Exists(key string) bool
	// Keys returns the list of key identifiers stored
	Keys() []string
}

// DB represents a database
type DB struct {
	// Storage stores the key storage mappings
	Storage map[string]interface{}
}

// Keys returns a slice containing all the keys stored
func (db *DB) Keys() []string {
	keys := []string{}
	for k := range db.Storage {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Set allows storing a generic value indexed by the provided key.
// If an existing record already exists for the same key, it will be overwritten
func (db *DB) Set(key string, value interface{}) {
	db.Storage[key] = value
}

// Get allows retrieving a previously stored value indexed for 'key'.
// If the key does not exists, it will return a nil value.
func (db *DB) Get(key string) (value interface{}) {
	if db.Exists(key) {
		value = db.Storage[key]
	} else {
		value = nil
	}
	return value
}

// Exists allows checking for the existence of a record in the database
func (db *DB) Exists(key string) bool {
	_, present := db.Storage[key]
	return present
}

// Delete removes a record from the database.
// It returns true if the key was present and false otherwise
func (db *DB) Delete(key string) (deleted bool) {
	if db.Exists(key) {
		delete(db.Storage, key)
		deleted = true
	} else {
		deleted = false
	}
	return deleted
}

func (db *DB) deserializeFromReader(reader io.Reader) error {
	decoder := gob.NewDecoder(reader)
	// We use an intermediate DB so we can avoid merging Storage contents
	// (we want to reset it). We should use a mutex.
	tmpDB := &DB{}
	err := decoder.Decode(tmpDB)
	if err == nil {
		db.Storage = tmpDB.Storage
		return nil
	} else if err == io.EOF {
		// If the file is empty, lets treat it as it was not there
		return nil
	} else {
		return fmt.Errorf("Error reading possible malformed database file: %s", err.Error())
	}
}

func (db *DB) serializeToWriter(writer io.Writer) error {
	enc := gob.NewEncoder(writer)
	return enc.Encode(*db)
}

func (db *DB) initialize() {
	if len(db.Storage) == 0 {
		db.Storage = make(map[string]interface{})
	}
}

// FileDB implements a database backed up by a file
type FileDB struct {
	DB
	file string
}

// Deserialize reads the file storage from disk and re-populates the in-memory database
func (db *FileDB) Deserialize() error {
	err := db.deserializeFromFile(db.file)
	db.initialize()
	return err
}

// Serialize saves the in-memory database records to the on-disk storage file
func (db *FileDB) Serialize() error {
	if db.file == "" {
		return fmt.Errorf("Cannot serialize to empty file")
	}
	return db.serializeToFile(db.file)
}

func (db *FileDB) deserializeFromFile(file string) (err error) {
	var fh *os.File
	if fh, err = utils.OpenFileSecure(file, syscall.O_RDONLY, 0600); err != nil {
		return err
	}

	defer fh.Close()
	return db.DB.deserializeFromReader(fh)
}

func (db *FileDB) serializeToFile(file string) (err error) {
	var fh *os.File
	if fh, err = utils.OpenFileSecure(
		file, syscall.O_TRUNC|syscall.O_RDWR|syscall.O_CREAT, 0600,
	); err != nil {
		return err
	}
	defer fh.Close()
	return db.DB.serializeToWriter(fh)
}

func init() {
	gob.Register(time.Now())
}
