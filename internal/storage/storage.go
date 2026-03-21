package storage

import (
	"fmt"
	"time"
)

// Status represents the deployment status.
type Status string

const (
	StatusPending    Status = "pending"
	StatusRunning    Status = "running"
	StatusSuccess    Status = "success"
	StatusFailed     Status = "failed"
	StatusRolledBack Status = "rolled_back"
)

// Deployment represents a single deployment record.
type Deployment struct {
	ID            string
	Service       string
	Status        Status
	Mode          string
	Image         string
	PreviousImage string
	RollbackTag   string
	StartedAt     time.Time
	CompletedAt   *time.Time
	ErrorMessage  string
}

// Storage is the interface for persisting deployment records.
type Storage interface {
	// Save inserts or replaces a deployment record.
	Save(d *Deployment) error
	// Update reads the deployment with the given id, applies fn to it, and
	// persists the result. Returns an error if the id does not exist.
	Update(id string, fn func(*Deployment)) error
	// Get returns the deployment with the given id.
	Get(id string) (*Deployment, error)
	// List returns all deployments ordered by started_at descending.
	List() ([]*Deployment, error)
	// GetLatestByService returns the most recent deployment for the given
	// service name, or ErrNotFound if no deployments exist for that service.
	GetLatestByService(service string) (*Deployment, error)
	// Close releases any resources held by the storage backend.
	Close() error
}

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = fmt.Errorf("not found")
