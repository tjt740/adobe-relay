import datetime, hashlib, json, pathlib, subprocess
root=pathlib.Path('/opt/adobe-relay/releases/proxy-502-followup-20261112')
before=json.loads((root/'quarantine-before.json').read_text())
applied=json.loads((root/'quarantine-applied.json').read_text())
def sql(q):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='10s';"+q+';COMMIT;',text=True,capture_output=True,check=True,timeout=20)
 return json.loads(p.stdout.strip())
start=applied['at'];ids=','.join(str(a['id']) for a in before['accounts'])
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'since':start}
out['traffic']=sql("SELECT json_build_object('success_records',(SELECT count(*) FROM usage_logs WHERE created_at>='"+start+"'),'images',(SELECT coalesce(sum(image_count),0) FROM usage_logs WHERE created_at>='"+start+"'),'errors',(SELECT coalesce(json_agg(t),'[]') FROM (SELECT status_code,count(*) n FROM ops_error_logs WHERE created_at>='"+start+"' AND status_code>=400 GROUP BY status_code ORDER BY status_code)t))")
out['remaining_502']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT upstream_errors->-1->>'proxy_id' proxy,upstream_errors->-1->>'stage' stage,count(*) n,max(created_at) last_at FROM ops_error_logs WHERE created_at>='"+start+"' AND status_code=502 GROUP BY 1,2 ORDER BY 3 DESC)t")
out['new_requests_excluded_nodes']=sql("SELECT json_build_object('observations',count(*),'successes',count(*) FILTER(WHERE success)) FROM adobe_route_observations WHERE started_at>='"+start+"' AND proxy_id IN(1120,1149)")
out['accounts']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,status,schedulable,proxy_id,md5(credentials::text) credentials_hash,extra->>'okad_manual_pause' manual_pause,extra->>'manual_proxy_enabled' manual_proxy FROM accounts WHERE id IN("+ids+") ORDER BY id)t")
out['auto_switch_events']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT account_id,from_proxy_id,to_proxy_id,created_at FROM proxy_failover_events WHERE account_id IN("+ids+") AND created_at>='"+start+"' ORDER BY created_at)t")
by_id={a['id']:a for a in out['accounts']}
out['original_account_state_preserved']=all(all(by_id[a['id']][k]==a[k] for k in ['status','schedulable','credentials_hash','manual_pause','manual_proxy']) for a in before['accounts'])
out['excluded']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT p.id,p.status,e.reason,(SELECT count(*) FROM accounts a WHERE a.proxy_id=p.id AND a.deleted_at IS NULL) bindings FROM proxies p JOIN adobe_route_exclusions e ON p.id=e.proxy_id WHERE p.id IN(1120,1149,1124,1160,1948,1953) ORDER BY p.id)t")
out['runtime']=json.loads(pathlib.Path('/opt/adobe-relay/shared/data/mihomo-runtime.json').read_text())
out['configuration_preserved']=all(hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()==h for p,h in before['configuration'].items())
ds=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1','adobe-relay-mihomo-1']))
out['containers']={d['Name']:{'id':d['Id'],'image':d['Config']['Image'],'pid':d['State']['Pid'],'started':d['State']['StartedAt'],'restarts':d['RestartCount'],'health':d['State'].get('Health',{}).get('Status')}for d in ds}
print(json.dumps(out,ensure_ascii=False,indent=2))
