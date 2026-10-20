package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

type adobeImageAPI interface {
	GenerateImage(context.Context, adobe.GenerateImageInput) (*adobe.GenerateResult, error)
	UploadImage(context.Context, string, []byte, string) (string, error)
}
type adobeImageAuthRecovery func(context.Context, string, bool) (string, error)
type adobeImageAuthContextKey struct{}

func withAdobeImageAuthRecovery(ctx context.Context, p *AdobeTokenProvider, a *Account) context.Context {
	if p == nil || p.accountRepo == nil || p.refresher == nil || a == nil || a.GetCredential("cookie") == "" {
		return ctx
	}
	if _, ok := p.accountRepo.(AdobeTokenInvalidationRepository); !ok {
		return ctx
	}
	expected := snapshotOAuthRefreshAccount(a)
	expected.Extra = shallowCopyMap(a.Extra)
	if a.Proxy != nil {
		proxy := *a.Proxy
		expected.Proxy = &proxy
	}
	return context.WithValue(ctx, adobeImageAuthContextKey{}, adobeImageAuthRecovery(func(c context.Context, token string, renew bool) (string, error) {
		return p.recoverRejectedToken(c, expected, token, renew)
	}))
}
func (s *GatewayService) AdobeImageAuthContext(ctx context.Context, a *Account) context.Context {
	if s == nil {
		return ctx
	}
	return withAdobeImageAuthRecovery(ctx, s.adobeTokenProvider, a)
}

type adobeAuthRetryClient struct {
	api     adobeImageAPI
	source  context.Context
	recover adobeImageAuthRecovery
	mu      sync.RWMutex
	renewMu sync.Mutex
	token   string
}

func newAdobeAuthRetryClient(ctx context.Context, api adobeImageAPI, token string) adobeImageAPI {
	recovery, _ := ctx.Value(adobeImageAuthContextKey{}).(adobeImageAuthRecovery)
	if recovery == nil {
		return api
	}
	return &adobeAuthRetryClient{api: api, source: ctx, recover: recovery, token: token}
}
func (c *adobeAuthRetryClient) currentToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}
func (c *adobeAuthRetryClient) rejected(ctx context.Context, token string) {
	_, _ = c.recover(ctx, token, false)
}
func adobeAuth401(err error) bool {
	var auth *adobe.AuthError
	return errors.As(err, &auth) && auth.StatusCode == http.StatusUnauthorized
}
func (c *adobeAuthRetryClient) renew(ctx context.Context, rejected string) (string, error) {
	c.renewMu.Lock()
	defer c.renewMu.Unlock()
	if err := c.source.Err(); err != nil {
		c.rejected(ctx, rejected)
		return "", err
	}
	token, err := c.recover(ctx, rejected, true)
	if err != nil {
		return "", &adobe.OperationError{Stage: "token", Err: err}
	}
	if err = c.source.Err(); err != nil {
		return "", err
	}
	if token != "" && token != rejected {
		c.mu.Lock()
		c.token = token
		c.mu.Unlock()
	}
	return token, nil
}
func (c *adobeAuthRetryClient) GenerateImage(ctx context.Context, in adobe.GenerateImageInput) (*adobe.GenerateResult, error) {
	in.Token = c.currentToken()
	if in.ARPSessionID == "" {
		in.ARPSessionID = adobe.BuildARPSessionID()
	}
	started := time.Now()
	result, err := c.api.GenerateImage(ctx, in)
	if !adobeAuth401(err) {
		return result, err
	}
	var op *adobe.OperationError
	// Only a definitive authentication rejection before job acceptance permits
	// another submission. Never replay a poll/download/unknown outcome or batch.
	if result != nil || !errors.As(err, &op) || op.Stage != "submit" || op.JobID != "" {
		c.rejected(ctx, in.Token)
		return result, err
	}
	token, renewErr := c.renew(ctx, in.Token)
	if renewErr != nil {
		return nil, renewErr
	}
	if token == "" || token == in.Token {
		return result, err
	}
	firstMS := time.Since(started).Milliseconds()
	attempts := op.Attempts
	if attempts < 1 {
		attempts = 1
	}
	in.Token = token
	result, err = c.api.GenerateImage(ctx, in)
	if adobeAuth401(err) {
		c.rejected(ctx, token)
	}
	if err != nil {
		var final *adobe.OperationError
		if errors.As(err, &final) && final.Stage == "submit" {
			copy := *final
			copy.Attempts += attempts
			copy.ElapsedMS += firstMS
			err = &copy
		}
	}
	if result != nil {
		result.Timings.SubmitAttempts += attempts
		result.Timings.SubmitMS += firstMS
	}
	return result, err
}
func (c *adobeAuthRetryClient) UploadImage(ctx context.Context, _ string, data []byte, contentType string) (string, error) {
	token := c.currentToken()
	id, err := c.api.UploadImage(ctx, token, data, contentType)
	if id != "" || !adobeAuth401(err) {
		return id, err
	}
	renewed, renewErr := c.renew(ctx, token)
	if renewErr != nil {
		return "", renewErr
	}
	if renewed == "" || renewed == token {
		return id, err
	}
	id, err = c.api.UploadImage(ctx, renewed, data, contentType)
	if adobeAuth401(err) {
		c.rejected(ctx, renewed)
	}
	return id, err
}

// A failed fan-out may have successful or in-flight siblings. Returning the
// error must never trigger resubmission of the complete batch on another account.
type adobeImageBatchError struct{ cause error }

func (e *adobeImageBatchError) Error() string { return e.cause.Error() }
func (e *adobeImageBatchError) Unwrap() error { return e.cause }
