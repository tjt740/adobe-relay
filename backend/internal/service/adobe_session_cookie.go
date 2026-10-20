package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"time"
)

const adobeSessionCookieKey = "adobe_session_cookie"
const adobeSessionSourceHashKey = "adobe_session_cookie_source_hash"

// Keep the imported Cookie immutable for Okad source revisions and retry
// idempotency. The runtime session is valid only for that exact source Cookie.
func adobeEffectiveCookie(a *Account) string {
	source := a.GetCredential("cookie")
	if source == "" {
		return ""
	}
	if a.GetCredential(adobeSessionSourceHashKey) == fmt.Sprintf("%x", sha256.Sum256([]byte(source))) {
		return a.GetCredential(adobeSessionCookieKey)
	}
	return source
}

func adobeRefreshedSessionFields(a *Account, r *adobe.RefreshResult) map[string]any {
	fields := map[string]any{"access_token": r.AccessToken, "expires_at": adobeTokenExpiryString(r)}
	if r.CookieUpdated {
		fields[adobeSessionCookieKey] = r.Cookie
		fields[adobeSessionSourceHashKey] = fmt.Sprintf("%x", sha256.Sum256([]byte(a.GetCredential("cookie"))))
	}
	return fields
}

func adobeSessionRenewalOnly(fields map[string]any) map[string]any {
	if _, ok := fields[adobeSessionCookieKey]; !ok {
		return nil
	}
	return map[string]any{adobeSessionCookieKey: fields[adobeSessionCookieKey], adobeSessionSourceHashKey: fields[adobeSessionSourceHashKey]}
}

type AdobeSessionSnapshot struct {
	Cookie, SessionCookie, SourceHash string
	ProxyID                           *int64
	ProxyUpdatedAt                    *time.Time
}

func adobeSessionSnapshot(a *Account) AdobeSessionSnapshot {
	s := AdobeSessionSnapshot{Cookie: a.GetCredential("cookie"), SessionCookie: a.GetCredential(adobeSessionCookieKey), SourceHash: a.GetCredential(adobeSessionSourceHashKey)}
	if a.ProxyID != nil {
		proxyID := *a.ProxyID
		s.ProxyID = &proxyID
	}
	if a.Proxy != nil {
		updated := a.Proxy.UpdatedAt
		s.ProxyUpdatedAt = &updated
	}
	return s
}

type AdobeSessionCredentialsRepository interface {
	UpdateAdobeSessionIfUnchanged(context.Context, int64, AdobeSessionSnapshot, map[string]any) (bool, error)
}
