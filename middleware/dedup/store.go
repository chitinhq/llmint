// Package dedup provides a deduplication middleware that caches LLM responses
// and returns cached results for identical requests, avoiding redundant API calls.
package dedup

import (
	"context"
	"sync"
	"time"
)

// CacheStore is the storage backend for the dedup middleware.
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, data []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string)
}

// entry is a single item stored in MemoryStore.
type entry struct {
	data      []byte
	expiresAt time.Time // zero value means no expiry
}

// MemoryStore is a thread-safe in-memory CacheStore with TTL-based expiry.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]entry
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]entry)}
}

// Get retrieves a value by key. Returns (nil, false) on miss or expiry.
// Expired entries are lazily deleted on read.
func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[key]
	if !ok {
		return nil, false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(s.items, key)
		return nil, false
	}
	return e.data, true
}

// Set stores a value with an optional TTL. A zero TTL means no expiry.
func (s *MemoryStore) Set(_ context.Context, key string, data []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = entry{data: data, expiresAt: expiresAt}
	return nil
}

// Delete removes a key from the store. No-op if the key doesn't exist.
func (s *MemoryStore) Delete(_ context.Context, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}
