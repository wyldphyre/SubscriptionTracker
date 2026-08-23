package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/craigr/subscriptiontracker/internal/model"
)

func newTestStore(t *testing.T) *JSONStore {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return s
}

// TestReadersDoNotShareTagBacking guards the data race that let an in-place tag
// rename mutate slices already handed to a reader.
func TestReadersDoNotShareTagBacking(t *testing.T) {
	s := newTestStore(t)
	if err := s.Create(&model.Subscription{Name: "x", Tags: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}

	snapshot := s.GetAll()
	if err := s.RenameTag("alpha", "beta"); err != nil {
		t.Fatal(err)
	}
	if got := snapshot[0].Tags[0]; got != "alpha" {
		t.Errorf("snapshot tag = %q, want it unaffected by the later rename", got)
	}
}

// TestConcurrentReadAndRenameNoRace fails under -race if readers and writers
// share slice backing arrays.
func TestConcurrentReadAndRenameNoRace(t *testing.T) {
	s := newTestStore(t)
	if err := s.Create(&model.Subscription{Name: "x", Tags: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			for _, sub := range s.GetAll() {
				for _, tag := range sub.Tags {
					_ = tag
				}
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			_ = s.RenameTag("alpha", "beta")
			_ = s.RenameTag("beta", "alpha")
		}
	}()
	wg.Wait()
}

// TestCallerCannotMutateStoredRecord checks the write path copies too.
func TestCallerCannotMutateStoredRecord(t *testing.T) {
	s := newTestStore(t)
	tags := []string{"alpha"}
	sub := &model.Subscription{Name: "x", Tags: tags}
	if err := s.Create(sub); err != nil {
		t.Fatal(err)
	}

	tags[0] = "hijacked"

	stored, ok := s.GetByID(sub.ID)
	if !ok {
		t.Fatal("GetByID: subscription not found")
	}
	if stored.Tags[0] != "alpha" {
		t.Errorf("stored tag = %q, want %q", stored.Tags[0], "alpha")
	}
}

// TestFlushWritesCompleteJSON checks the persisted file is readable after a
// mutation, i.e. the sync/rename sequence leaves a valid file behind.
func TestFlushWritesCompleteJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(&model.Subscription{Name: "Netflix", Tags: []string{"tv"}}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st model.Store
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("persisted file is not valid JSON: %v", err)
	}
	if len(st.Subscriptions) != 1 || st.Subscriptions[0].Name != "Netflix" {
		t.Errorf("persisted subscriptions = %+v, want one named Netflix", st.Subscriptions)
	}

	// No temp files should be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
