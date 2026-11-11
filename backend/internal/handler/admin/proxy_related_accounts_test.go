package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProxyRelatedAccountsValidationAndUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/proxies/:id/related-accounts", NewProxyHandler(nil).RelatedAccounts)
	for _, path := range []string{
		"/proxies/invalid/related-accounts", "/proxies/0/related-accounts",
		"/proxies/1/related-accounts?page=-1", "/proxies/1/related-accounts?page_size=100",
		"/proxies/1/related-accounts?relation=invalid", "/proxies/1/related-accounts?page=100001",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, w.Code, path)
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/proxies/1/related-accounts", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code, "unavailable evidence is not an empty account list")
}
