"""Temporarily pause the authorized auto-push switch, then restore its exact row."""
import datetime
import fcntl
import importlib.util
import json
from pathlib import Path, PurePosixPath
import sqlite3
import sys
import time

ROOT = Path('/opt/adobe-relay/releases/account-check-state-20261020')
KEY = next((v for v in sys.argv[1:] if v in ['sub2_auto_push', 'sub2_reliable_enabled']), 'sub2_auto_push')
PREFIX = 'auto-push' if KEY == 'sub2_auto_push' else 'reliable-push'
spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)


def db_path():
    container = a.inspect('adobe-team-cookie-app-1')
    url = next(value.split('=', 1)[1] for value in container['Config']['Env'] if value.startswith('DATABASE_URL='))
    assert url.startswith('sqlite:///')
    db = PurePosixPath(url[len('sqlite:///'):])
    mount = next(m for m in sorted(container['Mounts'], key=lambda m: len(m['Destination']), reverse=True) if db.is_relative_to(m['Destination']))
    return Path(mount['Source']) / str(db.relative_to(mount['Destination']))


def locks():
    held = []
    for path in ['/opt/adobe-relay/.release.lock', '/opt/adobe-team-cookie/.release.lock', '/opt/adobe-team-cookie/.incremental-release.lock']:
        f = open(path, 'a')
        fcntl.flock(f, fcntl.LOCK_EX | fcntl.LOCK_NB)
        held.append(f)
    return held


def restore():
    held = locks()
    proof = json.loads((ROOT / (PREFIX + '-paused.json')).read_text())
    with sqlite3.connect(db_path(), timeout=5) as db:
        db.row_factory = sqlite3.Row
        db.execute('BEGIN IMMEDIATE')
        row = dict(db.execute("SELECT * FROM settings WHERE key=?", (KEY,)).fetchone())
        assert row == proof['paused'], 'The user changed auto-push; preserve their change and review'
        # Only this value was changed. Timestamp and all other columns were left intact.
        assert db.execute("UPDATE settings SET value=? WHERE key=? AND value='false'", (proof['before']['value'], KEY)).rowcount == 1
        restored = dict(db.execute("SELECT * FROM settings WHERE key=?", (KEY,)).fetchone())
        assert restored == proof['before']
    a.save(PREFIX + '-restored.json', {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'row_restored_exactly': True, 'value': restored['value']})
    print('Authorized auto-push pause ended; original row restored exactly', flush=True)


if 'restore' in sys.argv[1:]:
    restore()
else:
    held = locks()
    assert not (ROOT / (PREFIX + '-paused.json')).exists()
    assert a.inspect(a.APP)['Config']['Image'] == 'adobe-relay:' + (ROOT / 'expected-sha').read_text().strip()
    assert json.loads((ROOT / 'progress.json').read_text())['step'] == 'drain-canary'
    with sqlite3.connect(db_path(), timeout=5) as db:
        db.row_factory = sqlite3.Row
        db.execute('BEGIN IMMEDIATE')
        before = dict(db.execute("SELECT * FROM settings WHERE key=?", (KEY,)).fetchone())
        assert before['value'] == 'true'
        assert db.execute("UPDATE settings SET value='false' WHERE key=? AND value='true'", (KEY,)).rowcount == 1
        paused = dict(db.execute("SELECT * FROM settings WHERE key=?", (KEY,)).fetchone())
        a.save(PREFIX + '-paused.json', {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'before': before, 'paused': paused, 'scope': KEY + ' value only; user authorized short pause'})
    for f in held:
        f.close()
    print('Auto-push temporarily paused; original settings row saved for exact restoration', flush=True)
    end = time.monotonic() + 600
    # Keep a bounded, recoverable pause even if final verification needs review.
    while not (ROOT / 'rolling-finished.json').exists() and time.monotonic() < end:
        time.sleep(1)
    while True:
        try:
            restore()
            break
        except BlockingIOError:
            time.sleep(0.1)
