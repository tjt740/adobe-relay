//go:build unit

package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestAdobeImageQualityPricingGenerateAndBillMatrix(t *testing.T) {
	for mi, model := range []string{"gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "gpt-image-2.5-prism", "gpt-image-2.6-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-star-swirl", "gpt-image-2.6-aurora"} {
		for si, size := range []string{"1024x1024", "2048x2048", "3840x2160"} {
			for qi, quality := range []string{"low", "medium", "high", "xhigh", "max", "auto", ""} {
				t.Run(model+"/"+size+"/"+quality, func(t *testing.T) {
					transport := &adobeFakeTransport{}
					generator := newAdobeTestService(t, adobeSubmitPollDownload(t, transport, []byte("PNGDATA")), nil)
					output, err := generator.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{Model: model, Prompt: "quality pricing test", Size: size, Quality: quality, N: 2})
					require.NoError(t, err)
					effective := quality
					if quality == "auto" || quality == "" {
						effective = "medium"
					}
					if model == "gpt-image-2" && (quality == "xhigh" || quality == "max") {
						effective = "high"
					}
					require.Equal(t, effective, output.Forward.ImageQuality)
					tier := []string{"1K", "2K", "4K"}[si]
					require.Equal(t, tier, output.Forward.ImageSize)
					level := map[string]float64{"low": 1, "medium": 3, "high": 5, "xhigh": 6, "max": 7}[effective]
					for _, body := range adobeSubmitBodies(t, transport) {
						require.Equal(t, level, body["generationSettings"].(map[string]any)["detailLevel"])
					}
					price := float64(1+mi*30+si*7+qi) / 10000
					repo := &openAIRecordUsageLogRepoStub{inserted: true}
					svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					flat := 9.0
					group := &Group{ID: 12, Platform: PlatformAdobe, RateMultiplier: 7, ImageRateIndependent: true, ImageRateMultiplier: 1.5, ImagePrice1K: &flat, ImagePrice2K: &flat, ImagePrice4K: &flat,
						ImageQualityPricingEnabled: true, ImageQualityPrices: map[string]map[string]map[string]float64{adobeImagePriceModel(model): {tier: {effective: price}}}}
					err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: output.Forward, APIKey: &APIKey{ID: 11, GroupID: &group.ID, Group: group}, User: &User{ID: 10}, Account: adobeTestAccount()})
					require.NoError(t, err)
					require.NotNil(t, repo.lastLog)
					require.Equal(t, effective, *repo.lastLog.ImageQuality)
					require.Equal(t, tier, *repo.lastLog.ImageSize)
					require.InDelta(t, price*2, repo.lastLog.TotalCost, 1e-12)
					require.InDelta(t, price*2*1.5, repo.lastLog.ActualCost, 1e-12)
					require.Nil(t, repo.lastLog.ReasoningEffort)
				})
			}
		}
	}
}

func TestAdobeImageQualityPricingMissingZeroAndExistingSources(t *testing.T) {
	groupID := int64(12)
	for _, source := range []string{"fallback", "flat", "model", "channel"} {
		t.Run(source, func(t *testing.T) {
			repo := &openAIRecordUsageLogRepoStub{inserted: true}
			svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			group := &Group{ID: groupID, Platform: PlatformAdobe, ImageQualityPricingEnabled: true, RateMultiplier: 2, ImageRateMultiplier: 1, VideoRateMultiplier: 1}
			model := "gpt-image-2.6-flare"
			price := 0.63
			switch source {
			case "flat":
				group.ImagePrice4K = &price
			case "model":
				group.ModelPricing = []ChannelModelPricing{{Models: []string{model}, BillingMode: BillingModeImage, PerRequestPrice: &price}}
				svc.resolver = NewModelPricingResolver(nil, svc.billingService)
			case "channel":
				svc.resolver = newOpenAIImageChannelPricingResolverForTest(t, groupID, model, price)
			}
			key := &APIKey{ID: 11, GroupID: &groupID, Group: group}
			result := &OpenAIForwardResult{Model: model, ImageCount: 2, ImageSize: "4K", ImageQuality: "max", Duration: time.Second}
			before := svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe)
			group.ImageQualityPrices = map[string]map[string]map[string]float64{model: {"4K": {"xhigh": 8}, "1K": {"max": 3}}}
			missing := svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe)
			require.Equal(t, before, missing, "unset Max must preserve "+source)
			group.ImageQualityPrices[model]["4K"]["max"] = 0
			free := svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe)
			require.Zero(t, free.TotalCost)
			require.Zero(t, free.ActualCost)
			group.ImageQualityPricingEnabled = false
			require.Equal(t, before, svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe), "disabled mode ignores even a saved zero price")
			require.Equal(t, 0.0, group.ImageQualityPrices[model]["4K"]["max"], "turning off retains the saved price")
			group.ImageQualityPricingEnabled = true
			require.Equal(t, free, svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe), "re-enabling restores the saved price")
			delete(group.ImageQualityPrices[model]["4K"], "max")
			require.Equal(t, before, svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe))
			// Other platform/unknown effective quality never consumes the Adobe override.
			group.ImageQualityPrices[model]["4K"]["max"] = 999
			other := svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformOpenAI)
			require.NotEqual(t, 1998.0, other.TotalCost)
			result.ImageQuality = ""
			require.Equal(t, before, svc.calculateOpenAIImageCost(context.Background(), model, key, result, 2, PlatformAdobe))
		})
	}
}

