//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAdobeDeliveryLogsBrokenPipeSeparatelyFromGeneration(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
	core, logs := observer.New(zap.WarnLevel)
	_ = c.Error(errors.New("write: broken pipe"))
	ok := logAdobeImageDelivery(c, zap.New(core), 10, 13000000, 0, &service.OpenAIForwardResult{Duration: 90 * time.Second})
	require.False(t, ok)
	require.Len(t, logs.All(), 1)
	event := logs.All()[0]
	require.Equal(t, "adobe_images.delivery_failed", event.Message)
	require.Equal(t, true, event.ContextMap()["image_generated"])
	require.Equal(t, int64(13000000), event.ContextMap()["response_bytes"])
	require.Equal(t, int64(90000), event.ContextMap()["generation_elapsed_ms"])
	require.Contains(t, event.ContextMap()["error"], "broken pipe")
}

func TestAdobeDeliveryLogsCanceledClient(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(ctx)
	core, logs := observer.New(zap.WarnLevel)
	require.False(t, logAdobeImageDelivery(c, zap.New(core), 10, 12, 0, nil))
	require.Equal(t, "adobe_images.client_disconnected_after_generation", logs.All()[0].Message)
}
