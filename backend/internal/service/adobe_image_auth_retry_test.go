//go:build unit

package service

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type imageAuthFake struct {
	generate func(context.Context, adobe.GenerateImageInput) (*adobe.GenerateResult, error)
	upload   func(context.Context, string, []byte, string) (string, error)
}

func (f imageAuthFake) GenerateImage(c context.Context, in adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
	return f.generate(c, in)
}
func (f imageAuthFake) UploadImage(c context.Context, token string, b []byte, m string) (string, error) {
	return f.upload(c, token, b, m)
}
func rejectedImage(stage string) error {
	return &adobe.OperationError{Stage: stage, Attempts: 1, SubmissionRejected: stage == "submit", Err: adobe.NewAuthError("rejected", 401)}
}

func TestAdobeImageAuthRetryOnlyUnacceptedImageOnce(t *testing.T) {
	var calls []adobe.GenerateImageInput
	var invalidated []string
	renewals := 0
	api := imageAuthFake{generate: func(_ context.Context, in adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
		calls = append(calls, in)
		return nil, rejectedImage("submit")
	}}
	c := &adobeAuthRetryClient{api: api, source: context.Background(), token: "old", recover: func(_ context.Context, s string, renew bool) (string, error) {
		if renew {
			renewals++
			return "fresh", nil
		}
		invalidated = append(invalidated, s)
		return "", nil
	}}
	_, err := c.GenerateImage(context.Background(), adobe.GenerateImageInput{Token: "old", Options: adobe.ImagePayloadOptions{Prompt: "unchanged"}})
	require.Error(t, err)
	require.Len(t, calls, 2)
	require.Equal(t, 1, renewals)
	require.Equal(t, []string{"fresh"}, invalidated)
	require.Equal(t, "old", calls[0].Token)
	require.Equal(t, "fresh", calls[1].Token)
	require.NotEmpty(t, calls[0].ARPSessionID)
	require.Equal(t, calls[0].ARPSessionID, calls[1].ARPSessionID)
	require.Equal(t, calls[0].Options, calls[1].Options)
	var op *adobe.OperationError
	require.ErrorAs(t, err, &op)
	require.Equal(t, 2, op.Attempts)
}

func TestAdobeImageAuthRetryNeverReplaysAcceptedOrUnknownWork(t *testing.T) {
	for _, err := range []error{&adobe.OperationError{Stage: "submit", Err: adobe.NewAuthError("ambiguous authentication response", 401)}, rejectedImage("poll"), rejectedImage("download"), &adobe.OperationError{Stage: "submit", JobID: "accepted", Err: adobe.NewAuthError("rejected", 401)}, &adobe.OperationError{Stage: "submit", Err: context.DeadlineExceeded}, adobe.NewQuotaExhaustedError("quota", 401), adobe.NewAuthError("request blocked", 403)} {
		calls, renewals := 0, 0
		c := &adobeAuthRetryClient{source: context.Background(), token: "old", api: imageAuthFake{generate: func(context.Context, adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
			calls++
			return nil, err
		}}, recover: func(_ context.Context, _ string, renew bool) (string, error) {
			if renew {
				renewals++
			}
			return "new", nil
		}}
		_, got := c.GenerateImage(context.Background(), adobe.GenerateImageInput{})
		require.ErrorIs(t, got, err)
		require.Equal(t, 1, calls)
		require.Zero(t, renewals)
	}
}

