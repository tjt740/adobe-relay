"""Compare with the cutover backup; never persist credential values or restore data."""
import datetime
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path('/opt/adobe-relay/releases/adobe-default-test-flare-20261026')
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
after = json.loads((ROOT / 'after.json').read_text())
dump = Path(after['backup']) / 'database.dump'
raw = subprocess.check_output(
    ['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'pg_restore', '--data-only', '--table=accounts', '-f', '-'],
    input=dump.read_bytes(), timeout=90).decode()
escapes = {'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', '\\': '\\'}


def value(s):
    if s == '\\N':
        return None
    return re.sub(r'\\(.)', lambda m: escapes.get(m[1], m[1]), s)


old = {}
active = False
for line in raw.splitlines():
    if line.startswith('COPY public.accounts ('):
        columns = [x.strip().strip('"') for x in line.split('(', 1)[1].split(')', 1)[0].split(',')]
        active = True
        continue
    if active and line == '\\.':
        active = False
        continue
    if active:
        row = dict(zip(columns, map(value, line.split('\t'))))
        if row['deleted_at'] is None:
            old[int(row['id'])] = {
                'id': int(row['id']), 'credentials': json.loads(row['credentials']),
                'proxy_id': int(row['proxy_id']) if row['proxy_id'] else None,
                'status': row['status'], 'schedulable': row['schedulable'] == 't',
                'manual_pause': (json.loads(row['extra']) if row['extra'] else {}).get('okad_manual_pause'),
                'manual_proxy_enabled': (json.loads(row['extra']) if row['extra'] else {}).get('manual_proxy_enabled'),
            }
assert old, 'No active accounts were read from the verified cutover dump'
rows = json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('id',id,'credentials',credentials,'proxy_id',proxy_id,'status',status,'schedulable',schedulable,'manual_pause',extra->'okad_manual_pause','manual_proxy_enabled',extra->'manual_proxy_enabled') ORDER BY id),'[]') FROM accounts WHERE deleted_at IS NULL"))
current = {row['id']: row for row in rows}
cutover_at = datetime.datetime.strptime(dump.parent.name.split('-', 1)[0], '%Y%m%dT%H%M%SZ').replace(tzinfo=datetime.timezone.utc).isoformat()
events = json.loads(a.sql("SELECT coalesce(json_agg(e ORDER BY id),'[]') FROM (SELECT id,account_id,from_proxy_id,to_proxy_id,created_at FROM proxy_failover_events WHERE created_at >= '" + cutover_at + "'::timestamptz) e"))
changes = []
for account_id in sorted(set(old) & set(current)):
    previous, latest = old[account_id], current[account_id]
    fields = [key for key in previous if previous[key] != latest[key]]
    if not fields:
        continue
    cursor = previous['proxy_id'] or 0
    matched = []
    for event in events:
        if cursor == (latest['proxy_id'] or 0):
            break
        if event['account_id'] == account_id and (event['from_proxy_id'] or 0) == cursor:
            cursor = event['to_proxy_id'] or 0
            matched.append(event['id'])
    changes.append({
        'id': account_id, 'fields': fields,
        'credential_keys_changed': sorted(key for key in set(previous['credentials']) | set(latest['credentials']) if previous['credentials'].get(key) != latest['credentials'].get(key)),
        'proxy_chain_matches': cursor == (latest['proxy_id'] or 0), 'proxy_event_ids': matched,
    })
result = {
    'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'cutover_backup': str(dump.parent), 'proxy_events_since': cutover_at, 'before_count': len(old), 'current_count': len(current),
    'missing_ids': sorted(set(old) - set(current)), 'new_ids': sorted(set(current) - set(old)),
    'manual_pause_preserved': all(old[i]['manual_pause'] == current[i]['manual_pause'] for i in old if i in current),
    'manual_proxy_modes_preserved': all(old[i]['manual_proxy_enabled'] == current[i]['manual_proxy_enabled'] for i in old if i in current),
    'changes': changes, 'restored_old_business_data': False,
}
reviewed = '--reviewed' in sys.argv
if reviewed:
    from deletion_guard import validate
    import runpy
    evidence = runpy.run_path(str(ROOT/'review-deletions.py'))['collect'](a)
    result['reviewed_admin_deletions'] = validate(result, evidence, json.loads((ROOT/'reviewed-deletion-scope.json').read_text()))
    a.save('deletion-evidence-reviewed.json', evidence)
a.save('business-verification-reviewed.json' if reviewed else 'business-verification.json', result)
print(json.dumps(result, ensure_ascii=False))
assert (reviewed or not result['missing_ids']) and result['manual_pause_preserved'] and result['manual_proxy_modes_preserved'], 'Review account changes; never restore a stale database'
assert all(item['proxy_chain_matches'] for item in changes), 'Review unmatched live proxy changes'
