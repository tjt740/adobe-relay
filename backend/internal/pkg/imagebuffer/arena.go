// Package imagebuffer stores large, immutable image results in reserved local
// disk space. Read-only file mappings let existing image/storage APIs consume
// bytes without retaining anonymous heap for every waiting response.
package imagebuffer

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"sort"
	"sync"
)

var ErrCapacity = errors.New("image buffer capacity unavailable")
var ErrTooLarge = errors.New("image buffer exceeds reserved size")

const diskHeadroom = int64(8 << 30)

type extent struct{ offset, size int64 }

// Arena owns all returned slices. Close only after every consumer (including
// the response writer and storage uploader) has finished. Slices are immutable.
type Arena struct {
	mu       sync.Mutex
	file     *os.File
	free     []extent
	mappings [][]byte
	closed   bool
}

type contextKey struct{}

func WithArena(ctx context.Context, a *Arena) context.Context {
	return context.WithValue(ctx, contextKey{}, a)
}
func FromContext(ctx context.Context) *Arena {
	a, _ := ctx.Value(contextKey{}).(*Arena)
	return a
}

var reserveMu sync.Mutex
var pendingReservations int64

// New reserves physical disk space before any paid generation. The file is
// unlinked immediately: success, cancellation and process crashes leave no
// customer image files behind. Memory-backed filesystems are not eligible.
func New(size int64) (*Arena, error) {
	if size <= 0 || size > math.MaxInt64-diskHeadroom {
		return nil, ErrCapacity
	}
	f, err := os.CreateTemp("", "sub2-image-*")
	if err != nil {
		return nil, err
	}
	if err = os.Remove(f.Name()); err != nil {
		f.Close()
		return nil, err
	}
	reserveMu.Lock()
	available, checkErr := availableDisk(f)
	if checkErr == nil && available >= uint64(size)+uint64(diskHeadroom)+uint64(pendingReservations) {
		pendingReservations += size
	} else {
		if checkErr != nil {
			err = checkErr
		} else {
			err = ErrCapacity
		}
	}
	reserveMu.Unlock()
	if err == nil {
		// The slow allocation happens outside the accounting lock. Concurrent
		// uploads must not queue behind another request's filesystem operation.
		err = reserveFile(f, size)
		reserveMu.Lock()
		pendingReservations -= size
		reserveMu.Unlock()
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Arena{file: f, free: []extent{{0, size}}}, nil
}

// Build bounds writes before they occur. Allocation never waits for another
// request; failed downloads return their extent for the existing GET retries.
func (a *Arena) Build(limit int64, write func(io.Writer) error) ([]byte, error) {
	page := int64(os.Getpagesize())
	size := (limit + page - 1) / page * page
	if limit <= 0 || size < limit {
		return nil, ErrTooLarge
	}
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil, ErrCapacity
	}
	var slot extent
	for i, e := range a.free {
		if e.size >= size {
			slot = extent{e.offset, size}
			a.free[i].offset += size
			a.free[i].size -= size
			break
		}
	}
	a.mu.Unlock()
	if slot.size == 0 {
		return nil, ErrCapacity
	}
	w := &boundedWriter{w: io.NewOffsetWriter(a.file, slot.offset), remaining: limit}
	buffer := bufio.NewWriterSize(w, 64<<10)
	err := write(buffer)
	if err == nil {
		err = buffer.Flush()
	}
	var data []byte
	if err == nil && w.written > 0 {
		data, err = mapFile(a.file, slot.offset, int(w.written))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil || len(data) == 0 {
		a.free = append(a.free, slot)
		sort.Slice(a.free, func(i, j int) bool { return a.free[i].offset < a.free[j].offset })
		merged := a.free[:0]
		for _, e := range a.free {
			if e.size == 0 {
				continue
			}
			if len(merged) > 0 && merged[len(merged)-1].offset+merged[len(merged)-1].size == e.offset {
				merged[len(merged)-1].size += e.size
			} else {
				merged = append(merged, e)
			}
		}
		a.free = merged
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	a.mappings = append(a.mappings, data)
	return data, nil
}

func (a *Arena) Read(r io.Reader, limit int64) ([]byte, error) {
	return a.Build(limit, func(w io.Writer) error {
		// Deliberately hide WriterTo/ReaderFrom to bound the per-read buffer.
		_, err := io.CopyBuffer(w, struct{ io.Reader }{io.LimitReader(r, limit+1)}, make([]byte, 64<<10))
		return err
	})
}

func (a *Arena) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	var err error
	for _, data := range a.mappings {
		err = errors.Join(err, unmapFile(data))
	}
	a.mappings = nil
	err = errors.Join(err, a.file.Close())
	return err
}

type boundedWriter struct {
	w                  io.Writer
	remaining, written int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, ErrTooLarge
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	w.written += int64(n)
	return n, err
}
