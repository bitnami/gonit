package monitor

import (
	"encoding/gob"
	"sync"
	"time"

	"github.com/bitnami/gonit/database"
)

// ChecksDatabaseEntry defines a check entry in the monitor database
type ChecksDatabaseEntry struct {
	mutex           sync.RWMutex
	ID              string
	Monitored       bool
	DataCollectedAt time.Time
	Uptime          time.Duration
}

func (e *ChecksDatabaseEntry) rLock() {
	e.mutex.RLock()
}
func (e *ChecksDatabaseEntry) rUnlock() {
	e.mutex.RUnlock()
}

func (e *ChecksDatabaseEntry) lock() {
	e.mutex.Lock()
}
func (e *ChecksDatabaseEntry) unlock() {
	e.mutex.Unlock()
}

func init() {
	gob.Register(&ChecksDatabaseEntry{})
}

// ChecksDatabase defines a storage for saving process check information
type ChecksDatabase struct {
	database.Storer
}

// GetEntry returns an entry from the database
func (cd *ChecksDatabase) GetEntry(id string) *ChecksDatabaseEntry {
	if e, ok := cd.Get(id).(*ChecksDatabaseEntry); ok {
		return e
	}
	return nil
}

// AddEntry adds a check entry to the database
func (cd *ChecksDatabase) AddEntry(id string) *ChecksDatabaseEntry {
	e := &ChecksDatabaseEntry{ID: id, Monitored: true}
	cd.Set(id, e)
	return e
}

// NewDatabase returns a new database storage for checks information
func NewDatabase(file string) (*ChecksDatabase, error) {
	db, err := database.NewFileDatabase(file)
	return &ChecksDatabase{Storer: db}, err
}
