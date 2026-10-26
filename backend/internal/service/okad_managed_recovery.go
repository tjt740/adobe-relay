package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OkadRecoveryError struct {
	Code       string
	RetryAfter time.Time
}

func (e *OkadRecoveryError) Error() string { return "Okad recovery: " + e.Code }
func (c *OkadCookieRefreshClient) managedCall(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &OkadRecoveryError{Code: "service_unavailable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, &OkadRecoveryError{Code: "configuration"}
	}
	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		return nil, &OkadRecoveryError{Code: "service_unavailable"}
	}
	var result map[string]any
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil {
		return nil, &OkadRecoveryError{Code: "protocol"}
	}
	return result, nil
}
func (c *OkadCookieRefreshClient) managedBase() string { return strings.TrimSuffix(c.url, "/refresh") }
func okadSource(a *Account) string {
	state, _ := a.Extra["okad"].(map[string]any)
	v, _ := state["source_key"].(string)
	return v
}
func (c *OkadCookieRefreshClient) Preflight(ctx context.Context, a *Account) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result, err := c.managedCall(ctx, "GET", c.managedBase()+"/capability?email="+url.QueryEscape(adobeRecoveryEmail(a))+"&source_key="+url.QueryEscape(okadSource(a))+"&cookie_hash="+fmt.Sprintf("%x", sha256.Sum256([]byte(a.GetCredential("cookie")))), nil)
	if err != nil {
		return err
	}
	if result["ok"] != true {
		code, _ := result["code"].(string)
		return &OkadRecoveryError{Code: code}
	}
	email, _ := result["email"].(string)
	source, _ := result["source_key"].(string)
	if !strings.EqualFold(email, adobeRecoveryEmail(a)) || source == "" || (okadSource(a) != "" && source != okadSource(a)) {
		return &OkadRecoveryError{Code: "identity_mismatch"}
	}
	return nil
}
func (c *OkadCookieRefreshClient) RefreshManaged(ctx context.Context, a *Account, requestID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	result, err := c.managedCall(ctx, "POST", c.url, map[string]any{"email": adobeRecoveryEmail(a), "source_key": okadSource(a), "request_id": requestID})
	for err == nil {
		if result["managed"] != true {
			return &OkadRecoveryError{Code: "protocol"}
		}
		if email, ok := result["email"].(string); !ok || !strings.EqualFold(email, adobeRecoveryEmail(a)) {
			return &OkadRecoveryError{Code: "identity_mismatch"}
		}
		id, ok := result["job_id"].(string)
		if !ok || id != requestID {
			return &OkadRecoveryError{Code: "protocol"}
		}
		if result["state"] == "done" && result["ok"] == true {
			return nil
		}
		if result["state"] == "failed" {
			code, _ := result["code"].(string)
			retry, _ := toInt64(result["retry_after"])
			var next time.Time
			if retry > 0 {
				next = time.Unix(retry, 0)
			}
			return &OkadRecoveryError{Code: code, RetryAfter: next}
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return &OkadRecoveryError{Code: "outcome_unknown"}
		case <-timer.C:
		}
		result, err = c.managedCall(ctx, "GET", c.managedBase()+"/jobs/"+url.PathEscape(id), nil)
	}
	return err
}
func (s *OkadCookieRecovery) recordManagedState(ctx context.Context, a *Account, err error) {
	repo, ok := s.repo.(interface {
		RecordOkadRecoveryState(context.Context, *Account, string, time.Time) error
	})
	if !ok {
		return
	}
	code := "service_unavailable"
	var typed *OkadRecoveryError
	if errors.As(err, &typed) {
		code = typed.Code
	}
	allowed := map[string]bool{"awaiting_sync": true, "service_unavailable": true, "configuration": true, "protocol": true, "not_registered": true, "ambiguous": true, "needs_manual": true, "missing_mail": true, "identity_mismatch": true, "busy": true, "outcome_unknown": true, "rate_limited": true, "login_cooldown": true, "login_failed": true, "internal": true, "account_disabled": true}
	if !allowed[code] {
		code = "internal"
	}
	wait := time.Minute
	if code == "needs_manual" || code == "not_registered" || code == "missing_mail" || code == "ambiguous" {
		wait = 15 * time.Minute
	}
	save, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	next := time.Now().Add(wait)
	if code == "login_cooldown" {
		next = okadCooldownNext(typed, time.Now())
	}
	_ = repo.RecordOkadRecoveryState(save, a, code, next)
}

// The reservation and account scanner must use the same cooldown deadline.
// Managed legacy jobs omit RetryAfter; retry in five minutes without spending
// a login attempt, instead of silently holding the account for fifteen minutes.
func okadCooldownNext(err *OkadRecoveryError, now time.Time) time.Time {
	next := err.RetryAfter
	if next.IsZero() || next.After(now.Add(time.Hour)) {
		return now.Add(AdobeCookieRecoveryRetryDelay)
	}
	if next.Before(now.Add(time.Minute)) {
		return now.Add(time.Minute)
	}
	return next
}
