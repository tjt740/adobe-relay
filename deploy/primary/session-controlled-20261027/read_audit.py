import json,subprocess,datetime,sqlite3

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='35s';"+q+';COMMIT;',capture_output=True,text=True,timeout=45)
 if r.returncode:raise RuntimeError('read_query_failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

import hashlib,re
start='2026-10-27T19:58:00Z';end='2026-10-27T20:04:15Z'
r=subprocess.run(['docker','logs','--since',start,'--until',end,'adobe-team-cookie-app-1'],capture_output=True,text=True,timeout=30)
lines=(r.stdout+r.stderr).splitlines();matches=[l for l in lines if 'donaldsimril48' in l.lower() or re.search(r'(member_id|external_id|account_id)[= :]+69(?:\D|$)',l)]
emit('production_okad_donald_overlap',{'since_utc':start,'until_utc':end,'line_count':len(lines),'matching_line_count':len(matches),'matching_line_sha256':[hashlib.sha256(l.encode()).hexdigest() for l in matches],'note':'search original production container only; no raw logs or credentials exported'})
emit('production_adobe_activity_donald_window',sql("SELECT json_build_object('kind','errors','upstream_status',upstream_status_code,'count',count(*),'account_ids',array_agg(DISTINCT account_id) FILTER(WHERE account_id IS NOT NULL)) FROM ops_error_logs WHERE platform='adobe' AND created_at>='2026-10-27T19:58:00Z' AND created_at<='2026-10-27T20:04:15Z' GROUP BY upstream_status_code; SELECT json_build_object('kind','success','count',count(*)) FROM usage_logs u JOIN accounts a ON a.id=u.account_id WHERE a.platform='adobe' AND u.created_at>='2026-10-27T19:58:00Z' AND u.created_at<='2026-10-27T20:04:15Z'"))
emit('current_1335_account_bindings',sql("SELECT json_build_object('id',id,'status',status,'schedulable',schedulable) FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND proxy_id=1335"))