func TestAdobeImageQualityPricingValidationAndNull(t *testing.T) {
	var input ImageQualityPriceInput
	require.NoError(t, json.Unmarshal([]byte(`{"gpt-image-2.6-flare":{"4K":{"max":null,"xhigh":0}}}`), &input))
	_, ok := input["gpt-image-2.6-flare"]["4K"]["max"]
	require.False(t, ok)
	require.Equal(t, 0.0, input["gpt-image-2.6-flare"]["4K"]["xhigh"])
	require.Error(t, json.Unmarshal([]byte(`{"gpt-image-2.6-flare":{"4K":{"max":""}}}`), &input))
	for _, tc := range []struct {
		model, size, quality string
		price                float64
	}{
		{"gpt-image-2", "4K", "max", 1}, {"gpt-image-2.6-flare", "8K", "max", 1}, {"gpt-image-2.6-flare", "4K", "auto", 1}, {"gpt-image-2.6-flare", "4K", "max", -1}, {"gpt-image-2.6-flare", "4K", "max", math.Inf(1)},
	} {
		_, err := validateImageQualityPrices(PlatformAdobe, map[string]map[string]map[string]float64{tc.model: {tc.size: {tc.quality: tc.price}}})
		require.Error(t, err)
	}
	prices := map[string]map[string]map[string]float64{"gpt-image-2.6-flare": {"4K": {"max": 0}}}
	_, err := validateImageQualityPrices(PlatformOpenAI, prices)
	require.Error(t, err)
	_, err = validateImageQualityPrices(PlatformAdobe, prices)
	require.NoError(t, err)
}

func TestAdobeImageQualityPricingCacheAndDuplicate(t *testing.T) {
	groupID := int64(12)
	prices := map[string]map[string]map[string]float64{"gpt-image-2.6-flare": {"4K": {"max": 0.7}}}
	key := &APIKey{ID: 11, UserID: 10, GroupID: &groupID, Key: "sk-quality-test", Status: StatusActive, User: &User{ID: 10, Status: StatusActive}, Group: &Group{ID: groupID, Platform: PlatformAdobe, Status: StatusActive, Hydrated: true, ImageQualityPricingEnabled: true, ImageQualityPrices: prices}}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := svc.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, prices, materialized.Group.ImageQualityPrices)
	require.True(t, materialized.Group.ImageQualityPricingEnabled)
	duplicate := cloneGroupForDuplicate(key.Group, "quality-copy")
	require.Equal(t, prices, duplicate.ImageQualityPrices)
	require.True(t, duplicate.ImageQualityPricingEnabled)
	duplicate.ImageQualityPrices["gpt-image-2.6-flare"]["4K"]["max"] = 4
	require.Equal(t, 0.7, key.Group.ImageQualityPrices["gpt-image-2.6-flare"]["4K"]["max"])
}

