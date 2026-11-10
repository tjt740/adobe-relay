import datetime,gzip,json,pathlib,re,subprocess
def sql(q):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='15s'; "+q+'; COMMIT;',capture_output=True,text=True,timeout=22)
 if p.returncode:return {'error':p.stderr[:800]}
 return json.loads(p.stdout)
def scrub(v):
 if isinstance(v,dict):return {k:scrub(x) for k,x in v.items() if k.lower() not in ('authorization','cookie','access_token','refresh_token','prompt','request_body','response_body','request_headers','response_headers','b64_json')}
 if isinstance(v,list):return [scrub(x) for x in v]
 if isinstance(v,str):
  v=re.sub(r'https?://[^\s"<>]+','[URL]',v)
  v=re.sub(r'(?i)Bearer\s+[^\s"<>]+','Bearer [REDACTED]',v)
  v=re.sub(r'sk-[A-Za-z0-9_-]{8,}','[KEY]',v)
  return v[:3500]
 return v
window="group_id IN (28,51) AND ((created_at >= '2026-11-09T23:10:00Z' AND created_at < '2026-11-09T23:40:00Z') OR (created_at >= '2026-11-10T07:10:00Z' AND created_at < '2026-11-10T07:40:00Z'))"
queries={
 'usage_window':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,created_at-duration_ms*interval '1 millisecond' estimated_start,request_id,upstream_request_id,user_id,api_key_id,group_id,account_id,model,image_count,image_size,image_quality,total_cost,actual_cost,duration_ms FROM usage_logs WHERE "+window+" ORDER BY created_at LIMIT 240) t",
 'errors_window':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,created_at-duration_ms*interval '1 millisecond' estimated_start,request_id,client_request_id,user_id,api_key_id,group_id,account_id,model,request_path,status_code,error_phase,error_type,error_message,error_body,upstream_status_code,upstream_error_message,upstream_error_detail,upstream_errors,duration_ms FROM ops_error_logs WHERE "+window+" ORDER BY created_at LIMIT 160) t",
 'relevant_tables':"SELECT coalesce(json_agg(table_name),'[]') FROM information_schema.tables WHERE table_schema='public' AND (table_name LIKE '%log%' OR table_name LIKE '%task%' OR table_name LIKE '%image%')",
 'daily_count':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT group_id,count(*) n,min(created_at) first_at,max(created_at) last_at FROM usage_logs WHERE group_id IN (28,51) AND created_at>='2026-11-09T16:00:00Z' GROUP BY group_id) t"
}
ids=set();out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
for k,q in queries.items():
 value=sql(q);out[k]=scrub(value)
 if k in ('usage_window','errors_window') and isinstance(value,list):
  for row in value:
   for field in ('request_id','client_request_id'):
    if row.get(field):ids.add(row[field].removeprefix('client:'))
print(json.dumps(out,ensure_ascii=False),flush=True)
prefixes=('20261110071951','20261110071329','20261110071401')
files=sorted(pathlib.Path('/opt/adobe-relay/shared/data/logs').glob('sub2api*'))
files=[p for p in files if p.is_file()]
print(json.dumps({'log_files':[{'name':p.name,'bytes':p.stat().st_size,'mtime':p.stat().st_mtime} for p in files]},ensure_ascii=False),flush=True)
matches=0;scanned=0
for p in files:
 if p.stat().st_mtime < 1794240000:continue
 op=gzip.open if p.suffix=='.gz' else open
 with op(p,'rt',errors='replace') as f:
  for line in f:
   scanned+=1
   exact=any(x in line for x in prefixes)
   if exact or any(x in line for x in ids):
    try:v=json.loads(line)
    except:
     try:v={'prefix':line[:line.index('{')],'fields':json.loads(line[line.index('{'):])}
     except:v={'line':line.strip()}
    matches+=1
    if matches<=700:print(json.dumps({'log_file':p.name,'exact_external_id':exact,'match':scrub(v)},ensure_ascii=False),flush=True)
print(json.dumps({'scanned_lines':scanned,'matches':matches,'emitted_limit':700}),flush=True)
