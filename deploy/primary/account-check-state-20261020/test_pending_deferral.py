"""Exercise real PostgreSQL queue contention in an isolated, disposable schema."""
import importlib.util
import os
from pathlib import Path
import subprocess
import uuid

spec = importlib.util.spec_from_file_location('defer', Path(__file__).with_name('defer-pending-tests.py'))
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
dsn = os.environ['OKAD_RECOVERY_TEST_DATABASE_URL']
schema = 'gate_' + uuid.uuid4().hex


def sql(query):
    return subprocess.check_output(['psql', dsn, '-Atq', '-v', 'ON_ERROR_STOP=1'], input='SET search_path=' + schema + ';' + query, text=True).strip()


process = None
try:
    sql('CREATE SCHEMA ' + schema + ''';
CREATE TABLE accounts(id bigint PRIMARY KEY,deleted_at timestamptz,platform text,type text,extra jsonb,credentials jsonb,proxy_id bigint);
CREATE TABLE proxy_auto_allocation(enabled bool);
INSERT INTO accounts(id,platform,type,extra,credentials,proxy_id)
SELECT n,'adobe','oauth',jsonb_build_object('okad_connection_test',jsonb_build_object('state',CASE WHEN n=2 THEN 'running' ELSE 'pending' END,'require_verification',n=3,'key',encode(sha256(convert_to('cookie','UTF8')),'hex'))),'{"cookie":"cookie"}',7 FROM generate_series(1,3) n;''')
    original = sql('SELECT to_jsonb(a) FROM accounts a WHERE id=1')
    process = subprocess.Popen(['psql', dsn, '-Atq', '-v', 'ON_ERROR_STOP=1'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    m.exchange(process, 'BEGIN; SET LOCAL search_path=' + schema + "; SET LOCAL statement_timeout='2s'; SET LOCAL idle_in_transaction_session_timeout='5s'")
    assert m.exchange(process, m.PENDING) == ['1']
    assert sql(m.PENDING) == '', 'Claim workers must skip deferred pending rows'
    sql("SET lock_timeout='200ms'; UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,state}','\"success\"') WHERE id=2")
    assert sql("SELECT extra->'okad_connection_test'->>'state' FROM accounts WHERE id=2") == 'success', 'Running tests must finish without blocking'
    sql('INSERT INTO accounts SELECT 4,deleted_at,platform,type,extra,credentials,proxy_id FROM accounts WHERE id=1')
    assert m.exchange(process, m.PENDING) == ['1', '4'], 'New pending jobs are retained and deferred'
    assert sql(m.PENDING) == ''
    m.exchange(process, 'ROLLBACK')
    assert sql(m.PENDING).splitlines() == ['1', '4'], 'Pending jobs remain claimable after release'
    assert sql('SELECT to_jsonb(a) FROM accounts a WHERE id=1') == original, 'No persistent account field may change'
    print('Pending deferral SQL passed: skip locked, running completion, new arrivals, rollback, unchanged data')
finally:
    if process:
        process.stdin.close()
        process.wait(timeout=5)
    sql('DROP SCHEMA ' + schema + ' CASCADE')
