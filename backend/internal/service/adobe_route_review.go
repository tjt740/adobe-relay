package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/proxyfailover"
	"github.com/gin-gonic/gin"
)

type AdobeRouteReviewRepository interface {
	AdobeTestRoute(context.Context, *Account) (string, bool, error)
}
type adobeRouteReviewKey struct{}
type adobeRouteReview struct {
	initial      *Account
	account      *Account
	lease        *AdobeProxyTestLease
	source       string
	sources      []string
	attempts     int
	deferRelease bool
	proxyPending bool
	lastError    *TestEvent
	started      time.Time
}

const adobeRouteReviewEventKey = "adobe_route_review"

// TestAccountConnection keeps one bounded review around the ordinary full test.
// Every actual attempt still records its immutable route and real upstream code.
func (s *AccountTestService) TestAccountConnection(c *gin.Context, id int64, model, prompt, mode string, opts ...AccountTestOptions) error {
	a, err := s.accountRepo.GetByID(c.Request.Context(), id)
	_, supported := s.accountRepo.(AdobeRouteReviewRepository)
	if err != nil || a == nil || !supported || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || a.IsSyntheticUITest() || IsAdobeRelayAccount(a) {
		return s.testAccountConnectionOnce(c, id, model, prompt, mode, opts...)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	review, _ := ctx.Value(adobeRouteReviewKey{}).(*adobeRouteReview)
	if review == nil {
		review = &adobeRouteReview{}
	}
	review.initial = snapshotOAuthRefreshAccount(a)
	review.started = time.Now().UTC()
	c.Set(adobeRouteReviewEventKey, review)
	defer c.Set(adobeRouteReviewEventKey, nil)
	base := context.WithValue(ctx, adobeRouteReviewKey{}, review)
	for round := 0; round < 3; round++ {
		review.source = ""
		review.lastError = nil
		attemptCtx := proxyfailover.WithTestRouteOptions(base, proxyfailover.TestRouteOptions{ExcludeSources: review.sources, RequireClash: len(review.sources) > 0 && len(review.sources[0]) == 24})
		c.Request = c.Request.WithContext(adobe.WithSingleSubmission(attemptCtx))
		err = s.testAccountConnectionOnce(c, id, model, prompt, mode, opts...)
		if review.source != "" {
			review.sources = append(review.sources, review.source)
		}
		if err == nil || ctx.Err() != nil || !adobe.CanRetrySubmission(err) || (connectionTestStatusCode(err) != 408 && connectionTestStatusCode(err) != 429) || round == 2 {
			break
		}
		finishAdobeTestProxy(ctx, s.accountRepo, review.lease, false, false)
		review.lease = nil
		s.sendEvent(c, TestEvent{Type: "content", Text: fmt.Sprintf("出口未通过，正在更换不同 Clash 订阅复核（%d/3）", round+2)})
	}
	if err != nil && review.lastError != nil {
		event := *review.lastError
		if review.proxyPending {
			event.Error = fmt.Sprintf("已检测 %d 个独立出口；暂无其他可用订阅或名额，检测条件不足，待复核。", review.attempts)
		} else if review.attempts == 3 {
			event.Error = "3 个不同订阅的出口均未通过；" + event.Error
		}
		c.Set(adobeRouteReviewEventKey, nil)
		s.sendEvent(c, event)
	}
	if !review.deferRelease {
		allow, configured := ctx.Value(accountTestRecoveryPolicyKey{}).(bool)
		finishAdobeTestProxy(ctx, s.accountRepo, review.lease, err == nil, !configured || allow)
	}
	if recorder, ok := s.accountRepo.(interface {
		RecordAdobeRouteReview(context.Context, *Account, map[string]any) error
	}); ok {
		state := "failed"
		if err == nil {
			state = "success"
		} else if review.proxyPending {
			state = "proxy_pending"
		} else if ctx.Err() != nil {
			state = "interrupted"
		} else if !adobe.CanRetrySubmission(err) {
			state = "not_replayed"
		}
		persist, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stop()
		if e := recorder.RecordAdobeRouteReview(persist, review.initial, map[string]any{"state": state, "attempts": review.attempts, "sources": review.sources, "model": adobeHealthTestModel(a, model), "started_at": review.started, "checked_at": time.Now().UTC()}); e != nil {
			slog.Warn("adobe_route_review_save_failed", "account_id", id, "error", e)
		}
	}
	return err
}

func sameAdobeReviewCredentials(a, original *Account) bool {
	return a != nil && original != nil && a.GetCredential("cookie") == original.GetCredential("cookie") && a.GetCredential("api_key") == original.GetCredential("api_key")
}

// The queue owns final gate completion before returning the last route lease.
// A crashed/uncertain test retains the existing no-replay queue semantics.
func (s *AccountTestService) runOkadRouteReview(ctx context.Context, repo OkadConnectionTestRepository, job OkadConnectionTestJob) {
	review := &adobeRouteReview{deferRelease: true}
	testCtx, cancel := context.WithTimeout(context.WithValue(context.WithValue(ctx, okadConnectionTestContextKey{}, job), adobeRouteReviewKey{}, review), 3*time.Minute)
	defer cancel()
	result, err := s.RunTestBackground(testCtx, job.AccountID, "")
	state := "failed"
	if err == nil && result != nil && result.Status == "success" {
		state = "success"
	}
	if review.proxyPending {
		state = "proxy_pending"
	}
	if errors.Is(testCtx.Err(), context.Canceled) {
		state = "interrupted"
	}
	if review.account != nil {
		job.ProxyID = review.account.ProxyID
		if review.account.Proxy != nil {
			job.ProxyUpdatedAt = &review.account.Proxy.UpdatedAt
		}
	}
	persist, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if err := repo.FinishOkadConnectionTest(persist, job, state); err != nil {
		slog.Warn("okad_route_review_finish_failed", "account_id", job.AccountID, "error", err)
	}
	finishAdobeTestProxy(persist, s.accountRepo, review.lease, state == "success", true)
}
