package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// OkadCookieRefreshClient asks Okad to log an external Adobe member in again
// and returns the freshly issued browser cookie. The API key is only sent over
// the configured trusted connection and is never included in errors or logs.
type OkadCookieRefreshClient struct {
	url     string
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

func NewOkadCookieRefreshClient(cfg *config.Config) *OkadCookieRefreshClient {
	if cfg == nil {
		return nil
	}
	endpoint := strings.TrimSpace(cfg.Gateway.OkadCookieRefreshURL)
	apiKey := strings.TrimSpace(cfg.Gateway.OkadCookieRefreshAPIKey)
	if endpoint == "" || apiKey == "" {
		return nil
	}
	timeout := time.Duration(cfg.Gateway.OkadCookieRefreshTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &OkadCookieRefreshClient{
		url:     endpoint,
		apiKey:  apiKey,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

type okadCookieRefreshRequest struct {
	Email string `json:"email"`
}

type okadCookieRefreshResponse struct {
	OK      bool   `json:"ok"`
	Cookie  string `json:"cookie"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *OkadCookieRefreshClient) Enabled() bool {
	return c != nil && c.url != "" && c.apiKey != ""
}

func (c *OkadCookieRefreshClient) RefreshCookie(ctx context.Context, email string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("okad cookie refresh is not configured")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("adobe account email is empty")
	}
	payload, err := json.Marshal(okadCookieRefreshRequest{Email: email})
	if err != nil {
		return "", fmt.Errorf("marshal okad refresh request: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create okad refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("okad refresh request failed: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return "", fmt.Errorf("read okad refresh response: %w", readErr)
	}
	var result okadCookieRefreshResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("okad refresh response status %d is invalid", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || !result.OK || strings.TrimSpace(result.Cookie) == "" {
		// Do not copy remote response text into logs: it may contain credentials.
		return "", fmt.Errorf("okad cookie refresh rejected (HTTP %d)", resp.StatusCode)
	}
	return strings.TrimSpace(result.Cookie), nil
}
