package domain

import (
	"regexp"
	"testing"
)

func TestAdobeGroupModelNamesAndVariants(t *testing.T) {
	for _, tc := range []struct {
		filter, name string
		matches      bool
	}{
		{"GPT Image 2", "Adobe 2 - Aapeli", true},
		{"GPT Image 2", "Adobe GPT Image 2 - Aapeli", true},
		{"GPT Image 2", "Adobe GPT Image 2.5 Flare - Aapeli", false},
		{"GPT Image 2.6 Flare", "Adobe 2.6 - Flare/Sunburst - blue", true},
		{"GPT Image 2.6 Sunburst", "Adobe 2.6 - Flare/Sunburst - blue", true},
		{"GPT Image 2.6 Flare", "Adobe GPT Image 2.6 Sunburst - blue", false},
		{"GPT Image 2.6 Flare", "adobe 2.6 张昊", false},
		{"GPT Image 2.6 Flare", "Adobe 2.6 - customer Flare", false},
		{"My Model [v3]", "Adobe My Model [v3] - blue", true},
		{"My Model [v3]", "Adobe My Model v - blue", false},
		{".*", "Adobe GPT Image 2 - blue", false},
		{"GPT Image 2.5 Flare", "Adobe GPT Image 2 + GPT Image 2.5 Flare + GPT Image 2.6 Sunburst - blue", true},
		{"2.6", "Adobe GPT Image 2 + GPT Image 2.6 Sunburst - blue", true},
		{"Future [v3]", "Adobe GPT Image 2 + Future [v3] - blue", true},
		{"Future [v3]", "Adobe Future [v3] + GPT Image 2 - blue", true},
		{"GPT Image 2.5 Flare", "Adobe GPT Image 2 - blue + GPT Image 2.5 Flare", false},
		{"GPT Image 2.5 Flare", "Adobe GPT Image 2 - blue - X + GPT Image 2.5 Flare", false},
		{"Future Pro", "Adobe Future Pro Max - blue", false},
	} {
		pattern, valid := AdobeGroupModelPattern(tc.filter)
		if !valid {
			t.Fatalf("invalid filter %q", tc.filter)
		}
		if got := regexp.MustCompile("(?i)" + pattern).MatchString(tc.name); got != tc.matches {
			t.Errorf("%q / %q: got %v", tc.filter, tc.name, got)
		}
	}
}
