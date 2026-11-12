package clash

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrRelatedProxyNotFound = errors.New("proxy not found")
var ErrRelatedAccountsQuery = errors.New("invalid related accounts query")

type RelatedAccountsQuery struct {
	Page     int
	PageSize int
	Relation string
	Search   string
}

type RelatedProxyRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ProxyDeparture struct {
	At          time.Time        `json:"at"`
	Destination *RelatedProxyRef `json:"destination"`
}

type ProxyRelatedAccount struct {
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	Platform      string           `json:"platform"`
	UsingThisNode bool             `json:"using_this_node"`
	Mode          string           `json:"mode"`
	AccountState  string           `json:"account_state"`
	RouteState    string           `json:"route_state"`
	Impact        string           `json:"impact"`
	CurrentProxy  *RelatedProxyRef `json:"current_proxy"`
	LastDeparture *ProxyDeparture  `json:"last_departure"`
}

type ProxyRelatedAccountsView struct {
	Proxy         RelatedProxyRef       `json:"proxy"`
	CheckedAt     time.Time             `json:"checked_at"`
	HistorySince  time.Time             `json:"history_since"`
	CurrentCount  int                   `json:"current_count"`
	DepartedCount int                   `json:"departed_count"`
	Total         int                   `json:"total"`
	Page          int                   `json:"page"`
	PageSize      int                   `json:"page_size"`
	Items         []ProxyRelatedAccount `json:"items"`
}

// Use the latest recorded departure per account, not the latest destination
// as a substitute for its current binding. Accounts can move again or return.
const relatedAccountsCTE = `WITH departures AS (
 SELECT DISTINCT ON(account_id) account_id,to_proxy_id,created_at
 FROM proxy_failover_events
 WHERE from_proxy_id=$1 AND to_proxy_id<>from_proxy_id AND created_at>=$2 AND created_at<=$3
 ORDER BY account_id,id DESC
), related AS (
 SELECT a.id,a.name,a.proxy_id,d.to_proxy_id,d.created_at,COALESCE(a.proxy_id=$1,false) AS is_current
 FROM accounts a LEFT JOIN departures d ON d.account_id=a.id
 WHERE a.deleted_at IS NULL AND (a.proxy_id=$1 OR d.account_id IS NOT NULL)
) `
const relatedAccountsFilter = `($4='all' OR ($4='current' AND r.is_current) OR ($4='departed' AND NOT r.is_current))
 AND ($5='' OR strpos(lower(r.name),lower($5))>0 OR strpos(r.id::text,$5)>0)`

