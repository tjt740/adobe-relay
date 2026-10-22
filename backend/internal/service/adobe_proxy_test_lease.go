package service

import (
	"context"
	"log/slog"
	"time"
)

type AdobeProxyTestLease struct {
	AccountID int64
	Owner     string
}
type AdobeProxyTestLeaseRepository interface {
	AcquireAdobeProxyForTest(context.Context, *Account, string) (*Account, *AdobeProxyTestLease, error)
	FinishAdobeProxyTest(context.Context, AdobeProxyTestLease, bool, bool) error
}
type adobeProxyLeaseContextKey struct{}

func prepareAdobeTestProxy(ctx context.Context, repo AccountRepository, a *Account, model string) (*Account, *AdobeProxyTestLease, error) {
	if a == nil || a.Platform != PlatformAdobe || ctx.Value(adobeProxyLeaseContextKey{}) != nil {
		return a, nil, nil
	}
	if leaser, ok := repo.(AdobeProxyTestLeaseRepository); ok {
		return leaser.AcquireAdobeProxyForTest(ctx, a, model)
	}
	return a, nil, nil
}

func finishAdobeTestProxy(ctx context.Context, repo AccountRepository, lease *AdobeProxyTestLease, success, allowResume bool) {
	if lease == nil {
		return
	}
	persist, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if leaser, ok := repo.(AdobeProxyTestLeaseRepository); ok {
		if err := leaser.FinishAdobeProxyTest(persist, *lease, success, allowResume); err != nil {
			slog.Warn("adobe_test_proxy_release_failed", "account_id", lease.AccountID, "error", err)
		}
	}
}
