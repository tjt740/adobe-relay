//go:build unit

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAdobeImagesContentRejectionReturns451(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, upstreamStatus := range []int{http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusUnavailableForLegalReasons} {
		t.Run(fmt.Sprint(upstreamStatus), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			gateway := &service.GatewayService{}
			err := fmt.Errorf("generate: %w", adobe.NewContentRejectedError("private upstream details", upstreamStatus, ""))
			failover := gateway.AdobeFailover(c.Request.Context(), 1, "unused-token", err)

			adobeImagesFailoverError(c, failover)

			require.Equal(t, http.StatusUnavailableForLegalReasons, w.Code)
			require.Equal(t, "adobe_content_rejected", gjson.Get(w.Body.String(), "error.type").String())
			require.Equal(t, "Image content was rejected by the upstream safety filter", gjson.Get(w.Body.String(), "error.message").String())
			require.NotContains(t, w.Body.String(), "private upstream details")
		})
	}
}
