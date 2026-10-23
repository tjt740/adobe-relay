package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// The queue is persisted in account.extra, independent of credential validation.
// Claiming is atomic across processes. A claimed test is never automatically
// replayed after a crash: an upstream generation may already have been submitted.
type OkadConnectionTestJob struct {
	AccountID      int64
	Key            string
	ProxyID        *int64
	ProxyUpdatedAt *time.Time
	Manual         bool
}

const OkadConnectionTestPending = "Okad credentials pending connection test"

func okadTestProxyMatches(a *Account, job OkadConnectionTestJob) bool {
	if (a.ProxyID == nil) != (job.ProxyID == nil) || (a.ProxyID != nil && *a.ProxyID != *job.ProxyID) {
		return false
	}
	return job.ProxyUpdatedAt == nil || (a.Proxy != nil && a.Proxy.UpdatedAt.Equal(*job.ProxyUpdatedAt))
}

// Resume only pauses owned by authentication recovery, never operator/proxy/quota pauses.
func OkadConnectionTestMayResume(a *Account) bool {
	if a == nil || a.Extra["okad_manual_pause"] == true || (a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now())) {
		return false
	}
	prior, _ := a.Extra["okad_connection_test"].(map[string]any)
	return (a.Status == StatusActive && a.Schedulable) || isAdobeCookieRecoveryCandidate(a) || (a.Status == StatusError && a.ErrorMessage == AdobeCookieRecoveryExhausted) ||
		(a.Status == StatusError && a.ErrorMessage == OkadConnectionTestPending && prior["resume"] == true)
}

func NewOkadConnectionTestState(a *Account, requireVerification, resume bool) map[string]any {
	return map[string]any{"key": OkadConnectionTestKey(a), "state": "pending", "gated": true,
		"resume": resume, "require_verification": requireVerification, "queued_at": time.Now().UTC().Format(time.RFC3339Nano)}
}

type OkadConnectionTestRepository interface {
	QueueOkadConnectionTest(context.Context, *Account, string, bool) error
	ClaimOkadConnectionTest(context.Context) (*OkadConnectionTestJob, error)
	FinishOkadConnectionTest(context.Context, OkadConnectionTestJob, string) error
}

func OkadConnectionTestKey(a *Account) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(a.GetCredential("cookie"))))
}

func newOkadConnectionTestState(a *Account, requireVerification bool) map[string]any {
	return NewOkadConnectionTestState(a, requireVerification, OkadConnectionTestMayResume(a))
}

// QueueOkadConnectionTest is called only by successful Okad credential writes.
// It never infers pending work from older accounts when a server starts.
func (s *AccountTestService) QueueOkadConnectionTest(ctx context.Context, a *Account, requireVerification bool) error {
	if s == nil || a == nil || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || a.GetCredential("cookie") == "" {
		return nil
	}
	repo, ok := s.accountRepo.(OkadConnectionTestRepository)
	if !ok {
		return fmt.Errorf("Okad account test queue unavailable")
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return repo.QueueOkadConnectionTest(ctx, a, OkadConnectionTestKey(a), requireVerification)
}

type okadConnectionTestContextKey struct{}

type okadConnectionTestWorker struct {
	once   sync.Once
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// The existing account-test runner owns this worker's startup/shutdown.
func (s *AccountTestService) startOkadConnectionTests() {
	if s == nil {
		return
	}
	repo, ok := s.accountRepo.(OkadConnectionTestRepository)
	if !ok {
		return
	}
	s.okadTests.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.okadTests.cancel = cancel
		for i := 0; i < 3; i++ {
			s.okadTests.wg.Add(1)
			go func() {
				defer s.okadTests.wg.Done()
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for ctx.Err() == nil {
					claimCtx, stop := context.WithTimeout(ctx, 5*time.Second)
					job, err := repo.ClaimOkadConnectionTest(claimCtx)
					stop()
					if err != nil && ctx.Err() == nil {
						slog.Warn("okad_test_claim_failed", "error", err)
					}
					if job != nil && err == nil {
						s.runOkadConnectionTest(ctx, repo, *job)
						continue
					}
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
		}
	})
}

func (s *AccountTestService) stopOkadConnectionTests() {
	if s != nil && s.okadTests.cancel != nil {
		s.okadTests.cancel()
		s.okadTests.wg.Wait()
	}
}

func (s *AccountTestService) runOkadConnectionTest(ctx context.Context, repo OkadConnectionTestRepository, job OkadConnectionTestJob) {
	if _, ok := s.accountRepo.(AdobeRouteReviewRepository); ok {
		s.runOkadRouteReview(ctx, repo, job)
		return
	}
	// The normal test already has an upstream timeout. This outer bound also
	// covers token exchange. Do not reconnect/login or retry failed tests here.
	testCtx, cancel := context.WithTimeout(context.WithValue(ctx, okadConnectionTestContextKey{}, job), 3*time.Minute)
	defer cancel()
	state := "failed"
	a, err := s.accountRepo.GetByID(testCtx, job.AccountID)
	if err != nil {
		_ = repo.FinishOkadConnectionTest(ctx, job, "interrupted")
		return
	}
	if a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || OkadConnectionTestKey(a) != job.Key {
		_ = repo.FinishOkadConnectionTest(ctx, job, "interrupted")
		return
	}
	a, lease, err := prepareAdobeTestProxy(testCtx, s.accountRepo, a, adobeHealthTestModel(a, ""))
	if err != nil {
		_ = repo.FinishOkadConnectionTest(ctx, job, "proxy_pending")
		return
	}
	defer func() { finishAdobeTestProxy(ctx, s.accountRepo, lease, state == "success", true) }()
	if lease != nil {
		job.ProxyID = a.ProxyID
		if a.Proxy != nil {
			job.ProxyUpdatedAt = &a.Proxy.UpdatedAt
		}
		testCtx = context.WithValue(testCtx, okadConnectionTestContextKey{}, job)
		testCtx = context.WithValue(testCtx, adobeProxyLeaseContextKey{}, lease)
	}
	if verifier, ok := s.accountRepo.(interface {
		OkadVerificationProxyReady(context.Context, *Account) (bool, error)
	}); ok {
		ready, err := verifier.OkadVerificationProxyReady(testCtx, a)
		if err != nil || !ready {
			_ = repo.FinishOkadConnectionTest(ctx, job, "proxy_pending")
			return
		}
	}
	result, err := s.RunTestBackground(testCtx, job.AccountID, "")
	if err == nil && result != nil && result.Status == "success" {
		state = "success"
	}
	if testCtx.Err() == context.Canceled {
		state = "interrupted"
	} else if testCtx.Err() == context.DeadlineExceeded {
		state = "failed"
	}
	cancel()
	persist, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if err := repo.FinishOkadConnectionTest(persist, job, state); err != nil {
		slog.Warn("okad_test_finish_failed", "account_id", job.AccountID, "error", err)
	}
}
