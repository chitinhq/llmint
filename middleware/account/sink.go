// Package account provides a token accounting middleware that records usage,
// cost, and savings data for every LLM completion to a configurable Sink.
package account

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/AgentGuardHQ/llmint"
)

// Entry is a single accounting record for one LLM completion.
type Entry struct {
	Timestamp   time.Time
	Model       string
	RequestHash string
	InputTokens int
	Usage       llmint.Usage
	Savings     []llmint.Savings
	Duration    time.Duration
	Metadata    map[string]string
}

// Sink is the target that receives accounting entries.
type Sink interface {
	Record(Entry) error
}

// ---- SliceSink -------------------------------------------------------

// SliceSink accumulates entries in memory. Useful for testing.
type SliceSink struct {
	mu      sync.Mutex
	Entries []Entry
}

// Record appends the entry to the in-memory slice.
func (s *SliceSink) Record(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = append(s.Entries, e)
	return nil
}

// ---- WriterSink ------------------------------------------------------

// WriterSink writes JSON-encoded entries to an io.Writer, one per line.
type WriterSink struct {
	mu  sync.Mutex
	w   io.Writer
	enc *json.Encoder
}

// NewWriterSink returns a WriterSink that writes to w.
func NewWriterSink(w io.Writer) *WriterSink {
	return &WriterSink{w: w, enc: json.NewEncoder(w)}
}

// NewStdoutSink returns a WriterSink that writes to os.Stdout.
func NewStdoutSink() *WriterSink {
	return NewWriterSink(os.Stdout)
}

// Record encodes the entry as JSON to the writer.
func (s *WriterSink) Record(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(e)
}

// ---- FileSink --------------------------------------------------------

// FileSink appends JSONL records to a file.
type FileSink struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// NewFileSink opens (or creates) the file at path for appending.
// The caller must call Close() when done.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileSink{file: f, enc: json.NewEncoder(f)}, nil
}

// Record appends the entry as a JSON line to the file.
func (s *FileSink) Record(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(e)
}

// Close flushes and closes the underlying file.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}
