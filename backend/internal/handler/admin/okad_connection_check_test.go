//go:build unit

package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type okadCheckAdmin struct {
	service.AdminService
	a   *service.Account
	err error
}

func (s *okadCheckAdmin) ApplyOkadCredential(context.Context, int64, service.OkadCredentialUpdate) (*service.Account, error) {
	return s.a, s.err
}
func (s *okadCheckAdmin) ApplyLegacyOkadCookie(_ context.Context, _ int64, cookie, email string) (*service.Account, error) {
	s.a.Credentials["cookie"] = cookie
	s.a.Status, s.a.Schedulable, s.a.ErrorMessage = service.StatusError, false, service.OkadConnectionTestPending
	s.a.Extra["okad_connection_test"] = service.NewOkadConnectionTestState(s.a, false, true)
	return s.a, s.err
}
func (s *okadCheckAdmin) ListOkadInventory(context.Context, int64, int, *time.Time) ([]service.OkadInventoryItem, error) {
	return nil, nil
}
func (s *okadCheckAdmin) GetAccount(context.Context, int64) (*service.Account, error) {
	return s.a, nil
}
func (s *okadCheckAdmin) UpdateAccount(_ context.Context, _ int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.a.Credentials = input.Credentials
	return s.a, nil
}
func (s *okadCheckAdmin) UpdateAccountExtra(context.Context, int64, map[string]any) error { return nil }
func (s *okadCheckAdmin) ClearAccountError(context.Context, int64) (*service.Account, error) {
	return s.a, nil
}

type okadCheckQueue struct {
	service.AccountRepository
	keys   []string
	verify []bool
	err    error
}

func (r *okadCheckQueue) QueueOkadConnectionTest(_ context.Context, _ *service.Account, key string, verify bool) error {
	r.keys = append(r.keys, key)
	r.verify = append(r.verify, verify)
	return r.err
}
func (r *okadCheckQueue) ClaimOkadConnectionTest(context.Context) (*service.OkadConnectionTestJob, error) {
	return nil, nil
}
func (r *okadCheckQueue) FinishOkadConnectionTest(context.Context, service.OkadConnectionTestJob, string) error {
	return nil
}

func TestOkadCredentialPushQueuesInterfaceTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, body               string
		verify, conflict, queueFailure bool
		code, queued                   int
	}{
		{"reliable push", "/credentials", `{"version":1,"cookie":"cookie","email":"one@example.test","issuer":"okad","source_key":"external:1"}`, true, false, false, 200, 1},
		{"rejected push", "/credentials", `{"version":1}`, true, true, false, 409, 0},
		{"queue unavailable", "/credentials", `{"version":1}`, true, false, true, 503, 1},
		{"legacy push", "/oauth", `{"type":"oauth","credentials":{"cookie":"fresh"},"extra":{"adobeteam_external_email":"one@example.test"}}`, false, false, false, 200, 0},
		{"manual reauthorization", "/oauth", `{"type":"oauth","credentials":{"cookie":"fresh"}}`, false, false, false, 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admin := &okadCheckAdmin{a: &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Credentials: map[string]any{"cookie": "cookie"}, Extra: map[string]any{}}}
			if tc.conflict {
				admin.err = service.ErrOkadConflict
			}
			queue := &okadCheckQueue{}
			if tc.queueFailure {
				queue.err = errors.New("database unavailable")
			}
			h := &AccountHandler{adminService: admin, accountTestService: service.NewAccountTestService(queue, nil, nil, nil, nil, nil, nil, nil, nil)}
			router := gin.New()
			router.POST("/accounts/:id/credentials", h.OkadApplyCredential)
			router.POST("/accounts/:id/oauth", h.ApplyOAuthCredentials)
			req := httptest.NewRequest(http.MethodPost, "/accounts/1"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.Len(t, queue.keys, tc.queued)
			if tc.queued > 0 {
				require.Equal(t, service.OkadConnectionTestKey(admin.a), queue.keys[0])
				require.Equal(t, tc.verify, queue.verify[0])
			}
		})
	}
}
