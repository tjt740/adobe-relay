package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Persist the token and rotated session together. A stale response cannot
// overwrite a newer runtime session, imported Cookie, or proxy assignment.
func (r *accountRepository) UpdateAdobeSessionIfUnchanged(ctx context.Context, id int64, before service.AdobeSessionSnapshot, fields map[string]any) (bool, error) {
	if r == nil || r.sql == nil || before.Cookie == "" {
		return false, fmt.Errorf("adobe session update requires a source cookie and repository")
	}
	allowed := map[string]any{}
	for _, key := range []string{"access_token", "expires_at", "_token_version", "adobe_session_cookie", "adobe_session_cookie_source_hash"} {
		if v, ok := fields[key]; ok {
			allowed[key] = v
		}
	}
	raw, err := json.Marshal(allowed)
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `WITH updated AS (
 UPDATE accounts a SET credentials=COALESCE(credentials,'{}'::jsonb)||$1::jsonb,updated_at=NOW()
 WHERE id=$2 AND deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND credentials->>'cookie'=$3
 AND COALESCE(credentials->>'adobe_session_cookie','')=$4
 AND COALESCE(credentials->>'adobe_session_cookie_source_hash','')=$5
 AND proxy_id IS NOT DISTINCT FROM $6
 AND ($7::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies p WHERE p.id=a.proxy_id AND p.updated_at=$7))
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $8,id,NULL,NULL FROM updated`, string(raw), id, before.Cookie, before.SessionCookie, before.SourceHash, before.ProxyID, before.ProxyUpdatedAt, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, id)
	}
	return n > 0, err
}
