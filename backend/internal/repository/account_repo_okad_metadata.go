package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) SyncOkadAccountMetadata(ctx context.Context, items []service.OkadAccountMetadataUpdate) (int, error) {
	raw, err := json.Marshal(items)
	if err != nil {
		return 0, err
	}
	// One atomic, metadata-only update. Match the current identity/owner in SQL
	// so a concurrent relink cannot tag a different account. Legacy imports must
	// already have Okad's import marker; matching an email alone is insufficient.
	rows, err := r.sql.QueryContext(ctx, `WITH incoming AS (
 SELECT (p->>'id')::bigint AS id,p-'id' AS metadata FROM jsonb_array_elements($1::jsonb) p
 ) UPDATE accounts a SET extra=COALESCE(a.extra,'{}'::jsonb)||jsonb_build_object('okad_account',i.metadata),updated_at=NOW()
 FROM incoming i WHERE a.id=i.id AND a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth'
 AND LOWER(BTRIM(COALESCE(NULLIF(BTRIM(a.credentials->>'email'),''),a.extra->>'adobeteam_external_email','')))=i.metadata->>'email'
 AND (COALESCE(a.extra->>'adobeteam_external_email','')='' OR LOWER(BTRIM(a.extra->>'adobeteam_external_email'))=i.metadata->>'email')
 AND ((a.extra->'okad'->>'issuer'=i.metadata->>'issuer' AND a.extra->'okad'->>'source_key'=i.metadata->>'source_key')
   OR (COALESCE(a.extra->'okad'->>'issuer','')='' AND LOWER(BTRIM(a.extra->>'adobeteam_external_email'))=i.metadata->>'email'))
 AND (COALESCE(a.extra->'okad'->>'issuer','')<>'' OR COALESCE(a.extra->'okad_account'->>'issuer','')='' OR
   (a.extra->'okad_account'->>'issuer'=i.metadata->>'issuer' AND a.extra->'okad_account'->>'source_key'=i.metadata->>'source_key'))
 AND a.extra->'okad_account' IS DISTINCT FROM i.metadata RETURNING a.id`, string(raw))
	if err != nil {
		return 0, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		r.syncSchedulerAccountSnapshotDetached(ctx, id)
	}
	return len(ids), nil
}
