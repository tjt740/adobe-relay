package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Runtime filters must run before pagination: current concurrency lives in Redis,
// while connection-test results and current errors live on the account.
type accountRuntimeFilters struct {
	Capacity       string `json:"capacity"`
	ConcurrencyMin *int   `json:"concurrency_min"`
	ConcurrencyMax *int   `json:"concurrency_max"`
}

func (f accountRuntimeFilters) validate() error {
	switch f.Capacity {
	case "", "idle", "in_use", "full":
	default:
		return infraerrors.BadRequest("INVALID_CAPACITY_FILTER", "invalid capacity filter")
	}
	if (f.ConcurrencyMin != nil && *f.ConcurrencyMin < 1) ||
		(f.ConcurrencyMax != nil && *f.ConcurrencyMax < 1) ||
		(f.ConcurrencyMin != nil && f.ConcurrencyMax != nil && *f.ConcurrencyMin > *f.ConcurrencyMax) {
		return infraerrors.BadRequest("INVALID_CAPACITY_FILTER", "concurrency limits must be positive integers and min must not exceed max")
	}
	return nil
}

func runtimeFiltersFromQuery(c *gin.Context) (accountRuntimeFilters, error) {
	f := accountRuntimeFilters{Capacity: strings.TrimSpace(c.Query("capacity"))}
	for key, target := range map[string]**int{"concurrency_min": &f.ConcurrencyMin, "concurrency_max": &f.ConcurrencyMax} {
		if raw := strings.TrimSpace(c.Query(key)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return f, infraerrors.BadRequest("INVALID_CAPACITY_FILTER", "invalid concurrency limit")
			}
			*target = &value
		}
	}
	return f, f.validate()
}

func isRuntimeAccountStatus(status string) bool {
	switch status {
	case "active", "authentication_failed", "request_timeout", "test_passed", "test_failed", "untested":
		return true
	}
	return false
}

func (f accountRuntimeFilters) needed(status string) bool {
	return isRuntimeAccountStatus(status) || f.Capacity != "" || f.ConcurrencyMin != nil || f.ConcurrencyMax != nil
}

var timeoutStatusPattern = regexp.MustCompile(`\b408\b`)
var authStatusPattern = regexp.MustCompile(`(?i)(?:^\s*|\(|\b(?:http|oauth|status(?:\s+code)?|returned|rejected|upstream|failed)\s*[:=]?\s*\(?)401\b`)
var adobeCookieErrorPattern = regexp.MustCompile(`(?i)adobe cookie is no longer valid|\binvalid_credentials\b`)

func numericAccountStatus(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := strconv.Atoi(string(v))
		return n
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	return 0
}

// Keep the precedence aligned with frontend/utils/accountStatus.ts. A newer
// completed test replaces old timeout metadata, but not a current auth error.
func accountRuntimeStatus(a service.Account, now time.Time) (authFailed, timedOut bool) {
	currentError := a.ErrorMessage
	if currentError == "" && a.TempUnschedulableUntil != nil && a.TempUnschedulableUntil.After(now) {
		currentError = a.TempUnschedulableReason
	}
	if currentError != "" {
		var reason map[string]any
		_ = json.Unmarshal([]byte(currentError), &reason)
		authFailed = (a.Platform == service.PlatformAdobe && adobeCookieErrorPattern.MatchString(currentError)) ||
			authStatusPattern.MatchString(currentError) || numericAccountStatus(reason["status_code"]) == 401
	} else {
		for _, key := range []string{"last_status_code", "status_code", "upstream_status_code", "adobe_last_status_code"} {
			if numericAccountStatus(a.Extra[key]) == 401 {
				authFailed = true
			}
		}
	}
	if authFailed {
		return true, false
	}
	if timeoutStatusPattern.MatchString(currentError) {
		return false, true
	}
	if _, checked := a.Extra["last_check_success"].(bool); checked {
		return false, numericAccountStatus(a.Extra["last_check_status_code"]) == 408
	}
	for _, key := range []string{"last_check_status_code", "last_status_code", "status_code", "upstream_status_code", "adobe_last_status_code"} {
		if numericAccountStatus(a.Extra[key]) == 408 {
			return false, true
		}
		if value, ok := a.Extra[key].(string); ok && timeoutStatusPattern.MatchString(value) {
			return false, true
		}
	}
	return false, false
}

