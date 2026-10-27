import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/session-controlled-20261027');IDS=[1742,1744,1749,1758];MEMBERS={1742:247,1744:245,1749:240,1758:231};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def log(kind,**kw):
 with (ROOT/'controller.jsonl').open('a') as f:f.write(json.dumps({'at':now(),'event':kind,**kw},ensure_ascii=False)+'\n')
def sql(q,raw=False):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 if r.returncode:raise RuntimeError('database_read_failed')
 return r.stdout.strip() if raw else [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
def api(method,path,body=None,timeout=35):
 assert path=='/api/v1/admin/accounts/bulk-update' or any(path==f'/api/v1/admin/accounts/{i}'+suffix for i in IDS for suffix in ('','/schedulable','/test'))
 if path.endswith('bulk-update'):assert set(body['account_ids'])<=set(IDS) and 'filters' not in body
 key=sql("SELECT value FROM settings WHERE key='admin_api_key'",True)
 req=urllib.request.Request('http://127.0.0.1:6666'+path,data=json.dumps(body).encode() if body is not None else None,headers={'x-api-key':key,'Content-Type':'application/json','X-Request-ID':'session-controlled-20261027'},method=method)
 with urllib.request.urlopen(req,timeout=timeout) as r:
  if path.endswith('/test'):
   result=[]
   for line in r:
    if line.startswith(b'data: '):
     d=json.loads(line[6:]);result.append({k:d[k] for k in ['type','success','status_code'] if k in d})
   return result
  d=json.load(r);return {'code':d.get('code'),'success_ids':(d.get('data') or {}).get('success_ids')} if isinstance(d,dict) else {}
def rows(private=False):
 q="SELECT json_build_object('id',a.id,'email',a.name,'status',a.status,'schedulable',a.schedulable,'concurrency',a.concurrency,'manual_pause',a.extra->'okad_manual_pause','manual_proxy',a.extra->'manual_proxy_enabled','proxy_id',a.proxy_id,'proxy_updated_at',p.updated_at,'proxy_status',p.status,'proxy_excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=a.proxy_id),'other_bindings',(SELECT count(*) FROM accounts b WHERE b.deleted_at IS NULL AND b.proxy_id=a.proxy_id AND b.id<>a.id),'active_lease',EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW()),'version',a.extra->'okad'->'version','verification',a.extra->'okad'->'verification','test',a.extra->'okad_connection_test','error',a.error_message,'cookie_hash',md5(a.credentials->>'cookie'),'runtime_cookie_hash',md5(a.credentials->>'adobe_session_cookie'),'token_hash',md5(a.credentials->>'access_token'),'token',a.credentials->>'access_token','arp_session_id',a.credentials->>'arp_session_id','proxy',json_build_object('host',p.host,'port',p.port,'protocol',p.protocol,'username',p.username,'password',p.password)) FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id WHERE a.deleted_at IS NULL AND a.id IN ("+','.join(map(str,IDS))+') ORDER BY a.id'
 data=sql(q)
 if not private:
  for a in data:
   for k in ['token','arp_session_id','proxy']:a.pop(k,None)
 return data
def save(name,data):
 p=ROOT/name;tmp=ROOT/(name+'.tmp');tmp.write_text(json.dumps(data,ensure_ascii=False,indent=2));tmp.replace(p)
def others():
 return sql("SELECT json_build_object('id',id,'config_hash',md5(json_build_object('concurrency',concurrency,'priority',priority,'groups',(SELECT json_agg(group_id ORDER BY group_id) FROM account_groups WHERE account_id=accounts.id),'manual_pause',extra->'okad_manual_pause','manual_proxy',extra->'manual_proxy_enabled')::text),'cookie_hash',md5(credentials->>'cookie')) FROM accounts WHERE deleted_at IS NULL AND id NOT IN ("+','.join(map(str,IDS))+') ORDER BY id')
def login(roundno):
 code=r'''
import json,sqlite3,urllib.request,concurrent.futures,sys
from app.core.security import create_access_token
c=sqlite3.connect('file:/app/data/app.db?mode=ro',uri=True);uid,version=c.execute('SELECT id,token_version FROM users WHERE is_active=1 AND is_superuser=1 LIMIT 1').fetchone();c.close();token=create_access_token(uid,expires_minutes=10,token_version=version)
def one(mid):
 req=urllib.request.Request(f'http://127.0.0.1:9500/api/external/members/{mid}/login',data=b'{}',headers={'Authorization':'Bearer '+token,'Content-Type':'application/json'},method='POST')
 try:
  with urllib.request.urlopen(req,timeout=360) as r:d=json.load(r);return {'member_id':mid,'http_status':r.status,'ok':d.get('ok'),'code':d.get('code')}
 except Exception as e:return {'member_id':mid,'error':type(e).__name__}
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as ex:print(json.dumps(list(ex.map(one,[247,245,240,231]))))
'''
 log('login_start',round=roundno,member_ids=list(MEMBERS.values()))
 # This command is never replayed after an unknown outcome.
 r=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',code],capture_output=True,text=True,timeout=390)
 result=json.loads(r.stdout.strip().splitlines()[-1]) if r.returncode==0 else [{'error':'login_command_failed'}]
 save(f'round-{roundno}-login.json',result);log('login_finished',round=roundno,results=result)
 if len(result)!=4 or any(x.get('ok') is not True for x in result):raise RuntimeError('login_not_all_successful')
def ready(roundno,previous):
 until=time.monotonic()+300
 while time.monotonic()<until:
  current=rows();assert len(current)==4
  for a in current:
   b=previous[a['id']]
   if a['manual_pause'] is not True or a['manual_proxy'] is not True or a['proxy_id']!=b['proxy_id']:raise RuntimeError('isolation_changed')
  if all(a['version']>previous[a['id']]['version'] and a['verification']=='verified' and a['test']['state'] in ('success','failed','interrupted') and not a['active_lease'] and a['token_hash'] for a in current):
   time.sleep(15);after=rows()
   if all((x['token_hash'],x['runtime_cookie_hash'],x['version'])==(y['token_hash'],y['runtime_cookie_hash'],y['version']) for x,y in zip(current,after)):
    save(f'round-{roundno}-ready.json',after);return after
  time.sleep(5)
 raise RuntimeError('credential_or_connection_check_not_ready')
def run_round(n):
 assignments={1742:1 if n==1 else 3,1744:3 if n==1 else 1,1749:3 if n==1 else 1,1758:1 if n==1 else 3}
 for i,c in assignments.items():api('PUT',f'/api/v1/admin/accounts/{i}',{'concurrency':c})
 previous={x['id']:x for x in rows()};login(n);baseline=ready(n,previous);private=rows(True)
 for a in private:
  assert a['other_bindings']==0 and not a['proxy_excluded'] and a['proxy_status']=='active'
 c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);payload=[]
 for a in private:
  p=a['proxy'];auth=(urllib.parse.quote(p['username'] or '',safe='')+':'+urllib.parse.quote(p['password'] or '',safe='')+'@') if p['username'] else ''
  proxy=p['protocol']+'://'+auth+p['host']+':'+str(p['port']);original=c.execute('SELECT access_token FROM external_members WHERE id=?',(MEMBERS[a['id']],)).fetchone()[0]
  payload.append({'id':a['id'],'email':a['email'],'token':a['token'],'original_token':original,'proxy':proxy,'proxy_id':a['proxy_id'],'concurrency':assignments[a['id']],'arp_session_id':a['arp_session_id'] or ''})
 c.close();guard=ROOT/f'round-{n}-guard';guard.mkdir(exist_ok=True)
 cfg={'round':n,'accounts':payload,'duration_sec':900,'max_jobs':12,'model':'gpt-image-2.5-sunburst','guard_dir':'/probe/'+guard.name}
 container=f'adobe-session-controlled-r{n}-20261027';stream=open(ROOT/f'round-{n}.jsonl','x');errlog=open(ROOT/f'round-{n}-stderr.log','x')
 cmd=['docker','run','--rm','-i','--name',container,'--network','adobe-relay_default','--memory','512m','--cpus','2','--read-only','--mount',f'type=bind,src={ROOT},dst=/probe,readonly','--entrypoint','/probe/probe','adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c']
 proc=subprocess.Popen(cmd,stdin=subprocess.PIPE,stdout=stream,stderr=errlog,text=True);proc.stdin.write(json.dumps(cfg));proc.stdin.close();del cfg,payload,private
 log('round_running',round=n,assignments=assignments,container=container)
 initial={x['id']:x for x in baseline};guard_events=[];hard_end=time.monotonic()+1200
 while proc.poll() is None:
  if time.monotonic()>hard_end:(guard/'abort-all').write_text('hard deadline');raise RuntimeError('harness_deadline')
  if (ROOT/'abort-all').exists():(guard/'abort-all').write_text('operator abort')
  try:
   observed=[json.loads(line) for line in (ROOT/f'round-{n}.jsonl').read_text().splitlines() if line.strip()]
   exits={x['account_id']:x.get('ip_sha256') for x in observed if x.get('event')=='egress' and x.get('verified')}
   if len(exits)==4 and len(set(exits.values()))<4:
    (guard/'abort-all').write_text('shared physical exit');raise RuntimeError('selected_proxies_share_public_exit')
   snap=rows();issues=[]
   for a in snap:
    b=initial[a['id']];fields=[k for k in ['manual_pause','manual_proxy','proxy_id','proxy_updated_at','version','cookie_hash','runtime_cookie_hash','token_hash','concurrency'] if a[k]!=b[k]]
    if fields or a['schedulable'] or a['other_bindings'] or a['active_lease']:
     (guard/f"abort-{a['id']}").write_text('state changed');issues.append({'id':a['id'],'changed_fields':fields,'schedulable':a['schedulable'],'other_bindings':a['other_bindings'],'active_test_lease':a['active_lease']})
   if issues and (not guard_events or guard_events[-1]['issues']!=issues):guard_events.append({'at':now(),'issues':issues});log('guard_change',round=n,issues=issues)
  except Exception as e:
   (guard/'abort-all').write_text('guard query failed');log('guard_query_failed',round=n,error=type(e).__name__)
  time.sleep(5)
 stream.close();errlog.close();save(f'round-{n}-guard-events.json',guard_events);save(f'round-{n}-after.json',rows());log('round_finished',round=n,returncode=proc.returncode)
 if proc.returncode:raise RuntimeError('harness_failed')
 if guard_events:raise RuntimeError('isolation_guard_changed')
