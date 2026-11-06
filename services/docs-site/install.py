#!/usr/bin/env python3
"""Install only the standalone docs service. Never replace any Sub2 container."""
import fcntl, hashlib, json, os, pathlib, shutil, subprocess, sys, time, urllib.request
P=pathlib.Path
root=P('/opt/creative-docs')
src=P(__file__).resolve().parent
mode=sys.argv[1]
assert mode in ('install','https')
lock=open('/opt/adobe-relay/.release.lock','a')
fcntl.flock(lock, fcntl.LOCK_EX|fcntl.LOCK_NB)

def run(*args):
    return subprocess.check_output(args,text=True).strip()
def sha(p): return hashlib.sha256(P(p).read_bytes()).hexdigest()
def containers():
    ids=run('docker','ps','-q').split()
    return [{k:v[k] for k in ('Id','Image','RestartCount')} for v in json.loads(run('docker','inspect',*ids))]
def configs():
    paths=list(P('/etc/nginx/conf.d').glob('*'))+[P('/opt/adobe-relay/shared/.env'), P('/opt/adobe-relay/shared/resources.yml'),P('/opt/adobe-team-cookie/compose.yml')]
    return {str(p):sha(p) for p in paths if p.is_file() and not p.name.startswith(('creative-docs','00-creative-docs'))}
before={'containers':containers(),'configs':configs()}
audit=root/'audit'/time.strftime('%Y%m%dT%H%M%SZ',time.gmtime())
audit.mkdir(parents=True,exist_ok=False)
(audit/'before.json').write_text(json.dumps(before,indent=2))
if mode=='install':
    expected=sys.argv[2]
    assert len(expected)==64 and sha(src/'docs-server')==expected
    assert not P('/etc/systemd/system/creative-docs.service').exists(), 'already installed; review update explicitly'
    release=root/'releases'/expected[:16]
    release.mkdir(parents=True,exist_ok=False)
    shutil.copy2(src/'docs-server',release/'docs-server')
    os.chmod(release/'docs-server',0o755)
    os.symlink(release,root/'current')
    shutil.copy2(src/'docs-site.service','/etc/systemd/system/creative-docs.service')
    P('/var/www/creative-docs-acme').mkdir(parents=True,exist_ok=True)
    shutil.copy2(src/'nginx-http.conf','/etc/nginx/conf.d/creative-docs-http.conf')
    try: run('nginx','-t')
    except Exception:
        P('/etc/nginx/conf.d/creative-docs-http.conf').unlink();raise
    run('systemctl','daemon-reload');run('systemctl','enable','--now','creative-docs.service')
    for i in range(20):
        try:
            with urllib.request.urlopen('http://127.0.0.1:6670/healthz',timeout=2) as r: assert r.read()==b'ok\n'
            break
        except Exception:
            if i==19:raise
            time.sleep(.25)
    run('systemctl','reload','nginx')
else:
    assert run('systemctl','is-active','creative-docs')=='active'
    assert P('/etc/letsencrypt/live/docs.creative.cheap/fullchain.pem').exists()
    targets={'/etc/nginx/conf.d/creative-docs-https.conf':'nginx-https.conf','/etc/nginx/snippets/creative-docs-proxy.conf':'nginx-proxy.conf'}
    for dest,name in targets.items():
        assert not P(dest).exists(), 'already configured; review update explicitly'
        shutil.copy2(src/name,dest)
    log=P('/etc/nginx/conf.d/00-creative-docs-log.conf')
    assert not log.exists()
    log.write_text('log_format creative_docs \'$remote_addr [$time_local] "$request_method $uri $server_protocol" $status $body_bytes_sent\';\n')
    try:run('nginx','-t')
    except Exception:
        for path in targets:P(path).unlink()
        log.unlink();raise
    run('systemctl','reload','nginx')
after={'containers':containers(),'configs':configs()}
(audit/'after.json').write_text(json.dumps(after,indent=2))
assert before==after, 'Existing containers/configs changed; review evidence; do not roll back live business'
print(json.dumps({'mode':mode,'audit':str(audit),'service':run('systemctl','is-active','creative-docs'),'existing_containers_and_configs_preserved':True}))
