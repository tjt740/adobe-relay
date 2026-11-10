package domain

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type groupAdobeModelKey struct{}

// Apply the name classification before counting and pagination.
func WithGroupAdobeModelFilter(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, groupAdobeModelKey{}, model)
}
func GroupAdobeModelFilter(ctx context.Context) string {
	model, _ := ctx.Value(groupAdobeModelKey{}).(string)
	return model
}

// Official IDs/names recognize legacy groups. Unknown names are literal matches
// against the fixed name segment, so API additions and custom names remain usable.
func AdobeGroupModelPattern(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" || !utf8.ValidString(model) || utf8.RuneCountInString(model) > 128 || strings.ContainsFunc(model, unicode.IsControl) {
		return "", false
	}
	known := map[string]string{
		"gpt-image-2": "2", "gpt image 2": "2",
		"gpt-image-2.5-flare": "2.5-flare", "gpt image 2.5 flare": "2.5-flare",
		"gpt-image-2.5-sunburst": "2.5-sunburst", "gpt image 2.5 sunburst": "2.5-sunburst",
		"gpt-image-2.6-flare": "2.6-flare", "gpt image 2.6 flare": "2.6-flare",
		"gpt-image-2.6-sunburst": "2.6-sunburst", "gpt image 2.6 sunburst": "2.6-sunburst",
	}
	normalized := strings.ToLower(model)
	if value, ok := known[normalized]; ok {
		normalized = value
	}
	// Match a selected model within the header only; never cross the customer separator.
	multiPrefix := "^[[:space:]]*adobe[[:space:]]*(([^+-]|-[^[:space:]])+[[:space:]]+\\+[[:space:]]+)*"
	prefix := multiPrefix + "(gpt[[:space:]]+image[[:space:]]+)?"
	boundary := "([[:space:]/&-]|$)"
	switch normalized {
	case "2", "2.5", "2.6":
		version := regexp.QuoteMeta(normalized)
		mixed := "2([.][56])?[[:space:]]*(and|&|/)[[:space:]]*"
		return prefix + version + boundary + "|" + prefix + mixed + version + "([[:space:]-]|$)", true
	case "2.5-flare", "2.5-sunburst", "2.6-flare", "2.6-sunburst":
		parts := strings.Split(normalized, "-")
		variant, other := parts[1], "sunburst"
		if variant == "sunburst" {
			other = "flare"
		}
		variants := "(" + variant + "([[:space:]]*/[[:space:]]*" + other + ")?|" + other + "[[:space:]]*/[[:space:]]*" + variant + ")"
		return prefix + regexp.QuoteMeta(parts[0]) + "[[:space:]]*-?[[:space:]]*" + variants + "([[:space:]-]|$)", true
	default:
		return multiPrefix + regexp.QuoteMeta(model) + "([[:space:]]+[+-][[:space:]]+|$)", true
	}
}