def restore():
 if (ROOT/'restored.json').exists():return
 before=json.loads((ROOT/'before.json').read_text());log('restore_start')
 current={x['id']:x for x in rows()};actions=[]
 for a in before:
  i=a['id'];cur=current.get(i)
  if not cur:actions.append({'id':i,'error':'account_missing'});continue
  if cur['manual_pause'] is not True or cur['manual_proxy'] not in (True,a['manual_proxy']) or cur['proxy_id']!=a['proxy_id']:
   actions.append({'id':i,'error':'settings_changed_outside_test_preserved'});continue
  # Preserve all new credentials, recovery budgets, and current health/status.
  api('PUT',f'/api/v1/admin/accounts/{i}',{'concurrency':a['concurrency'],'manual_proxy_enabled':bool(a['manual_proxy']),'proxy_id':a['proxy_id']})
  api('POST',f'/api/v1/admin/accounts/{i}/schedulable',{'schedulable':a['schedulable']})
  api('POST','/api/v1/admin/accounts/bulk-update',{'account_ids':[i],'extra':{'okad_manual_pause':a['manual_pause'],'manual_proxy_enabled':a['manual_proxy']}})
  actions.append({'id':i,'restored':True})
 after=rows();save('restored.json',{'at':now(),'actions':actions,'accounts':after});save('other-accounts-after.json',others());log('restore_finished',actions=actions)
