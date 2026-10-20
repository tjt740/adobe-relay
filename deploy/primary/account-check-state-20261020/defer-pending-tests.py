"""Briefly defer unclaimed tests with bounded transaction locks; never alter data."""
import argparse
import datetime
import fcntl
import json
from pathlib import Path
import subprocess
import threading
import time

ROOT = Path('/opt/adobe-relay/releases/account-check-state-20261020')
PENDING = """SELECT id FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND extra->'okad_connection_test'->>'state' IN ('pending','proxy_pending')
 AND COALESCE(credentials->>'cookie','')<>''
 AND extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(credentials->>'cookie','UTF8')),'hex')
 AND (extra->'okad_connection_test'->>'require_verification'='false' OR extra->'okad'->>'verification'='verified')
 AND (proxy_id IS NOT NULL OR NOT EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled))
 ORDER BY id FOR UPDATE SKIP LOCKED"""


def exchange(process, command):
    process.stdin.write(command + ";\n\\echo gate-ready\n")
    process.stdin.flush()
    rows = []
    # readline may already have buffered output, so use a separate psql-side
    # statement timeout plus a parent watchdog rather than select per line.
    while True:
        line = process.stdout.readline()
        assert line, 'Queue lock connection closed; all transaction locks released'
        if line.strip() == 'gate-ready':
            return rows
        rows.append(line.strip())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('phase', choices=['old', 'canary'])
    parser.add_argument('--attempt', type=int, choices=[1, 2], default=1)
    args = parser.parse_args()
    target = ROOT / ('rolling-route-canonical.json' if args.phase == 'old' else 'rolling-finished.json')
    assert not target.exists(), 'Phase already complete'
    state = json.loads((ROOT / 'progress.json').read_text())
    assert state['step'] == 'drain-' + args.phase and state['state'] == 'running'
    progress = json.loads((ROOT / ('rolling-' + args.phase + '-drain-progress.json')).read_text())['samples'][-1]
    assert progress['elapsed_seconds'] >= 210 and progress['remaining_tracked_slots'] == 0 and progress['pending_input_sockets'] == 0
    lease = subprocess.check_output(['docker', 'exec', 'adobe-relay-redis-1', 'redis-cli', '--raw', 'GET', 'concurrency:rolling_deployment'], text=True).strip()
    assert lease == 'account-check-state-20261020'
    lock = open(ROOT / 'defer-pending-tests.lock', 'a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    suffix = '' if args.attempt == 1 else '-retry'
    output = ROOT / ('deferred-' + args.phase + suffix + '.json')
    assert not output.exists()
    started = time.monotonic()
    result = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'phase': args.phase, 'attempt': args.attempt, 'persistent_data_updates': False, 'interrupted_running_tests': False}
    process = subprocess.Popen(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'psql', '-U', 'sub2api', '-d', 'sub2api', '-Atq', '-v', 'ON_ERROR_STOP=1'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    # Even if this script is killed, the database releases locks within 5s.
    # A running generation has a 180s outer timeout; keep enough room for that
    # completion plus the unchanged 45s stable-zero guard and cleanup.
    watchdog = threading.Timer(300, process.kill)
    watchdog.daemon = True
    watchdog.start()
    peak = 0
    try:
        exchange(process, "BEGIN; SET LOCAL statement_timeout='2s'; SET LOCAL lock_timeout='100ms'; SET LOCAL idle_in_transaction_session_timeout='5s'")
        while not target.exists() and time.monotonic() - started < 295:
            peak = max(peak, len(exchange(process, PENDING)))
            time.sleep(0.1)
        result['target_reached'] = target.exists()
    finally:
        try:
            exchange(process, 'ROLLBACK')
        finally:
            process.stdin.close()
            process.wait(timeout=5)
            watchdog.cancel()
        result.update(seconds=round(time.monotonic() - started, 2), peak_pending_locked=peak, locks_released=True)
        output.write_text(json.dumps(result, indent=2))
        output.chmod(0o600)
        print(json.dumps(result), flush=True)
    assert result.get('target_reached'), 'Deferral expired safely; continue waiting for active tests'


if __name__ == '__main__':
    main()