func TestAdobeImageQualityPricingAdminSaveOmitAndClear(t *testing.T) {
	prices := map[string]map[string]map[string]float64{"gpt-image-2": {"1K": {"low": 0.0001}}, "gpt-image-2.5-flare": {"2K": {"xhigh": 0.1234}}, "gpt-image-2.6-flare": {"4K": {"max": 0}}}
	repo := &groupRepoStubForAdmin{createID: 10}
	svc := &adminServiceImpl{groupRepo: repo}
	created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "quality", Platform: PlatformAdobe, RateMultiplier: 1, ImageQualityPrices: prices})
	require.NoError(t, err)
	require.Equal(t, prices, created.ImageQualityPrices)
	require.True(t, created.ImageQualityPricingEnabled, "Adobe creation defaults to new billing")
	repo.getByID = created
	updated, err := svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.Equal(t, prices, updated.ImageQualityPrices)

	on, off := true, false
	updated, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ImageQualityPricingEnabled: &on})
	require.NoError(t, err)
	require.True(t, updated.ImageQualityPricingEnabled)
	require.Equal(t, prices, updated.ImageQualityPrices)
	updated, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{Name: "ordinary edit"})
	require.NoError(t, err)
	require.True(t, updated.ImageQualityPricingEnabled, "omitting the switch preserves the saved mode")
	updated, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ImageQualityPricingEnabled: &off})
	require.NoError(t, err)
	require.False(t, updated.ImageQualityPricingEnabled)
	require.Equal(t, prices, updated.ImageQualityPrices)
	updated, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ImageQualityPricingEnabled: &on})
	require.NoError(t, err)
	require.True(t, updated.ImageQualityPricingEnabled)
	updated, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ImageQualityPrices: map[string]map[string]map[string]float64{}})
	require.NoError(t, err)
	require.Empty(t, updated.ImageQualityPrices)
}

func TestAdobeQualityBillingModeCacheAndPlatform(t *testing.T) {
	prices := map[string]map[string]map[string]float64{"gpt-image-2.6-flare": {"4K": {"max": 0.1234}}}
	groupID := int64(12)
	for _, enabled := range []bool{false, true} {
		key := &APIKey{ID: 11, UserID: 10, GroupID: &groupID, Key: "sk-mode-test", Status: StatusActive, User: &User{ID: 10, Status: StatusActive}, Group: &Group{ID: groupID, Platform: PlatformAdobe, Status: StatusActive, Hydrated: true, ImageQualityPrices: prices, ImageQualityPricingEnabled: enabled}}
		svc := &APIKeyService{}
		encoded, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
		require.NoError(t, err)
		var entry APIKeyAuthCacheEntry
		require.NoError(t, json.Unmarshal(encoded, &entry))
		restored, used, err := svc.applyAuthCacheEntry(key.Key, &entry)
		require.NoError(t, err)
		require.True(t, used)
		require.Equal(t, enabled, restored.Group.ImageQualityPricingEnabled)
		require.Equal(t, prices, restored.Group.ImageQualityPrices)
		require.Equal(t, enabled, cloneGroupForDuplicate(key.Group, "copy").ImageQualityPricingEnabled)
		entry.Snapshot.Version = 27
		restored, used, err = svc.applyAuthCacheEntry(key.Key, &entry)
		require.NoError(t, err)
		require.False(t, used)
		require.Nil(t, restored)
	}
	svc := &adminServiceImpl{groupRepo: &groupRepoStubForAdmin{createID: 10}}
	on := true
	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "unsupported", Platform: PlatformOpenAI, RateMultiplier: 1, ImageQualityPricingEnabled: &on})
	require.Error(t, err)
}

func TestAdobeQualityBillingCreationDefaultAndExplicitOff(t *testing.T) {
	for _, platform := range []string{PlatformAdobe, PlatformOpenAI} {
		for _, state := range []string{"unset", "off", "on"} {
			if platform != PlatformAdobe && state == "on" {
				continue
			}
			t.Run(platform+"/"+state, func(t *testing.T) {
				svc := &adminServiceImpl{groupRepo: &groupRepoStubForAdmin{createID: 10}}
				var enabled *bool
				if state != "unset" {
					value := state == "on"
					enabled = &value
				}
				group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "default-mode", Platform: platform, RateMultiplier: 1, ImageQualityPricingEnabled: enabled})
				require.NoError(t, err)
				require.Equal(t, platform == PlatformAdobe && state != "off", group.ImageQualityPricingEnabled)
			})
		}
	}
}
