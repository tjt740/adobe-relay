package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Company follows Okad's supply-mail tag, not the Adobe profile or email domain.
// Keep this metadata separate from credential verification and scheduling state.
type OkadAccountMetadata struct {
	Issuer      string `json:"issuer"`
	SourceKey   string `json:"source_key"`
	Email       string `json:"email"`
	IsCompany   *bool  `json:"is_company"`
	AccountType string `json:"account_type,omitempty"`
	GroupID     *int64 `json:"group_id"`
	GroupName   string `json:"group_name"`
}

type OkadAccountMetadataUpdate struct {
	ID int64 `json:"id"`
	OkadAccountMetadata
}

type OkadAccountMetadataAdmin interface {
	SyncOkadAccountMetadata(context.Context, []OkadAccountMetadataUpdate) (int, error)
}

var okadSourceKeyPattern = regexp.MustCompile(`^(external|pool):[1-9][0-9]*$`)

func (p OkadAccountMetadata) Valid() bool {
	kindValid := p.AccountType == "" || p.AccountType == "company" || p.AccountType == "non_company" || p.AccountType == "vietnam"
	groupValid := (p.GroupID == nil && p.GroupName == "") || (p.GroupID != nil && *p.GroupID > 0 && strings.TrimSpace(p.GroupName) != "" && utf8.RuneCountInString(p.GroupName) <= 64 && strings.HasPrefix(p.SourceKey, "external:"))
	return kindValid && groupValid && p.IsCompany != nil && (p.AccountType == "" || (p.AccountType == "company") == *p.IsCompany) && (p.AccountType != "vietnam" || strings.HasPrefix(p.SourceKey, "external:")) && p.Email != "" && len(p.Email) <= 320 &&
		p.Issuer != "" && len(p.Issuer) <= 100 && len(p.SourceKey) <= 100 && okadSourceKeyPattern.MatchString(p.SourceKey) &&
		(!*p.IsCompany || strings.HasPrefix(p.SourceKey, "external:"))
}

// Empty means not an identified Okad import. Missing or inconsistent type data
// on a known import is unknown, never implicitly non-company.
func OkadAccountKind(a *Account) string {
	if a == nil || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth {
		return ""
	}
	owner, _ := a.Extra["okad"].(map[string]any)
	issuer, _ := owner["issuer"].(string)
	source, _ := owner["source_key"].(string)
	marker, _ := a.Extra["adobeteam_external_email"].(string)
	if issuer == "" && strings.TrimSpace(marker) == "" {
		return ""
	}
	metadata, _ := a.Extra["okad_account"].(map[string]any)
	p := OkadAccountMetadata{}
	raw, err := json.Marshal(metadata)
	if err != nil || json.Unmarshal(raw, &p) != nil {
		return "unknown"
	}
	if !p.Valid() || !strings.EqualFold(p.Email, adobeRecoveryEmail(a)) ||
		(issuer != "" && (issuer != p.Issuer || source != p.SourceKey)) {
		return "unknown"
	}
	if p.AccountType != "" {
		return p.AccountType
	}
	if *p.IsCompany {
		return "company"
	}
	return "non_company"
}

// The instance issuer namespaces IDs so equal names or IDs on different Okad sites never merge.
func OkadAccountGroup(a *Account) (key, name string) {
	kind := OkadAccountKind(a)
	if kind == "" || kind == "unknown" {
		return "", ""
	}
	metadata, _ := a.Extra["okad_account"].(map[string]any)
	raw, err := json.Marshal(metadata)
	var p OkadAccountMetadata
	if err != nil || json.Unmarshal(raw, &p) != nil || !p.Valid() {
		return "", ""
	}
	if p.GroupID == nil {
		return "ungrouped", ""
	}
	return p.Issuer + ":" + strconv.FormatInt(*p.GroupID, 10), p.GroupName
}

func (s *adminServiceImpl) SyncOkadAccountMetadata(ctx context.Context, items []OkadAccountMetadataUpdate) (int, error) {
	if len(items) == 0 || len(items) > 200 {
		return 0, ErrOkadConflict
	}
	seen := make(map[int64]bool, len(items))
	for i := range items {
		items[i].Email = strings.ToLower(strings.TrimSpace(items[i].Email))
		if items[i].ID <= 0 || seen[items[i].ID] || !items[i].Valid() {
			return 0, ErrOkadConflict
		}
		seen[items[i].ID] = true
	}
	repo, ok := s.accountRepo.(interface {
		SyncOkadAccountMetadata(context.Context, []OkadAccountMetadataUpdate) (int, error)
	})
	if !ok {
		return 0, errors.New("metadata storage unavailable")
	}
	return repo.SyncOkadAccountMetadata(ctx, items)
}
