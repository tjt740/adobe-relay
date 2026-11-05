"""Admit the reviewed upstream merge, retaining all installed SQL and Adobe submission guards."""
import hashlib,json,subprocess,sys
from pathlib import Path
BASE='4569e1bd2e1e8957d1a6bbba677f67d76d56b47e'
SCOPE=['backend','frontend','Dockerfile','deploy/docker-entrypoint.sh','docs/legal','.github/workflows/deploy-adobe-relay.yml']
def blob(ref,path):
 p=subprocess.run(['git','show',ref+':'+path],capture_output=True)
 return hashlib.sha256(p.stdout).hexdigest() if p.returncode==0 else None
def main(sha):
 manifest=json.loads(Path(__file__).with_name('source-manifest.json').read_text())
 changed=subprocess.check_output(['git','diff','--name-only',BASE,sha,'--',*SCOPE],text=True).splitlines()
 assert set(changed)==set(manifest),'Source scope differs from reviewed manifest'
 assert all(blob(BASE,p)==v['before'] and blob(sha,p)==v['candidate'] for p,v in manifest.items()),'Source hash differs'
 old_sql=subprocess.check_output(['git','ls-tree','-r','--name-only',BASE,'backend/migrations'],text=True).splitlines()
 assert all(blob(BASE,p)==blob(sha,p) for p in old_sql if p.endswith('.sql')),'Installed SQL modified'
 assert blob(BASE,'.github/workflows/deploy-adobe-relay.yml')==blob(sha,'.github/workflows/deploy-adobe-relay.yml'),'Formal build changed'
 from retry_guard import validate_retry_guards
 validate_retry_guards(BASE,sha)
 print(json.dumps({'baseline':BASE,'application_sha':sha,'reviewed_source_files':len(manifest),'historical_sql_unchanged':True,'submission_guards_unchanged':True}))
if __name__=='__main__':main(sys.argv[1])
