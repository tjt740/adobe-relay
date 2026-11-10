import datetime,json,os,pathlib,re,subprocess,time
IDS=['7792ed46-03df-412f-9c90-f3a2a3eb0616','14a3ccc8-e8d7-4a0e-b721-62062bd73ee8','c4a2dfa3-c7b4-428b-aafb-3b1ca1adfc32','946668ca-f4fa-4668-917f-c40735e97662','c017d781-e63c-4855-b902-8188a1c9cd71','a5265452-0362-4947-af8d-d5268612b80a','381d8732-ac51-4088-b6be-f101624cc0ed','acb4bdfe-6219-4fa3-b30c-85a514ca5e78','8cc904c8-c79d-45b8-8d7f-708ae5ca812d']
def cmd(args,timeout=15):
 p=subprocess.run(args,capture_output=True,text=True,timeout=timeout)
 if p.returncode:raise RuntimeError('command_failed:'+args[0]+':'+str(p.returncode))
 return p.stdout
def sql(q):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='12s'; "+q+'; COMMIT;',capture_output=True,text=True,timeout=20)
 if p.returncode:return {'error':p.stderr[:500]}
 return json.loads(p.stdout)
def redis(*args):return cmd(['docker','exec','adobe-relay-redis-1','redis-cli','--raw',*map(str,args)])

out={'at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'read_only':True}
ids=','.join("'"+x+"'" for x in IDS)
out['system_log_payload_sizes']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT request_id,client_request_id,count(*) rows,max(octet_length(message)) max_message_bytes,max(octet_length(extra::text)) max_extra_bytes FROM ops_system_logs WHERE request_id IN ("+ids+") OR client_request_id IN ("+ids+") GROUP BY request_id,client_request_id) t")
out['idempotency_response_metadata']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT scope,count(*) rows,max(octet_length(response_body)) max_response_bytes FROM idempotency_records WHERE created_at>='2026-11-10T07:10:00Z' AND created_at<'2026-11-10T07:30:00Z' GROUP BY scope) t")
print(json.dumps(out,ensure_ascii=False,indent=2))
