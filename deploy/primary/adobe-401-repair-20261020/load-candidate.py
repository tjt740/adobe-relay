"""Load only the checksum-verified, successful exact CI candidate; no routing change."""
import datetime,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/adobe-401-repair-20261020')
SHA='cc5265944792edbccae56c2fa327efb272221c46'
os.umask(0o077)
def inspect(name):return json.loads(subprocess.check_output(['docker','inspect',name],text=True))[0]
proof=json.loads((ROOT/'artifact/ci-proof.json').read_text())
assert proof['application_sha']==SHA and proof['workflow_run']==39428800847 and proof['conclusion']=='success'
archive=ROOT/'artifact/release-images.tar.gz'
assert hashlib.sha256(archive.read_bytes()).hexdigest()==proof['archive_sha256']
assert (ROOT/'artifact/release-images.tar.gz.sha256').read_text().split()==[proof['archive_sha256'],'release-images.tar.gz']
before=json.loads((ROOT/'before.json').read_text())
old=inspect('adobe-relay-app-1')
assert old['Id']==before['containers']['adobe-relay-app-1']['id']
assert old['Config']['Image']=='adobe-relay:6941bd6d54de19e4085698fc69147c2fcac88b0c'
marker=ROOT/'image-loaded.json'
if marker.exists():
 saved=json.loads(marker.read_text());assert saved['image_id']==inspect('adobe-relay:'+SHA)['Id']
 print(json.dumps({'loaded':True,'already_verified':True,'sha':SHA}));raise SystemExit()
r=subprocess.run(['docker','load','--input',str(archive)],text=True,capture_output=True,timeout=180)
(ROOT/'image-load.log').write_text(r.stdout+r.stderr)
assert r.returncode==0,'Image load failed; no traffic changed'
image=inspect('adobe-relay:'+SHA)
assert old['Id']==inspect('adobe-relay-app-1')['Id']
expected=ROOT/'expected-sha'
if expected.exists():assert expected.read_text().strip()==SHA
else:expected.write_text(SHA+'\n')
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sha':SHA,'workflow_run':proof['workflow_run'],'image_id':image['Id'],'archive_sha256':proof['archive_sha256'],'changed_routing':False}
marker.write_text(json.dumps(out,indent=2));print(json.dumps(out))
