#!/usr/bin/env python3
"""Guarded binary-only docs update. Existing Sub2/Okad/nginx remain untouched."""
import fcntl,hashlib,json,os,pathlib,subprocess,sys,time,urllib.request
P=pathlib.Path;root=P('/opt/creative-docs');src=P(__file__).resolve().parent
new_sha,old_sha=sys.argv[1:]
assert all(len(s)==64 and all(c in '0123456789abcdef' for c in s) for s in (new_sha,old_sha))
def run(*a):return subprocess.check_output(a,text=True).strip()
def sha(p):return hashlib.sha256(P(p).read_bytes()).hexdigest()
def state():
 ids=run('docker','ps','-q').split()
 containers=sorted([{k:v[k] for k in ('Id','Image','RestartCount')} for v in json.loads(run('docker','inspect',*ids))],key=lambda v:v['Id'])
 paths=list(P('/etc/nginx/conf.d').glob('*'))+list(P('/etc/nginx/snippets').glob('creative-docs*'))+[P('/etc/systemd/system/creative-docs.service'),P('/opt/adobe-relay/shared/.env'),P('/opt/adobe-relay/shared/resources.yml'),P('/opt/adobe-team-cookie/compose.yml')]
 return {'containers':containers,'configs':{str(p):sha(p) for p in paths if p.is_file()}}
lock=open('/opt/adobe-relay/.release.lock','a');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
assert sha(src/'docs-server')==new_sha
old=(root/'current').resolve();assert old.parent==root/'releases' and sha(old/'docs-server')==old_sha
assert run('systemctl','is-active','creative-docs')=='active'
before=state();audit=root/'audit'/('update-'+time.strftime('%Y%m%dT%H%M%SZ',time.gmtime()));audit.mkdir(parents=True,exist_ok=False)
(audit/'before.json').write_text(json.dumps(before,indent=2));(audit/'previous-release.txt').write_text(str(old)+'\n')
release=root/'releases'/new_sha[:16];release.mkdir(exist_ok=True)
if (release/'docs-server').exists():assert sha(release/'docs-server')==new_sha
else:(release/'docs-server').write_bytes((src/'docs-server').read_bytes());os.chmod(release/'docs-server',0o755)
assert (root/'current').resolve()==old
link=root/'next';assert not link.exists();os.symlink(release,link);os.replace(link,root/'current')
try:
 run('systemctl','restart','creative-docs')
 for i in range(30):
  try:
   with urllib.request.urlopen('http://127.0.0.1:6670/docs/login',timeout=2) as r:
    body=r.read().decode();assert r.headers['Referrer-Policy']=='same-origin' and '登录文档' in body and '仅向已注册' not in body
   break
  except Exception:
   if i==29:raise
   time.sleep(.2)
except Exception:
 assert (root/'current').resolve()==release
 os.symlink(old,link);os.replace(link,root/'current');run('systemctl','restart','creative-docs')
 (audit/'failed-restored-previous.txt').write_text('Health check failed; only previous docs binary restored.\n');raise
after=state();(audit/'after.json').write_text(json.dumps(after,indent=2));assert before==after,'review concurrent changes; never restore old business data'
result={'audit':str(audit),'binary_sha256':new_sha,'previous_binary_sha256':old_sha,'service':run('systemctl','is-active','creative-docs'),'existing_containers_and_configs_preserved':True,'referrer_policy':'same-origin','login_copy_simplified':True}
(audit/'result.json').write_text(json.dumps(result,indent=2));print(json.dumps(result))
