package runtimediag

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryCaptureProducesPrivateBoundedAllocationProfile(t *testing.T) {
	dir := t.TempDir()
	for range 2 {
		if err := CaptureMemory(dir); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected output: %v %v", entries, err)
	}
	for _, name := range []string{"heap.pprof", "memory.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private output: %v %v", info, err)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "heap.pprof"))
	p, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("invalid profile: %v", err)
	}
	decoded, err := io.ReadAll(p)
	_ = p.Close()
	if err != nil || len(decoded) == 0 {
		t.Fatalf("empty profile: %v", err)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, "memory.json"))
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m["Before"] == nil || m["After"] == nil {
		t.Fatalf("invalid stats: %s %v", raw, err)
	}
}
