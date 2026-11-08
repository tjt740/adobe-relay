package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// WebsiteModelsURL is the public discovery endpoint used by firefly.adobe.com.
// The public web client ID is not an account credential. Never forward user
// cookies, authorization headers or a user-controlled destination here.
const WebsiteModelsURL = "https://gateway.ff.adobe.io/v2/models/discovery?resolveSchema=true&includeFirstParty=true"
const websiteModelsMaxBytes = 4 << 20

type WebsiteModelVersion struct {
	DisplayName    string   `json:"modelDisplayName"`
	Enabled        bool     `json:"enabled"`
	Deprecated     bool     `json:"isDeprecated,omitempty"`
	OutputModality []string `json:"outputModality"`
}

type WebsiteModel struct {
	ID       string                         `json:"modelId"`
	Versions map[string]WebsiteModelVersion `json:"modelVersions"`
}

// Only discovery metadata is exposed, not the upstream schemas or other data.
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
		Models []WebsiteModel `json:"models"`
	}
	if json.Unmarshal(body, &data) != nil || len(data.Models) == 0 {
		return nil, errors.New("empty or invalid Adobe model discovery response")
	}
	return &WebsiteModelsResult{
		Source: "adobe", SourceURL: WebsiteModelsURL,
		FetchedAt: time.Now().UTC(), Models: data.Models,
	}, nil
}