def watchdog():
 deadline=time.monotonic()+4200
 while time.monotonic()<deadline:
  if (ROOT/'restored.json').exists():return
  time.sleep(20)
 (ROOT/'abort-all').write_text('watchdog deadline');log('watchdog_abort')
 # Let accepted jobs finish naturally before removing isolation.
 for n in [1,2]:
  p=ROOT/f'round-{n}-guard'
  if p.exists():(p/'abort-all').write_text('watchdog deadline')
 time.sleep(240);restore()
def main():
 if len(sys.argv)>1 and sys.argv[1]=='watchdog':watchdog();return
 if len(sys.argv)>1 and sys.argv[1]=='restore':restore();return
 if (ROOT/'started.json').exists():raise RuntimeError('already_started_no_replay')
 assert subprocess.check_output(['docker','inspect','--format','{{.Config.Image}}','adobe-relay-app-1'],text=True).strip()==EXPECTED
 before=rows();assert len(before)==4
 for a in before:
  assert a['status']=='error' and not a['schedulable'] and a['manual_pause'] is not True and a['manual_proxy'] is not True and a['other_bindings']==0 and not a['active_lease'] and not a['proxy_excluded'] and a['test']['state'] not in ('running','pending','proxy_pending')
 save('before.json',before);save('other-accounts-before.json',others());save('started.json',{'at':now(),'ids':IDS,'max_rounds':2,'max_generation_jobs_per_round_per_account':12,'max_seconds_per_round':900})
 subprocess.Popen(['python3',str(ROOT/'controller.py'),'watchdog'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
 try:
  for i in IDS:
   api('POST',f'/api/v1/admin/accounts/{i}/schedulable',{'schedulable':False});api('PUT',f'/api/v1/admin/accounts/{i}',{'manual_proxy_enabled':True})
  log('isolated',ids=IDS);time.sleep(15)
  for n in [1,2]:run_round(n)
  save('completed.json',{'at':now(),'rounds':2})
 except Exception as e:
  log('controller_error',error=type(e).__name__,reason=str(e)[:180]);save('failed.json',{'at':now(),'error':type(e).__name__,'reason':str(e)[:180]})
 finally:
  # No test worker is force-killed; let any already accepted work drain.
  running=subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines()
  if any(x.startswith('adobe-session-controlled-r') for x in running):
   for n in [1,2]:
    p=ROOT/f'round-{n}-guard'
    if p.exists():(p/'abort-all').write_text('controller exit')
   for _ in range(48):
    running=subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines()
    if not any(x.startswith('adobe-session-controlled-r') for x in running):break
    time.sleep(5)
   else:log('restore_deferred_running_probe');return
  restore()
if __name__=='__main__':main()
