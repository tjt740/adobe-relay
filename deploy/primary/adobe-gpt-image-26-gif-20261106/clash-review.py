"""Observe a concurrent Clash change without exposing or restoring credentials."""
import base64
import datetime
import hashlib
import importlib.util
import json
from pathlib import Path
from collections import Counter
import re
import subprocess

ROOT = Path('/opt/adobe-relay/releases/adobe-gpt-image-26-gif-20261106')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def contract_expression(revision=True):
    return "md5(jsonb_build_object('url',document->'url'," + ("'revision',document->'revision'," if revision else '') + "'nodes',(SELECT coalesce(jsonb_agg(n-'source_id'-'original_name' ORDER BY ord),'[]') FROM jsonb_array_elements(document->'nodes') WITH ORDINALITY AS e(n,ord)))::text)"


def main():
    spec = importlib.util.spec_from_file_location('audit', ROOT / 'audit.py')
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    baseline = json.loads((ROOT / 'before.json').read_text())
    backup = Path(json.loads((ROOT / 'after.json').read_text())['backup'])
    raw = subprocess.check_output(['docker', 'exec', '-i', 'adobe-relay-postgres-1', 'pg_restore', '--data-only', '--table=clash_subscription', '-f', '-'], input=(backup / 'database.dump').read_bytes(), timeout=90).decode()
    columns, rows = None, []
    escapes = {'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v', '\\': '\\'}
    for line in raw.splitlines():
        if line.startswith('COPY public.clash_subscription ('):
            columns = [x.strip().strip('"') for x in line.split('(', 1)[1].split(')', 1)[0].split(',')]
        elif line == '\\.':
            columns = None
        elif columns:
            rows.append(dict(zip(columns, [re.sub(r'\\(.)', lambda m: escapes.get(m[1], m[1]), v) for v in line.split('\t')])))
    old = json.loads(next(row['document'] for row in rows if row['id'] == '1'))
    current = json.loads(audit.sql("SELECT json_build_object('document',document,'updated_at',updated_at) FROM clash_subscription WHERE id=1"))
    new = current['document']
    encoded = base64.b64encode(json.dumps(old).encode()).decode()
    old_from = " FROM (SELECT convert_from(decode('" + encoded + "','base64'),'UTF8')::jsonb AS document) s"
    old_hash = audit.sql('SELECT ' + contract_expression() + old_from)
    assert old_hash == baseline['clash_contract_hash'], 'The actual backup does not match the protected pre-release contract'
    new_hash = audit.sql('SELECT ' + contract_expression() + ' FROM clash_subscription WHERE id=1')
    captured = base64.b64encode(json.dumps(new).encode()).decode()
    captured_from = " FROM (SELECT convert_from(decode('" + captured + "','base64'),'UTF8')::jsonb AS document) s"
    assert new_hash == audit.sql('SELECT ' + contract_expression() + captured_from), 'Concurrent import changed the snapshot; observe again'
    old_semantic = audit.sql('SELECT ' + contract_expression(False) + old_from)
    new_semantic = audit.sql('SELECT ' + contract_expression(False) + ' FROM clash_subscription WHERE id=1')
    events = json.loads(audit.sql("SELECT coalesce(json_agg(x ORDER BY id),'[]') FROM (SELECT id,created_at,actor_user_id,actor_role,auth_method,action,method,path,status_code,latency_ms,request_body FROM audit_logs WHERE created_at>='" + baseline['at'] + "'::timestamptz AND path LIKE '%clash%') x"))
    safe_events = []
    for event in events:
        body = event.pop('request_body') or ''
        event['request_body_sha256'] = hashlib.sha256(body.encode()).hexdigest()
        try:
            params = json.loads(body)
        except ValueError:
            params = {}
        if not isinstance(params, dict):
            params = {}
        names = params.get('names', [])
        if not isinstance(names, list) or any(not isinstance(name, str) for name in names):
            names = []
        event.update(request_revision=params.get('revision'), requested_name_count=len(names) if isinstance(names, list) else None, requested_names_sha256=digest(sorted(names)) if isinstance(names, list) else None, request_url_matches_current=params.get('url') == new.get('url'), request_field_names=sorted(params))
        safe_events.append(event)
    old_nodes, new_nodes = old.get('nodes', []), new.get('nodes', [])
    old_names = {node['name'] for node in old_nodes}
    new_names = {node['name'] for node in new_nodes}
    old_by_name = {n['name']: n for n in old_nodes}
    changed_fields = Counter()
    changed_config_fields = Counter()
    changed_proxy_ids = []
    for node in new_nodes:
        prev = old_by_name.get(node['name'])
        if prev is None:
            continue
        fields = [k for k in prev.keys() | node.keys() if prev.get(k) != node.get(k)]
        changed_fields.update(fields)
        if fields:
            changed_proxy_ids.append(node.get('proxy_id'))
        old_config, new_config = prev.get('config', {}), node.get('config', {})
        if isinstance(old_config, dict) and isinstance(new_config, dict):
            changed_config_fields.update(k for k in old_config.keys() | new_config.keys() if old_config.get(k) != new_config.get(k))
    recent = json.loads(audit.sql("SELECT coalesce(json_agg(x ORDER BY id),'[]') FROM (SELECT id,created_at,actor_user_id,actor_role,auth_method,action,method,path,status_code FROM audit_logs WHERE created_at>='2026-11-06T06:09:00Z'::timestamptz AND created_at<='2026-11-06T06:13:00Z'::timestamptz) x"))
    log_raw = subprocess.run(['docker', 'logs', '--since', '2026-11-06T06:09:00Z', '--until', '2026-11-06T06:13:00Z', audit.APP], capture_output=True, text=True, check=True, timeout=30)
    log_text = log_raw.stdout + log_raw.stderr
    safe_logs = []
    for line in log_text.splitlines():
        if '/clash/import' not in line:
            continue
        try:
            entry = json.loads(line[line.index('{'):])
        except ValueError:
            safe_logs.append({'line_sha256': hashlib.sha256(line.encode()).hexdigest(), 'format': 'non_json'})
            continue
        safe_logs.append({'line_sha256': hashlib.sha256(line.encode()).hexdigest(), **{k:v for k,v in entry.items() if k in ['time','timestamp','ts','method','path','status','status_code','latency','latency_ms','request_id','component','completed_at']}})
    out = {'baseline_at': baseline['at'], 'node_changed_field_counts': dict(changed_fields), 'config_changed_field_counts': dict(changed_config_fields), 'changed_proxy_ids': changed_proxy_ids,
           'node_order_changed': [n['name'] for n in old_nodes] != [n['name'] for n in new_nodes], 'nodes_equal_ignoring_order': digest(sorted(old_nodes,key=lambda n:n['name'])) == digest(sorted(new_nodes,key=lambda n:n['name'])),
           'audit_flush_failure_count_in_window': log_text.count('audit log flush failed'), 'nearby_authenticated_events': recent, 'import_log_entries': safe_logs, 'app_window_log_sha256': hashlib.sha256(log_text.encode()).hexdigest(),
           'at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'cutover_backup': str(backup), 'before_contract_hash': old_hash, 'current_contract_hash': new_hash,
           'before_revision': old.get('revision'), 'current_revision': new.get('revision'), 'current_updated_at': current['updated_at'],
           'contract_equal_without_revision': old_semantic == new_semantic, 'before_semantic_hash': old_semantic, 'current_semantic_hash': new_semantic,
           'before_nodes': len(old_nodes), 'current_nodes': len(new_nodes), 'before_disabled_nodes': sum(not node.get('enabled') for node in old_nodes), 'current_disabled_nodes': sum(not node.get('enabled') for node in new_nodes),
           'added_names': len(new_names - old_names), 'removed_names': len(old_names - new_names), 'current_names_sha256': digest(sorted(new_names)),
           'url_unchanged': old.get('url') == new.get('url'), 'changed_document_keys': sorted(k for k in old.keys() | new.keys() if old.get(k) != new.get(k)),
           'authenticated_import_events': safe_events, 'read_only': True, 'restored_old_data': False, 'credentials_in_output': False}
    audit.save('clash-observation-' + datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ') + '.json', out)
    print(json.dumps(out), flush=True)


if __name__ == '__main__':
    main()
