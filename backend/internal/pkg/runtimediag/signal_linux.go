//go:build linux

package runtimediag

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// StartMemoryDiagnostics handles SIGUSR1 only; normal shutdown signals retain
// their existing behavior. The private temporary directory is logged only when
// an operator requests a capture. No periodic profiling runs in the background.
func StartMemoryDiagnostics() func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, syscall.SIGUSR1)
	go func() {
		var dir string
		var last time.Time
		for {
			select {
			case <-done:
				return
			case <-signals:
				if time.Since(last) < 30*time.Second {
					continue
				}
				last = time.Now()
				var err error
				if dir == "" {
					dir, err = os.MkdirTemp("", "sub2api-memory-*")
				}
				if err == nil {
					err = CaptureMemory(dir)
				}
				if err != nil {
					log.Printf("[MemoryDiagnostics] capture failed: %v", err)
				} else {
					log.Printf("[MemoryDiagnostics] allocation profile saved: %s", dir)
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { signal.Stop(signals); close(done) }) }
}
