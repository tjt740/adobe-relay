package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

const OkadVerificationPending = "Okad credentials pending verification"

// Only credential fields cross this boundary. Scheduling/configuration belongs to Sub2.
type OkadCredentialUpdate struct {
	Email     string `json:"email"`
	Issuer    string `json:"issuer"`
	SourceKey string `json:"source_key"`
	Version   int64  `json:"version"`
	Cookie    string `json:"cookie"`
}

type OkadInventoryItem struct {
	ID             int64          `json:"id"`
	Email          string         `json:"email"`
	Status         string         `json:"status"`
	Schedulable    bool           `json:"schedulable"`
	ProxyID        *int64         `json:"proxy_id"`
	State          map[string]any `json:"integration"`
	ConnectionTest map[string]any `json:"connection_test,omitempty"`
	Attempts       int            `json:"attempts"`
	NextAttemptAt  *time.Time     `json:"next_attempt_at,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type OkadIntegrationRepository interface {
	OkadVerificationProxyReady(context.Context, *Account) (bool, error)
	ApplyOkadCredential(context.Context, *Account, OkadCredentialUpdate) (bool, error)
	CompleteOkadVerification(context.Context, *Account, map[string]any, string, string) (bool, error)
	ListOkadInventory(context.Context, int64, int, *time.Time) ([]OkadInventoryItem, error)
}

type OkadIntegrationAdmin interface {
	ApplyOkadCredential(context.Context, int64, OkadCredentialUpdate) (*Account, error)
	ListOkadInventory(context.Context, int64, int, *time.Time) ([]OkadInventoryItem, error)
}

var ErrOkadConflict = errors.New("credential version, identity or account state conflict")
var errOkadIdentityMismatch = errors.New("adobe identity does not match linked email")

type okadVerificationClient interface {
	RefreshAccessTokenFromCookie(context.Context, any, adobe.RefreshOptions) (*adobe.RefreshResult, error)
	FetchAccountInfo(context.Context, string) (*adobe.AccountInfo, error)
	FetchCreditsBalance(context.Context, string) (*adobe.CreditsBalance, error)
}

func probeOkadCredential(ctx context.Context, a *Account, client okadVerificationClient) (map[string]any, *adobe.CreditsBalance, error) {
	result, err := client.RefreshAccessTokenFromCookie(ctx, a.GetCredential("cookie"), adobe.RefreshOptions{SkipAccountFetch: true})
	if err != nil {
		return nil, nil, &adobe.OperationError{Stage: "token", Err: err}
	}
	identity, err := client.FetchAccountInfo(ctx, result.AccessToken)
	if err != nil {
		return nil, nil, &adobe.OperationError{Stage: "identity", Err: err}
	}
	if identity == nil || !strings.EqualFold(strings.TrimSpace(identity.Email), adobeRecoveryEmail(a)) {
		return nil, nil, &adobe.OperationError{Stage: "identity", Err: errOkadIdentityMismatch}
	}
	balance, err := client.FetchCreditsBalance(ctx, result.AccessToken)
	if err != nil {
		return nil, nil, &adobe.OperationError{Stage: "credits", Err: err}
	}
	return map[string]any{"access_token": result.AccessToken, "expires_at": adobeTokenExpiryString(result)}, balance, nil
}

func okadVerificationStage(err error) string {
	var op *adobe.OperationError
	if errors.As(err, &op) {
		switch op.Stage {
		case "token", "identity", "credits":
			return op.Stage
		}
	}
	return "unknown"
}

func okadAuthVerificationMessage(err error) string {
	switch okadVerificationStage(err) {
	case "token":
		return "Authentication failed (401): Adobe rejected the Cookie during token exchange"
	case "identity":
		return "Authentication failed (401): Adobe rejected the token during identity verification"
	case "credits":
		return "Authentication failed (401): Adobe rejected the token during credits verification"
	default:
		return "Authentication failed (401): Adobe rejected the new cookie"
	}
}

func (s *adminServiceImpl) ApplyOkadCredential(ctx context.Context, id int64, p OkadCredentialUpdate) (*Account, error) {
	if p.Version < 1 || p.Version > 9007199254740991 || len(p.Cookie) > 262144 || strings.TrimSpace(p.Cookie) == "" || len(p.Issuer) > 100 || p.Issuer == "" || len(p.SourceKey) > 100 || p.SourceKey == "" {
		return nil, ErrOkadConflict
	}
	a, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth || !strings.EqualFold(adobeRecoveryEmail(a), strings.TrimSpace(p.Email)) {
		return nil, ErrOkadConflict
	}
	repo, ok := s.accountRepo.(OkadIntegrationRepository)
	if !ok {
		return nil, errors.New("integration storage unavailable")
	}
	applied, err := repo.ApplyOkadCredential(ctx, a, p)
	if err != nil {
		return nil, err
	}
	latest, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !applied {
		state, _ := latest.Extra["okad"].(map[string]any)
		if state["issuer"] != p.Issuer || state["source_key"] != p.SourceKey || okadVersion(state["version"]) != p.Version || latest.GetCredential("cookie") != p.Cookie {
			return nil, ErrOkadConflict
		}
	}
	return latest, nil
}

func (s *adminServiceImpl) ListOkadInventory(ctx context.Context, after int64, limit int, since *time.Time) ([]OkadInventoryItem, error) {
	repo, ok := s.accountRepo.(OkadIntegrationRepository)
	if !ok {
		return nil, errors.New("integration storage unavailable")
	}
	return repo.ListOkadInventory(ctx, after, limit, since)
}

// A fresh IMS exchange and balance request use the account's actual proxy. No
// cached balances, automatic logins or model generation are used for validation.
func (s *AccountUsageService) VerifyOkadCredential(ctx context.Context, a *Account) (*Account, error) {
	if a == nil || a.Platform != PlatformAdobe || a.Type != AccountTypeOAuth {
		return nil, ErrOkadConflict
	}
	repo, ok := s.accountRepo.(OkadIntegrationRepository)
	if !ok {
		return nil, errors.New("integration storage unavailable")
	}
	state, _ := a.Extra["okad"].(map[string]any)
	if state["version"] == nil {
		return nil, ErrOkadConflict
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ready, err := repo.OkadVerificationProxyReady(ctx, a)
	var fields map[string]any
	var balance *adobe.CreditsBalance
	if err == nil && ready {
		fields, balance, err = probeOkadCredential(ctx, a, adobeUsageClients.clientForAccount(a))
	}
	code, message := "verified", "凭据与实时额度已验证；模型出图尚未验证"
	if !ready {
		code, message = "proxy_pending", "等待 Sub2 配置的代理可用，再验证凭据"
	}
	if err == nil && ready {
		if balance == nil || balance.Available == nil || *balance.Available <= 0 {
			code, message = "quota_blocked", "凭据有效，额度不足或未知，暂不恢复调度"
		}
	}
	if err != nil {
		code, message = "verification_retry", "验证暂不可用，将重试验证，不重新登录"
		var auth *adobe.AuthError
		var entitlement *adobe.NotEntitledError
		if errors.As(err, &auth) && auth.StatusCode == 401 {
			code, message = "auth_failed", okadAuthVerificationMessage(err)
		}
		if errors.As(err, &entitlement) {
			code, message = "entitlement_blocked", "凭据已更新，但缺少模型权益"
		}
		if errors.Is(err, errOkadIdentityMismatch) {
			code, message = "identity_mismatch", "Cookie 对应邮箱与关联账号不一致，请核对来源"
		}
		fields = nil
		// Persist only allowlisted stage/result metadata in logs. The upstream
		// body can contain credentials and must not be logged here.
		slog.Warn("okad_credential_verification_failed", "account_id", a.ID, "version", okadVersion(state["version"]), "stage", okadVerificationStage(err), "result", code)
	}
	// Persist even when the HTTP client disconnects, but never apply a stale result.
	persist, cancelPersist := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelPersist()
	applied, saveErr := repo.CompleteOkadVerification(persist, a, fields, code, message)
	if saveErr != nil {
		return nil, saveErr
	}
	if !applied {
		return nil, ErrOkadConflict
	}
	latest, saveErr := s.accountRepo.GetByID(persist, a.ID)
	if saveErr == nil && code == "verified" {
		confirmAdobeCookieRecovery(persist, s.accountRepo, latest)
	}
	return latest, saveErr
}

func okadVersion(value any) int64 { n, _ := toInt64(value); return n }
