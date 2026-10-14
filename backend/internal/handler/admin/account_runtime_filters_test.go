package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type filterConcurrencyCache struct {
	service.ConcurrencyCache
	counts map[int64]int
	err    error
	calls  int
}

func (c *filterConcurrencyCache) GetAccountConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	c.calls++
	out := map[int64]int{}
	for _, id := range ids {
		out[id] = c.counts[id]
	}
	return out, c.err
}

func TestAccountRuntimeFilterStatuses(t *testing.T) {
	now := time.Now()
	expired := now.Add(-time.Minute)
	cases := []struct {
		name    string
		account service.Account
		status  string
		want    bool
	}{
		{"old timeout superseded", service.Account{Extra: map[string]any{"last_status_code": 408, "last_check_success": true, "last_check_status_code": 200}}, "request_timeout", false},
		{"latest timeout", service.Account{Extra: map[string]any{"last_check_success": false, "last_check_status_code": 408}}, "request_timeout", true},
		{"expired reason ignored", service.Account{TempUnschedulableUntil: &expired, TempUnschedulableReason: "HTTP 408", Extra: map[string]any{"last_check_success": true, "last_check_status_code": 200}}, "test_passed", true},
		{"current auth wins", service.Account{Platform: "adobe", ErrorMessage: "invalid_credentials", Extra: map[string]any{"last_check_success": true, "last_check_status_code": 200}}, "test_passed", false},
		{"current cookie failure", service.Account{Platform: "adobe", ErrorMessage: "invalid_credentials"}, "authentication_failed", true},
		{"latest auth failure", service.Account{Extra: map[string]any{"last_check_success": false, "last_check_status_code": 401}}, "authentication_failed", true},
		{"untested", service.Account{}, "untested", true},
		{"tested failure", service.Account{Extra: map[string]any{"last_check_success": false}}, "untested", false},
		{"failure", service.Account{Extra: map[string]any{"last_check_success": false}}, "test_failed", true},
		{"normal excludes timeout", service.Account{Extra: map[string]any{"last_check_success": false, "last_check_status_code": 408}}, "active", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, matchesRuntimeAccountStatus(tc.account, tc.status, now)) })
	}
}

func TestAccountRuntimeFiltersBeforePaginationAndReuseCounts(t *testing.T) {
	svc := newStubAdminService()
	cache := &filterConcurrencyCache{counts: map[int64]int{}}
	// More than the backing fetch page, with matches only near the end.
	for i := 1; i <= 1205; i++ {
		svc.accounts = append(svc.accounts, service.Account{ID: int64(i), Concurrency: 4, Extra: map[string]any{"last_check_success": false, "last_check_status_code": 408}})
		if i > 1200 {
			cache.counts[int64(i)] = 4
		}
	}
	h := &AccountHandler{adminService: svc, concurrencyService: service.NewConcurrencyService(cache)}
	f := accountRuntimeFilters{Capacity: "full"}
	rows, total, counts, err := h.listAccountsWithRuntimeFilters(context.Background(), 2, 2, "adobe", "oauth", "request_timeout", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, rows, 2)
	require.EqualValues(t, 1203, rows[0].ID)
	require.Equal(t, 4, counts[1203])
	require.Equal(t, 1, cache.calls)
	require.Equal(t, "", svc.lastListAccounts.status)
	all, total, _, err := h.listAccountsWithRuntimeFilters(context.Background(), 0, 0, "adobe", "oauth", "request_timeout", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.Len(t, all, 5)
	require.EqualValues(t, 5, total)
	rows, _, _, err = h.listAccountsWithRuntimeFilters(context.Background(), int(^uint(0)>>1), 1000, "", "", "", "", 0, "", "", "", f)
	require.NoError(t, err)
	require.Empty(t, rows)
	cache.err = errors.New("redis offline")
	_, _, _, err = h.listAccountsWithRuntimeFilters(context.Background(), 1, 20, "", "", "", "", 0, "", "", "", f)
	require.Error(t, err)
}

func TestAccountRuntimeCapacityBoundsAndValidation(t *testing.T) {
	lo, hi := 2, 4
	f := accountRuntimeFilters{ConcurrencyMin: &lo, ConcurrencyMax: &hi}
	require.NoError(t, f.validate())
	require.True(t, f.matchesCapacity(service.Account{Concurrency: 2}, 0))
	require.True(t, f.matchesCapacity(service.Account{Concurrency: 4}, 0))
	require.False(t, f.matchesCapacity(service.Account{Concurrency: 1}, 0))
	for _, tc := range []struct {
		state   string
		current int
		want    bool
	}{{"idle", 0, true}, {"idle", 1, false}, {"in_use", 1, true}, {"in_use", 4, false}, {"full", 4, true}, {"full", 5, true}} {
		t.Run(fmt.Sprintf("%s-%d", tc.state, tc.current), func(t *testing.T) {
			f.Capacity = tc.state
			require.Equal(t, tc.want, f.matchesCapacity(service.Account{Concurrency: 4}, tc.current))
		})
	}
	for _, query := range []string{"capacity=wrong", "concurrency_min=-1", "concurrency_min=2.5", "concurrency_min=8&concurrency_max=2"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+query, nil)
		_, err := runtimeFiltersFromQuery(c)
		require.Error(t, err)
	}
}

