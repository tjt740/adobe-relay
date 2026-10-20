package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

type adobeTokenIdentityContextKey struct{}

func sameAdobeAuthBoundary(a, b *Account) bool {
	if a == nil || b == nil || a.ID != b.ID || b.Platform != PlatformAdobe || b.Type != AccountTypeOAuth || a.GetCredential("cookie") != b.GetCredential("cookie") || a.Status != b.Status || a.Schedulable != b.Schedulable {
		return false
	}
	if (a.Extra["okad_manual_pause"] == true) != (b.Extra["okad_manual_pause"] == true) {
		return false
	}
	if b.ExpiresAt != nil && !b.ExpiresAt.After(time.Now()) {
		return false
	}
	if (a.ProxyID == nil) != (b.ProxyID == nil) || (a.ProxyID != nil && *a.ProxyID != *b.ProxyID) {
		return false
	}
	return a.Proxy == nil || (b.Proxy != nil && a.Proxy.UpdatedAt.Equal(b.Proxy.UpdatedAt))
}

// This is Cookie renewal only. It never invokes Okad login or increases its OTP
// budget. The existing token CAS and refresh singleflight/Redis lock are shared.
func (p *AdobeTokenProvider) recoverRejectedToken(ctx context.Context, a *Account, rejected string, renew bool) (string, error) {
	if p == nil || p.accountRepo == nil || p.refresher == nil || a == nil {
		return "", errors.New("adobe token recovery repository is unavailable")
	}
	repo, ok := p.accountRepo.(AdobeTokenInvalidationRepository)
	if !ok {
		return "", errors.New("adobe token invalidation repository is unavailable")
	}
	current, err := p.accountRepo.GetByID(ctx, a.ID)
	if err != nil {
		return "", err
	}
	if !sameAdobeAuthBoundary(a, current) {
		return "", errOAuthRefreshAccountStateChanged
	}
	if _, err = repo.InvalidateAdobeAccessTokenIfUnchanged(ctx, a.ID, rejected); err != nil {
		return "", err
	}
	if !renew {
		return "", nil
	}
	current, err = p.accountRepo.GetByID(ctx, a.ID)
	if err != nil {
		return "", err
	}
	if !sameAdobeAuthBoundary(a, current) {
		return "", errOAuthRefreshAccountStateChanged
	}
	if !p.refresher.CanRefresh(current) && !adobeTokenUsable(current.GetCredential("access_token")) {
		return "", adobe.NewAuthError("adobe session is unavailable for token renewal", 401)
	}
	ctx = context.WithValue(ctx, adobeTokenIdentityContextKey{}, adobe.AccountIDFromToken(rejected))
	token, err := p.GetAccessToken(ctx, current)
	if err != nil {
		return "", err
	}
	current, err = p.accountRepo.GetByID(ctx, a.ID)
	if err != nil {
		return "", err
	}
	if !sameAdobeAuthBoundary(a, current) {
		return "", errOAuthRefreshAccountStateChanged
	}
	durable := current.GetCredential("access_token")
	if !adobeTokenUsable(durable) || durable == rejected {
		return "", adobe.NewAuthError("adobe token changed during renewal", 401)
	}
	oldID, newID := adobe.AccountIDFromToken(rejected), adobe.AccountIDFromToken(durable)
	if oldID != "" && oldID != newID {
		return "", errOAuthRefreshAccountStateChanged
	}
	token = durable
	if err == nil && token != "" && token != rejected {
		slog.Info("adobe_token_renewal_after_rejection", "account_id", a.ID)
	}
	return token, err
}
