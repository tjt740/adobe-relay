#!/usr/bin/env python3
"""Install /docs only, preserving the current Sub2/Okad application and routes."""
import fcntl, hashlib, json, os, pathlib, shutil, subprocess, sys, time, urllib.request
P=pathlib.Path
root=P('/opt/creative-docs');src=P(__file__).resolve().parent
lock=open('/opt/adobe-relay/.release.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
def run(*args):return subprocess.check_output(args,text=True).strip()
def sha(p):return hashlib.sha256(P(p).read_bytes()).hexdigest()
def containers():
    ids=run('docker','ps','-q').split()
    return sorted([{k:v[k] for k in ('Id','Image','RestartCount')} for v in json.loads(run('docker','inspect',*ids))],key=lambda v:v['Id'])
def configs():
    paths=list(P('/etc/nginx/conf.d').glob('*'))+[P('/opt/adobe-relay/shared/.env'),P('/opt/adobe-relay/shared/resources.yml'),P('/opt/adobe-team-cookie/compose.yml')]
    return {str(p):sha(p) for p in paths if p.is_file() and not p.name.startswith('00-creative-docs')}
expected=sys.argv[1]
assert len(expected)==64 and sha(src/'docs-server')==expected
assert not P('/etc/systemd/system/creative-docs.service').exists(), 'already installed; review update explicitly'
assert not root.joinpath('current').exists(), 'existing version must be reviewed'
nginx=P('/etc/nginx/conf.d/creative.cheap-https.conf')
old=nginx.read_text();marker='    server_name creative.cheap www.creative.cheap;\n'
include='    include /etc/nginx/snippets/creative-docs-locations.conf;\n'
assert old.count(marker)==1 and '/docs' not in old and 'creative-docs' not in old
new=old.replace(marker,marker+include)
assert new.replace(include,'')==old
before={'containers':containers(),'configs':configs()}
audit=root/'audit'/time.strftime('%Y%m%dT%H%M%SZ',time.gmtime());audit.mkdir(parents=True,exist_ok=False)
(audit/'before.json').write_text(json.dumps(before,indent=2));shutil.copy2(nginx,audit/'creative.cheap-https.before.conf')
release=root/'releases'/expected[:16];release.mkdir(parents=True,exist_ok=False)
shutil.copy2(src/'docs-server',release/'docs-server');os.chmod(release/'docs-server',0o755)
os.symlink(release,root/'current');shutil.copy2(src/'docs-site.service','/etc/systemd/system/creative-docs.service')
run('systemctl','daemon-reload');run('systemctl','enable','--now','creative-docs.service')
for i in range(20):
    try:
        with urllib.request.urlopen('http://127.0.0.1:6670/docs/healthz',timeout=2) as r:assert r.read()==b'ok\n'
        break
    except Exception:
        if i==19:raise
        time.sleep(.25)
targets={'/etc/nginx/conf.d/00-creative-docs-limits.conf':'nginx-limits.conf','/etc/nginx/snippets/creative-docs-proxy.conf':'nginx-proxy.conf','/etc/nginx/snippets/creative-docs-locations.conf':'nginx-locations.conf'}
for dest,name in targets.items():
    assert not P(dest).exists(), 'existing config must be reviewed'
    shutil.copy2(src/name,dest)
assert nginx.read_text()==old, 'nginx changed concurrently'
tmp=nginx.with_suffix('.new');tmp.write_text(new);os.chmod(tmp,0o644);os.replace(tmp,nginx)
try:run('nginx','-t')
except Exception:
    nginx.write_text(old)
    for dest in targets:P(dest).unlink()
    run('systemctl','disable','--now','creative-docs.service')
    raise
run('systemctl','reload','nginx')
after={'containers':containers(),'configs':configs()}
(audit/'after.json').write_text(json.dumps(after,indent=2))
assert before['containers']==after['containers'], 'container state changed; review, do not restore old business state'
expected_configs=dict(before['configs']);expected_configs[str(nginx)]=hashlib.sha256(new.encode()).hexdigest()
assert after['configs']==expected_configs,'unexpected config change'
assert nginx.read_text().replace(include,'')==old
print(json.dumps({'audit':str(audit),'service':run('systemctl','is-active','creative-docs'),'sub2_and_okad_preserved':True,'only_existing_nginx_change':'include docs locations','url':'https://creative.cheap/docs'}))
