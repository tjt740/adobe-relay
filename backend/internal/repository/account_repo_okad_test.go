package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRecoverAdobeCookieUsesConditionalMergeAndAtomicOutbox(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows int64
		err  error
	}{
		{"recovered", 1, nil}, {"concurrent edit wins", 0, nil}, {"outbox failed", 0, errors.New("outbox insert failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &accountRepository{sql: db}
			proxy := int64(7)
			a := &service.Account{ID: 233, Status: service.StatusError, ErrorMessage: "OAuth 401 (no refresh_token): Invalid bearer token", ProxyID: &proxy, Credentials: map[string]any{"cookie": "old", "email": "fixture@example.test"}}
			e := mock.ExpectExec(`(?s)WITH updated AS.*credentials = .* - 'access_token' - 'expires_at' - '_token_version'.*a.status = \$3.*error_message.*\$4.*a.credentials->>'cookie' = \$5.*a.credentials->>'email'.*\$6.*a.schedulable = \$7.*a.proxy_id IS NOT DISTINCT FROM \$8.*adobeteam_external_email.*\$9.*INSERT INTO scheduler_outbox.*SELECT \$10`).WithArgs("fresh", int64(233), a.Status, a.ErrorMessage, "old", "fixture@example.test", false, proxy, "", service.SchedulerOutboxEventAccountChanged)
			if tc.err != nil {
				e.WillReturnError(tc.err)
			} else {
				e.WillReturnResult(sqlmock.NewResult(0, tc.rows))
			}
			applied, err := repo.RecoverAdobeCookieIfUnchanged(context.Background(), a, "fresh")
			require.Equal(t, tc.rows == 1, applied)
			if tc.err != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
