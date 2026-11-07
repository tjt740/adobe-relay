//go:build unit

package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdobeImage26RenamedModelCompatibility(t *testing.T) {
	for _, pair := range [][3]string{{"gpt-image-2.6-flare", "gpt-image-2.6-star-swirl", "gpt-image-2.6-sunburst"}, {"gpt-image-2.6-sunburst", "gpt-image-2.6-aurora", "gpt-image-2.6-flare"}} {
		for _, saved := range pair[:2] {
			account := &Account{Platform: PlatformAdobe, Credentials: map[string]any{"model_mapping": map[string]any{saved: saved}}}
			for _, requested := range pair[:2] {
				require.True(t, account.IsModelSupported(requested))
				mapped, matched := account.ResolveMappedModel(requested)
				require.True(t, matched)
				require.Equal(t, saved, mapped)
				conf, err := adobe.ResolveImage(adobe.ImageRequest{ModelID: mapped, Size: "3840x2160"})
				require.NoError(t, err)
				require.Equal(t, 6, adobe.GPTImageDetailLevelFromQualityForVersion("xhigh", conf.UpstreamModelVersion))
				require.Equal(t, 7, adobe.GPTImageDetailLevelFromQualityForVersion("max", conf.UpstreamModelVersion))
			}
			require.False(t, account.IsModelSupported(pair[2]))
			require.False(t, account.IsModelSupported("gpt-image-2"))
			require.Len(t, account.GetModelMapping(), 1, "compatibility does not mutate stored mapping")
			cfg := GroupModelAllowlist{Enabled: true, Models: []string{saved}}
			require.True(t, cfg.Allows(pair[0]))
			require.True(t, cfg.Allows(pair[1]))
			require.False(t, cfg.Allows(pair[2]))
			require.Equal(t, []string{pair[0]}, cfg.FilterForListing([]string{pair[0], pair[2]}))
			wildcard := GroupModelAllowlist{Enabled: true, Models: []string{saved + "*"}}
			require.True(t, wildcard.Allows(pair[0]))
			require.Equal(t, []string{pair[0]}, wildcard.FilterForListing([]string{pair[0], pair[2]}))
		}
	}
}

func TestAdobeImage26RenamingPreservesExplicitMappingPrecedence(t *testing.T) {
	account := &Account{Platform: PlatformAdobe, Credentials: map[string]any{"model_mapping": map[string]any{
		"gpt-image-2.6-star-swirl": "legacy-override", "gpt-image-2.6-flare": "explicit-override", "gpt-image-2.6-*": "wildcard-override",
	}}}
	require.Equal(t, "explicit-override", account.GetMappedModel("gpt-image-2.6-flare"))
	require.Equal(t, "legacy-override", account.GetMappedModel("gpt-image-2.6-star-swirl"))
	require.Equal(t, "wildcard-override", account.GetMappedModel("gpt-image-2.6-sunburst"))
	account.Platform = PlatformOpenAI
	account.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-image-2.6-star-swirl": "old"}}
	require.False(t, account.IsModelSupported("gpt-image-2.6-flare"), "Adobe aliases do not change other platforms")
}
