import json,subprocess,datetime,collections

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20);assert r.returncode==0
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
a=sql("SELECT json_build_object('at',created_at,'request_id',regexp_replace(coalesce(nullif(client_request_id,''),request_id),'^client:',''),'model',model,'status',status_code,'upstream_status',upstream_status_code,'phase',error_phase,'response_latency_ms',response_latency_ms,'upstream_latency_ms',upstream_latency_ms) FROM ops_error_logs WHERE account_id=1738 AND created_at>='2026-10-27T18:14:01+08:00' AND created_at<'2026-10-27T18:17:33.235153+08:00' ORDER BY created_at")
b=sql("SELECT json_build_object('at',created_at,'request_id',regexp_replace(request_id,'^client:',''),'model',model,'duration_ms',duration_ms,'image_count',image_count,'image_size',image_size,'inbound_endpoint',inbound_endpoint) FROM usage_logs WHERE account_id=1738 AND created_at>='2026-10-27T18:14:01+08:00' AND created_at<'2026-10-27T18:17:33.235153+08:00' ORDER BY created_at")
allids=[x['request_id'] for x in a+b if x.get('request_id')]
emit('failure_window_volume',{'successes':b,'errors':a,'unique_request_ids':len(set(allids)),'error_codes':dict(collections.Counter(str(x['upstream_status'])+':'+str(x['status']) for x in a)),'id_overlap_success_error':len(set(x['request_id'] for x in a)&set(x['request_id'] for x in b))})
