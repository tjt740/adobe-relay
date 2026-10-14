package service

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A read-only projection: no credentials or upstream error bodies cross the link.
type OkadAccountFailure struct {
	Code     int        `json:"code"`
	Reason   string     `json:"reason"`
	Source   string     `json:"source"`
	Stage    string     `json:"stage,omitempty"`
	FailedAt *time.Time `json:"failed_at,omitempty"`
}

type OkadAccountHealth struct {
	State         string               `json:"state"`
	Errors        []OkadAccountFailure `json:"errors"`
	LastSuccessAt *time.Time           `json:"last_success_at,omitempty"`
}

var okadStatusCode = regexp.MustCompile(`(?i)(?:^\s*|\(|\b(?:http(?: status)?|oauth|status(?:[ _]code)?|code|returned|rejected|upstream|failed|error|错误|状态码)[\s:=(]+)([45][0-9]{2})\b`)

func okadErrorCode(message string) int {
	var object struct {
		StatusCode int `json:"status_code"`
	}
	if json.Unmarshal([]byte(message), &object) == nil && object.StatusCode >= 400 && object.StatusCode <= 599 {
		return object.StatusCode
	}
	if match := okadStatusCode.FindStringSubmatch(message); len(match) == 2 {
		code, _ := strconv.Atoi(match[1])
		return code
	}
	lower := strings.ToLower(message)
	if strings.Contains(lower, "adobe cookie is no longer valid") || strings.Contains(lower, "invalid_credentials") {
		return 401
	}
	return 0
}

// Preserve all unresolved model failures. A success on one model must not hide
// a failure on another. Runtime errors remain independent of successful evidence.
func ProjectOkadAccountHealth(h OkadAccountHealth, message string, check map[string]any, rateLimit, overload *time.Time, now time.Time) OkadAccountHealth {
	if h.Errors == nil {
		h.Errors = []OkadAccountFailure{}
	}
	add := func(code int, reason string) {
		for _, e := range h.Errors {
			if e.Code == code {
				return
			}
		}
		h.Errors = append(h.Errors, OkadAccountFailure{Code: code, Reason: reason, Source: "account"})
	}
	// A legacy test is a fallback only; complete evidence takes precedence.
	if len(h.Errors) == 0 && h.LastSuccessAt == nil && check["last_check_success"] == false {
		code, _ := toInt64(check["last_check_status_code"])
		if code < 400 || code > 599 {
			code = 0
		}
		add(int(code), "legacy")
	}
	if strings.TrimSpace(message) != "" && message != OkadVerificationPending && message != OkadConnectionTestPending {
		// The text is intentionally discarded, including for unclassified errors.
		add(okadErrorCode(message), "account_error")
	}
	if rateLimit != nil && rateLimit.After(now) {
		add(429, "rate_limit")
	}
	if overload != nil && overload.After(now) {
		add(529, "overload")
	}
	h.State = "unknown"
	if len(h.Errors) > 0 {
		h.State = "error"
	} else if h.LastSuccessAt != nil && message != OkadVerificationPending && message != OkadConnectionTestPending && check["state"] != "pending" && check["state"] != "running" {
		h.State = "recovered"
	}
	return h
}
