package imagebuffer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func testArena(t *testing.T, size int64) *Arena {
	t.Helper()
	a, err := New(size)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	return a
}

func TestArenaBoundedReadRetryAndCleanup(t *testing.T) {
	a := testArena(t, 3<<20)
	name := a.file.Name()
	_, err := os.Stat(name)
	require.True(t, os.IsNotExist(err), "buffer must be unlinked while in use")
	for i := 0; i < 6; i++ {
		_, err = a.Read(bytes.NewReader(make([]byte, (1<<20)+1)), 1<<20)
		require.ErrorIs(t, err, ErrTooLarge)
	}
	raw := bytes.Repeat([]byte("hello"), 100000)
	got, err := a.Read(bytes.NewReader(raw), 1<<20)
	require.NoError(t, err)
	require.Equal(t, raw, got)
	body, err := OpenAIResponse(a, map[string]any{"created": 1, "model": "with\"escape", "usage": map[string]int{"output_tokens": 2345}}, [][]byte{got})
	require.NoError(t, err)
	var decoded struct {
		Model string `json:"model"`
		Data  []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
		Usage map[string]int `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, "with\"escape", decoded.Model)
	require.Equal(t, 2345, decoded.Usage["output_tokens"])
	img, err := base64.StdEncoding.DecodeString(decoded.Data[0].B64)
	require.NoError(t, err)
	require.Equal(t, raw, img)
	require.NoError(t, a.Close())
	require.NoError(t, a.Close())
	_, err = a.Read(bytes.NewReader(raw), 1<<20)
	require.ErrorIs(t, err, ErrCapacity)
}

func TestArenaConcurrentSlotsAndFailedWriterRelease(t *testing.T) {
	a := testArena(t, 16<<20)
	injected := errors.New("write failed")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := a.Build(1<<20, func(w io.Writer) error {
				_, e := w.Write([]byte{byte(i)})
				if e != nil {
					return e
				}
				return injected
			})
			require.ErrorIs(t, err, injected)
			data, err := a.Read(bytes.NewReader(bytes.Repeat([]byte{byte(i)}, 1<<20)), 1<<20)
			require.NoError(t, err)
			require.Equal(t, byte(i), data[len(data)-1])
		}(i)
	}
	wg.Wait()
	require.Len(t, a.mappings, 8)
}

func TestArenaDoesNotFallBackToUnboundedAllocation(t *testing.T) {
	a := testArena(t, int64(os.Getpagesize()))
	_, err := a.Read(bytes.NewReader([]byte("x")), 1<<20)
	require.ErrorIs(t, err, ErrCapacity)
	_, err = a.Build(100, func(io.Writer) error { return io.ErrUnexpectedEOF })
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	got, err := a.Read(bytes.NewReader([]byte("still available")), 100)
	require.NoError(t, err)
	require.Equal(t, "still available", string(got))
}
