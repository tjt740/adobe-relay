package repository

import (
	"context"
	"database/sql"
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
