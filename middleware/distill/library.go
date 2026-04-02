// Package distill provides a middleware that replaces verbose system prompts
// with pre-registered distilled equivalents, saving tokens on every call.
package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Library maps original prompts to their distilled equivalents.
type Library interface {
	// Lookup returns the distilled version of original, or ("", false) if not found.
	Lookup(original string) (string, bool)
	// Register stores a distilled version for original.
	Register(original, distilled string) error
}

// MemoryLibrary is an in-memory Library backed by a sync.RWMutex-protected map.
type MemoryLibrary struct {
	mu      sync.RWMutex
	entries map[string]string
}

// NewMemoryLibrary returns an empty MemoryLibrary.
func NewMemoryLibrary() *MemoryLibrary {
	return &MemoryLibrary{entries: make(map[string]string)}
}

// Lookup returns the distilled version of original, or ("", false) if not found.
func (m *MemoryLibrary) Lookup(original string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.entries[original]
	return v, ok
}

// Register stores a distilled version for original.
func (m *MemoryLibrary) Register(original, distilled string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[original] = distilled
	return nil
}

// FileLibrary loads a JSON file mapping original prompts to their distilled
// equivalents. The file must be a JSON object: {"original": "distilled", ...}.
type FileLibrary struct {
	entries map[string]string
}

// NewFileLibrary loads a FileLibrary from path. The file must contain a valid
// JSON object where each key is an original prompt and each value is the
// distilled replacement.
func NewFileLibrary(path string) (*FileLibrary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("distill: open library file: %w", err)
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("distill: parse library file: %w", err)
	}
	return &FileLibrary{entries: entries}, nil
}

// Lookup returns the distilled version of original, or ("", false) if not found.
func (f *FileLibrary) Lookup(original string) (string, bool) {
	v, ok := f.entries[original]
	return v, ok
}

// Register is not supported for FileLibrary after load; it always returns an error.
func (f *FileLibrary) Register(original, distilled string) error {
	return fmt.Errorf("distill: FileLibrary is read-only after load")
}
