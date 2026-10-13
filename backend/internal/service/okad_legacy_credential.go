package service

import (
	"context"
	"errors"
	"strings"
)

type OkadLegacyCredentialAdmin interface {
	ApplyLegacyOkadCookie(context.Context, int64, string, string) (*Account, error)
}

func (s *adminServiceImpl) ApplyLegacyOkadCookie(ctx context.Context, id int64, cookie, email string) (*Account, error) {
	if strings.TrimSpace(cookie) == "" || len(cookie) > 262144 || strings.TrimSpace(email) == "" {
		return nil, ErrOkadConflict
	}
	a, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	identity := adobeRecoveryEmail(a)
	if identity == "" && a.GetCredential("email") == "" && a.Extra["adobeteam_external_email"] == nil {
		identity = strings.TrimSpace(a.Name)
	}
	if a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || !strings.EqualFold(identity, strings.TrimSpace(email)) {
		return nil, ErrOkadConflict
	}
	repo, ok := s.accountRepo.(interface {
		StageOkadCookie(context.Context, *Account, string) (bool, error)
	})
	if !ok {
		return nil, errors.New("credential verification storage unavailable")
	}
	applied, err := repo.StageOkadCookie(ctx, a, cookie)
	if err != nil {
		return nil, err
	}
	latest, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	state, _ := latest.Extra["okad_connection_test"].(map[string]any)
	if !applied && (latest.GetCredential("cookie") != cookie || state["key"] != OkadConnectionTestKey(latest)) {
		return nil, ErrOkadConflict
	}
	return latest, nil
}
