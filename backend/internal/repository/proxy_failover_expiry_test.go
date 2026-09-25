package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestExpiryRespectsLiveFailoverPolicy(t *testing.T) {
	dsn := os.Getenv("PROXY_FAILOVER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PROXY_FAILOVER_TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer root.Close()
	schema := fmt.Sprintf("expiry_failover_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	defer root.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE proxies(id bigint PRIMARY KEY,status text,updated_at timestamptz,deleted_at timestamptz);
 CREATE TABLE accounts(id bigint PRIMARY KEY,proxy_id bigint,proxy_fallback_origin_id bigint,type text,extra jsonb,updated_at timestamptz,deleted_at timestamptz);
 CREATE TABLE account_proxy_failover(account_id bigint PRIMARY KEY,enabled bool);
 INSERT INTO proxies(id,status) VALUES(1,'active'),(2,'active');
 INSERT INTO accounts(id,proxy_id,type,extra) VALUES(1,1,'oauth','{}'),(2,1,'oauth','{}'),(3,1,'oauth','{}');
 INSERT INTO account_proxy_failover(account_id,enabled) VALUES(1,true),(2,false);`)
	require.NoError(t, err)
	repo := &proxyRepository{}
	for _, target := range []*int64{nil, new(int64(2))} {
		_, err = db.Exec("UPDATE accounts SET proxy_id=1")
		require.NoError(t, err)
		changed, err := repo.sweepOneExpiredProxyOnExec(context.Background(), db, 1, target, true)
		require.NoError(t, err)
		require.ElementsMatch(t, []int64{2, 3}, changed)
		var current int64
		require.NoError(t, db.QueryRow("SELECT proxy_id FROM accounts WHERE id=1").Scan(&current))
		require.EqualValues(t, 1, current, "enabled health policy owns routing, including when all backups fail")
	}
}
