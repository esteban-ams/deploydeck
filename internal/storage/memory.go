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
	// Deep-copy so callers cannot mutate the stored record through the pointer.
	cp := copyDeployment(d)
	m.mu.Lock()
	m.deployments[d.ID] = cp
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
	// Return a deep copy so callers cannot mutate internal state.
	return copyDeployment(d), nil
}

// List returns all deployments ordered by StartedAt descending.
func (m *MemoryStorage) List() ([]*Deployment, error) {
	m.mu.RLock()
	result := make([]*Deployment, 0, len(m.deployments))
	for _, d := range m.deployments {
		result = append(result, copyDeployment(d))
	}
	m.mu.RUnlock()

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})
	return result, nil
}

// GetLatestByService returns the most recently started deployment for the given
// service, or ErrNotFound if no deployments exist for that service.
func (m *MemoryStorage) GetLatestByService(service string) (*Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *Deployment
	for _, d := range m.deployments {
		if d.Service != service {
			continue
		}
		if latest == nil || d.StartedAt.After(latest.StartedAt) {
			latest = d
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("service %q: %w", service, ErrNotFound)
	}
	return copyDeployment(latest), nil
}

// AppendLog appends a single log line to the deployment with the given id.
func (m *MemoryStorage) AppendLog(id string, line string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.deployments[id]
	if !ok {
		return fmt.Errorf("deployment %q not found", id)
	}
	d.Logs = append(d.Logs, line)
	return nil
}

// copyDeployment returns a deep copy of d, including its Logs slice.
func copyDeployment(d *Deployment) *Deployment {
	cp := *d
	if d.Logs != nil {
		cp.Logs = make([]string, len(d.Logs))
		copy(cp.Logs, d.Logs)
	}
	return &cp
}

// Close is a no-op for the in-memory backend.
func (m *MemoryStorage) Close() error { return nil }

// entryCount returns the number of stored records. Used in tests only.
func (m *MemoryStorage) entryCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.deployments)
}
