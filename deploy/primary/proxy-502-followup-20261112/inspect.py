import collections, datetime, json, pathlib, re, subprocess

def sql(q):
    p = subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'], input="BEGIN READ ONLY; SET LOCAL statement_timeout='12s'; "+q+'; COMMIT;', text=True,capture_output=True,timeout=25)
    if p.returncode:
        return {'query_error':p.stderr[:1000]}
    return json.loads(p.stdout.strip())

def redact(s):
    s=re.sub(r'https?://[^\s"<>]+','[url]',str(s))
    s=re.sub(r'(?i)(bearer\s+)\S+',r'\1[redacted]',s)
    s=re.sub(r'eyJ[A-Za-z0-9_.-]+','[jwt]',s)
    s=re.sub(r'(?i)((?:token|cookie|password|authorization)\s*[:=]\s*)[^,;\s]+',r'\1[redacted]',s)
    return s[:500]

out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
out['runtime']=json.loads(pathlib.Path('/opt/adobe-relay/shared/data/mihomo-runtime.json').read_text())
out['containers']=[]
for n in ['adobe-relay-app-1','adobe-relay-mihomo-1']:
    d=json.loads(subprocess.check_output(['docker','inspect',n]))[0]
    out['containers'].append({'name':n,'id':d['Id'],'image':d['Config']['Image'],'started':d['State']['StartedAt'],'restarts':d['RestartCount'],'health':d['State'].get('Health',{}).get('Status'),'oom':d['State']['OOMKilled'],'memory':d['HostConfig']['Memory']})
out['hourly']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT date_trunc('hour',created_at) AS bucket,status_code,count(*) n FROM ops_error_logs WHERE created_at>now()-interval '8 hours' AND status_code>=400 GROUP BY 1,2 ORDER BY 1,2) t")
out['usage_2h']=sql("SELECT json_build_object('records',count(*),'images',sum(image_count)) FROM usage_logs WHERE created_at>now()-interval '2 hours'")
rows=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,account_id,upstream_status_code,upstream_errors->-1 last,upstream_error_message FROM ops_error_logs WHERE created_at>now()-interval '2 hours' AND status_code=502 ORDER BY id DESC LIMIT 5000) t")
if isinstance(rows,dict):out['query_error']=rows
else:
    items=[]
    for r in rows:
        e=r.get('last') or {}; detail=e.get('detail') or '{}'
        try:d=json.loads(detail) if isinstance(detail,str) else detail
        except ValueError:d={}
        items.append({'id':r['id'],'at':r['created_at'],'outer_account':r['account_id'],'account':e.get('account_id'),'proxy':e.get('proxy_id'),'status':r['upstream_status_code'],'stage':d.get('stage'),'code':d.get('code'),'kind':e.get('kind'),'message':redact(e.get('message') or r['upstream_error_message']),'detail_keys':list(d),'detail_safe':{k:d.get(k) for k in ['stage','code','reason','status_code','upstream_status_code']},'credential_fingerprint':e.get('credential_fingerprint'),'proxy_fingerprint':e.get('proxy_fingerprint')})
    out['recent_502_count']=len(items)
    out['groups_502']=[dict(zip(['proxy','stage','code','status','message','n'],(*key,n))) for key,n in collections.Counter((i['proxy'],i['stage'],i['code'],i['status'],i['message']) for i in items).most_common()]
    out['latest_502']=items[:30]
out['quarantine']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT p.id,p.status,e.reason,(SELECT count(*) FROM accounts a WHERE a.deleted_at IS NULL AND a.proxy_id=p.id) bindings,(SELECT count(*) FROM adobe_proxy_test_leases l WHERE l.proxy_id=p.id) leases FROM proxies p LEFT JOIN adobe_route_exclusions e ON e.proxy_id=p.id WHERE p.id IN (1124,1160,1948,1953)) t")
out['query_failures']=[k for k,v in out.items() if isinstance(v,dict) and ('query_error' in v or 'error' in v)]
print(json.dumps(out,ensure_ascii=False,indent=2))
if out['query_failures']:raise SystemExit(1)
