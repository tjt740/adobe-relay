package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"golang.org/x/sync/singleflight"
)

// The request path and background scanner share one in-process login per account.
var okadCookieRecoveries singleflight.Group

type AdobeCookieRecoveryRepository interface {
	RecoverAdobeCookieIfUnchanged(context.Context, *Account, string) (bool, error)
}

type okadRecoveryAttempt struct {
	cookie string
	next   time.Time
}

type OkadCookieRecovery struct {
	repo        AccountRepository
	client      *OkadCookieRefreshClient
	retry       map[int64]okadRecoveryAttempt // owned by the scanner goroutine
	invalidator TokenCacheInvalidator
}

func newOkadCookieRecovery(repo AccountRepository, cfg *config.Config) *OkadCookieRecovery {
	client := NewOkadCookieRefreshClient(cfg)
	if repo == nil || client == nil {
		return nil
	}
	return &OkadCookieRecovery{repo: repo, client: client, retry: make(map[int64]okadRecoveryAttempt)}
}

// Match authentication failures only. Proxy removal, quota/entitlement failures,
// and administratively disabled/expired accounts must never be revived here.
func isAdobeCookieRecoveryCandidate(a *Account) bool {
	if a == nil || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || a.Status != StatusError {
		return false
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now()) {
		return false
	}
	if strings.TrimSpace(a.GetCredential("cookie")) == "" || adobeRecoveryEmail(a) == "" {
		return false
	}
	msg := strings.ToLower(a.ErrorMessage)
	if strings.HasPrefix(msg, "oauth 401 (no refresh_token):") || strings.HasPrefix(msg, "authentication failed (401):") {
		return true
	}
	return strings.HasPrefix(msg, "token refresh failed (non-retryable): adobe cookie is no longer valid") &&
		!strings.Contains(msg, "invalid_client") && !strings.Contains(msg, "invalid_scope")
}

func adobeRecoveryEmail(a *Account) string {
	if a == nil {
		return ""
	}
	email := strings.TrimSpace(a.GetCredential("email"))
	marker, _ := a.Extra["adobeteam_external_email"].(string)
	marker = strings.TrimSpace(marker)
	if email != "" && marker != "" && !strings.EqualFold(email, marker) {
		return ""
	}
	if email == "" {
		email = marker
	}
	return email
}

func (s *OkadCookieRecovery) recover(ctx context.Context, snapshot *Account) (bool, error) {
	if s == nil || snapshot == nil || snapshot.Platform != PlatformAdobe || snapshot.Type != AccountTypeOAuth {
		return false, nil
	}
	result, err, _ := okadCookieRecoveries.Do(fmt.Sprintf("%s/%d", s.client.url, snapshot.ID), func() (any, error) {
		updater, ok := s.repo.(AdobeCookieRecoveryRepository)
		if !ok {
			return false, errors.New("adobe cookie recovery persistence unavailable")
		}
		latest, err := s.repo.GetByID(ctx, snapshot.ID)
		if err != nil {
			return false, err
		}
		if latest == nil || latest.Platform != PlatformAdobe || latest.Type != AccountTypeOAuth {
			return false, nil
		}
		if latest.Status == StatusActive && !latest.Schedulable {
			return false, nil
		}
		if latest.Status != StatusActive && !isAdobeCookieRecoveryCandidate(latest) {
			return false, nil
		}
		if latest.ExpiresAt != nil && !latest.ExpiresAt.After(time.Now()) {
			return false, nil
		}
		email := adobeRecoveryEmail(latest)
		if email == "" || !strings.EqualFold(email, adobeRecoveryEmail(snapshot)) {
			return false, nil
		}
		// A concurrent Okad push/admin edit already replaced the rejected cookie.
		if latest.GetCredential("cookie") != snapshot.GetCredential("cookie") && latest.Status == StatusActive {
			return true, nil
		}
		cookie, err := s.client.RefreshCookie(ctx, email)
		if err != nil {
			return false, err
		}
		applied, err := updater.RecoverAdobeCookieIfUnchanged(ctx, latest, cookie)
		if err != nil {
			return false, err
		}
		if !applied {
			current, err := s.repo.GetByID(ctx, latest.ID)
			if err != nil {
				return false, err
			}
			if current == nil || current.Status != StatusActive || !current.Schedulable || current.Platform != PlatformAdobe || current.Type != AccountTypeOAuth ||
				!strings.EqualFold(adobeRecoveryEmail(current), email) || current.GetCredential("cookie") == latest.GetCredential("cookie") {
				return false, nil
			}
			latest = current
		}
		if s.invalidator != nil {
			if err := s.invalidator.InvalidateToken(ctx, latest); err != nil {
				slog.Warn("adobe_okad_invalidate_failed", "account_id", latest.ID)
			}
		}
		slog.Info("adobe_okad_cookie_recovered", "account_id", latest.ID, "source", "okad")
		return true, nil
	})
	if err != nil {
		return false, err
	}
	recovered, _ := result.(bool)
	return recovered, nil
}

func (s *OkadCookieRecovery) run(ctx context.Context) {
	if ctx == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	slog.Info("adobe_okad_recovery_started", "interval_seconds", 30)
	for {
		s.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Normal OAuth refresh candidates exclude error accounts. Scan a bounded page
// separately; collect the pages before recovering so updates do not shift offsets.
func (s *OkadCookieRecovery) sweep(ctx context.Context) {
	var candidates []Account
	for page := 1; page <= 10; page++ {
		accounts, info, err := s.repo.ListWithFilters(ctx, pagination.PaginationParams{Page: page, PageSize: 200, SortBy: "id", SortOrder: "asc"}, PlatformAdobe, AccountTypeOAuth, StatusError, "", 0, "")
		if err != nil {
			slog.Warn("adobe_okad_recovery_scan_failed")
			return
		}
		candidates = append(candidates, accounts...)
		if info == nil || page >= info.Pages || len(accounts) == 0 {
			break
		}
	}
	seen := make(map[int64]bool)
	for i := range candidates {
		a := &candidates[i]
		if !isAdobeCookieRecoveryCandidate(a) {
			continue
		}
		seen[a.ID] = true
		prior, ok := s.retry[a.ID]
		if ok && prior.cookie == a.GetCredential("cookie") && time.Now().Before(prior.next) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		recovered, err := s.recover(ctx, a)
		// Persisted error remains eligible for a later retry, with a bounded cadence.
		s.retry[a.ID] = okadRecoveryAttempt{cookie: a.GetCredential("cookie"), next: time.Now().Add(5 * time.Minute)}
		if err != nil {
			slog.Warn("adobe_okad_cookie_recovery_failed", "account_id", a.ID, "error", err)
		}
		if recovered {
			delete(s.retry, a.ID)
		}
	}
	for id := range s.retry {
		if !seen[id] {
			delete(s.retry, id)
		}
	}
}
