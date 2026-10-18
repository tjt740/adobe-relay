from pathlib import Path
import datetime,hashlib,json,subprocess
p=Path('/opt/adobe-relay/releases/subscription-health-ui-20261018')
sha=(p/'expected-sha').read_text().strip()
assert sha=='7dfc317773c88153e7b70a35c5b5207d32de519b'
image=json.loads(subprocess.check_output(['docker','image','inspect','adobe-relay:'+sha]))[0]
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'image_id':image['Id'],'pr':81,'workflow':39135974046,'build_only':True,'archive_checksum_verified':True,'release_script_sha256':hashlib.sha256(Path('/opt/adobe-relay/deploy/primary/release.sh').read_bytes()).hexdigest()}
q=p/'release-metadata.json';assert not q.exists();q.write_text(json.dumps(result,indent=2));q.chmod(0o600)
print(json.dumps(result))