func matchesRuntimeAccountStatus(a service.Account, status string, now time.Time) bool {
	auth, timeout := accountRuntimeStatus(a, now)
	success, checked := a.Extra["last_check_success"].(bool)
	switch status {
	case "active":
		return !auth && !timeout
	case "authentication_failed":
		return auth || (checked && !success && numericAccountStatus(a.Extra["last_check_status_code"]) == 401)
	case "request_timeout":
		return timeout
	case "test_passed":
		return checked && success && !auth && !timeout
	case "test_failed":
		return checked && !success
	case "untested":
		return !checked
	default:
		return true
	}
}

func (f accountRuntimeFilters) matchesCapacity(a service.Account, current int) bool {
	if f.ConcurrencyMin != nil && a.Concurrency < *f.ConcurrencyMin {
		return false
	}
	if f.ConcurrencyMax != nil && a.Concurrency > *f.ConcurrencyMax {
		return false
	}
	switch f.Capacity {
	case "idle":
		return current == 0
	case "in_use":
		return current > 0 && current < a.Concurrency
	case "full":
		return current >= a.Concurrency
	default:
		return true
	}
}

// page == 0 returns the entire filtered set for export or a single bulk target
// resolution. Never filter only the displayed page or silently ignore Redis errors.
func (h *AccountHandler) listAccountsWithRuntimeFilters(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode, sortBy, sortOrder string, f accountRuntimeFilters) ([]service.Account, int64, map[int64]int, error) {
	if err := f.validate(); err != nil {
		return nil, 0, nil, err
	}
	sortCapacity := strings.EqualFold(strings.TrimSpace(sortBy), "capacity")
	if !sortCapacity && !f.needed(status) && page > 0 {
		accounts, total, err := h.adminService.ListAccounts(ctx, page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
		return accounts, total, nil, err
	}
	baseStatus := status
	if isRuntimeAccountStatus(status) && status != "active" {
		baseStatus = ""
	}
	baseSortBy, baseSortOrder := sortBy, sortOrder
	if sortCapacity {
		// Fetch in a stable database order; live occupancy is sorted below.
		baseSortBy, baseSortOrder = "id", "asc"
	}
	accounts, err := h.listAccountsFiltered(ctx, platform, accountType, baseStatus, search, groupID, privacyMode, baseSortBy, baseSortOrder)
	if err != nil {
		return nil, 0, nil, err
	}
	now := time.Now()
	matched := make([]service.Account, 0, len(accounts))
	for _, a := range accounts {
		if matchesRuntimeAccountStatus(a, status, now) && (accountRuntimeFilters{ConcurrencyMin: f.ConcurrencyMin, ConcurrencyMax: f.ConcurrencyMax}).matchesCapacity(a, 0) {
			matched = append(matched, a)
		}
	}
	var counts map[int64]int
	if (f.Capacity != "" || sortCapacity) && len(matched) > 0 {
		if h.concurrencyService == nil {
			return nil, 0, nil, infraerrors.New(http.StatusServiceUnavailable, "CAPACITY_UNAVAILABLE", "current capacity is unavailable")
		}
		ids := make([]int64, len(matched))
		for i := range matched {
			ids[i] = matched[i].ID
		}
		counts, err = h.concurrencyService.GetAccountConcurrencyBatch(ctx, ids)
		if err != nil {
			return nil, 0, nil, infraerrors.New(http.StatusServiceUnavailable, "CAPACITY_UNAVAILABLE", "current capacity is unavailable")
		}
		filtered := make([]service.Account, 0, len(matched))
		for _, a := range matched {
			if f.matchesCapacity(a, counts[a.ID]) {
				filtered = append(filtered, a)
			}
		}
		matched = filtered
	}
	if sortCapacity {
		descending := strings.EqualFold(strings.TrimSpace(sortOrder), "desc")
		// Use the same occupancy snapshot for filtering, sorting and display.
		// ID breaks ties consistently in both directions and across pages.
		sort.Slice(matched, func(i, j int) bool {
			left, right := counts[matched[i].ID], counts[matched[j].ID]
			if left == right {
				return matched[i].ID < matched[j].ID
			}
			if descending {
				return left > right
			}
			return left < right
		})
	}
	total := int64(len(matched))
	if page > 0 {
		start := len(matched)
		if pageSize > 0 && page-1 <= len(matched)/pageSize {
			start = (page - 1) * pageSize
		}
		end := start + min(pageSize, len(matched)-start)
		matched = matched[start:end]
	}
	return matched, total, counts, nil
}
