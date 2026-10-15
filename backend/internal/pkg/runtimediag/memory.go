// Package runtimediag provides operator-triggered runtime diagnostics without
// exposing a network listener or recording request data and credentials.
package runtimediag

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"
)

// CaptureMemory writes allocation stacks (not heap contents) and GC statistics.
// Files are replaced on each capture so repeated signals cannot grow disk use.
func CaptureMemory(dir string) error {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	runtime.GC()
	runtime.ReadMemStats(&after)
	if err := writePrivate(dir, "heap.pprof", func(f *os.File) error { return pprof.WriteHeapProfile(f) }); err != nil {
		return err
	}
	return writePrivate(dir, "memory.json", func(f *os.File) error {
		return json.NewEncoder(f).Encode(struct {
			At            time.Time
			Goroutines    int
			Before, After runtime.MemStats
		}{time.Now().UTC(), runtime.NumGoroutine(), before, after})
	})
}

func writePrivate(dir, name string, write func(*os.File) error) error {
	f, err := os.CreateTemp(dir, ".capture-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := write(f); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