// RelatedAccounts reads one consistent snapshot. It does not probe, switch,
// release an exclusion, or return credentials, notes or raw upstream errors.
func (m *Manager) RelatedAccounts(ctx context.Context, proxyID int64, q RelatedAccountsQuery) (ProxyRelatedAccountsView, error) {
	v := ProxyRelatedAccountsView{Page: q.Page, PageSize: q.PageSize, Items: []ProxyRelatedAccount{}}
	if proxyID <= 0 || q.Page < 1 || q.Page > 100000 || q.PageSize < 1 || q.PageSize > 50 ||
		(q.Relation != "all" && q.Relation != "current" && q.Relation != "departed") || len(q.Search) > 200 {
		return v, ErrRelatedAccountsQuery
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT id,name FROM proxies WHERE id=$1 AND deleted_at IS NULL`, proxyID).Scan(&v.Proxy.ID, &v.Proxy.Name)
	if err == sql.ErrNoRows {
		return v, ErrRelatedProxyNotFound
	}
	if err != nil {
		return v, err
	}
	var automatic bool
	var proxyCount int
	err = tx.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP,COALESCE((SELECT enabled FROM proxy_auto_allocation WHERE id=1),false),(SELECT count(*) FROM proxies WHERE deleted_at IS NULL)`).Scan(&v.CheckedAt, &automatic, &proxyCount)
	if err != nil {
		return v, err
	}
	v.HistorySince = v.CheckedAt.Add(-24 * time.Hour)
	args := []any{proxyID, v.HistorySince, v.CheckedAt, q.Relation, strings.TrimSpace(q.Search)}
	err = tx.QueryRowContext(ctx, relatedAccountsCTE+`SELECT count(*) FILTER(WHERE r.is_current),count(*) FILTER(WHERE NOT r.is_current),
 count(*) FILTER(WHERE `+relatedAccountsFilter+`) FROM related r`, args...).Scan(&v.CurrentCount, &v.DepartedCount, &v.Total)
	if err != nil {
		return v, err
	}
	sources, err := LoadProxySources(ctx, tx)
	if err != nil {
		return v, err
	}
	rows, err := tx.QueryContext(ctx, relatedAccountsCTE+`SELECT a.id,a.name,a.platform,r.is_current,
 COALESCE(a.extra->'manual_proxy_enabled'='true'::jsonb,false),COALESCE(f.enabled,false),
 a.status,a.schedulable,a.proxy_auto_paused,
 COALESCE(a.extra->'okad_manual_pause'='true'::jsonb,false),
 COALESCE(a.auto_pause_on_expired AND a.expires_at<=$3,false),
 COALESCE(a.temp_unschedulable_until>$3,false) OR COALESCE(a.overload_until>$3,false) OR COALESCE(a.rate_limit_reset_at>$3,false),
 a.proxy_id,COALESCE(p.name,''),COALESCE(p.deleted_at IS NULL AND p.status='active' AND (p.expires_at IS NULL OR p.expires_at>$3),false),
 EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=a.proxy_id),
 EXISTS(SELECT 1 FROM adobe_route_health g WHERE g.proxy_id=a.proxy_id AND g.proxy_updated_at=p.updated_at AND g.retry_after>$3),
 COALESCE(h.status,''),COALESCE(h.failures,0),h.checked_at,p.updated_at,
 r.created_at,r.to_proxy_id,COALESCE(destination.name,'')
 FROM related r JOIN accounts a ON a.id=r.id
 LEFT JOIN proxies p ON p.id=a.proxy_id
 LEFT JOIN proxy_failover_health h ON h.proxy_id=a.proxy_id
 LEFT JOIN account_proxy_failover f ON f.account_id=a.id
 LEFT JOIN proxies destination ON destination.id=r.to_proxy_id
 WHERE `+relatedAccountsFilter+` ORDER BY r.is_current DESC,a.id LIMIT $6 OFFSET $7`,
		append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var item ProxyRelatedAccount
		var evidence relatedAccountEvidence
		var pid, destinationID sql.NullInt64
		var currentName, destinationName string
		var departed, probeAt, revision sql.NullTime
		err = rows.Scan(&item.ID, &item.Name, &item.Platform, &item.UsingThisNode,
			&evidence.manual, &evidence.failover, &evidence.status, &evidence.schedulable, &evidence.waiting,
			&evidence.manuallyPaused, &evidence.expired, &evidence.accountCooldown,
			&pid, &currentName, &evidence.proxyActive, &evidence.excluded, &evidence.routeCooldown,
			&evidence.probeStatus, &evidence.probeFailures, &probeAt, &revision,
			&departed, &destinationID, &destinationName)
		if err != nil {
			rows.Close()
			return v, err
		}
		if pid.Valid {
			item.CurrentProxy = &RelatedProxyRef{ID: pid.Int64, Name: currentName}
			evidence.sourceBlocked = sources[pid.Int64].Blocked
		}
		if departed.Valid {
			item.LastDeparture = &ProxyDeparture{At: departed.Time}
			if destinationID.Valid && destinationID.Int64 > 0 {
				item.LastDeparture.Destination = &RelatedProxyRef{ID: destinationID.Int64, Name: destinationName}
			}
		}
		evidence.automatic = automatic
		evidence.probeFresh = probeAt.Valid && revision.Valid && !probeAt.Time.Before(revision.Time) &&
			!probeAt.Time.After(v.CheckedAt) && v.CheckedAt.Sub(probeAt.Time) <= ProbeFreshness(proxyCount)
		describeRelatedAccount(&item, evidence)
		v.Items = append(v.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}

type relatedAccountEvidence struct {
	manual, automatic, failover, schedulable, waiting, manuallyPaused, expired, accountCooldown bool
	proxyActive, excluded, routeCooldown, sourceBlocked, probeFresh                             bool
	status, probeStatus                                                                         string
	probeFailures                                                                               int
}

func describeRelatedAccount(item *ProxyRelatedAccount, e relatedAccountEvidence) {
	switch {
	case e.manual && item.CurrentProxy == nil:
		item.Mode = "manual_direct"
	case e.manual:
		item.Mode = "manual"
	case e.automatic:
		item.Mode = "automatic"
	case e.failover:
		item.Mode = "failover"
	default:
		item.Mode = "unmanaged"
	}
	switch {
	case e.waiting:
		item.AccountState = "waiting_proxy"
	case e.manuallyPaused:
		item.AccountState = "paused"
	case e.status == "disabled" || e.status == "inactive":
		item.AccountState = "disabled"
	case e.status != "active":
		item.AccountState = "error"
	case e.expired:
		item.AccountState = "expired"
	case !e.schedulable:
		item.AccountState = "paused"
	case e.accountCooldown:
		item.AccountState = "cooldown"
	default:
		item.AccountState = "eligible"
	}
	switch {
	case item.CurrentProxy == nil && (e.waiting || (e.automatic && !e.manual)):
		item.RouteState = "waiting_proxy"
	case item.CurrentProxy == nil:
		item.RouteState = "direct"
	case !e.proxyActive:
		item.RouteState = "unavailable"
	case e.sourceBlocked:
		item.RouteState = "subscription_blocked"
	case item.Platform == "adobe" && (e.excluded || e.routeCooldown):
		item.RouteState = "generation_paused"
	case !e.probeFresh:
		item.RouteState = "unconfirmed"
	case e.probeStatus == "unhealthy" && e.probeFailures >= 2:
		item.RouteState = "unavailable"
	case e.probeStatus == "healthy":
		item.RouteState = "reachable"
	default:
		item.RouteState = "unconfirmed"
	}
	switch {
	case !item.UsingThisNode:
		item.Impact = "left_node"
	case item.RouteState == "generation_paused" || item.RouteState == "subscription_blocked" || item.RouteState == "unavailable":
		item.Impact = "affected"
	case item.RouteState == "unconfirmed":
		item.Impact = "unconfirmed"
	default:
		item.Impact = "not_blocked"
	}
}
