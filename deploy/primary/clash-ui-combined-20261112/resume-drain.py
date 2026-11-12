"""Continue only the naturally draining canary after the original bounded wait."""
import fcntl, hashlib, importlib.util, json, pathlib, subprocess, sys, traceback
ROOT = pathlib.Path('/opt/adobe-relay/releases/clash-ui-combined-20261112')
SHA = 'e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b'

def validate(progress, prepared, image, canary, routed, finished, rule):
 assert progress['step'] == 'failed' and "'drain-canary'" in progress['error']
 assert prepared['candidate_sha'] == SHA and image == 'adobe-relay:' + SHA
 assert canary == prepared['canary_id'] and routed and not finished and not rule

def main():
 held=[]
 for name in ['/opt/adobe-relay/shared/.adobe-deploy.lock', str(ROOT/'driver.lock')]:
  f=open(name,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
 sys.path.insert(0,str(ROOT))
 import rolling as r
 import driver as d
 locks=r.locks()
 assert (ROOT/'expected-sha').read_text().strip()==SHA
 progress=r.load('progress.json');prepared=r.load('rolling-prepared.json')
 validate(progress,prepared,r.inspect(r.APP)['Config']['Image'],r.inspect(r.CANARY)['Id'],(ROOT/'rolling-route-canonical.json').exists(),(ROOT/'rolling-finished.json').exists(),r.rule_present())
 assert r.health(6666) and r.health(6667)
 assert not (ROOT/'canary-drain-retry-1.json').exists()
 spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
 preserved=a.check_preserved(r.load('before.json'),a.snapshot())
 r.save('canary-drain-initial-failure.json',progress)
 r.save('canary-drain-initial-samples.json',r.load('rolling-canary-drain-progress.json'))
 r.save('canary-drain-retry-1.json',{'at':r.now(),'application_sha':SHA,'preserved':preserved,'reason':'Initial bounded wait ended with automatic checks still running; continue the same natural drain without routing, restarting, queue deferral or request replay','original_log_sha256':hashlib.sha256((ROOT/'driver.log').read_bytes()).hexdigest()})
 r.own_lease()
 for f in locks:f.close()
 try:
  d.run_step('drain-canary',[sys.executable,str(ROOT/'rolling.py'),'drain-canary'])
  d.run_step('cleanup',[sys.executable,str(ROOT/'rolling.py'),'cleanup'])
  d.run_step('preservation-verification',[sys.executable,str(ROOT/'audit.py'),'after'],timeout=180)
  d.run_step('runtime-verification',[sys.executable,str(ROOT/'audit.py'),'verify'],timeout=180)
  d.progress('complete','done')
 except Exception as e:
  d.progress('failed','attention_required',error=str(e)[:400]);raise

if __name__=='__main__':main()
