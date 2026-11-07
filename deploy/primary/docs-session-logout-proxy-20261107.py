#!/usr/bin/env python3
"""Exact, endpoint-only Nginx header-buffer correction; default is read-only."""
import difflib, fcntl, hashlib, json, os, pathlib, re, subprocess, sys, time
P = pathlib.Path
nginx = P('/etc/nginx/conf.d/creative.cheap-https.conf')
snippet = P('/etc/nginx/snippets/creative-logout-proxy.conf')
audit = P('/opt/creative-docs/audit/update-20261107T031739Z/logout-proxy-fix')
expected = '0d7d1bb58b389198e0ce2a9aaad1474d556cd998e7febb37f97ab3883341108e'
sha = lambda b: hashlib.sha256(b).hexdigest()
def run(*a):
    return subprocess.check_output(a, text=True, stderr=subprocess.STDOUT).strip()
def state():
    ids = run('docker', 'ps', '-q').split()
    containers = sorted([{k: v[k] for k in ('Id', 'Image', 'RestartCount')} for v in json.loads(run('docker', 'inspect', *ids))], key=lambda v: v['Id'])
    paths = list(P('/etc/nginx/conf.d').glob('*')) + list(P('/etc/nginx/snippets').glob('*')) + [P('/etc/systemd/system/creative-docs.service'), P('/opt/adobe-relay/shared/.env'), P('/opt/adobe-relay/shared/resources.yml'), P('/opt/adobe-team-cookie/compose.yml')]
    return {'containers': containers, 'configs': {str(p): sha(p.read_bytes()) for p in paths if p.is_file()}}
def logout(direct=False):
    args = ['curl', '--silent', '--show-error', '--max-time', '20', '--dump-header', '-', '-X', 'POST', '-H', 'Content-Type: application/json', '-H', 'X-Forwarded-Proto: https', '--data', '{}']
    if direct:
        args += ['http://127.0.0.1:6666/api/v1/auth/logout']
    else:
        args += ['--resolve', 'creative.cheap:443:127.0.0.1', 'https://creative.cheap/api/v1/auth/logout']
    raw = subprocess.check_output(args)
    headers, body = raw.split(b'\r\n\r\n', 1)
    return {'status': int(headers.splitlines()[0].split()[1]), 'cleared_cookies': sum(x.lower().startswith(b'set-cookie:') for x in headers.splitlines()), 'body': body.decode()}
lock = open('/opt/adobe-relay/.release.lock', 'a')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
assert sha(P('/opt/creative-docs/current/docs-server').read_bytes()) == '15488921fd149a9902b63035826c3f7866a8dc4ccac3bfb0f9d0f89224b83114'
old = nginx.read_text(); assert sha(old.encode()) == expected
assert not snippet.exists()
match = re.search(r'^    location / \{\n(.*?)^    \}', old, re.M | re.S)
assert match and 'proxy_pass http://127.0.0.1:6666;' in match[1]
assert 'proxy_buffer_size' not in match[1] and 'proxy_buffers ' not in match[1]
new_snippet = '# Sub2 logout clears 23 OAuth cookies; retain all response headers.\nlocation = /api/v1/auth/logout {\n' + match[1] + '        proxy_buffer_size 16k;\n        proxy_buffers 4 16k;\n}\n'
anchor = '    include /etc/nginx/snippets/creative-docs-locations.conf;\n'
assert old.count(anchor) == 1
addition = '    include /etc/nginx/snippets/creative-logout-proxy.conf;\n'
new = old.replace(anchor, anchor + addition)
assert new.replace(addition, '', 1) == old
print(''.join(difflib.unified_diff(old.splitlines(True), new.splitlines(True), fromfile=str(nginx), tofile=str(nginx))))
print(new_snippet)
if '--apply' not in sys.argv:
    print(json.dumps({'dry_run': True, 'config_sha256': expected})); sys.exit(0)
assert sys.argv[1:] == ['--apply']
run('nginx', '-t')
baseline_direct, baseline_proxy = logout(True), logout()
assert baseline_direct['status'] == 200 and baseline_direct['cleared_cookies'] == 23
assert baseline_proxy['status'] == 502
assert json.loads(baseline_direct['body'])['code'] == 0
before = state(); audit.mkdir(mode=0o700)
(audit / 'nginx.before.conf').write_text(old)
(audit / 'before.json').write_text(json.dumps(before, indent=2))
(audit / 'baseline.json').write_text(json.dumps({'direct': baseline_direct, 'proxy': baseline_proxy}, indent=2))
assert nginx.read_text() == old and not snippet.exists()
with snippet.open('x') as f: f.write(new_snippet)
snippet.chmod(0o644)
tmp = nginx.with_suffix('.docs-session-new'); assert not tmp.exists()
tmp.write_text(new); tmp.chmod(0o644); os.replace(tmp, nginx)
try:
    (audit / 'nginx-test.txt').write_text(run('nginx', '-t') + '\n')
    run('systemctl', 'reload', 'nginx')
    time.sleep(0.5)
    verified = logout()
    assert verified['status'] == 200 and verified['cleared_cookies'] == 23 and json.loads(verified['body'])['code'] == 0
except Exception:
    assert nginx.read_text() == new and snippet.read_text() == new_snippet
    nginx.write_text(old); snippet.unlink()
    run('nginx', '-t'); run('systemctl', 'reload', 'nginx')
    (audit / 'failed-restored-own-change.txt').write_text('Only this include and snippet were reverted.\n')
    raise
after = state(); (audit / 'after.json').write_text(json.dumps(after, indent=2))
expected_after = dict(before['configs']); expected_after[str(nginx)] = sha(new.encode()); expected_after[str(snippet)] = sha(new_snippet.encode())
assert after['containers'] == before['containers'] and after['configs'] == expected_after, 'Review concurrent changes; do not restore other configuration'
assert run('systemctl', 'is-active', 'creative-docs') == 'active'
result = {'audit': str(audit), 'logout_status': verified['status'], 'cleared_cookies': verified['cleared_cookies'], 'existing_containers_preserved': True, 'only_config_changes': [str(nginx), str(snippet)], 'before_main_sha256': expected, 'after_main_sha256': sha(new.encode()), 'snippet_sha256': sha(new_snippet.encode()), 'docs_binary_preserved': True}
(audit / 'result.json').write_text(json.dumps(result, indent=2));print(json.dumps(result))
