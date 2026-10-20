//go:build adobe_live && unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

type adobeLiveCountTransport struct {
	adobe.Transport
	exchanges, submits *atomic.Int32
}

func (t adobeLiveCountTransport) Do(c context.Context, r *adobe.Request) (*adobe.Response, error) {
	if strings.Contains(r.URL, "/check/v6/token") {
		t.exchanges.Add(1)
	}
	if r.URL == adobe.ImageSubmitURL {
		t.submits.Add(1)
	}
	return t.Transport.Do(c, r)
}

// Explicit opt-in only: one real 1K image using the supplied owned account.
// The input/results stay in owner-readable files; no credentials are logged.
func TestAdobeLiveRejectedTokenRecovery(t *testing.T) {
	path := os.Getenv("ADOBE_LIVE_RENEWAL_INPUT")
	if path == "" {
		t.Skip("requires explicit private input file")
	}
	var in struct{ Cookie, Proxy, UserID, Model string }
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal("private input unavailable")
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatal("input file must be owner-readable only")
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &in) != nil || in.Cookie == "" || in.UserID == "" {
		t.Fatal("invalid private input")
	}
	if in.Model == "" {
		in.Model = "gpt-image-2.5-flare"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	initial, err := adobe.NewClient(adobe.ClientConfig{ProxyURL: in.Proxy}).RefreshAccessTokenFromCookie(ctx, in.Cookie, adobe.RefreshOptions{SkipAccountFetch: true})
	if err != nil {
		fmt.Println(`LIVE_ADOBE_AUTH_RECOVERY {"preflight":false}`)
		t.Fatal("session preflight failed")
	}
	if adobe.AccountIDFromToken(initial.AccessToken) != in.UserID {
		t.Fatal("account identity mismatch")
	}
	parts := strings.Split(initial.AccessToken, ".")
	if len(parts) != 3 || len(parts[2]) == 0 {
		t.Fatal("preflight token is not signed JWT")
	}
	signature := []byte(parts[2])
	if signature[0] == 'A' {
		signature[0] = 'B'
	} else {
		signature[0] = 'A'
	}
	parts[2] = string(signature)
	rejected := strings.Join(parts, ".")
	a := &Account{ID: 777777001, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": in.Cookie, "access_token": rejected}}
	a.Credentials = MergeCredentials(a.Credentials, adobeRefreshedSessionFields(a, initial))
	a.Credentials["access_token"] = rejected
	repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
	var exchanges, submits atomic.Int32
	cache := &adobeClientCache{newClient: func(string) *adobe.Client {
		return adobe.NewClient(adobe.ClientConfig{ProxyURL: in.Proxy, Transport: adobeLiveCountTransport{adobe.NewImpersonateTransport(adobe.DefaultIdentity, in.Proxy), &exchanges, &submits}})
	}}
	provider := newAdobeTokenProvider(repo, nil, &AdobeTokenRefresher{clients: cache})
	svc := NewAdobeImageService(nil)
	svc.clients = cache
	result, genErr := svc.Generate(withAdobeImageAuthRecovery(ctx, provider, a), a, rejected, &OpenAIImagesRequest{Model: in.Model, Prompt: "A single blue circle on a plain white background.", N: 1, Size: "1024x1024", Quality: "medium", ResponseFormat: "b64_json"})
	out := map[string]any{"preflight": true, "success": genErr == nil, "token_exchanges": exchanges.Load(), "submit_requests": submits.Load()}
	if result != nil {
		out["images"] = len(result.Images)
		if len(result.Images) > 0 {
			out["image_bytes"] = len(result.Images[0])
		}
	}
	if genErr != nil {
		var op *adobe.OperationError
		if errors.As(genErr, &op) {
			out["stage"] = op.Stage
		}
		var req *adobe.RequestError
		if errors.As(genErr, &req) {
			out["http"] = req.StatusCode
		}
		var auth *adobe.AuthError
		out["auth_failure"] = errors.As(genErr, &auth)
		if auth != nil {
			out["http"] = auth.StatusCode
		}
	}
	encoded, _ := json.Marshal(out)
	fmt.Println("LIVE_ADOBE_AUTH_RECOVERY " + string(encoded))
	if f := os.Getenv("ADOBE_LIVE_RENEWAL_RESULT"); f != "" {
		repo.mu.Lock()
		saved, _ := json.Marshal(repo.account.Credentials)
		repo.mu.Unlock()
		if os.WriteFile(f, saved, 0600) != nil {
			t.Fatal("cannot save private recovery result")
		}
	}
	if exchanges.Load() != 1 {
		t.Fatal("expected exactly one token renewal")
	}
	if genErr != nil {
		t.Fatal("live generation failed; see sanitized result")
	}
	if result == nil || len(result.Images) != 1 {
		t.Fatal("expected one completed image")
	}
}
