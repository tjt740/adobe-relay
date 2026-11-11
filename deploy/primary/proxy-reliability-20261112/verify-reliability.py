"""Read-only production verification: exclusions, telemetry, rules and natural diagnostics."""
import datetime,hashlib,importlib.util,json,pathlib,re,subprocess,urllib.request
ROOT=pathlib.Path('/opt/adobe-relay/releases/proxy-reliability-20261112')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
sha=(ROOT/'expected-sha').read_text().strip();assert a.inspect(a.APP)['Config']['Image']=='adobe-relay:'+sha
quarantine=json.loads(a.sql("SELECT json_agg(json_build_object('id',p.id,'status',p.status,'exclusion',e.reason,'bindings',(SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=p.id),'leases',(SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=p.id))ORDER BY p.id)FROM proxies p JOIN adobe_route_exclusions e ON e.proxy_id=p.id WHERE p.id IN(1124,1160,1948,1953)"))
assert len(quarantine)==4 and all(r['status']=='inactive'and r['exclusion']==('manual_quarantine_submission_eof'if r['id']==1948 else 'manual_quarantine_https_failure')and r['bindings']==0 and r['leases']==0 for r in quarantine)
installed=json.loads((ROOT/'alerts-installed.json').read_text());before=json.loads((ROOT/'alerts-before.json').read_text());ids=[r['id']for r in installed['added']]
rows=json.loads(a.sql("SELECT json_agg(to_jsonb(r)-'updated_at'-'last_triggered_at' ORDER BY id)FROM ops_alert_rules r"))
assert {r['id']:r for r in rows if r['id']not in ids}=={r['id']:r for r in before}
assert [r for r in rows if r['id']in ids]==installed['added']
key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
req=urllib.request.Request('https://creative.cheap/api/v1/admin/ops/alert-rules',headers={'x-api-key':key})
with urllib.request.urlopen(req,timeout=25)as res:alert_api=json.load(res)['data']
if isinstance(alert_api,dict):alert_api=alert_api.get('rules',alert_api.get('items'))
assert isinstance(alert_api,list)and len([r for r in alert_api if r['id']in ids and r['enabled']and not r['notify_email']])==4
s=json.loads(subprocess.check_output(['docker','exec',a.APP,'cat','/app/data/mihomo-runtime.json'],text=True));now=datetime.datetime.now(datetime.timezone.utc)
assert s['schema_version']==1 and s['sample_ok']and s['running']and s['memory_limit_bytes']==1610612736
assert 0<=(now-datetime.datetime.fromisoformat(s['observed_at'])).total_seconds()<60
original=json.loads((ROOT/'observer-installed.json').read_text())['snapshot']
assert all(s[k]==original[k]for k in ['container_id','started_at','restart_count','oom_events','oom_kills'])
assert subprocess.check_output(['systemctl','is-active','adobe-mihomo-observer.timer'],text=True).strip()=='active'
heartbeat=json.loads(a.sql("SELECT to_jsonb(h)FROM ops_job_heartbeats h WHERE job_name='ops_alert_evaluator'"))
cleanup=json.loads((ROOT/'rolling-finished.json').read_text())
assert datetime.datetime.fromisoformat(heartbeat['last_success_at'])>datetime.datetime.fromisoformat(cleanup['at'])
assert not heartbeat['last_error_at']or heartbeat['last_success_at']>heartbeat['last_error_at']
assert re.search(r'rules=14 enabled=14 evaluated=(?:1[2-4]) ',heartbeat['last_result']), 'New metrics must be evaluated by the formal instance'
health=json.loads(a.sql("SELECT json_build_object('checked',(SELECT count(*)FROM proxy_failover_health WHERE checked_at>now()-interval '2 minutes'),'https_evidence',(SELECT count(*)FROM proxy_failover_health WHERE checked_at>now()-interval '2 minutes'AND message LIKE '%https_api=%'),'examples',(SELECT coalesce(json_agg(x),'[]')FROM(SELECT proxy_id,status,message,checked_at FROM proxy_failover_health WHERE checked_at>now()-interval '2 minutes'AND message LIKE '%https_api=%' ORDER BY checked_at DESC LIMIT 3)x))"))
assert health['https_evidence']>0,'Wait for natural HTTPS health checks from new application'
# Only return bounded metadata; never extract complete event details or user requests.
cutover=json.loads((ROOT/'rolling-route-canary.json').read_text())['at'];assert re.fullmatch(r'[0-9T:+.Z-]+',cutover)
diag=json.loads(a.sql("SELECT json_build_object('final_403',count(*) FILTER(WHERE upstream_status_code=403),'with_fingerprint',count(*) FILTER(WHERE upstream_status_code=403 AND upstream_errors::text LIKE '%credential_fingerprint%')) FROM ops_error_logs WHERE created_at>='"+cutover+"'::timestamptz"))
traffic=json.loads(a.sql("SELECT json_build_object('usage',(SELECT json_build_object('records',count(*),'images',coalesce(sum(image_count),0))FROM usage_logs WHERE created_at>='"+cutover+"'::timestamptz),'errors',(SELECT coalesce(json_agg(e),'[]')FROM(SELECT status_code,count(*) AS count FROM ops_error_logs WHERE created_at>='"+cutover+"'::timestamptz GROUP BY status_code ORDER BY status_code)e))"))
mail_events=int(a.sql('SELECT count(*)FROM ops_alert_events WHERE rule_id IN('+','.join(map(str,ids))+')AND email_sent'))
assert mail_events==0
out={'at':now.isoformat(),'passed':True,'application_sha':sha,'quarantine':quarantine,'rule_ids':ids,'original_alert_rules_unchanged':True,'alert_api_verified':True,'site_only_alerts':True,'observer_snapshot':s,'memory_percent':s['memory_current_bytes']*100/s['memory_limit_bytes'],'formal_alert_heartbeat':heartbeat,'https_probe_evidence':health,'natural_403_diagnostics':diag,'natural_traffic_since_cutover':traffic,'active_generation_tests':0,'forced_online_failures':0,'test_mail_sent':False}
a.save('reliability-verification.json',out);print(json.dumps(out,ensure_ascii=False))
