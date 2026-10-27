import pathlib,subprocess,json,datetime
R=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r2')
assert (R/'handoff.json').exists() and not (R/'cleaned.json').exists() and not (R/'continuation-started.json').exists()
code=r'''
import pathlib,os,signal,json,time,concurrent.futures,threading
R=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r2')
source=(R/'run_trial.py').read_text().split("\nassert not (ROOT/'trial-started.json')")[0]
assert "fields={'model':model" in source and '4096x4096' in source
exec(compile(source.replace('4096x4096','3840x2160'),'continuation_helpers','exec'),globals())
assert not (ROOT/'continuation-started.json').exists()
handoff=json.loads((ROOT/'handoff.json').read_text());pid=handoff['pid'];cmd=(pathlib.Path('/proc')/str(pid)/'cmdline').read_bytes().split(b'\0');assert cmd[1]==str(ROOT/'run_trial.py').encode()
# All prior business calls completed and none were accepted; transfer ownership of cleanup to this continuation.
os.kill(pid,signal.SIGTERM);os.kill(pid,signal.SIGCONT)
SECRET=json.loads((PRIVATE/'secrets.json').read_text());PNG=test_png()
save('continuation-started.json',{'at':now(),'preserves_same_session':True,'output_size':'3840x2160','maximum_additional_business_calls':16,'reason':'upstream explicitly rejected 4096 square dimensions; use documented landscape 4K for both models','old_controller_pid':pid})
try:
 thread=threading.Thread(target=monitor,daemon=True);thread.start()
 # Confirm one accepted request per model before spending the remaining bounded generation budget.
 for j,model in [(17,'gpt-image-2.5-sunburst'),(18,'gpt-image-2.5-flare')]:
  if STOP.is_set():break
  one(j,model,0,time.time())
  finished=[json.loads(l) for l in (ROOT/'controller.jsonl').read_text().splitlines() if 'business_finish' in l]
  result=next((x for x in finished if x.get('job')==j),{})
  if result.get('status')!=200:emit('valid_size_probe_stopped',job=j,status=result.get('status'));STOP.set();break
 for rnd,nums in [(0,list(range(19,27))),(1,list(range(27,33)))]:
  if STOP.is_set():break
  if rnd==1:
   choices=crows("SELECT json_build_object('id',p.id) FROM proxies p JOIN proxy_failover_health h ON h.proxy_id=p.id WHERE p.deleted_at IS NULL AND p.status='active' AND h.status='healthy' AND h.checked_at>NOW()-INTERVAL '3 minutes' AND p.id<>1290 AND p.id<>(SELECT proxy_id FROM accounts WHERE id=1738) AND (SELECT count(*) FROM accounts a WHERE a.deleted_at IS NULL AND a.proxy_id=p.id)<3 AND NOT EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=p.id) AND NOT EXISTS(SELECT 1 FROM adobe_route_health rh WHERE rh.proxy_id=p.id AND rh.retry_after>NOW()) ORDER BY h.latency_ms,p.id LIMIT 1")
   if choices:
    previous=selected()['proxy_id'];status,_=request('PUT','/api/v1/admin/accounts/1738',{'proxy_id':choices[0]['id'],'manual_proxy_enabled':False},{'X-API-Key':SECRET['admin_key']},timeout=30);emit('controlled_route_change_valid_size',from_proxy_id=previous,to_proxy_id=choices[0]['id'],status=status,replica_only=True)
    # Let native allocation settle before a new batch; do not force scheduling or change manual mode.
    deadline=time.time()+45
    while time.time()<deadline:
     if selected()['schedulable']:break
     time.sleep(2)
  offsets=[0,1.912,5.198,9.180,10.798,11.550,15,18];start=time.time()+3
  with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
   fs=[pool.submit(one,j,'gpt-image-2.5-flare' if i>=len(nums)-2 else 'gpt-image-2.5-sunburst',offsets[i],start) for i,j in enumerate(nums)]
   for f in fs:f.result()
  if not STOP.is_set():STOP.wait(60)
 if not STOP.is_set():STOP.wait(180)
 if auth_failed():
  status,_=request('POST','/api/v1/integrations/okad/accounts/1738/verify',{}, {'X-Integration-Key':SECRET['integration_key']},timeout=150);emit('post_failure_verification',status=status,selected=selected())
 export();save('continuation-finished.json',{'at':now(),'auth_failure':auth_failed(),'selected':selected()})
except BaseException as e:
 import traceback
 save('continuation-error.json',{'at':now(),'class':type(e).__name__,'frames':[{'name':x.name,'line':x.lineno} for x in traceback.extract_tb(e.__traceback__)]})
 try:export()
 except Exception:pass
finally:
 STOP.set();cleanup('valid_size_continuation_complete')
'''
compile(code,'continuation.py','exec');(R/'continuation.py').write_text(code)
f=open(R/'continuation-process.log','a');p=subprocess.Popen(['python3',str(R/'continuation.py')],stdout=f,stderr=f,start_new_session=True)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'valid_size_continuation_launched','pid':p.pid,'new_login':False,'max_calls':16,'requested_output_size':'3840x2160'}))
