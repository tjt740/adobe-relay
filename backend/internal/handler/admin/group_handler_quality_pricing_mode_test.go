package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupQualityBillingModeJSONContract(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		body, err := json.Marshal(map[string]any{"image_quality_pricing_enabled": enabled, "image_quality_prices": map[string]any{"gpt-image-2.6-flare": map[string]any{"4K": map[string]any{"max": 0.1234}}}})
		require.NoError(t, err)
		var create CreateGroupRequest
		var update UpdateGroupRequest
		require.NoError(t, json.Unmarshal(body, &create))
		require.NoError(t, json.Unmarshal(body, &update))
		require.NotNil(t, create.ImageQualityPricingEnabled)
		require.Equal(t, enabled, *create.ImageQualityPricingEnabled)
		require.NotNil(t, update.ImageQualityPricingEnabled)
		require.Equal(t, enabled, *update.ImageQualityPricingEnabled)
		require.Equal(t, 0.1234, update.ImageQualityPrices["gpt-image-2.6-flare"]["4K"]["max"])
		response, err := json.Marshal(dto.GroupFromServiceAdmin(&service.Group{Platform: service.PlatformAdobe, ImageQualityPricingEnabled: enabled, ImageQualityPrices: map[string]map[string]map[string]float64(create.ImageQualityPrices)}))
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(response, &decoded))
		require.Equal(t, enabled, decoded["image_quality_pricing_enabled"], "false must be explicit in read responses")
		require.NotNil(t, decoded["image_quality_prices"])
	}
	var create CreateGroupRequest
	var update UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &create))
	require.NoError(t, json.Unmarshal([]byte(`{}`), &update))
	require.Nil(t, create.ImageQualityPricingEnabled, "omitted creation mode uses the platform default")
	require.Nil(t, update.ImageQualityPricingEnabled, "omitted update must not reset the saved mode")
	require.Nil(t, update.ImageQualityPrices)
	require.NoError(t, json.Unmarshal([]byte(`{"image_quality_pricing_enabled":false}`), &update))
	require.NotNil(t, update.ImageQualityPricingEnabled)
	require.False(t, *update.ImageQualityPricingEnabled)
	require.Nil(t, update.ImageQualityPrices, "turning off must not imply clearing prices")
}
