package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/proxyfailover"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) AcquireAdobeProxyForTest(ctx context.Context, a *service.Account, scope string) (*service.Account, *service.AdobeProxyTestLease, error) {
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return nil, nil, errors.New("temporary proxy storage unavailable")
	}
	m := proxyfailover.NewManager(db)
	lease, err := m.AcquireTestProxy(ctx, a.ID, scope)
	if err != nil || lease == nil {
		return a, nil, err
	}
	updated, err := r.GetByID(ctx, a.ID)
	if err != nil {
		persist, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		_ = m.FinishTestProxy(persist, *lease, false, false)
		return nil, nil, err
	}
	return updated, &service.AdobeProxyTestLease{AccountID: lease.AccountID, Owner: lease.Owner}, nil
}

func (r *accountRepository) FinishAdobeProxyTest(ctx context.Context, l service.AdobeProxyTestLease, success, allowResume bool) error {
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return errors.New("temporary proxy storage unavailable")
	}
	return proxyfailover.NewManager(db).FinishTestProxy(ctx, proxyfailover.TestLease{AccountID: l.AccountID, Owner: l.Owner}, success, allowResume)
}

func (r *accountRepository) AdobeTestRoute(ctx context.Context, a *service.Account) (string, bool, error) {
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return "", false, errors.New("proxy routing storage unavailable")
	}
	if a.ProxyID == nil {
		if a.Extra["manual_proxy_enabled"] == true {
			return "direct", true, nil
		}
		var automatic bool
		if err := db.QueryRowContext(ctx, `SELECT enabled FROM proxy_auto_allocation WHERE id=1`).Scan(&automatic); err != nil {
			return "", false, err
		}
		return "direct", !automatic, nil
	}
	if a.Proxy == nil {
		return "", false, nil
	}
	return proxyfailover.NewManager(db).RouteReady(ctx, *a.ProxyID, &a.Proxy.UpdatedAt)
}

func (r *accountRepository) RecordAdobeRouteReview(ctx context.Context, a *service.Account, summary map[string]any) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = r.sql.ExecContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('adobe_route_review',$4::jsonb),updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND COALESCE(credentials->>'cookie','')=$2 AND COALESCE(credentials->>'api_key','')=$3
 AND (extra->'adobe_route_review'->>'started_at' IS NULL OR (extra->'adobe_route_review'->>'started_at')::timestamptz<=($4::jsonb->>'started_at')::timestamptz)`, a.ID, a.GetCredential("cookie"), a.GetCredential("api_key"), string(raw))
	return err
}
