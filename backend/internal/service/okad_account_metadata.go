package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// Company follows Okad's supply-mail tag, not the Adobe profile or email domain.
// Keep this metadata separate from credential verification and scheduling state.
type OkadAccountMetadata struct {
	Issuer    string `json:"issuer"`
	SourceKey string `json:"source_key"`
	Email     string `json:"email"`
	IsCompany *bool  `json:"is_company"`
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
	return p.IsCompany != nil && p.Email != "" && len(p.Email) <= 320 &&
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
	p.Issuer, _ = metadata["issuer"].(string)
	p.SourceKey, _ = metadata["source_key"].(string)
	p.Email, _ = metadata["email"].(string)
	if v, ok := metadata["is_company"].(bool); ok {
		p.IsCompany = &v
	}
	if !p.Valid() || !strings.EqualFold(p.Email, adobeRecoveryEmail(a)) ||
		(issuer != "" && (issuer != p.Issuer || source != p.SourceKey)) {
		return "unknown"
	}
	if *p.IsCompany {
		return "company"
	}
	return "non_company"
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
