"""Review the preserved, explicit 408 rejection before one bounded confirmation."""
import datetime,hashlib,json,pathlib,re,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/adobe-image26-pricing-20261107')
RID='94b51d40-5f1a-448d-9edf-6d1ce29ee896'

def validate(events, failure, key_id, usage_count):
 assert failure['http_status']==408 and not failure['success'] and failure['outer_posts']==1
 assert failure['error']['stage']=='submit' and failure['error']['source']=='upstream' and failure['error']['upstream_status']==408
 assert failure['error']['request_id']==RID and key_id==177 and usage_count==0
 assert all(e['request_id']==RID for e in events)
 assert not any(e['msg']=='adobe_images.pipeline_completed' for e in events)
 failed=[e for e in events if e['msg']=='adobe_images.upstream_failed']
 failover=[e for e in events if e['msg']=='adobe_images.account_failover']
 terminal=[e for e in events if e['msg']=='http request completed']
 assert len(failed)==len(failover)==16 and len(terminal)==1 and terminal[0]['status_code']==408
 assert {e['account_id'] for e in failed}=={e['account_id'] for e in failover}
 assert sorted(e['attempts'] for e in failover)==list(range(1,17))
 for e in failed+failover:
  assert e['api_key_id']==key_id and e['stage']=='submit' and e['job_id']==''
  assert e['upstream_http_status']==408 and e['stage_attempts']==3 and e['known_load_shedding']
 return {'request_id':RID,'api_key_id':key_id,'outer_posts':1,'rejected_accounts':16,'rejected_submit_attempts':48,'usage_count':0,'explicit_preacceptance_rejection':True,'accepted_or_unknown':False,'bounded_confirmation_cases':6,'recheck_case':'flare-xhigh'}

def main():
 failure_path=ROOT/'public/flare-xhigh-result.json';failure=json.loads(failure_path.read_text())
 key=json.loads((ROOT/'business-key-created.json').read_text())['id']
 assert [p.name for p in (ROOT/'public').glob('*-started.json')]==['flare-xhigh-started.json']
 cleanup=json.loads((ROOT/'business-access-cleanup.json').read_text())
 assert cleanup['temporary_key_deleted'] and cleanup['temporary_group_deleted']
 events=[]
 for command in [['docker','exec','adobe-relay-app-1','tail','-c','67108864','/app/data/logs/sub2api.log'],['docker','logs','--since','60m','--tail','60000','adobe-relay-app-1']]:
  p=subprocess.run(command,capture_output=True,text=True,timeout=45)
  for line in (p.stdout+'\n'+p.stderr).splitlines():
   if RID not in line:continue
   try:d=json.loads(line)
   except ValueError:
    match=re.search(r'\t([^\t]+)\t(\{.*\})$',line)
    if not match:continue
    try:d=json.loads(match[2]);d['msg']=match[1];d['time']=line.split('\t')[0]
    except ValueError:continue
   if d.get('msg') not in ['adobe_images.upstream_failed','adobe_images.account_failover','adobe_images.pipeline_completed','http request completed']:continue
   e={k:d[k] for k in ['time','msg','request_id','api_key_id','account_id','stage','job_id','upstream_http_status','stage_attempts','attempts','status_code'] if k in d}
   error=d.get('error','').lower();e['known_load_shedding']='system under load' in error or 'timeout_error' in error
   if e not in events:events.append(e)
 q='BEGIN READ ONLY; SELECT count(*) FROM usage_logs WHERE api_key_id='+str(key)+'; COMMIT;'
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=q,text=True,capture_output=True,check=True)
 out=validate(events,failure,key,int(p.stdout.strip()))
 out.update(at=datetime.datetime.now(datetime.timezone.utc).isoformat(),original_result_sha256=hashlib.sha256(failure_path.read_bytes()).hexdigest(),events=events,read_only=True)
 with (ROOT/'explicit-rejection-review.json').open('x') as f:json.dump(out,f,indent=2)
 print(json.dumps({k:v for k,v in out.items() if k!='events'}))

if __name__=='__main__':main()
