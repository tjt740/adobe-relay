package repository

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestManualProxyModeUsesLatestLockedValueUnlessExplicitlyEdited(t *testing.T) {
	for _, explicit := range []*bool{nil, new(false), new(true)} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
		mock.ExpectQuery(`(?s)SELECT.*FOR NO KEY UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"identity", "ollama_group", "ollama_proxy", "enabled", "rate_sync", "snapshot", "session", "auto", "ollama_snapshot", "opencode_group", "opencode_auto", "opencode_snapshot", "extra"}).AddRow(true, false, true, nil, nil, nil, nil, nil, nil, false, nil, nil, []byte(`{"manual_proxy_enabled":true}`)))
		account := &service.Account{ID: 1, Platform: "adobe", Type: "oauth", Extra: map[string]any{"manual_proxy_enabled": false}, ManualProxyEnabledUpdate: explicit}
		extra, err := lockAndMergeAccountProbeExtra(context.Background(), client, account, nil, nil)
		require.NoError(t, err)
		want := true
		if explicit != nil {
			want = *explicit
		}
		require.Equal(t, want, extra["manual_proxy_enabled"])
		require.NoError(t, mock.ExpectationsWereMet())
		mock.ExpectClose()
		require.NoError(t, client.Close())
	}
}

func TestManualProxyBindingPreservesLatestRouteAcrossBackgroundUpdate(t *testing.T) {
	for _, value := range []any{int64(2), nil} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
		mock.ExpectQuery(`SELECT proxy_id FROM accounts WHERE id=\$1`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"proxy_id"}).AddRow(value))
		stale := int64(1)
		account := &service.Account{ID: 1, ProxyID: &stale, Extra: map[string]any{"manual_proxy_enabled": true}}
		require.NoError(t, preserveManualProxyBinding(context.Background(), client, account))
		if value == nil {
			require.Nil(t, account.ProxyID)
		} else {
			require.EqualValues(t, 2, *account.ProxyID)
		}
		// An explicit operator choice still takes effect.
		selected := int64(3)
		account.ProxyID = &selected
		account.ProxyIDExplicitUpdate = true
		require.NoError(t, preserveManualProxyBinding(context.Background(), client, account))
		require.Equal(t, &selected, account.ProxyID)
		require.NoError(t, mock.ExpectationsWereMet())
		mock.ExpectClose()
		require.NoError(t, client.Close())
	}
}

func TestManualProxyDirectRouteRemainsAvailableWithAutomaticAllocationEnabled(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`UPDATE proxy_auto_allocation SET enabled=true WHERE id=1`)
	require.NoError(t, err)
	r := &accountRepository{sql: db}
	account := &service.Account{ID: 99, Extra: map[string]any{"manual_proxy_enabled": true}}
	source, ready, err := r.AdobeTestRoute(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "direct", source)
	require.True(t, ready)
	account.Extra = nil
	_, ready, err = r.AdobeTestRoute(context.Background(), account)
	require.NoError(t, err)
	require.False(t, ready, "automatic accounts still cannot bypass proxy allocation with direct egress")
}