func TestAccountRuntimeCapacitySortBeforePagination(t *testing.T) {
	svc := newStubAdminService()
	svc.accounts = nil
	cache := &filterConcurrencyCache{counts: map[int64]int{}}
	// Occupancy order differs from account order and crosses the fetch page.
	for i := 1; i <= 1205; i++ {
		svc.accounts = append(svc.accounts, service.Account{ID: int64(i), Concurrency: 2000})
		cache.counts[int64(i)] = 1206 - i
	}
	h := &AccountHandler{adminService: svc, concurrencyService: service.NewConcurrencyService(cache)}
	for _, tc := range []struct {
		order string
		ids   []int64
	}{
		{"asc", []int64{1203, 1202}},
		{"desc", []int64{3, 4}},
	} {
		t.Run(tc.order, func(t *testing.T) {
			cache.calls = 0
			rows, total, counts, err := h.listAccountsWithRuntimeFilters(context.Background(), 2, 2, "", "", "", "", 0, "", "capacity", tc.order, accountRuntimeFilters{})
			require.NoError(t, err)
			require.EqualValues(t, 1205, total)
			require.Len(t, rows, 2)
			require.Equal(t, tc.ids, []int64{rows[0].ID, rows[1].ID})
			require.Equal(t, cache.counts[rows[0].ID], counts[rows[0].ID])
			require.Equal(t, 1, cache.calls)
		})
	}
	cache.err = errors.New("redis offline")
	_, _, _, err := h.listAccountsWithRuntimeFilters(context.Background(), 1, 20, "", "", "", "", 0, "", "capacity", "asc", accountRuntimeFilters{})
	require.Error(t, err)
	h.concurrencyService = nil
	_, _, _, err = h.listAccountsWithRuntimeFilters(context.Background(), 1, 20, "", "", "", "", 0, "", "capacity", "desc", accountRuntimeFilters{})
	require.Error(t, err)
}

