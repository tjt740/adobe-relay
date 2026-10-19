package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOkadAccountKindUsesExplicitMatchingSource(t *testing.T) {
	base := `{"Platform":"adobe","Type":"oauth","Credentials":{"email":"A@outlook.com"},"Extra":{"adobeteam_external_email":"a@outlook.com","okad_account":{"issuer":"site","source_key":"external:7","email":"a@outlook.com","is_company":true}}}`
	for _, tc := range []struct {
		name   string
		change func(*Account)
		want   string
	}{
		{"company", func(a *Account) {}, "company"},
		{"explicit non-company", func(a *Account) { a.Extra["okad_account"].(map[string]any)["is_company"] = false }, "non_company"},
		{"missing metadata", func(a *Account) { delete(a.Extra, "okad_account") }, "unknown"},
		{"missing flag", func(a *Account) { delete(a.Extra["okad_account"].(map[string]any), "is_company") }, "unknown"},
		{"false string", func(a *Account) { a.Extra["okad_account"].(map[string]any)["is_company"] = "false" }, "unknown"},
		{"email changed", func(a *Account) { a.Credentials["email"] = "b@outlook.com" }, "unknown"},
		{"owner changed", func(a *Account) { a.Extra["okad"] = map[string]any{"issuer": "other", "source_key": "external:7"} }, "unknown"},
		{"source changed", func(a *Account) { a.Extra["okad"] = map[string]any{"issuer": "site", "source_key": "external:8"} }, "unknown"},
		{"versioned owner", func(a *Account) {
			delete(a.Extra, "adobeteam_external_email")
			a.Extra["okad"] = map[string]any{"issuer": "site", "source_key": "external:7"}
		}, "company"},
		{"unlinked email match", func(a *Account) { delete(a.Extra, "adobeteam_external_email") }, ""},
		{"other platform", func(a *Account) { a.Platform = "openai" }, ""},
		{"other type", func(a *Account) { a.Type = "apikey" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var a Account
			require.NoError(t, json.Unmarshal([]byte(base), &a))
			tc.change(&a)
			require.Equal(t, tc.want, OkadAccountKind(&a))
		})
	}
}

type metadataOnlyRepo struct {
	AccountRepository
	calls int
}

func (r *metadataOnlyRepo) SyncOkadAccountMetadata(_ context.Context, items []OkadAccountMetadataUpdate) (int, error) {
	r.calls++
	return len(items), nil
}

func TestOkadMetadataValidationBeforeAnyWrite(t *testing.T) {
	yes := true
	valid := OkadAccountMetadataUpdate{ID: 1, OkadAccountMetadata: OkadAccountMetadata{Issuer: "site", SourceKey: "external:7", Email: "a@outlook.com", IsCompany: &yes}}
	repo := &metadataOnlyRepo{}
	s := &adminServiceImpl{accountRepo: repo}
	for _, tc := range []struct {
		name   string
		change func(*OkadAccountMetadataUpdate)
	}{
		{"missing flag", func(p *OkadAccountMetadataUpdate) { p.IsCompany = nil }},
		{"missing owner", func(p *OkadAccountMetadataUpdate) { p.Issuer = "" }},
		{"invalid source", func(p *OkadAccountMetadataUpdate) { p.SourceKey = "external:0" }},
		{"pool cannot be company", func(p *OkadAccountMetadataUpdate) { p.SourceKey = "pool:1" }},
		{"missing email", func(p *OkadAccountMetadataUpdate) { p.Email = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := valid
			bad.ID = 2
			tc.change(&bad)
			_, err := s.SyncOkadAccountMetadata(context.Background(), []OkadAccountMetadataUpdate{valid, bad})
			require.ErrorIs(t, err, ErrOkadConflict)
		})
	}
	_, err := s.SyncOkadAccountMetadata(context.Background(), []OkadAccountMetadataUpdate{valid, valid})
	require.ErrorIs(t, err, ErrOkadConflict)
	require.Zero(t, repo.calls)
	n, err := s.SyncOkadAccountMetadata(context.Background(), []OkadAccountMetadataUpdate{valid})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 1, repo.calls)
}

func TestOkadVietnamAndGroupsUseVerifiedOwner(t *testing.T) {
	var a Account
	require.NoError(t, json.Unmarshal([]byte(`{"platform":"adobe","type":"oauth","credentials":{"email":"vn@example.com"},"extra":{"adobeteam_external_email":"vn@example.com","okad_account":{"issuer":"site","source_key":"external:7","email":"vn@example.com","is_company":false,"account_type":"vietnam","group_id":3,"group_name":"客户 A"}}}`), &a))
	require.Equal(t, "vietnam", OkadAccountKind(&a))
	key, name := OkadAccountGroup(&a)
	require.Equal(t, "site:3", key)
	require.Equal(t, "客户 A", name)
	a.Extra["okad_account"].(map[string]any)["group_id"] = nil
	a.Extra["okad_account"].(map[string]any)["group_name"] = ""
	key, _ = OkadAccountGroup(&a)
	require.Equal(t, "ungrouped", key)
	a.Extra["okad"] = map[string]any{"issuer": "foreign", "source_key": "external:7"}
	key, _ = OkadAccountGroup(&a)
	require.Empty(t, key)
	require.Equal(t, "unknown", OkadAccountKind(&a))
}
