"""Read-only authentication preflight; output aggregate metadata, never credentials."""
import collections,datetime,hashlib,importlib.util,json,pathlib,re,subprocess
root=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020')
s=importlib.util.spec_from_file_location('audit',root/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
rows=json.loads(a.sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,type,status,error_message,schedulable,credentials,extra FROM accounts WHERE deleted_at IS NULL AND platform='adobe')t"))
counts=collections.Counter();status=collections.Counter();errors=collections.Counter();details=[]
for r in rows:
 c=r.pop('credentials')or{};e=r.pop('extra')or{};o=e.get('okad')or{};h=(e.get('adobe_health')or{}).get('latest')or{};t=e.get('okad_connection_test')or{}
 status[r['status']]+=1
 msg=str(r.get('error_message')or'').lower()
 if '401' not in msg and h.get('http_status')!=401 and not ('cookie is no longer valid'in msg):continue
 counts['matching_accounts']+=1
 source=c.get('cookie')or'';runtime=c.get('adobe_session_cookie')or''
 counts['with_source_cookie']+=bool(source);counts['with_token']+=bool(c.get('access_token'));counts['runtime_bound']+=bool(runtime)and c.get('adobe_session_cookie_source_hash')==hashlib.sha256(source.encode()).hexdigest();counts['operator_paused']+=e.get('okad_manual_pause')is True
 for marker in ['automatic login limit reached','recovery pending','cookie is no longer valid','invalid bearer','credits verification','token exchange','identity verification']:
  if marker in msg:errors[marker]+=1;break
 else:errors['other_401']+=1
 details.append({'id':r['id'],'status':r['status'],'schedulable':r['schedulable'],'source_version':o.get('version'),'runtime_bound':bool(runtime)and c.get('adobe_session_cookie_source_hash')==hashlib.sha256(source.encode()).hexdigest(),'token_present':bool(c.get('access_token')),'token_expiry':c.get('expires_at'),'manual_pause':e.get('okad_manual_pause'),'recovery_state':e.get('okad_recovery'),'connection_state':t.get('state'),'health':{k:v for k,v in h.items()if k in ['at','http_status','error_code','success','model']}})
# Keep individual operational metadata on the primary host with restrictive permissions.
p=root/'401-preflight-private.json';p.write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'accounts':details},ensure_ascii=False));p.chmod(0o600)
images={}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
 d=a.inspect(name);images[name]={'image':d['Config']['Image'],'health':d['State'].get('Health',{}).get('Status'),'restarts':d['RestartCount']}
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'total_adobe_accounts':len(rows),'status_counts':dict(status),'auth_counts':dict(counts),'error_categories':dict(errors),'runtime':images,'operational_fields_changed':False,'credentials_in_output':False}),flush=True)
print(a.sql("SELECT coalesce(json_agg(table_name),'[]') FROM information_schema.tables WHERE table_schema='public' AND (table_name LIKE '%adobe%' OR table_name LIKE '%usage%')"))