func TestAccountRuntimeCapacitySortWithFiltersAndStableTies(t *testing.T) {
	svc := newStubAdminService()
	svc.accounts = nil
	for _, id := range []int64{5, 4, 3, 2, 1} {
		svc.accounts = append(svc.accounts, service.Account{ID: id, Concurrency: 8, Extra: map[string]any{"last_check_success": id != 1}})
	}
	cache := &filterConcurrencyCache{counts: map[int64]int{1: 8, 2: 2, 3: 6, 4: 2}}
	h := &AccountHandler{adminService: svc, concurrencyService: service.NewConcurrencyService(cache)}
	lo, hi := 4, 8
	f := accountRuntimeFilters{Capacity: "in_use", ConcurrencyMin: &lo, ConcurrencyMax: &hi}
	for _, tc := range []struct {
		order string
		ids   []int64
	}{
		{"asc", []int64{2, 4, 3}},
		{"desc", []int64{3, 2, 4}},
	} {
		rows, total, _, err := h.listAccountsWithRuntimeFilters(context.Background(), 0, 0, "", "", "test_passed", "", 0, "", "capacity", tc.order, f)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		ids := make([]int64, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		require.Equal(t, tc.ids, ids)
	}
}

func TestAccountRuntimeBulkAndExportPreserveFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newStubAdminService()
	svc.accounts = []service.Account{{ID: 1, Concurrency: 2, Extra: map[string]any{"last_check_success": true}}, {ID: 2, Concurrency: 4, Extra: map[string]any{"last_check_success": false, "last_check_status_code": 408}}}
	h := &AccountHandler{adminService: svc, concurrencyService: service.NewConcurrencyService(&filterConcurrencyCache{counts: map[int64]int{2: 4}})}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"filters":{"status":"request_timeout","capacity":"full","concurrency_min":3,"concurrency_max":5},"priority":7}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.BulkUpdate(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, []int64{2}, svc.lastBulkUpdateAccountInput.AccountIDs)
	require.Nil(t, svc.lastBulkUpdateAccountInput.Filters)
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?status=request_timeout&capacity=full&concurrency_min=3&concurrency_max=5", nil)
	rows, err := h.resolveExportAccounts(context.Background(), nil, c)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 2, rows[0].ID)
}

func TestOkadCompanyFilterBeforePaginationExportAndBulk(t *testing.T) {
	svc := newStubAdminService()
	svc.accounts = nil
	for i := 1; i <= 1205; i++ {
		extra := map[string]any{"adobeteam_external_email": "company@outlook.com"}
		if i > 1200 {
			extra["okad_account"] = map[string]any{"issuer": "site", "source_key": "external:1", "email": "company@outlook.com", "is_company": true}
		}
		svc.accounts = append(svc.accounts, service.Account{ID: int64(i), Platform: "adobe", Type: "oauth", Credentials: map[string]any{"email": "company@outlook.com"}, Extra: extra})
	}
	h := &AccountHandler{adminService: svc}
	f := accountRuntimeFilters{OkadAccountType: "company"}
	rows, total, _, err := h.listAccountsWithRuntimeFilters(context.Background(), 2, 2, "adobe", "oauth", "", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, rows, 2)
	require.EqualValues(t, 1203, rows[0].ID)
	all, total, _, err := h.listAccountsWithRuntimeFilters(context.Background(), 0, 0, "adobe", "oauth", "", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.Len(t, all, 5)
	require.EqualValues(t, 5, total)
	f.OkadAccountType = "unknown"
	_, total, _, err = h.listAccountsWithRuntimeFilters(context.Background(), 1, 10, "adobe", "oauth", "", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.EqualValues(t, 1200, total)
	f.OkadAccountType = "non_company"
	_, total, _, err = h.listAccountsWithRuntimeFilters(context.Background(), 1, 10, "adobe", "oauth", "", "", 0, "", "id", "asc", f)
	require.NoError(t, err)
	require.Zero(t, total, "unclassified imports must not become non-company")
	f.OkadAccountType = "bad-value"
	require.Error(t, f.validate())
}

func TestOkadCompanyBulkUpdateAndExportResolveSameIDs(t *testing.T) {
	svc := newStubAdminService()
	svc.accounts = nil
	for i := 1; i <= 3; i++ {
		extra := map[string]any{"adobeteam_external_email": "a@outlook.com"}
		if i < 3 {
			extra["okad_account"] = map[string]any{"issuer": "site", "source_key": "external:1", "email": "a@outlook.com", "is_company": i == 2}
		}
		svc.accounts = append(svc.accounts, service.Account{ID: int64(i), Platform: "adobe", Type: "oauth", Extra: extra})
	}
	h := &AccountHandler{adminService: svc}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"filters":{"okad_account_type":"company"},"priority":7}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.BulkUpdate(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, []int64{2}, svc.lastBulkUpdateAccountInput.AccountIDs)
	require.Nil(t, svc.lastBulkUpdateAccountInput.Filters)
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?okad_account_type=company", nil)
	rows, err := h.resolveExportAccounts(context.Background(), nil, c)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 2, rows[0].ID)
}
