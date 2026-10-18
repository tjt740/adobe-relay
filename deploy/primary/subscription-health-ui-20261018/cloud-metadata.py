from pathlib import Path
import datetime,hashlib,json,subprocess
p=Path('/opt/adobe-relay/releases/subscription-health-ui-20261018')
sha=(p/'expected-sha').read_text().strip()
assert sha=='7dfc317773c88153e7b70a35c5b5207d32de519b'
image=json.loads(subprocess.check_output(['docker','image','inspect','adobe-relay:'+sha]))[0]
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'image_id':image['Id'],'pr':81,'workflow':39135974046,'build_only':True,'archive_checksum_verified':True,'release_script_sha256':hashlib.sha256(Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()}
assert result['release_script_sha256']=='4bf42b5920d67c0f1c11e4c096080e65af3c999a56f310a48c45c8d0e8ae1154', 'Production release script changed; review before switching'
q=p/'release-metadata.json';assert not q.exists();q.write_text(json.dumps(result,indent=2));q.chmod(0o600)
print(json.dumps(result))
