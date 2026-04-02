package distill_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/AgentGuardHQ/llmint/middleware/distill"
)

func TestMemoryLibraryRegisterLookup(t *testing.T) {
	lib := distill.NewMemoryLibrary()

	original := "You are a very verbose and detailed helpful assistant."
	compact := "You are a helpful assistant."

	if err := lib.Register(original, compact); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok := lib.Lookup(original)
	if !ok {
		t.Fatal("Lookup: expected to find registered entry")
	}
	if got != compact {
		t.Errorf("Lookup returned %q, want %q", got, compact)
	}
}

func TestMemoryLibraryMiss(t *testing.T) {
	lib := distill.NewMemoryLibrary()

	_, ok := lib.Lookup("prompt that was never registered")
	if ok {
		t.Error("Lookup: expected miss for unregistered prompt, got hit")
	}
}

func TestFileLibraryLoadAndLookup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	data := map[string]string{
		"You are a very verbose assistant.": "You are an assistant.",
		"Please be extremely detailed.":     "Be detailed.",
	}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	lib, err := distill.NewFileLibrary(path)
	if err != nil {
		t.Fatalf("NewFileLibrary: %v", err)
	}

	got, ok := lib.Lookup("You are a very verbose assistant.")
	if !ok {
		t.Fatal("Lookup: expected hit for registered entry")
	}
	if got != "You are an assistant." {
		t.Errorf("Lookup returned %q, want %q", got, "You are an assistant.")
	}

	_, ok = lib.Lookup("unregistered prompt")
	if ok {
		t.Error("Lookup: expected miss for unregistered prompt")
	}
}
