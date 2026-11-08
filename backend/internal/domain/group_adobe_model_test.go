package domain

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestAdobeGroupModelPattern(t *testing.T) {
	cases := []struct {
		name   string
		models []string
	}{
		{"Adobe 2 - Aapeli", []string{"2"}},
		{"Adobe 2.5 - Flare/Sunburst - blue", []string{"2.5"}},
		{"adobe 2.6-Fridayz1024", []string{"2.6"}},
		{"adobe 2.6 刘壮壮", []string{"2.6"}},
		{"Adobe 2.5 and 2 ----南小镇", []string{"2", "2.5"}},
		{"Adobe2.6 - customer", []string{"2.6"}},
		{"Adobe 2.60 - customer", nil},
		{"Adobe 20 - customer", nil},
		{"Adobe 2.7 - customer", nil},
		{"客户测试专用分组 Adobe 2", nil},
		{"adobe账号转移分组", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, model := range []string{"2", "2.5", "2.6"} {
				pattern, valid := AdobeGroupModelPattern(model)
				if !valid {
					t.Fatal("supported model rejected")
				}
				want := false
				for _, expected := range tc.models {
					if expected == model {
						want = true
					}
				}
				if got := regexp.MustCompile("(?i)" + pattern).MatchString(tc.name); got != want {
					t.Errorf("model %s: got %v, want %v", model, got, want)
				}
			}
		})
	}
	for _, invalid := range []string{"", "a\x00b", strings.Repeat("x", 129)} {
		if _, valid := AdobeGroupModelPattern(invalid); valid {
			t.Errorf("accepted %q", invalid)
		}
	}
	ctx := context.Background()
	if GroupAdobeModelFilter(ctx) != "" {
		t.Fatal("unexpected default filter")
	}
	if GroupAdobeModelFilter(WithGroupAdobeModelFilter(ctx, "2.6")) != "2.6" {
		t.Fatal("filter lost")
	}
}