func TestAdobeImageAuthRetryHonorsSDKAcceptanceProofAndSourceBudget(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accepted bool
		headers  map[string]string
		body     string
	}{
		{"unaccepted 401 can renew once", false, nil, `{"error_code":"invalid_credentials"}`},
		{"401 with accepted job id", true, nil, `{"jobId":"accepted","error":"unauthorized"}`},
		{"401 with job link", true, map[string]string{"x-override-status-link": "https://firefly-3p.ff.adobe.io/jobs/accepted"}, `{"error":"unauthorized"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &adobeFakeTransport{}
			client := adobeSubmitPollDownload(t, api, []byte("PNG"))
			success := api.handler
			submits := 0
			api.handler = func(req *adobe.Request, n int) (*adobe.Response, error) {
				if req.URL == adobe.ImageSubmitURL {
					submits++
					if submits == 1 {
						return &adobe.Response{StatusCode: 401, Headers: tc.headers, Body: []byte(tc.body)}, nil
					}
				}
				return success(req, n)
			}
			ctx := adobe.WithSingleSubmission(context.Background())
			renewals := 0
			c := &adobeAuthRetryClient{api: client, source: ctx, token: "old", recover: func(_ context.Context, _ string, renew bool) (string, error) {
				if renew {
					renewals++
				}
				return "fresh", nil
			}}
			result, err := c.GenerateImage(ctx, adobe.GenerateImageInput{Options: adobe.ImagePayloadOptions{Prompt: "fixture", AspectRatio: "1:1", UpstreamModelID: "gpt-image"}})
			if tc.accepted {
				require.Error(t, err)
				require.Equal(t, 1, submits)
				require.Zero(t, renewals)
			} else {
				require.NoError(t, err)
				require.Equal(t, []byte("PNG"), result.Bytes)
				require.Equal(t, 2, submits)
				require.Equal(t, 1, renewals)
			}
		})
	}
}

func TestAdobeImageAuthRetryDoesNotReplaySuccessfulBatchSibling(t *testing.T) {
	var mu sync.Mutex
	counts := map[int]int{}
	renewals := 0
	api := imageAuthFake{generate: func(_ context.Context, in adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
		mu.Lock()
		defer mu.Unlock()
		seed := *in.Options.Seed
		counts[seed]++
		if len(counts) == 2 && counts[seed] == 1 {
			return nil, rejectedImage("submit")
		}
		return &adobe.GenerateResult{Bytes: []byte("image")}, nil
	}}
	c := &adobeAuthRetryClient{api: api, source: context.Background(), token: "old", recover: func(_ context.Context, _ string, renew bool) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if renew {
			renewals++
		}
		return "new", nil
	}}
	images, err := generateAdobeImages(context.Background(), c, "old", "", adobe.ImagePayloadOptions{}, 2)
	require.NoError(t, err)
	require.Len(t, images, 2)
	require.Len(t, counts, 2)
	total := 0
	for _, count := range counts {
		total += count
	}
	require.Equal(t, 3, total)
	require.Equal(t, 1, renewals)
}

func TestAdobeImageAuthRetryStopsIfCallerLeavesBeforeRenewal(t *testing.T) {
	source, cancel := context.WithCancel(context.Background())
	calls, renewals := 0, 0
	c := &adobeAuthRetryClient{source: source, token: "old", api: imageAuthFake{generate: func(context.Context, adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
		calls++
		cancel()
		return nil, rejectedImage("submit")
	}}, recover: func(_ context.Context, _ string, renew bool) (string, error) {
		if renew {
			renewals++
		}
		return "new", nil
	}}
	_, err := c.GenerateImage(context.Background(), adobe.GenerateImageInput{})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
	require.Zero(t, renewals)
}

func TestAdobeUploadAuthRetryCarriesRenewedTokenIntoGeneration(t *testing.T) {
	var uploads []string
	c := &adobeAuthRetryClient{source: context.Background(), token: "old", recover: func(context.Context, string, bool) (string, error) { return "fresh", nil }, api: imageAuthFake{
		upload: func(_ context.Context, token string, _ []byte, _ string) (string, error) {
			uploads = append(uploads, token)
			if token == "old" {
				return "", adobe.NewAuthError("rejected", 401)
			}
			return "source-id", nil
		},
		generate: func(_ context.Context, in adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
			if in.Token != "fresh" {
				return nil, errors.New("stale token")
			}
			return &adobe.GenerateResult{Bytes: []byte("image")}, nil
		},
	}}
	id, err := c.UploadImage(context.Background(), "old", []byte("input"), "image/png")
	require.NoError(t, err)
	require.Equal(t, "source-id", id)
	require.Equal(t, []string{"old", "fresh"}, uploads)
	_, err = c.GenerateImage(context.Background(), adobe.GenerateImageInput{Token: "old"})
	require.NoError(t, err)
}

func TestAdobeAcceptedAuthFailureNeverFailsOverToNewGeneration(t *testing.T) {
	for _, err := range []error{rejectedImage("poll"), rejectedImage("download"), &adobeImageBatchError{rejectedImage("submit")}} {
		failure := classifyAdobeError(err)
		require.NotNil(t, failure.Failover)
		require.Equal(t, NextAccountStop, failure.Failover.NextAccountAction)
	}
}
