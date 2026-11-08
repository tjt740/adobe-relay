package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adobeModelAdminStub struct {
	*stubAdminService
	model  string
	called bool
}

func (s *adobeModelAdminStub) ListGroups(ctx context.Context, page, pageSize int, platform, status, search string, exclusive *bool, sortBy, sortOrder string) ([]service.Group, int64, error) {
	s.called = true
	s.model = domain.GroupAdobeModelFilter(ctx)
	return nil, 0, nil
}
func TestGroupListAdobeModelFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		model  string
		status int
	}{
		{"2", 200}, {"2.5", 200}, {"2.6", 200}, {"", 200}, {"gpt-image-2.6-flare", 200}, {"custom-model", 200}, {strings.Repeat("x", 129), 400},
	} {
		t.Run(tc.model, func(t *testing.T) {
			svc := &adobeModelAdminStub{stubAdminService: newStubAdminService()}
			handler := NewGroupHandler(svc, nil, nil)
			r := gin.New()
			r.GET("/groups", handler.List)
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest("GET", "/groups?adobe_model="+tc.model, nil))
			require.Equal(t, tc.status, res.Code)
			require.Equal(t, tc.status == 200, svc.called)
			if tc.status == 200 {
				require.Equal(t, tc.model, svc.model)
			}
		})
	}
}

type adobeWebsiteModelStub struct{ err error }

func (s adobeWebsiteModelStub) Fetch(ctx context.Context) (*adobe.WebsiteModelsResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &adobe.WebsiteModelsResult{Source: "adobe", SourceURL: adobe.WebsiteModelsURL, Models: []adobe.WebsiteModel{{ID: "future-model"}}}, nil
}
func TestGroupAdobeWebsiteModels(t *testing.T) {
	for _, fail := range []bool{false, true} {
		h := NewGroupHandler(newStubAdminService(), nil, nil)
		stub := adobeWebsiteModelStub{}
		if fail {
			stub.err = errors.New("upstream private diagnostic")
		}
		h.adobeWebsiteModels = stub
		r := gin.New()
		r.GET("/groups/adobe-models", h.GetAdobeWebsiteModels)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest("GET", "/groups/adobe-models", nil))
		require.Equal(t, "no-store", res.Header().Get("Cache-Control"))
		if fail {
			require.Equal(t, 503, res.Code)
			require.NotContains(t, res.Body.String(), "private")
			continue
		}
		require.Equal(t, 200, res.Code)
		var response struct {
			Data adobe.WebsiteModelsResult `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &response))
		require.Equal(t, "adobe", response.Data.Source)
		require.Equal(t, "future-model", response.Data.Models[0].ID)
	}
}
