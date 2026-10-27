import json,subprocess,datetime

def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='20s';"+q+';COMMIT;',capture_output=True,text=True,timeout=30)
 if r.returncode:raise RuntimeError('readonly_query_failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
windows=[('r5','2026-10-27T18:18:35Z','2026-10-27T18:21:10Z'),('r6','2026-10-27T18:30:00Z','2026-10-27T18:40:55Z'),('r7','2026-10-27T18:47:27Z','2026-10-27T18:54:00Z')]
for tag,start,end in windows:
 q="SELECT json_build_object('kind','success','accounts',count(DISTINCT u.account_id),'requests',count(*)) FROM usage_logs u JOIN accounts a ON a.id=u.account_id WHERE a.platform='adobe' AND u.created_at>='"+start+"' AND u.created_at<'"+end+"'; SELECT json_build_object('kind','errors','accounts',count(DISTINCT account_id),'requests',count(*),'phase',error_phase,'status',status_code,'upstream_status',upstream_status_code) FROM ops_error_logs WHERE platform='adobe' AND created_at>='"+start+"' AND created_at<'"+end+"' GROUP BY error_phase,status_code,upstream_status_code; SELECT json_build_object('kind','error_events_proxy_1335','events',count(*),'accounts',count(DISTINCT e->>'account_id'),'upstream_status',e->>'upstream_status_code') FROM ops_error_logs l CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(l.upstream_errors)='array' THEN l.upstream_errors ELSE '[]'::jsonb END) e WHERE l.created_at>='"+start+"' AND l.created_at<'"+end+"' AND e->>'proxy_id'='1335' GROUP BY e->>'upstream_status_code'"
 print(json.dumps({'kind':'production_context','trial':tag,'start':start,'end':end,'data':sql(q)}))
print(json.dumps({'kind':'current_proxy_1335_occupancy','data':sql("SELECT json_build_object('status',status,'schedulable',schedulable,'accounts',count(*)) FROM accounts WHERE deleted_at IS NULL AND proxy_id=1335 GROUP BY status,schedulable")}))
