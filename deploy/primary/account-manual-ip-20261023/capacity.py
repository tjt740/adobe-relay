"""Read-only before/after capacity accounting; counts accounts once, not events."""
import datetime
import importlib.util
import json
from pathlib import Path
import sys
import urllib.request

ROOT = Path('/opt/adobe-relay/releases/account-manual-ip-20261023')


def account_map(snapshot):
    return {row['id']: row for row in snapshot['accounts']}


def compare(before, after):
    old, current = account_map(before), account_map(after)
    assert set(old) <= set(current), 'An original account disappeared; do not restore old data'
    assert all(old[i]['manual_pause'] == current[i]['manual_pause'] for i in old), 'Manual pause changed; review independently'
    assert all(old[i]['intent_hash'] == current[i]['intent_hash'] for i in old), 'Account configuration changed; review independently'
    blocked_bound = {i for i, row in old.items() if row['quarantined'] and row['proxy_id'] is not None}
    released = {i for i in blocked_bound if current[i]['proxy_id'] is None}
    healthy_before = {n['id'] for n in before['allocation']['nodes'] if n['status'] == 'healthy'}
    healthy_after = {n['id'] for n in after['allocation']['nodes'] if n['status'] == 'healthy'}
    old_nodes, new_nodes = {}, {}
    for i, row in old.items():
        if row['proxy_id'] is not None:
            old_nodes.setdefault(row['proxy_id'], set()).add(i)
    for i, row in current.items():
        if row['proxy_id'] is not None:
            new_nodes.setdefault(row['proxy_id'], set()).add(i)
    only_408_nodes = {p for p, ids in old_nodes.items() if ids <= blocked_bound}
    emptied = {p for p in only_408_nodes if not new_nodes.get(p)}
    unresolved_bound = [i for i, row in current.items() if row['quarantined'] and row['proxy_id'] is not None and not row['temporary']]
    assert not unresolved_bound, '408 accounts still have regular bindings: ' + str(unresolved_bound)
    assert all(not row['schedulable'] for row in current.values() if row['quarantined']), 'A quarantined 408 account is schedulable'
    scheduling_changes = [dict(id=i, before_status=old[i]['status'], status=current[i]['status'],
                               before_schedulable=old[i]['schedulable'], schedulable=current[i]['schedulable'],
                               quarantined=current[i]['quarantined'])
                          for i in old if old[i]['status'] != current[i]['status'] or old[i]['schedulable'] != current[i]['schedulable']]
    return {
        'before_at': before['at'], 'after_at': after['at'],
        'original_accounts': len(old), 'current_accounts': len(current),
        'new_account_ids': sorted(set(current) - set(old)),
        'initial_408_bindings': len(blocked_bound),
        'initial_408_bindings_now_released': len(released),
        'released_on_initially_healthy_nodes': sum(old[i]['proxy_id'] in healthy_before for i in released),
        'initial_408_bindings_currently_testing': sum(current[i]['temporary'] for i in blocked_bound),
        'initial_408_bindings_recovered_permanently': sum(not current[i]['quarantined'] and current[i]['proxy_id'] is not None for i in blocked_bound),
        'previously_all_408_nodes_now_empty': len(emptied),
        'previously_all_408_nodes_healthy_before_and_after_now_empty': len(emptied & healthy_before & healthy_after),
        'total_bound_before': sum(r['proxy_id'] is not None for r in old.values()),
        'total_bound_after': sum(r['proxy_id'] is not None for r in current.values()),
        'free_healthy_slots_before': before['allocation']['available_slots'],
        'free_healthy_slots_after': after['allocation']['available_slots'],
        'free_healthy_slots_net_change': after['allocation']['available_slots'] - before['allocation']['available_slots'],
        'healthy_nodes_before': len(healthy_before), 'healthy_nodes_after': len(healthy_after),
        'quarantined_accounts_after': sum(r['quarantined'] and not r['temporary'] for r in current.values()),
        'temporary_test_bindings_after': sum(r['temporary'] for r in current.values()),
        'unresolved_408_regular_bindings_after': 0,
        'manual_pauses_and_account_configuration_preserved': True,
        'scheduling_changes_for_review': scheduling_changes,
        'released_account_ids': sorted(released), 'emptied_node_ids': sorted(emptied),
        'distinct_egress_ips_not_measured': True,
    }


def snapshot(a):
    migrated = a.sql("SELECT to_regclass('adobe_proxy_408_blocks') IS NOT NULL") == 't'
    blocked = "EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id) OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=a.id)" if migrated else "EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=a.id AND h.status_code=408 AND h.failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.failed_at))"
    leased = "EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW())" if migrated else 'false'
    rows = json.loads(a.sql("""SELECT json_agg(json_build_object('id',a.id,'proxy_id',a.proxy_id,
 'quarantined',a.platform='adobe' AND (""" + blocked + "),'temporary'," + leased + """,
 'status',a.status,'schedulable',a.schedulable,'manual_pause',a.extra->'okad_manual_pause',
 'credentials_hash',md5(a.credentials::text),'error_hash',md5(COALESCE(a.error_message,'')),
 'intent_hash',md5(jsonb_build_object('concurrency',a.concurrency,'priority',a.priority,'expires_at',a.expires_at,
 'auto_pause_on_expired',a.auto_pause_on_expired,'platform',a.platform,'type',a.type,
 'model_mapping',a.credentials->'model_mapping','groups',(SELECT jsonb_agg(group_id ORDER BY group_id) FROM account_groups ag WHERE ag.account_id=a.id))::text)) ORDER BY a.id)
 FROM accounts a WHERE a.deleted_at IS NULL"""))
    key = a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
    req = urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/proxy-allocation', headers={'x-api-key': key})
    with urllib.request.urlopen(req, timeout=25) as response:
        allocation = json.load(response)['data']
    return {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'accounts': rows, 'allocation': allocation}


def main():
    action = sys.argv[1]
    assert action in ('before', 'pre-switch', 'after')
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    a = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(a)
    result = snapshot(a)
    if action in ('before', 'pre-switch'):
        filename = 'capacity-before-start.json' if action == 'pre-switch' else 'capacity-before.json'
        assert not (ROOT / filename).exists(), 'Do not overwrite an immutable baseline'
        a.save(filename, result)
        print(json.dumps({'capacity_baseline_saved': True, 'accounts': len(result['accounts']),
                          'bound_408': sum(r['quarantined'] and r['proxy_id'] is not None for r in result['accounts'])}))
    else:
        baseline = ROOT / 'capacity-before-start.json'
        assert baseline.exists(), 'Missing snapshot immediately before candidate startup'
        summary = compare(json.loads(baseline.read_text()), result)
        a.save('capacity-after.json', result)
        a.save('capacity-release.json', summary)
        print(json.dumps({k: v for k, v in summary.items() if k not in ('released_account_ids', 'emptied_node_ids', 'scheduling_changes_for_review')}))


if __name__ == '__main__':
    main()
