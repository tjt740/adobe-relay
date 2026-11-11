"""Check natural production success diagnostics; no requests, secrets or raw logs are saved."""
import datetime,json,pathlib,re,subprocess
r=pathlib.Path('/opt/adobe-relay/releases/proxy-reliability-20261112');sha=(r/'expected-sha').read_text().strip()
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0];assert d['Config']['Image']=='adobe-relay:'+sha
cutover=json.loads((r/'rolling-route-canonical.json').read_text())['at']
p=subprocess.run(['docker','logs','--since',cutover,'--tail','5000','adobe-relay-app-1'],capture_output=True,text=True,check=True)
counts={'pipeline_records':0,'image_diagnostics':0,'three_fingerprints_present':0,'declared_expiry_present':0};keys=set()
allowed={'credential_fingerprint','bearer_fingerprint','session_fingerprint','fingerprint_version','credential_expires_at','expired_at_start','upstream_error_code','access_error','response_body_sha256','upstream_request_id'}
for line in (p.stdout+'\n'+p.stderr).splitlines():
 if 'adobe_images.pipeline_completed'not in line:continue
 try:record=json.loads(line[line.index('{'):])
 except ValueError:continue
 counts['pipeline_records']+=1
 for image in record.get('timings',{}).get('images',[]):
  diag=image.get('request_diagnostics');assert isinstance(diag,dict),'Completed image missing diagnostics'
  counts['image_diagnostics']+=1;keys.update(diag);assert set(diag)<=allowed
  assert diag['fingerprint_version']=='adobe-bearer-arp-v1'
  assert all(re.fullmatch('[a-f0-9]{64}',diag.get(k,''))for k in ['credential_fingerprint','bearer_fingerprint','session_fingerprint'])
  counts['three_fingerprints_present']+=1
  if diag.get('credential_expires_at',0)>0:counts['declared_expiry_present']+=1
assert counts['image_diagnostics']>0 and counts['three_fingerprints_present']==counts['image_diagnostics']
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'passed':True,'counts':counts,'observed_safe_field_names':sorted(keys),'raw_credentials_or_logs_saved':False,'extra_generation_requests':0,'live_403_observed':False,'limitation':'403 failed-stage persistence and redaction were tested in isolation; no live 403 was forced. This supplements the first verifier, which did not parse the existing console log prefix.'}
path=r/'diagnostic-log-verification.json';assert not path.exists();path.write_text(json.dumps(out,indent=2));path.chmod(0o600);print(json.dumps(out))
