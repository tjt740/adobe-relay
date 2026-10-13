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
	AccountID int64
	Key       string
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
	return map[string]any{"key": OkadConnectionTestKey(a), "state": "pending", "require_verification": requireVerification, "queued_at": time.Now().UTC().Format(time.RFC3339Nano)}
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
	// The normal test already has an upstream timeout. This outer bound also
	// covers token exchange. Do not reconnect/login or retry failed tests here.
	testCtx, cancel := context.WithTimeout(context.WithValue(ctx, okadConnectionTestContextKey{}, job), 3*time.Minute)
	result, err := s.RunTestBackground(testCtx, job.AccountID, "")
	state := "failed"
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
