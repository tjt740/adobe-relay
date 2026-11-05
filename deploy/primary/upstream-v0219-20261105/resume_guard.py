"""Resume only the recorded checksum-format failure before any routing."""
import importlib.util,json,subprocess

def validate_records(progress,log,prepared,old,canary,proof,sha,route_present,lease,rid):
 assert progress['step']=='failed' and progress['state']=='attention_required'
 assert 'schema-verify.py' in progress['error']
 assert 'AssertionError: Unexpected new migration set' in log
 assert prepared['candidate_sha']==sha
 assert old['Id']==prepared['old_id']
 assert old['Config']['Image']=='adobe-relay:4569e1bd2e1e8957d1a6bbba677f67d76d56b47e'
 assert canary['Id']==prepared['canary_id']
 assert canary['Config']['Image']=='adobe-relay:'+sha and canary['Image']==proof['image_id']
 assert all(d['State']['Health']['Status']=='healthy' for d in [old,canary])
 assert not route_present and lease in ['',rid]

def verify(root,sha):
 spec=importlib.util.spec_from_file_location('rolling',root/'rolling.py');r=importlib.util.module_from_spec(spec);spec.loader.exec_module(r)
 held=r.locks()
 assert not any((root/name).exists() for name in ['rolling-route-canary.json','rolling-route-canonical.json','after.json','rolling-finished.json','resume-validation.json'])
 load=lambda name:json.loads((root/name).read_text())
 validate_records(load('initial-progress.json'),(root/'driver.log').read_text(),load('rolling-prepared.json'),r.inspect(r.APP),r.inspect(r.CANARY),load('image-loaded.json'),sha,r.rule_present(),r.redis('GET',r.LEASE),r.RID)
 version=subprocess.check_output(['docker','exec',r.CANARY,'/app/sub2api','-version'],stderr=subprocess.STDOUT,text=True,timeout=30).strip()
 assert '0.2.19' in version and sha[:8] in version
 assert r.health(6666) and r.health(6667)
 spec=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 a.check_preserved(load('before.json'),a.snapshot())
 r.own_lease()
 r.save('resume-validation.json',{'at':r.now(),'only_checksum_format_corrected':True,'existing_candidate_reused':True,'routing_unchanged':True,'old_container_unchanged':True,'candidate_version':version,'historical_failure_preserved':True})
