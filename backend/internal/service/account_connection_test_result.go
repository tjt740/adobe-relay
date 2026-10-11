package service

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

var connectionTestHTTPCode = regexp.MustCompile(`(?i)(?:\b(?:HTTP|status(?:\s+code)?|returned|rejected)\s*[:=!]?[\s(]*|\()(4\d\d|5\d\d)\b`)

func connectionTestStatusCode(err error) int {
	if err == nil {
		return 200
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
	if ctx.Err() != nil || account == nil {
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
	if code == 401 && account.Platform == PlatformAdobe && account.Type == AccountTypeOAuth {
		if recovery, ok := s.accountRepo.(AdobeCookieRecoveryRepository); ok {
			// This CAS protects manual pauses/new cookies. The existing scanner
			// applies the shared durable two-attempt budget; never login here.
			if _, err := recovery.QueueAdobeCookieRecoveryIfUnchanged(ctx, account); err != nil {
				slog.Warn("account_test_recovery_queue_failed", "account_id", account.ID, "error", err)
			}
		}
	}
}
