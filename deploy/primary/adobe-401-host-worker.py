"""Authoritative database guards plus one manual authorization per confirmed 401."""
import ast,collections,concurrent.futures,datetime,importlib.util,json,os,pathlib,subprocess,time,urllib.request
os.umask(0o077)
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');batch=root/'401-manual-batch-v2'
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
rows=json.loads((batch/'input-private.json').read_text());key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
program=(batch/'login-worker-private.py').read_text()

def current(row):
 values=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('status',status,'paused',extra->'okad_manual_pause','cookie_hash',md5(credentials->>'cookie'),'email',COALESCE(credentials->>'email',''),'source_key',extra->'okad'->>'source_key','expired',expires_at IS NOT NULL AND expires_at<=NOW())),'[]') FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND id="+str(int(row['id']))))
 return values[0]if values else{}

def matches(row,r):return r.get('status')=='error'and r.get('paused')is not True and not r.get('expired')and r.get('cookie_hash')==row['source_hash']and r.get('email','').lower()==row['email'].lower()and r.get('source_key')==row['source_key']

def repair(row):
 marker=batch/('account-'+str(row['id'])+'.json')
 if marker.exists():return {'account_id':row['id'],**json.loads(marker.read_text())}
 out={'account_id':row['id'],'ok':False,'code':'not_started','at':time.time()}
 try:
  if not matches(row,current(row)):return {**out,'code':'account_changed_or_paused'}
  req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/'+str(row['id'])+'/usage?source=active&force=true',headers={'x-api-key':key})
  with urllib.request.urlopen(req,timeout=110)as response:usage=json.loads(response.read()).get('data')or{}
  if not usage.get('needs_reauth')and usage.get('error_code')!='unauthenticated':
   result={**out,'ok':not bool(usage.get('error_code')or usage.get('error')),'code':'auth_valid_no_login'if not(usage.get('error_code')or usage.get('error'))else'temporary_or_non_auth_failure'}
   if result['ok']:marker.write_text(json.dumps(result))
   else:
    message=str(usage.get('error')or'').lower()
    result['error_code']=usage.get('error_code')if usage.get('error_code')in['network_error','unauthenticated','rate_limited']else'other'
    result['kind']='entitlement'if any(x in message for x in['not entitled','not_entitled','403'])else'quota'if any(x in message for x in['quota','credits exhausted','429'])else'network'if any(x in message for x in['proxy','connect','timeout','deadline','eof'])else'verification_error'
   return result
  if not matches(row,current(row)):return {**out,'code':'account_changed_or_paused'}
  # The child also has an exclusive durable account marker and the normal coordinator.
  marker.write_text(json.dumps({**out,'code':'started_outcome_unknown'}))
  p=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-u','-c',program],input=json.dumps([row])+'\n',capture_output=True,text=True,timeout=430)
  (batch/('account-'+str(row['id'])+'-stdout-private.log')).write_text(p.stdout);(batch/('account-'+str(row['id'])+'-stderr-private.log')).write_text(p.stderr)
  result={**out,'code':'child_result_unavailable'}
  for line in p.stdout.splitlines():
   if line.startswith('401_REPAIR_RESULT '):result={**out,**json.loads(line.split(' ',1)[1])[0]}
  marker.write_text(json.dumps(result));return result
 except Exception as error:
  result={**out,'code':type(error).__name__}
  if marker.exists():marker.write_text(json.dumps(result))
  return result

results=[]
with concurrent.futures.ThreadPoolExecutor(max_workers=3)as pool:
 for future in concurrent.futures.as_completed([pool.submit(repair,r)for r in rows]):
  result=future.result();results.append(result)
  (batch/('result-'+str(result['account_id'])+'-private.json')).write_text(json.dumps(result))
  summary={'completed':len(results),'selected':len(rows),'success':sum(bool(r.get('ok'))for r in results),'codes':dict(collections.Counter(r.get('code')for r in results)),'automatic_attempts_reset':False,'at':time.time()}
  (batch/'progress.json').write_text(json.dumps(summary))
(batch/'finished.json').write_text(json.dumps(summary if results else{'selected':0,'completed':0}));print(json.dumps(summary if results else{}),flush=True)
