"""Read-only, sanitized end-of-experiment audit; never initiates account requests."""
import datetime, importlib.util, json, pathlib, re, sqlite3, subprocess

ROOT = pathlib.Path('/opt/adobe-relay/releases/session-controlled-20261027')
spec = importlib.util.spec_from_file_location('session_controller', ROOT / 'controller.py')
controller = importlib.util.module_from_spec(spec)
spec.loader.exec_module(controller)
audit = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
         'accounts': controller.rows(), 'other_accounts_current': controller.others()}
containers = []
for name in ['adobe-relay-app-1', 'adobe-team-cookie-app-1']:
    d = json.loads(subprocess.check_output(['docker', 'inspect', name], text=True))[0]
    containers.append({'name': name, 'image': d['Config']['Image'], 'id': d['Id'],
                       'started_at': d['State']['StartedAt'], 'restart_count': d['RestartCount'],
                       'running': d['State']['Running'], 'oom_killed': d['State']['OOMKilled']})
audit['containers'] = containers
audit['test_containers'] = [n for n in subprocess.check_output(
    ['docker', 'ps', '--format', '{{.Names}}'], text=True).splitlines()
    if n.startswith('adobe-session-controlled-r')]
db = sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro', uri=True)
db.row_factory = sqlite3.Row
audit['okad_members'] = [dict(r) for r in db.execute(
    'SELECT id,email,last_login_at,expires_at,profile_preference,otp_mode,force_code_login '
    'FROM external_members WHERE id IN (247,245,240,231) ORDER BY id')]
db.close()
started = json.loads((ROOT / 'started.json').read_text())['at']
p = subprocess.run(['docker', 'logs', '--timestamps', '--since', started,
                    'adobe-relay-app-1'], capture_output=True, text=True, timeout=30)
logs = []
for line in (p.stdout + p.stderr).splitlines():
    pos = line.find('{')
    if pos < 0:
        continue
    try:
        d = json.loads(line[pos:])
    except ValueError:
        continue
    m = re.search(r'/accounts/(\d+)(?:/|$)', d.get('path', ''))
    aid = d.get('account_id', int(m.group(1)) if m else None)
    if aid not in controller.IDS:
        continue
    safe = {k: d[k] for k in ['path', 'method', 'status_code', 'request_id', 'stage',
                              'version', 'attempt', 'proxy_id', 'upstream_http_status',
                              'stage_attempts', 'stage_elapsed_ms', 'processing_ms'] if k in d}
    safe.update(account_id=aid, at=line.split()[0], event=line[:pos].strip().split('\t')[-1])
    logs.append(safe)
audit['selected_application_logs'] = sorted(logs, key=lambda x: x['at'])
audit['log_read_returncode'] = p.returncode
print(json.dumps(audit, ensure_ascii=False))
