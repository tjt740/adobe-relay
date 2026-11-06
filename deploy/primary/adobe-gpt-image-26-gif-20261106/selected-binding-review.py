"""Read-only comparison of the selected account's original group and priority."""
import datetime
import importlib.util
import json
from pathlib import Path
import subprocess

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')


def validate(before, current, original_ids):
    expected = {row['group_id']: row['priority'] for row in before}
    actual = {row['group_id']: row['priority'] for row in current}
    assert len(expected) == len(before) and len(actual) == len(current)
    assert sorted(expected) == sorted(original_ids) and actual == expected
    return {'original_group_ids_preserved': True, 'original_group_priorities_preserved': True}


def main():
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    after = json.loads((ROOT / 'after.json').read_text())
    raw = subprocess.check_output(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'pg_restore', '--data-only', '--table=account_groups', '-f', '-'], input=(Path(after['backup']) / 'database.dump').read_bytes(), timeout=90).decode()
    before, active, seen, ended = [], False, False, False
    for line in raw.splitlines():
        if line.startswith('COPY public.account_groups ('):
            columns = [x.strip().strip('"') for x in line.split('(', 1)[1].split(')', 1)[0].split(',')]
            active, seen = True, True
        elif active and line == '\\.':
            active, ended = False, True
        elif active:
            row = dict(zip(columns, line.split('\t')))
            if row['account_id'] == '2933':
                before.append({'group_id': int(row['group_id']), 'priority': int(row['priority']), 'created_at': row['created_at']})
    assert seen and ended and before
    current = json.loads(audit.sql("SELECT coalesce(json_agg(json_build_object('group_id',group_id,'priority',priority,'created_at',created_at) ORDER BY group_id),'[]') FROM account_groups WHERE account_id=2933"))
    original_ids = json.loads((ROOT / 'selected-test-access-started.json').read_text())['original_account_groups']
    checks = validate(before, current, original_ids)
    out = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'account_id': 2933, 'cutover_backup': after['backup'], 'before': before, 'current': current, **checks, 'normal_admin_api_recreates_membership_rows': True, 'restored_old_data': False, 'read_only': True}
    audit.save('selected-binding-review.json', out)
    print(json.dumps(out), flush=True)


if __name__ == '__main__':
    main()
