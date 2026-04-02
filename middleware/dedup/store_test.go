package dedup

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreSetGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("hello"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := s.Get(ctx, "k")
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if string(got) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(got))
	}
}

func TestMemoryStoreMiss(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_, ok := s.Get(ctx, "nonexistent")
	if ok {
		t.Fatal("expected miss, got hit")
	}
}

func TestMemoryStoreTTLExpiry(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Set(ctx, "k", []byte("data"), 1*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Confirm it's there immediately.
	if _, ok := s.Get(ctx, "k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(5 * time.Millisecond)
	_, ok := s.Get(ctx, "k")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_ = s.Set(ctx, "k", []byte("v"), 0)
	s.Delete(ctx, "k")
	_, ok := s.Get(ctx, "k")
	if ok {
		t.Fatal("expected miss after delete")
	}
}
