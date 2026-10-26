"""Read the excluded route's actual cutover state without changing any proxy."""
import datetime
import importlib.util
import json
from pathlib import Path
import subprocess

ROOT = Path('/opt/adobe-relay/releases/account-bulk-test-model-20261026')
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)
backup = Path(json.loads((ROOT / 'after.json').read_text())['backup'])

def backup_rows(table):
    raw = subprocess.check_output(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'pg_restore', '--data-only', '--table=' + table, '-f', '-'], input=(backup / 'database.dump').read_bytes(), timeout=90).decode()
    columns = None
    rows = []
    for line in raw.splitlines():
        if line.startswith('COPY public.' + table + ' ('):
            columns = [part.strip().strip('"') for part in line.split('(', 1)[1].split(')', 1)[0].split(',')]
        elif line == '\\.':
            columns = None
        elif columns is not None:
            rows.append(dict(zip(columns, line.split('\t'))))
    return rows

proxy = next(row for row in backup_rows('proxies') if row['id'] == '1290')
excluded_before = any(row['proxy_id'] == '1290' for row in backup_rows('adobe_route_exclusions'))
current = json.loads(a.sql("SELECT json_build_object('excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290),'status',(SELECT status FROM proxies WHERE id=1290),'bindings',(SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=1290),'leases',(SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=1290))"))
out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'cutover_backup': str(backup), 'proxy_id': 1290, 'backup_status': proxy['status'], 'backup_excluded': excluded_before, 'current': current, 'read_only': True, 'restored_old_data': False}
if not (ROOT / 'route-guard-review.json').exists():
    a.save('route-guard-review.json', out)
print(json.dumps(out))
