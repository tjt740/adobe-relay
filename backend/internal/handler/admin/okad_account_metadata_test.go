package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type metadataAdminStub struct {
	service.AdminService
	items []service.OkadAccountMetadataUpdate
}

func (s *metadataAdminStub) SyncOkadAccountMetadata(_ context.Context, items []service.OkadAccountMetadataUpdate) (int, error) {
	s.items = items
	return len(items), nil
}

func TestOkadMetadataHandlerDoesNotQueueCredentialWork(t *testing.T) {
	svc := &metadataAdminStub{}
	// Credential invalidation, verification and test services deliberately absent.
	h := &AccountHandler{adminService: svc}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"items":[{"id":1,"issuer":"site","source_key":"external:1","email":"a@example.com","is_company":true,"cookie":"must-not-be-forwarded","proxy_id":999,"schedulable":true}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.OkadSyncAccountMetadata(c)
	require.Equal(t, 200, rec.Code)
	require.Len(t, svc.items, 1)
	require.True(t, *svc.items[0].IsCompany)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"updated":1}}`, rec.Body.String())
}
