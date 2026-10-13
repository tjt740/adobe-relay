package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) GetAdobeHealthReport(ctx context.Context, start, end time.Time, groupID *int64) (*service.AdobeHealthReport, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	report := &service.AdobeHealthReport{GeneratedAt: end, WindowStart: start, Accounts: []service.AdobeHealthAccount{}}
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts a WHERE platform='adobe' AND deleted_at IS NULL
 AND ($1::bigint IS NULL OR EXISTS(SELECT 1 FROM account_groups g WHERE g.account_id=a.id AND g.group_id=$1))`, groupID).Scan(&report.Total)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.name,h.scope,h.status_code,h.reason,h.stage,h.source,h.failed_at,h.success_at,
 h.failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.failed_at) AS pending,
 h.alert_failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.alert_failed_at),h.alert_failed_at
 FROM adobe_account_health h JOIN accounts a ON a.id=h.account_id
 WHERE a.platform='adobe' AND a.deleted_at IS NULL
 AND ($1::bigint IS NULL OR EXISTS(SELECT 1 FROM account_groups g WHERE g.account_id=a.id AND g.group_id=$1))
 ORDER BY pending DESC,h.failed_at DESC NULLS LAST,a.id,h.scope`, groupID)
	if err != nil {
		return nil, err
	}
	pending := map[int64]bool{}
	recent := map[int64]bool{}
	recovered := map[int64]bool{}
	known := map[int64]bool{}
	for rows.Next() {
		var a service.AdobeHealthAccount
		if err := rows.Scan(&a.ID, &a.Name, &a.Scope, &a.Code, &a.Reason, &a.Stage, &a.Source, &a.FailedAt, &a.SuccessAt, &a.Pending, &a.AlertPending, &a.AlertFailedAt); err != nil {
			rows.Close()
			return nil, err
		}
		report.Accounts = append(report.Accounts, a)
		if a.FailedAt != nil || a.SuccessAt != nil {
			known[a.ID] = true
		}
		if a.AlertPending {
			pending[a.ID] = true
			if !a.AlertFailedAt.Before(start) {
				recent[a.ID] = true
			}
		}
		if !a.Pending && a.AlertFailedAt != nil && a.SuccessAt != nil {
			recovered[a.ID] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, a := range report.Accounts {
		if a.Pending {
			delete(recovered, a.ID)
		}
	}
	report.Pending = len(pending)
	report.RecentPending = len(recent)
	report.Recovered = len(recovered)
	report.Unknown = int(report.Total) - len(known)
	if report.Total > 0 {
		report.Ratio = float64(report.Pending) * 100 / float64(report.Total)
	}
	history, err := queryAdobeErrorAccounts(ctx, tx, start, end, groupID, true)
	if err != nil {
		return nil, err
	}
	report.Historical = len(history)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return report, nil
}

func (r *opsRepository) UpdateAdobeAlertReport(ctx context.Context, e *service.OpsAlertEvent) error {
	raw, err := json.Marshal(e.Dimensions)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE ops_alert_events SET title=$2,description=$3,dimensions=$4::jsonb WHERE id=$1 AND status='firing'`, e.ID, e.Title, e.Description, string(raw))
	return err
}
