package storage

import (
	"fmt"
	"sort"
	"sync"
)

// MemoryStorage is an in-memory Storage implementation. Deployment history is
// lost when the process exits. Close is a no-op.
type MemoryStorage struct {
	mu          sync.RWMutex
	deployments map[string]*Deployment
}

// NewMemoryStorage returns an initialised MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		deployments: make(map[string]*Deployment),
	}
}

// Save inserts or replaces the deployment in memory.
func (m *MemoryStorage) Save(d *Deployment) error {
	// Copy so callers cannot mutate the stored record through the pointer.
	cp := *d
	m.mu.Lock()
	m.deployments[d.ID] = &cp
	m.mu.Unlock()
	return nil
}

// Update reads the deployment with the given id, applies fn, and stores the
// result. Returns an error when the id does not exist.
func (m *MemoryStorage) Update(id string, fn func(*Deployment)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %q not found", id)
	}
	fn(d)
	return nil
}

// Get returns the deployment with the given id.
func (m *MemoryStorage) Get(id string) (*Deployment, error) {
	m.mu.RLock()
	d, ok := m.deployments[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("deployment %q not found", id)
	}
	// Return a copy so callers cannot mutate internal state.
	cp := *d
	return &cp, nil
}

// List returns all deployments ordered by StartedAt descending.
func (m *MemoryStorage) List() ([]*Deployment, error) {
	m.mu.RLock()
	result := make([]*Deployment, 0, len(m.deployments))
	for _, d := range m.deployments {
		cp := *d
		result = append(result, &cp)
	}
	m.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result, nil
}

// Close is a no-op for the in-memory backend.
func (m *MemoryStorage) Close() error { return nil }

// entryCount returns the number of stored records. Used in tests only.
func (m *MemoryStorage) entryCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.deployments)
}
