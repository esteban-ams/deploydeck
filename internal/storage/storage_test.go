package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/esteban-ams/deploydeck/internal/storage"
)

// testStorage runs the shared contract tests against any Storage implementation.
func testStorage(t *testing.T, s storage.Storage) {
	t.Helper()

	t.Run("Save then Get returns equal record", func(t *testing.T) {
		t.Parallel()
		d := &storage.Deployment{
			ID:            "dep_save_get",
			Service:       "svc-a",
			Status:        storage.StatusPending,
			Mode:          "pull",
			Image:         "myimage:latest",
			PreviousImage: "",
			RollbackTag:   "",
			StartedAt:     time.Unix(0, 1711000000000000000),
			CompletedAt:   nil,
			ErrorMessage:  "",
		}
		if err := s.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Get(d.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		assertDeploymentEqual(t, d, got)
	})

	t.Run("Update modifies only targeted fields", func(t *testing.T) {
		t.Parallel()
		d := &storage.Deployment{
			ID:        "dep_update",
			Service:   "svc-b",
			Status:    storage.StatusRunning,
			Mode:      "build",
			Image:     "img:1",
			StartedAt: time.Unix(0, 1711000001000000000),
		}
		if err := s.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}

		now := time.Unix(0, 1711000002000000000)
		if err := s.Update(d.ID, func(u *storage.Deployment) {
			u.Status = storage.StatusSuccess
			u.CompletedAt = &now
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, err := s.Get(d.ID)
		if err != nil {
			t.Fatalf("Get after Update: %v", err)
		}
		if got.Status != storage.StatusSuccess {
			t.Errorf("Status: want %q, got %q", storage.StatusSuccess, got.Status)
		}
		if got.Image != "img:1" {
			t.Errorf("Image should not change: want %q, got %q", "img:1", got.Image)
		}
		if got.CompletedAt == nil {
			t.Fatal("CompletedAt should not be nil after Update")
		}
		if !got.CompletedAt.Equal(now) {
			t.Errorf("CompletedAt: want %v, got %v", now, got.CompletedAt)
		}
	})

	t.Run("Update on missing ID returns error", func(t *testing.T) {
		t.Parallel()
		err := s.Update("nonexistent_id_xyz", func(d *storage.Deployment) {})
		if err == nil {
			t.Fatal("expected error for missing id, got nil")
		}
	})

	t.Run("List returns all records", func(t *testing.T) {
		t.Parallel()
		// Use a fresh storage for this sub-test to avoid interference.
		// We test basic presence — order is covered by the SQLite ORDER BY and
		// by the memory implementation's sort.
		ids := []string{"dep_list_1", "dep_list_2", "dep_list_3"}
		for i, id := range ids {
			d := &storage.Deployment{
				ID:        id,
				Service:   "svc-list",
				Status:    storage.StatusPending,
				StartedAt: time.Unix(0, int64(1711000010000000000+i*1000000000)),
			}
			if err := s.Save(d); err != nil {
				t.Fatalf("Save %q: %v", id, err)
			}
		}

		list, err := s.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		found := make(map[string]bool)
		for _, d := range list {
			found[d.ID] = true
		}
		for _, id := range ids {
			if !found[id] {
				t.Errorf("List missing expected id %q", id)
			}
		}
	})

	t.Run("nil CompletedAt round-trips correctly", func(t *testing.T) {
		t.Parallel()
		d := &storage.Deployment{
			ID:          "dep_nil_completed",
			Service:     "svc-c",
			Status:      storage.StatusRunning,
			StartedAt:   time.Unix(0, 1711000020000000000),
			CompletedAt: nil,
		}
		if err := s.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Get(d.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.CompletedAt != nil {
			t.Errorf("CompletedAt: want nil, got %v", got.CompletedAt)
		}
	})

	t.Run("non-nil CompletedAt round-trips correctly", func(t *testing.T) {
		t.Parallel()
		ts := time.Unix(0, 1711000030000000000)
		d := &storage.Deployment{
			ID:          "dep_nonnull_completed",
			Service:     "svc-d",
			Status:      storage.StatusSuccess,
			StartedAt:   time.Unix(0, 1711000025000000000),
			CompletedAt: &ts,
		}
		if err := s.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, err := s.Get(d.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.CompletedAt == nil {
			t.Fatal("CompletedAt: want non-nil, got nil")
		}
		if !got.CompletedAt.Equal(ts) {
			t.Errorf("CompletedAt: want %v, got %v", ts, got.CompletedAt)
		}
	})
}

// assertDeploymentEqual fails the test if two Deployment values differ.
func assertDeploymentEqual(t *testing.T, want, got *storage.Deployment) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("ID: want %q, got %q", want.ID, got.ID)
	}
	if got.Service != want.Service {
		t.Errorf("Service: want %q, got %q", want.Service, got.Service)
	}
	if got.Status != want.Status {
		t.Errorf("Status: want %q, got %q", want.Status, got.Status)
	}
	if got.Mode != want.Mode {
		t.Errorf("Mode: want %q, got %q", want.Mode, got.Mode)
	}
	if got.Image != want.Image {
		t.Errorf("Image: want %q, got %q", want.Image, got.Image)
	}
	if got.PreviousImage != want.PreviousImage {
		t.Errorf("PreviousImage: want %q, got %q", want.PreviousImage, got.PreviousImage)
	}
	if got.RollbackTag != want.RollbackTag {
		t.Errorf("RollbackTag: want %q, got %q", want.RollbackTag, got.RollbackTag)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Errorf("StartedAt: want %v, got %v", want.StartedAt, got.StartedAt)
	}
	if (want.CompletedAt == nil) != (got.CompletedAt == nil) {
		t.Errorf("CompletedAt nil mismatch: want %v, got %v", want.CompletedAt, got.CompletedAt)
	} else if want.CompletedAt != nil && !got.CompletedAt.Equal(*want.CompletedAt) {
		t.Errorf("CompletedAt: want %v, got %v", want.CompletedAt, got.CompletedAt)
	}
	if got.ErrorMessage != want.ErrorMessage {
		t.Errorf("ErrorMessage: want %q, got %q", want.ErrorMessage, got.ErrorMessage)
	}
}

func TestMemoryStorage(t *testing.T) {
	t.Parallel()
	testStorage(t, storage.NewMemoryStorage())
}

func TestSQLiteStorage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := storage.NewSQLiteStorage(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		os.Remove(filepath.Join(dir, "test.db"))
	})
	testStorage(t, s)
}
