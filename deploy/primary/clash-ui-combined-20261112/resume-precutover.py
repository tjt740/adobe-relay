"""One-time retry after the recorded pre-creation lock contention. Never resume a partial handoff."""
import datetime,fcntl,hashlib,importlib.util,json,pathlib,subprocess
r=pathlib.Path('/opt/adobe-relay/releases/clash-ui-combined-20261112')
sha='e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b'
held=[]
for name in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(name,'r');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
spec=importlib.util.spec_from_file_location('audit',r/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
assert (r/'expected-sha').read_text().strip()==sha
progress=json.loads((r/'progress.json').read_text())
assert progress['step']=='failed' and progress['state']=='attention_required'
assert "'prepare', '--sha', '"+sha+"'" in progress['error']
log=(r/'driver.log').read_text()
assert 'BlockingIOError: [Errno 11]' in log and 'route-canary' not in log
for name in ['rolling-prepared.json','rolling-route-canary.json','rolling-old-drained.json','rolling-route-canonical.json','rolling-finished.json','after.json','precutover-retry-1.json']:
 assert not (r/name).exists(),name
assert subprocess.run(['docker','inspect','adobe-relay-memory-canary'],capture_output=True).returncode!=0
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
assert 'clash-ui-combined-20261112' not in subprocess.check_output(['iptables-save','-t','nat'],text=True)
unit=subprocess.run(['systemctl','is-active','clash-ui-combined-release-20261112.service'],text=True,capture_output=True)
assert unit.stdout.strip() not in ['active','activating','reloading']
before=json.loads((r/'before.json').read_text());current=a.snapshot()
assert current['containers'][a.APP]['id']==before['containers'][a.APP]['id']
assert current['containers'][a.APP]['image']==a.OLD_IMAGE
assert current['containers'][a.APP]['health']=='healthy'

# Reconcile only the exact two completed, independently authorized Okad releases.
receipts=[];evidence={}
for rid in RELEASES:
 stage=pathlib.Path('/opt/adobe-team-cookie/.local')/rid
 assert not (stage/'automatic-rollback.json').exists() and not (stage/'rolled-back.json').exists()
 item={}
 for name in ['manifest','before','after']:
  p=stage/(name+'.json');data=p.read_bytes();item[name]=json.loads(data)
  evidence[str(p)]=hashlib.sha256(data).hexdigest()
 receipts.append(item)
adjusted=reviewed_baseline(before,current,receipts)
preserved=a.check_preserved(adjusted,current)
a.save('before-initial-preparation.json',before)
a.save('reviewed-concurrent-okad.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'reason':'Preserve independently authorized Okad import-group release and transport fix before the Sub2 handoff','release_chain':RELEASES,'receipt_sha256':evidence,'original_snapshot_sha256':hashlib.sha256((r/'before.json').read_bytes()).hexdigest(),'original_okad':before['containers'][OKAD],'retained_okad':current['containers'][OKAD],'compose_before':before['config_hashes'][COMPOSE],'compose_current':current['config_hashes'][COMPOSE],'only_reviewed_baseline_fields_changed':['containers.'+OKAD,'config_hashes.'+COMPOSE],'other_preservation_checks':preserved,'production_configuration_written':False,'production_data_written':False})
p=r/'before.reviewed.tmp';p.write_text(json.dumps(adjusted,ensure_ascii=False,indent=2));p.chmod(0o600);p.replace(r/'before.json')
before=adjusted

preserved=a.check_preserved(before,current)
loaded=json.loads((r/'image-loaded.json').read_text())
assert loaded['sha']==sha and a.inspect('adobe-relay:'+sha)['Id']==loaded['image_id']
assert json.loads((r/'premigration-backup.json').read_text())['verified']
a.save('initial-start-failure.json',progress)
a.save('precutover-retry-1.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':sha,'reason':'Initial prepare was blocked by a held release lock before candidate creation; all four locks are now available','initial_progress':progress,'original_driver_log_sha256':hashlib.sha256(log.encode()).hexdigest(),'original_container_retained':True,'candidate_absent':True,'lease_empty':True,'routing_unchanged':True,'preserved':preserved,'reuse_checked_image_and_backup':True,'replay_business_requests':False})
for f in reversed(held):f.close()
subprocess.run(['systemd-run','--unit=clash-ui-combined-release-20261112-retry1','--collect','--property=WorkingDirectory='+str(r),'--property=StandardOutput=append:'+str(r/'driver.log'),'--property=StandardError=append:'+str(r/'driver.log'),'/usr/bin/python3',str(r/'driver.py')],check=True)
print(json.dumps({'retry_dispatched':True,'application_sha':sha,'original_failure_preserved':True,'preserved':preserved}))
