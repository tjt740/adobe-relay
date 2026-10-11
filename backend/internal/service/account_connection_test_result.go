package service

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

var connectionTestHTTPCode = regexp.MustCompile(`(?i)(?:\b(?:HTTP|status(?:\s+code)?|returned|rejected)\s*[:=!]?[\s(]*|\()(4\d\d|5\d\d)\b`)

func connectionTestStatusCode(err error) int {
	if err == nil {
		return 200
	}
	var upstream *adobe.RequestError
	if errors.As(err, &upstream) && upstream.StatusCode > 0 {
		return upstream.StatusCode
	}
	message := err.Error()
	if match := connectionTestHTTPCode.FindStringSubmatch(message); len(match) > 1 {
		code, _ := strconv.Atoi(match[1])
		return code
	}
	if accountTest408Pattern.MatchString(message) || strings.Contains(strings.ToLower(message), "timed out") || strings.Contains(strings.ToLower(message), "deadline exceeded") {
		return 408
	}
	return 0 // no upstream HTTP response; never invent a status code
}

func (s *AccountTestService) recordConnectionTestResult(ctx context.Context, account *Account, testErr error) {
	if ctx.Err() != nil || errors.Is(testErr, context.Canceled) || account == nil {
		return
	} // closing/stopping a test is not an account failure
	code := connectionTestStatusCode(testErr)
	if recorder, ok := s.accountRepo.(interface {
		RecordConnectionTestResult(context.Context, int64, bool, int) error
	}); ok {
		if err := recorder.RecordConnectionTestResult(ctx, account.ID, testErr == nil, code); err != nil {
			slog.Warn("account_test_result_save_failed", "account_id", account.ID, "error", err)
		}
	}
	if account.Platform == PlatformAdobe && testErr != nil && (code == 401 || code == 408) {
		if recorder, ok := s.accountRepo.(interface {
			RecordAdobeConnectionTestError(context.Context, int64, int, string, bool) error
		}); ok {
			reason := ""
			if failure := classifyAdobeError(testErr); failure.Failover != nil {
				reason = string(failure.Failover.Reason)
			}
			var upstream *adobe.RequestError
			explicitHTTP := errors.As(testErr, &upstream) && upstream.StatusCode > 0
			isTimeout := code == 408 && !explicitHTTP && !connectionTestHTTPCode.MatchString(testErr.Error())
			// Telemetry failure must not hold a finished test open indefinitely.
			recordCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := recorder.RecordAdobeConnectionTestError(recordCtx, account.ID, code, reason, isTimeout); err != nil {
				slog.Warn("adobe_test_alert_record_save_failed", "account_id", account.ID, "error", err)
			}
		}
	}
	if account.Platform == PlatformAdobe && account.Type == AccountTypeOAuth {
		// Preserve typed Adobe semantics. Display strings and HTTP 401 alone
		// cannot authorize a fresh login. Dead cookies are marked by the shared
		// token provider and recovered by the scanner under its durable budget.
		failure := classifyAdobeError(testErr)
		applyAdobeCooldown(ctx, s.accountRepo, account.ID, failure)
		applyAdobeTokenInvalidation(ctx, s.accountRepo, account.ID, account.GetCredential("access_token"), failure)
	}
}
