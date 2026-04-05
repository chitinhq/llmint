package main

// #include <stdlib.h>
import "C"

import (
	"context"
	"encoding/json"
	"sync"
	"unsafe"

	"github.com/chitinhq/llmint"
	"github.com/chitinhq/llmint/middleware/account"
	"github.com/chitinhq/llmint/middleware/cascade"
	"github.com/chitinhq/llmint/middleware/dedup"
	"github.com/chitinhq/llmint/provider/mock"
)

// ---- Local stats sink --------------------------------------------------

type statsSink struct {
	mu              sync.Mutex
	totalCalls      int
	totalTokensSaved int
	totalCostSaved  float64
}

func (s *statsSink) Record(e account.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalCalls++
	for _, sv := range e.Savings {
		s.totalTokensSaved += sv.TokensSaved
		s.totalCostSaved += sv.CostSaved
	}
	return nil
}

func (s *statsSink) Stats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"total_calls":        s.totalCalls,
		"total_tokens_saved": s.totalTokensSaved,
		"total_cost_saved":   s.totalCostSaved,
	}
}

// ---- Pipeline registry -------------------------------------------------

var (
	registryMu sync.Mutex
	nextID     int64 = 1
	pipelines        = make(map[int64]*pipelineEntry)
)

type pipelineEntry struct {
	provider llmint.Provider
	sink     *statsSink
}

// ---- Config structs ----------------------------------------------------

type middlewareConfig struct {
	Type   string                 `json:"type"`
	Params map[string]interface{} `json:"params"`
}

type pipelineConfig struct {
	Provider   string             `json:"provider"`
	APIKey     string             `json:"api_key"`
	Middleware []middlewareConfig `json:"middleware"`
}

// ---- Helpers -----------------------------------------------------------

func errJSON(msg string) *C.char {
	out, _ := json.Marshal(map[string]interface{}{"error": msg})
	return C.CString(string(out))
}

func buildMiddleware(cfgs []middlewareConfig, sink *statsSink) []llmint.Middleware {
	var mws []llmint.Middleware
	for _, cfg := range cfgs {
		switch cfg.Type {
		case "dedup":
			store := dedup.NewMemoryStore()
			mws = append(mws, dedup.New(store))
		case "cascade":
			// params.models: []interface{} of model name strings
			var models []cascade.Model
			if raw, ok := cfg.Params["models"]; ok {
				if arr, ok2 := raw.([]interface{}); ok2 {
					for i, v := range arr {
						if name, ok3 := v.(string); ok3 {
							p := mock.New(name, "cascade response from "+name)
							threshold := 0.5
							if i == len(arr)-1 {
								threshold = 0 // last tier always accepts
							}
							models = append(models, cascade.Model{
								Provider:  p,
								Name:      name,
								Threshold: threshold,
							})
						}
					}
				}
			}
			if len(models) > 0 {
				mws = append(mws, cascade.New(models))
			}
		case "account":
			mws = append(mws, account.New(sink))
		}
	}
	return mws
}

// ---- Exported C functions ----------------------------------------------

//export LLMintCreatePipeline
func LLMintCreatePipeline(configJSON *C.char) C.longlong {
	raw := C.GoString(configJSON)
	var cfg pipelineConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return -1
	}

	// Build base provider.
	var base llmint.Provider
	switch cfg.Provider {
	case "anthropic":
		// Use mock; real Anthropic adapter requires live HTTP transport.
		base = mock.New("claude-3-haiku-20240307", "mock anthropic response")
	default:
		base = mock.New("mock-model", "mock response")
	}

	sink := &statsSink{}
	mws := buildMiddleware(cfg.Middleware, sink)

	provider := llmint.Chain(mws...)(base)

	registryMu.Lock()
	id := nextID
	nextID++
	pipelines[id] = &pipelineEntry{provider: provider, sink: sink}
	registryMu.Unlock()

	return C.longlong(id)
}

//export LLMintComplete
func LLMintComplete(pipelineID C.longlong, requestJSON *C.char) *C.char {
	id := int64(pipelineID)
	registryMu.Lock()
	entry, ok := pipelines[id]
	registryMu.Unlock()
	if !ok {
		return errJSON("pipeline not found")
	}

	raw := C.GoString(requestJSON)
	var req llmint.Request
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return errJSON("invalid request JSON: " + err.Error())
	}

	resp, err := entry.provider.Complete(context.Background(), &req)
	if err != nil {
		return errJSON(err.Error())
	}

	type responseOut struct {
		Content     []llmint.ContentBlock `json:"content"`
		Usage       llmint.Usage          `json:"usage"`
		Model       string                `json:"model"`
		CacheStatus string                `json:"cache_status"`
		Savings     []llmint.Savings      `json:"savings"`
	}
	out := responseOut{
		Content:     resp.Content,
		Usage:       resp.Usage,
		Model:       resp.Model,
		CacheStatus: resp.CacheStatus.String(),
		Savings:     resp.Savings,
	}
	data, err := json.Marshal(out)
	if err != nil {
		return errJSON("marshal error: " + err.Error())
	}
	return C.CString(string(data))
}

//export LLMintGetStats
func LLMintGetStats(pipelineID C.longlong) *C.char {
	id := int64(pipelineID)
	registryMu.Lock()
	entry, ok := pipelines[id]
	registryMu.Unlock()
	if !ok {
		return errJSON("pipeline not found")
	}

	data, _ := json.Marshal(entry.sink.Stats())
	return C.CString(string(data))
}

//export LLMintFreePipeline
func LLMintFreePipeline(pipelineID C.longlong) {
	id := int64(pipelineID)
	registryMu.Lock()
	delete(pipelines, id)
	registryMu.Unlock()
}

//export LLMintFreeString
func LLMintFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func main() {}
