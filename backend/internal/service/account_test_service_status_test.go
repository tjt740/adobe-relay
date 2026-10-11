//go:build unit

package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestServiceSendErrorAndEndIncludesRequestTimeoutStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	err := (&AccountTestService{}).sendErrorAndEnd(ctx, "Adobe upstream returned 408")

	require.EqualError(t, err, "Adobe upstream returned 408")
	require.Contains(t, recorder.Body.String(), `"type":"error"`)
	require.Contains(t, recorder.Body.String(), `"status_code":408`)
}
