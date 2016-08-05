package monitor

import (
	"encoding/gob"
	"time"

	"github.com/bitnami/gonit/database"
)

// ChecksDatabaseEntry defines a check entry in the monitor database
type ChecksDatabaseEntry struct {
	ID              string
	Monitored       bool
	DataCollectedAt time.Time
	Uptime          time.Duration
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
