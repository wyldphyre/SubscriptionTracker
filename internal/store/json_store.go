package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/craigr/subscriptiontracker/internal/model"
)

// JSONStore persists subscriptions to a JSON file with an in-memory cache.
// All mutations use an atomic rename to avoid corrupt writes on crash.
type JSONStore struct {
	path  string
	mu    sync.RWMutex
	cache *model.Store
}

func New(path string) (*JSONStore, error) {
	s := &JSONStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load reads the JSON file into the in-memory cache. If the file does not
// exist, it initialises an empty store and persists it.
func (s *JSONStore) load() error {
	if dir := filepath.Dir(s.path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating data directory: %w", err)
		}
	}
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.cache = &model.Store{Version: 1, Subscriptions: []model.Subscription{}, Tags: []string{}}
		return s.flush()
	}
	if err != nil {
		return fmt.Errorf("reading store: %w", err)
	}
	var st model.Store
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("parsing store: %w", err)
	}
	if st.Subscriptions == nil {
		st.Subscriptions = []model.Subscription{}
	}
	if st.Tags == nil {
		st.Tags = []string{}
	}
	s.cache = &st
	return nil
}

// flush writes the in-memory cache to disk atomically.
// Caller must hold s.mu (write lock).
func (s *JSONStore) flush() error {
	data, err := json.MarshalIndent(s.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling store: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "subtracker-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func (s *JSONStore) GetAll() []model.Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.Subscription, len(s.cache.Subscriptions))
	copy(result, s.cache.Subscriptions)
	return result
}

func (s *JSONStore) GetByID(id string) (*model.Subscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.cache.Subscriptions {
		if s.cache.Subscriptions[i].ID == id {
			sub := s.cache.Subscriptions[i]
			return &sub, true
		}
	}
	return nil, false
}

func (s *JSONStore) Create(sub *model.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub.ID = newUUID()
	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	if sub.Status == "" {
		sub.Status = model.StatusActive
	}

	s.cache.Subscriptions = append(s.cache.Subscriptions, *sub)
	s.upsertTagsLocked(sub.Tags)
	return s.flush()
}

func (s *JSONStore) Update(sub *model.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.cache.Subscriptions {
		if s.cache.Subscriptions[i].ID == sub.ID {
			sub.UpdatedAt = time.Now().UTC()
			sub.CreatedAt = s.cache.Subscriptions[i].CreatedAt
			s.cache.Subscriptions[i] = *sub
			s.upsertTagsLocked(sub.Tags)
			return s.flush()
		}
	}
	return fmt.Errorf("subscription %q not found", sub.ID)
}

func (s *JSONStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sub := range s.cache.Subscriptions {
		if sub.ID == id {
			s.cache.Subscriptions = append(s.cache.Subscriptions[:i], s.cache.Subscriptions[i+1:]...)
			return s.flush()
		}
	}
	return fmt.Errorf("subscription %q not found", id)
}

func (s *JSONStore) ListTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.cache.Tags))
	copy(result, s.cache.Tags)
	return result
}

// UpsertTags adds any new tags to the master tag list, maintaining sorted order.
func (s *JSONStore) UpsertTags(tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertTagsLocked(tags)
	return s.flush()
}

// upsertTagsLocked must be called with s.mu held (write).
func (s *JSONStore) upsertTagsLocked(tags []string) {
	existing := make(map[string]bool, len(s.cache.Tags))
	for _, t := range s.cache.Tags {
		existing[t] = true
	}
	for _, t := range tags {
		if t != "" && !existing[t] {
			s.cache.Tags = append(s.cache.Tags, t)
			existing[t] = true
		}
	}
	sort.Strings(s.cache.Tags)
}

// RenameTag renames a tag in the master list and in every subscription that uses it.
func (s *JSONStore) RenameTag(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, t := range s.cache.Tags {
		if t == oldName {
			s.cache.Tags[i] = newName
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tag %q not found", oldName)
	}
	sort.Strings(s.cache.Tags)

	for i := range s.cache.Subscriptions {
		for j, t := range s.cache.Subscriptions[i].Tags {
			if t == oldName {
				s.cache.Subscriptions[i].Tags[j] = newName
			}
		}
	}
	return s.flush()
}

// DeleteTag removes a tag from the master list and from every subscription that uses it.
func (s *JSONStore) DeleteTag(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	newMaster := s.cache.Tags[:0]
	for _, t := range s.cache.Tags {
		if t == name {
			found = true
		} else {
			newMaster = append(newMaster, t)
		}
	}
	if !found {
		return fmt.Errorf("tag %q not found", name)
	}
	s.cache.Tags = newMaster

	for i := range s.cache.Subscriptions {
		var kept []string
		for _, t := range s.cache.Subscriptions[i].Tags {
			if t != name {
				kept = append(kept, t)
			}
		}
		s.cache.Subscriptions[i].Tags = kept
	}
	return s.flush()
}

// ReplaceAll replaces all subscriptions and rebuilds the tag list from scratch.
// Used by the xlsx import with "replace all" option.
func (s *JSONStore) ReplaceAll(subs []model.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Subscriptions = subs
	// Rebuild tag list from all subscriptions
	tagSet := map[string]bool{}
	for _, sub := range subs {
		for _, t := range sub.Tags {
			if t != "" {
				tagSet[t] = true
			}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	s.cache.Tags = tags
	return s.flush()
}

// AppendAll adds a batch of subscriptions (used by xlsx import, additive mode).
func (s *JSONStore) AppendAll(subs []model.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Subscriptions = append(s.cache.Subscriptions, subs...)
	for _, sub := range subs {
		s.upsertTagsLocked(sub.Tags)
	}
	return s.flush()
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
