package account

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chitinhq/llmint"
)

func sampleEntry() Entry {
	return Entry{
		Timestamp:   time.Now(),
		Model:       "test-model",
		RequestHash: "abc123",
		InputTokens: 50,
		Usage: llmint.Usage{
			InputTokens:  50,
			OutputTokens: 20,
		},
		Savings:  []llmint.Savings{{TokensSaved: 10, Technique: "dedup"}},
		Duration: 100 * time.Millisecond,
		Metadata: map[string]string{"env": "test"},
	}
}

func TestSliceSinkRecord(t *testing.T) {
	s := &SliceSink{}
	e := sampleEntry()

	if err := s.Record(e); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(s.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s.Entries))
	}
	if s.Entries[0].Model != "test-model" {
		t.Errorf("unexpected Model: %q", s.Entries[0].Model)
	}
}

func TestStdoutSink(t *testing.T) {
	var buf bytes.Buffer
	s := NewWriterSink(&buf)
	e := sampleEntry()

	if err := s.Record(e); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output from WriterSink")
	}
	// Must be valid JSON.
	var decoded Entry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if decoded.Model != "test-model" {
		t.Errorf("decoded Model=%q, want test-model", decoded.Model)
	}
}

func TestFileSink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entries.jsonl")

	s, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	if err := s.Record(sampleEntry()); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := s.Record(sampleEntry()); err != nil {
		t.Fatalf("Record 2: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}
	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}
