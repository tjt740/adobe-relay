"""Review the observed automatic Codex version advance before restarting preparation."""
import datetime, fcntl, hashlib, importlib.util, json, pathlib, re, shutil, subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-route-health-20261023')
held=[]
for path in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(path,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
before=json.loads((ROOT/'before.json').read_text());current=a.snapshot()
assert current['containers'][a.APP]['id']==before['containers'][a.APP]['id']
assert current['containers'][a.APP]['image']==a.OLD_IMAGE
assert not (ROOT/'rolling-prepared.json').exists() and not (ROOT/'rolling-route-canary.json').exists()
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert a.sql("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename LIKE '253%')")=='f'
assert 'adobe-route-health-20261023' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
progress=json.loads((ROOT/'progress.json').read_text());assert progress['step']=='failed' and progress['state']=='attention_required'
key='openai_codex_client_version_synced'
old=before['settings_hashes'];new=current['settings_hashes']
assert {k for k in old.keys()|new.keys() if old.get(k)!=new.get(k)}=={key}
p=subprocess.run(['docker','logs','--since',before['at'],a.APP],capture_output=True,text=True,check=True)
matches=[]
for line in (p.stdout+'\n'+p.stderr).splitlines():
 if 'openai_codex_version_synced' not in line:continue
 previous=re.search(r'(?:"previous"\s*:\s*"|previous=)(\d+\.\d+\.\d+)',line)
 version=re.search(r'(?:"version"\s*:\s*"|version=)(\d+\.\d+\.\d+)',line)
 if previous and version and hashlib.md5(previous[1].encode()).hexdigest()==old[key] and hashlib.md5(version[1].encode()).hexdigest()==new[key]:matches.append((line,previous[1],version[1]))
assert len(matches)==1,'Require exactly matching automatic synchronization evidence'
line,previous,version=matches[0]
checksum='677845f2ce6082ca3c9a651669efda3e6db43f3f7140dc29e715f71cd5625b83'
review={'key':key,'old_value':previous,'new_value':version,'old_hash':old[key],'new_hash':new[key],'sync_source_path':'backend/internal/service/openai_codex_version_sync_service.go','sync_source_before':checksum,'sync_source_candidate':checksum,'event':'openai_codex_version_synced','evidence_log_sha256':hashlib.sha256(line.encode()).hexdigest()}
a.reviewed_setting_hashes(old,review)
a.save('reviewed-automatic-settings.json',review)
checks=a.check_preserved(before,current)
archive=ROOT/'attempt-history'/'pre-routing-automatic-codex-sync';archive.mkdir(parents=True,exist_ok=False,mode=0o700)
for name in ['progress.json','capacity-before-start.json','driver.log']:
 p=ROOT/name
 if p.exists():shutil.move(p,archive/name)
a.save('preflight-resume.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'no_candidate_started':True,'routing_unchanged':True,'preserved':checks,'automatic_setting_review':review,'archived_attempt':str(archive)})
print(json.dumps({'preparation_can_restart':True,'review':review,'preserved':checks}))
