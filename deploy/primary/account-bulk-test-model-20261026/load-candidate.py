"""Load only the exact successful CI artifact; leave routing and current data intact."""
import datetime, hashlib, json, os, subprocess
from pathlib import Path

ROOT=Path('/opt/adobe-relay/releases/account-bulk-test-model-20261026')
SHA=(ROOT/'expected-sha').read_text().strip()
RUN=int((ROOT/'expected-run').read_text())
os.umask(0o077)
def inspect(name):return json.loads(subprocess.check_output(['docker','inspect',name],text=True))[0]
proof=json.loads((ROOT/'artifact/ci-proof.json').read_text())
assert proof['application_sha']==SHA and proof['workflow_run']==RUN and proof['conclusion']=='success'
archive=ROOT/'artifact/release-images.tar.gz'
digest=hashlib.sha256(archive.read_bytes()).hexdigest()
assert digest==proof['archive_sha256']
assert (ROOT/'artifact/release-images.tar.gz.sha256').read_text().split()==[digest,'release-images.tar.gz']
before=json.loads((ROOT/'before.json').read_text())
old=inspect('adobe-relay-app-1')
assert old['Id']==before['containers']['adobe-relay-app-1']['id']
assert old['Config']['Image']=='adobe-relay:de64302ab60f3c48cb1a748957b28b4a35f7044e'
assert not (ROOT/'image-loaded.json').exists(),'Candidate already loaded; inspect its saved evidence'
p=subprocess.run(['docker','load','--input',str(archive)],text=True,capture_output=True,timeout=180)
(ROOT/'image-load.log').write_text(p.stdout+p.stderr)
assert p.returncode==0,'Image load failed; no traffic changed'
image=inspect('adobe-relay:'+SHA)
assert old['Id']==inspect('adobe-relay-app-1')['Id']
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sha':SHA,'workflow_run':RUN,'image_id':image['Id'],'archive_sha256':digest,'changed_routing':False}
(ROOT/'image-loaded.json').write_text(json.dumps(out,indent=2))
print(json.dumps(out))
