//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStorageFunc func(context.Context, string, string, []byte) (string, error)

func (f imageStorageFunc) Save(ctx context.Context, key, contentType string, data []byte) (string, error) {
	return f(ctx, key, contentType, data)
}

func TestAdobeImageExplicitBase64SkipsStorage(t *testing.T) {
	api := &adobeFakeTransport{}
	svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, pngBytes), func() (*ImageResultUploader, bool) {
		t.Fatal("explicit Base64 must not resolve or upload to storage")
		return nil, false
	})
	result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
		Model: "gpt-image-2.5-flare", Prompt: "x", Size: "1024x1024", N: 2, Quality: "max", ResponseFormat: "b64_json",
	})
	require.NoError(t, err)
	require.Equal(t, "b64_json", result.Timings.Delivery)
	require.Len(t, result.Timings.Images, 2)
	for _, stage := range result.Timings.Images {
		require.Equal(t, 1, stage.SubmitAttempts)
		require.Equal(t, 1, stage.PollCount)
		require.Equal(t, 1, stage.DownloadAttempts)
	}
	var body struct {
		Quality string
		Data    []struct {
			B64JSON string `json:"b64_json"`
		}
	}
	require.NoError(t, json.Unmarshal(result.Body, &body))
	require.Equal(t, "max", body.Quality)
	require.Len(t, body.Data, 2)
	for _, item := range body.Data {
		decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
		require.NoError(t, err)
		require.Equal(t, pngBytes, decoded)
	}
	// Generation parameters are preserved, including n=1 for each upstream job.
	for _, submitted := range adobeSubmitBodies(t, api) {
		require.Equal(t, float64(1), submitted["n"])
		require.Equal(t, float64(7), submitted["generationSettings"].(map[string]any)["detailLevel"])
	}
}

func TestImageResultUploaderDirectUploadsBoundConcurrencyAndPreserveOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := make(chan string, 4)
	release := make(chan struct{}, 4)
	var active, maxActive atomic.Int32
	storage := imageStorageFunc(func(ctx context.Context, key, contentType string, data []byte) (string, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for prev := maxActive.Load(); current > prev; prev = maxActive.Load() {
			if maxActive.CompareAndSwap(prev, current) {
				break
			}
		}
		if contentType != "image/png" || !bytes.Equal(data, pngBytes) {
			return "", errors.New("image bytes or MIME changed")
		}
		started <- key
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
		}
		return "https://storage.example/" + key, nil
	})
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	type outcome struct {
		urls []string
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		urls, err := uploader.UploadImages(ctx, "job", [][]byte{pngBytes, pngBytes, pngBytes, pngBytes})
		done <- outcome{urls, err}
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("uploads did not run concurrently")
		}
	}
	require.EqualValues(t, 2, active.Load())
	// Only the first two are allowed to start until a slot is released.
	select {
	case <-started:
		t.Fatal("exceeded concurrency limit")
	default:
	}
	for range 4 {
		release <- struct{}{}
	}
	var result outcome
	select {
	case result = <-done:
	case <-ctx.Done():
		t.Fatal("uploads did not finish")
	}
	require.NoError(t, result.err)
	require.Equal(t, []string{"https://storage.example/images/job-0.png", "https://storage.example/images/job-1.png", "https://storage.example/images/job-2.png", "https://storage.example/images/job-3.png"}, result.urls)
	require.EqualValues(t, 2, maxActive.Load())
}

func TestAdobeImageStorageFailureRetainsCompleteBatch(t *testing.T) {
	for _, emptyURL := range []bool{false, true} {
		t.Run(fmt.Sprint("empty_url=", emptyURL), func(t *testing.T) {
			storage := imageStorageFunc(func(_ context.Context, key, _ string, _ []byte) (string, error) {
				if emptyURL {
					return "", nil
				}
				return "", errors.New("storage down")
			})
			svc := NewAdobeImageService(func() (*ImageResultUploader, bool) { return NewImageResultUploader(storage, "", 0, nil), true })
			images := [][]byte{pngBytes, []byte("second image")}
			timings := AdobeImageTimings{}
			body, err := svc.buildResponseBody(context.Background(), "job", images, "max", "url", &timings)
			require.NoError(t, err)
			require.True(t, timings.StorageFallback)
			require.Equal(t, "b64_json", timings.Delivery)
			var payload struct {
				Quality string
				Data    []map[string]string
			}
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "max", payload.Quality)
			require.Len(t, payload.Data, 2)
			for i, item := range payload.Data {
				require.NotContains(t, item, "url")
				decoded, err := base64.StdEncoding.DecodeString(item["b64_json"])
				require.NoError(t, err)
				require.Equal(t, images[i], decoded)
			}
		})
	}
}

func TestImageResultUploaderDirectUploadCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	uploader := NewImageResultUploader(imageStorageFunc(func(context.Context, string, string, []byte) (string, error) {
		t.Error("canceled upload must not reach storage")
		return "", nil
	}), "", 0, nil)
	urls, err := uploader.UploadImages(ctx, "job", [][]byte{pngBytes, pngBytes, pngBytes})
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, urls)
}

// Compare response processing only; this benchmark does not simulate Adobe generation.
func BenchmarkAdobeImageResponseStorage(b *testing.B) {
	image := make([]byte, 4<<20)
	copy(image, pngBytes)
	images := [][]byte{image, image}
	for _, delay := range []time.Duration{0, 20 * time.Millisecond} {
		for _, legacy := range []bool{true, false} {
			b.Run(fmt.Sprintf("upload_delay=%s/legacy=%t", delay, legacy), func(b *testing.B) {
				uploader := NewImageResultUploader(imageStorageFunc(func(_ context.Context, key, _ string, _ []byte) (string, error) {
					if delay > 0 {
						time.Sleep(delay)
					}
					return "https://storage.example/" + key, nil
				}), "images/", 0, nil)
				svc := NewAdobeImageService(func() (*ImageResultUploader, bool) { return uploader, true })
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					var err error
					if legacy {
						data := make([]map[string]string, len(images))
						for i, img := range images {
							data[i] = map[string]string{"b64_json": base64.StdEncoding.EncodeToString(img)}
						}
						var body []byte
						body, err = json.Marshal(map[string]any{"created": time.Now().Unix(), "quality": "max", "data": data})
						if err == nil {
							_, err = uploader.Rewrite(context.Background(), "job", body)
						}
					} else {
						_, err = svc.buildResponseBody(context.Background(), "job", images, "max", "url", &AdobeImageTimings{})
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
