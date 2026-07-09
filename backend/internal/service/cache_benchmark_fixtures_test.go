//go:build unit

package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type cacheBenchmarkFixtureFile struct {
	SchemaVersion int                           `json:"schema_version"`
	Samples       []cacheBenchmarkFixtureSample `json:"samples"`
}

type cacheBenchmarkFixtureSample struct {
	ID        string         `json:"id"`
	Category  string         `json:"category"`
	Cacheable bool           `json:"cacheable"`
	Body      map[string]any `json:"body"`
}

func TestCacheBenchmarkFixtures(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	path := filepath.Join(repoRoot, "tools", "cache_benchmark", "fixtures.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache benchmark fixtures: %v", err)
	}

	var fixtures cacheBenchmarkFixtureFile
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("parse cache benchmark fixtures: %v", err)
	}
	if fixtures.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", fixtures.SchemaVersion)
	}
	if got := len(fixtures.Samples); got != 10 {
		t.Fatalf("sample count = %d, want 10", got)
	}

	seenIDs := map[string]bool{}
	categoryCounts := map[string]int{}
	streamSamples := 0
	cacheableSamples := 0
	for _, sample := range fixtures.Samples {
		if sample.ID == "" {
			t.Fatal("sample id must not be empty")
		}
		if seenIDs[sample.ID] {
			t.Fatalf("duplicate sample id %q", sample.ID)
		}
		seenIDs[sample.ID] = true
		categoryCounts[sample.Category]++
		if sample.Cacheable {
			cacheableSamples++
		}

		if sample.Body["model"] == "" {
			t.Fatalf("sample %s missing model", sample.ID)
		}
		messages, ok := sample.Body["messages"].([]any)
		if !ok || len(messages) == 0 {
			t.Fatalf("sample %s missing messages", sample.ID)
		}
		if _, hasTemperature := sample.Body["temperature"]; !hasTemperature {
			t.Fatalf("sample %s missing deterministic temperature", sample.ID)
		}
		if stream, _ := sample.Body["stream"].(bool); stream {
			streamSamples++
			if sample.Cacheable {
				t.Fatalf("streaming control sample %s should not be cacheable by default", sample.ID)
			}
		}
	}

	requiredCategories := []string{
		"short_system",
		"short_tools",
		"complex_system",
		"complex_tools",
		"stream_control",
	}
	for _, category := range requiredCategories {
		if categoryCounts[category] == 0 {
			t.Fatalf("missing fixture category %s", category)
		}
	}
	if streamSamples != 1 {
		t.Fatalf("streaming sample count = %d, want 1", streamSamples)
	}
	if cacheableSamples != 9 {
		t.Fatalf("cacheable sample count = %d, want 9", cacheableSamples)
	}
}
