import datetime,json,subprocess

def sql(q):
    p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='12s'; "+q+'; COMMIT;',capture_output=True,text=True,timeout=20)
    if p.returncode:return {'error':p.stderr[:800]}
    try:return json.loads(p.stdout)
    except:return p.stdout[:1200]

queries={
 'groups':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,name,platform,status FROM groups WHERE name ILIKE '%blue%') t",
 'columns':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('usage_logs','ops_error_logs','image_tasks') ORDER BY table_name,ordinal_position) t",
 'exact_usage':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,request_id,to_jsonb(usage_logs)->>'upstream_request_id' AS upstream_request_id,user_id,api_key_id,group_id,account_id,model,image_count,image_size,image_quality,total_cost,actual_cost,duration_ms FROM usage_logs WHERE created_at>='2026-11-09T16:00:00Z' AND created_at<'2026-11-10T16:00:00Z' AND (request_id LIKE '%202611100719514982342308268d9d6%' OR request_id LIKE '%202611100713296715092628268d9d6%' OR request_id LIKE '%202611100714014629667448268d9d6%' OR to_jsonb(usage_logs)->>'upstream_request_id' LIKE '%202611100719514982342308268d9d6%' OR to_jsonb(usage_logs)->>'upstream_request_id' LIKE '%202611100713296715092628268d9d6%' OR to_jsonb(usage_logs)->>'upstream_request_id' LIKE '%202611100714014629667448268d9d6%') LIMIT 20) t",
 'exact_errors':"SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,request_id,user_id,api_key_id,group_id,account_id,model,request_path,status_code,error_phase,error_type,upstream_status_code,duration_ms FROM ops_error_logs WHERE created_at>='2026-11-09T16:00:00Z' AND created_at<'2026-11-10T16:00:00Z' AND (request_id LIKE '%202611100719514982342308268d9d6%' OR request_id LIKE '%202611100713296715092628268d9d6%' OR request_id LIKE '%202611100714014629667448268d9d6%') LIMIT 20) t"
}
out={'at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'read_only':True}
for k,q in queries.items():out[k]=sql(q)
print(json.dumps(out,ensure_ascii=False,indent=2))

import datetime,json,re,subprocess
prefixes=('20261110071951','20261110071329','20261110071401')
def scrub(v):
    if isinstance(v,dict):return {k:scrub(x) for k,x in v.items() if k.lower() not in ('authorization','cookie','access_token','refresh_token','prompt','request_body','response_body','request_headers','response_headers','b64_json')}
    if isinstance(v,list):return [scrub(x) for x in v]
    if isinstance(v,str):
        v=re.sub(r'https?://[^\s"<>]+','[URL]',v)
        v=re.sub(r'(?i)Bearer\s+[^\s"<>]+','Bearer [REDACTED]',v)
        return v[:3000]
    return v
p=subprocess.run(['docker','inspect','adobe-relay-app-1'],capture_output=True,text=True,timeout=10)
d=json.loads(p.stdout)[0]
out={'at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'image':d['Config']['Image'],'started':d['State']['StartedAt'],'mounts':[{'source':x['Source'],'dest':x['Destination']} for x in d['Mounts']],'matches':[]}
print(json.dumps({'runtime':out},ensure_ascii=False),flush=True)
p=subprocess.Popen(['docker','logs','--since','2026-11-09T16:00:00Z','--until','2026-11-10T09:30:00Z','adobe-relay-app-1'],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
n=0
for line in p.stdout:
    n+=1
    if any(x in line for x in prefixes):
        try:entry=json.loads(line)
        except:entry=line.strip()
        print(json.dumps({'match':scrub(entry)},ensure_ascii=False),flush=True)
print(json.dumps({'lines_scanned':n,'exit_code':p.wait()}),flush=True)
