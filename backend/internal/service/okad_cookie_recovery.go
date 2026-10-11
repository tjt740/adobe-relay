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
	QueueAdobeCookieRecoveryIfUnchanged(context.Context, *Account) (bool, error)
	ClaimAdobeCookieRecovery(context.Context, *Account, time.Duration) (*AdobeCookieRecoveryAttempt, error)
	FinishAdobeCookieRecovery(context.Context, int64, string, bool) error
	SetAdobeCookieRecoveryErrorIfUnchanged(context.Context, *Account, string) (bool, error)
}

const (
	AdobeCookieRecoveryMaxAttempts  = 2
	AdobeCookieRecoveryRetryDelay   = 5 * time.Minute
	AdobeCookieRecoveryHealthyDelay = 15 * time.Minute
	AdobeCookieRecoveryPending      = "Authentication failed (401): Okad recovery pending"
	AdobeCookieRecoveryExhausted    = "Authentication failed (401): Okad automatic login limit reached (2/2); manual reauthorization required"
)

// A durable reservation is shared by every caller and process. Cookie rotation
// and a successful Okad response do not reset the attempt budget.
type AdobeCookieRecoveryAttempt struct {
	ID       string
	Count    int
	Allowed  bool
	InFlight bool
}

type OkadCookieRecovery struct {
	repo        AccountRepository
	client      *OkadCookieRefreshClient
	invalidator TokenCacheInvalidator
}

func newOkadCookieRecovery(repo AccountRepository, cfg *config.Config) *OkadCookieRecovery {
	client := NewOkadCookieRefreshClient(cfg)
	if repo == nil || client == nil {
		return nil
	}
	return &OkadCookieRecovery{repo: repo, client: client}
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
	if a.ErrorMessage == AdobeCookieRecoveryExhausted {
		return false
	}
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
		attempt, err := updater.ClaimAdobeCookieRecovery(ctx, latest, s.client.timeout)
		if err != nil {
			// Fail closed: never contact Okad without a durable reservation.
			return false, err
		}
		if !attempt.Allowed {
			if attempt.InFlight {
				return true, nil // another process owns the current login
			}
			return false, s.queue(ctx, updater, latest, attempt.Count)
		}
		// A cancelled user request must not lose the completion/cooldown write.
		loginOK := false
		defer func() {
			finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := updater.FinishAdobeCookieRecovery(finishCtx, latest.ID, attempt.ID, loginOK); err != nil {
				slog.Warn("adobe_okad_recovery_finish_failed", "account_id", latest.ID, "error", err)
			}
		}()
		slog.Info("adobe_okad_login_attempt", "account_id", latest.ID, "attempt", attempt.Count, "max_attempts", AdobeCookieRecoveryMaxAttempts)
		cookie, err := s.client.RefreshCookie(ctx, email)
		if err != nil {
			queueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if queueErr := s.queue(queueCtx, updater, latest, attempt.Count); queueErr != nil {
				slog.Warn("adobe_okad_queue_failed", "account_id", latest.ID, "error", queueErr)
			}
			return false, err
		}
		loginOK = true
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

func (s *OkadCookieRecovery) queue(ctx context.Context, repo AdobeCookieRecoveryRepository, a *Account, count int) error {
	message := AdobeCookieRecoveryPending
	if count >= AdobeCookieRecoveryMaxAttempts {
		message = AdobeCookieRecoveryExhausted
	}
	_, err := repo.SetAdobeCookieRecoveryErrorIfUnchanged(ctx, a, message)
	return err
}

// Only a real authenticated Adobe response after the stabilization period may
// close an incident. Cached usage, clearing an error, or pushing a cookie cannot.
func confirmAdobeCookieRecovery(ctx context.Context, repo AccountRepository, a *Account) {
	if a == nil || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth {
		return
	}
	if confirmer, ok := repo.(interface {
		ConfirmAdobeCookieRecovery(context.Context, *Account) error
	}); ok {
		if err := confirmer.ConfirmAdobeCookieRecovery(ctx, a); err != nil {
			slog.Warn("adobe_okad_recovery_confirmation_failed", "account_id", a.ID, "error", err)
		}
	}
}

func (s *GatewayService) ConfirmAdobeCookieRecovery(ctx context.Context, a *Account) {
	confirmAdobeCookieRecovery(ctx, s.accountRepo, a)
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
	for i := range candidates {
		a := &candidates[i]
		if !isAdobeCookieRecoveryCandidate(a) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		_, err := s.recover(ctx, a)
		if err != nil {
			slog.Warn("adobe_okad_cookie_recovery_failed", "account_id", a.ID, "error", err)
		}
	}
}
