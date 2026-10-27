import json,subprocess,datetime,sqlite3

def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='35s';"+q+';COMMIT;',capture_output=True,text=True,timeout=45)
 if r.returncode:raise RuntimeError('read_query_failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

import hashlib,re
c=sqlite3.connect('file:/opt/adobe-team-cookie/data/app.db?mode=ro',uri=True);c.row_factory=sqlite3.Row
cols=[x['name'] for x in c.execute('PRAGMA table_info(settings)')];emit('okad_settings_columns',cols)
rows=[]
for r in c.execute('SELECT * FROM settings ORDER BY rowid'):
 d=dict(r);item={}
 for k,v in d.items():
  if k in ('id','key','name','created_at','updated_at'):item[k]=v
  else:item[k+'_sha256']=hashlib.sha256(str(v).encode()).hexdigest()
 rows.append(item)
emit('okad_settings_metadata',rows)
emit('okad_audit_tables',[x[0] for x in c.execute("SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%log%' OR name LIKE '%audit%')")]);c.close()
base="FROM ops_error_logs l CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(l.upstream_errors)='array' THEN l.upstream_errors ELSE '[]'::jsonb END) e WHERE l.platform='adobe' AND l.created_at >= '2026-10-26T13:12:02Z' AND l.created_at < '2026-10-27T19:12:02Z'"
emit('entry_status_distribution',sql("SELECT json_build_object('parent_status',l.upstream_status_code,'entry_status',e->>'upstream_status_code','stage',e->>'stage','kind',e->>'kind','count',count(*)) "+base+" AND l.upstream_status_code=408 GROUP BY l.upstream_status_code,e->>'upstream_status_code',e->>'stage',e->>'kind'"))
emit('true_408_entries',sql("SELECT json_build_object('entry_status',e->>'upstream_status_code','stage',e->>'stage','attempts',substring(e->>'detail' FROM '\"stage_attempts\"[[:space:]]*:[[:space:]]*([0-9]+)'),'count',count(*),'accounts',count(DISTINCT e->>'account_id'),'proxies',count(DISTINCT e->>'proxy_id'),'load_message',bool_and(coalesce(e->>'message','') ILIKE '%system under load%')) "+base+" AND e->>'upstream_status_code'='408' GROUP BY e->>'upstream_status_code',e->>'stage',substring(e->>'detail' FROM '\"stage_attempts\"[[:space:]]*:[[:space:]]*([0-9]+)')"))
emit('recent_activity',sql("SELECT json_build_object('hour',date_trunc('hour',created_at),'status',upstream_status_code,'count',count(*),'accounts',count(DISTINCT account_id)) FROM ops_error_logs WHERE platform='adobe' AND created_at>now()-interval '7 hours' GROUP BY date_trunc('hour',created_at),upstream_status_code ORDER BY date_trunc('hour',created_at)"))
emit('recent_successes',sql("SELECT json_build_object('hour',date_trunc('hour',u.created_at),'count',count(*),'accounts',count(DISTINCT u.account_id)) FROM usage_logs u JOIN accounts a ON a.id=u.account_id WHERE a.platform='adobe' AND u.created_at>now()-interval '7 hours' GROUP BY date_trunc('hour',u.created_at) ORDER BY date_trunc('hour',u.created_at)"))
