package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebsiteModelsURL is the public discovery endpoint used by firefly.adobe.com.
// The public web client ID is not an account credential. Never forward user
// cookies, authorization headers or a user-controlled destination here.
const WebsiteModelsURL = "https://gateway.ff.adobe.io/v2/models/discovery?resolveSchema=true&includeFirstParty=true"
const websiteModelsMaxBytes = 4 << 20

type WebsiteModelVersion struct {
	DisplayName    string         `json:"modelDisplayName"`
	Enabled        bool           `json:"enabled"`
	Deprecated     bool           `json:"isDeprecated,omitempty"`
	OutputModality []string       `json:"outputModality"`
	RequestSchema  map[string]any `json:"requestSchema,omitempty"`
}

type WebsiteModel struct {
	ID       string                         `json:"modelId"`
	Versions map[string]WebsiteModelVersion `json:"modelVersions"`
}

// Only naming metadata and resolution/quality schema fragments are exposed.
type WebsiteModelsResult struct {
	Source    string         `json:"source"`
	SourceURL string         `json:"source_url"`
	FetchedAt time.Time      `json:"fetched_at"`
	Models    []WebsiteModel `json:"models"`
}

type WebsiteModelsClient struct {
	client *http.Client
}

func NewWebsiteModelsClient() *WebsiteModelsClient {
	return &WebsiteModelsClient{client: &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

// Fetch reads a fresh public directory. Failures are returned to the UI so its
// fallback cannot be mislabeled as a successful Adobe lookup.
func (c *WebsiteModelsClient) Fetch(ctx context.Context) (*WebsiteModelsResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, WebsiteModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("x-api-key", "clio-playground-web")
	req.Header.Set("Origin", "https://firefly.adobe.com")
	req.Header.Set("Referer", "https://firefly.adobe.com/")
	res, err := c.client.Do(req)
	if err != nil {
		return nil, errors.New("Adobe model discovery unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("Adobe model discovery unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, websiteModelsMaxBytes+1))
	if err != nil || len(body) > websiteModelsMaxBytes {
		return nil, errors.New("invalid Adobe model discovery response")
	}
	var data struct {
		Models              []WebsiteModel `json:"models"`
		CommonRequestSchema map[string]any `json:"commonRequestSchema"`
	}
	if json.Unmarshal(body, &data) != nil || len(data.Models) == 0 {
		return nil, errors.New("empty or invalid Adobe model discovery response")
	}
	for i := range data.Models {
		for key, model := range data.Models[i].Versions {
			model.RequestSchema = websitePricingSchema(model.RequestSchema, data.CommonRequestSchema, 0)
			data.Models[i].Versions[key] = model
		}
	}
	return &WebsiteModelsResult{
		Source: "adobe", SourceURL: WebsiteModelsURL,
		FetchedAt: time.Now().UTC(), Models: data.Models,
	}, nil
}

// Resolve only in-document references, never fetch schema URLs. Limit recursion
// and retain only fields needed to interpret the website's pricing controls.
func websitePricingSchema(schema, common map[string]any, depth int) map[string]any {
	if schema == nil || depth > 12 {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		target := any(common)
		if strings.HasPrefix(ref, "#/") {
			for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				node, _ := target.(map[string]any)
				target = node[strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")]
			}
		} else {
			target = nil
		}
		resolved, _ := target.(map[string]any)
		if resolved == nil {
			return map[string]any{"$ref": ref}
		}
		merged := make(map[string]any, len(resolved)+len(schema))
		for k, v := range resolved {
			merged[k] = v
		}
		for k, v := range schema {
			if k != "$ref" {
				merged[k] = v
			}
		}
		return websitePricingSchema(merged, common, depth+1)
	}
	out := make(map[string]any)
	for _, key := range []string{"type", "enum", "const", "minimum", "maximum", "default"} {
		if value, ok := schema[key]; ok {
			out[key] = value
		}
	}
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if values, ok := schema[key].([]any); ok {
			items := make([]any, 0, len(values))
			for _, value := range values {
				node, _ := value.(map[string]any)
				items = append(items, websitePricingSchema(node, common, depth+1))
			}
			out[key] = items
		}
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		filtered := make(map[string]any)
		for _, key := range []string{"size", "resolution", "generationSettings", "quality", "detailLevel", "width", "height"} {
			if value, exists := properties[key]; exists {
				node, _ := value.(map[string]any)
				filtered[key] = websitePricingSchema(node, common, depth+1)
			}
		}
		out["properties"] = filtered
	}
	return out
}
