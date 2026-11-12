"""Bounded read-only evidence; no account API, login, refresh or generation."""
import collections, datetime, hashlib, json, pathlib, re, subprocess

UTC=datetime.timezone.utc
end=datetime.datetime.now(UTC)
start=end-datetime.timedelta(minutes=20)
def ts(v):return datetime.datetime.fromisoformat(v.replace('Z','+00:00'))
window="created_at>='"+start.isoformat()+"' AND created_at<'"+end.isoformat()+"'"
def sql(q):
    p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=25)
    if p.returncode:raise RuntimeError(p.stderr[:1500])
    return json.loads(p.stdout.strip())
def safe(s):
    s=re.sub(r'https?://[^\s"<>]+','[url]',str(s or ''))
    s=re.sub(r'eyJ[A-Za-z0-9_.-]+','[jwt]',s)
    s=re.sub(r'(?i)(bearer\s+)\S+',r'\1[redacted]',s)
    s=re.sub(r'(?i)((?:token|cookie|password|authorization)\s*[:=]\s*)[^,;\s]+',r'\1[redacted]',s)
    s=re.sub(r'[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}','[email]',s)
    return s[:500]
def quality(q):return 'medium' if not q or q=='auto' else q
out={'window_start':start.isoformat(),'window_end':end.isoformat(),'read_only':True}
out['hourly']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT date_trunc('hour',created_at) bucket,status_code,count(*) n FROM ops_error_logs WHERE created_at>now()-interval '5 hours' AND status_code IN (401,403,429,502) GROUP BY 1,2 ORDER BY 1,2)t")
out['usage']=sql("SELECT json_build_object('records',count(*),'images',sum(image_count)) FROM usage_logs WHERE "+window)
out['attempt_counts']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT status_code outer_status,count(*) logged_requests,sum(jsonb_array_length(coalesce(upstream_errors,'[]'::jsonb))) recorded_attempts,sum((SELECT count(*) FROM jsonb_array_elements(coalesce(upstream_errors,'[]'::jsonb)) e WHERE e->>'upstream_status_code'='403')) attempts_403 FROM ops_error_logs WHERE "+window+" AND platform='adobe' GROUP BY 1 ORDER BY 1)t")
rows=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT id,created_at,account_id,model,NULL::text quality,NULL::text size,upstream_status_code,upstream_errors->-1 last,upstream_error_message,error_message FROM ops_error_logs WHERE "+window+" AND status_code=403 ORDER BY id DESC LIMIT 10000)t")
assert len(rows)<10000,'Window truncated; narrow it'
failures=[]
for r in rows:
    e=r.get('last') or {}
    try:d=json.loads(e.get('detail') or '{}')
    except (ValueError,TypeError):d={}
    diag=d.get('request_diagnostics') or {}
    failures.append({'id':r['id'],'at':r['created_at'],'account':e.get('account_id') or r['account_id'],'proxy':e.get('proxy_id'),'proxy_revision':d.get('proxy_updated_at'),'model':r['model'],'quality':r['quality'],'size':r['size'],'upstream_status':r['upstream_status_code'],'stage':d.get('stage'),'code':d.get('code'),'assessment':d.get('forbidden_assessment'),'message':safe(e.get('message') or r.get('upstream_error_message') or r.get('error_message')),'diagnostics':{k:diag[k] for k in ['credential_fingerprint','bearer_fingerprint','session_fingerprint','credential_expires_at','expired_at_start','upstream_error_code','access_error','response_body_sha256'] if k in diag}})
out['final_403']={'records':len(failures),'accounts':len({x['account'] for x in failures}),'proxies':len({x['proxy'] for x in failures if x['proxy'] is not None}),'with_credential_fp':sum(bool(x['diagnostics'].get('credential_fingerprint')) for x in failures),'expired_at_start':sum(bool(x['diagnostics'].get('expired_at_start')) for x in failures),'declared_expiry_present':sum(bool(x['diagnostics'].get('credential_expires_at')) for x in failures)}
out['classifications']=[dict(zip(['stage','code','assessment','upstream_code','access_error','message','n'],(*k,n))) for k,n in collections.Counter((x['stage'],x['code'],x['assessment'],x['diagnostics'].get('upstream_error_code'),x['diagnostics'].get('access_error'),x['message']) for x in failures).most_common(25)]
out['by_account']=[{'account':k,'final_403':n} for k,n in collections.Counter(x['account'] for x in failures).most_common(30)]
out['by_proxy']=[{'proxy':k,'final_403':n} for k,n in collections.Counter(x['proxy'] for x in failures).most_common(30)]
out['by_model']=[{'model':k,'final_403':n} for k,n in collections.Counter(x['model'] for x in failures).most_common()]
out['proxy_sources']=sql("SELECT coalesce(json_agg(json_build_object('proxy_id',n->'proxy_id','source_index',s->'index','source_id',n->'source_id','name',n->'name')),'[]') FROM clash_subscription c CROSS JOIN LATERAL jsonb_array_elements(c.document->'nodes') n LEFT JOIN LATERAL jsonb_array_elements(c.document->'sources')s ON s->>'id'=n->>'source_id' WHERE c.id=1")
# Read only current app logs, selecting success diagnostics before serializing.
success=[]
app=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
out['application']={'image':app['Config']['Image'],'started_at':app['State']['StartedAt'],'log_window_complete':ts(app['State']['StartedAt'])<=start}
proc=subprocess.Popen(['docker','logs','--timestamps','--since',start.isoformat(),'--until',end.isoformat(),'adobe-relay-app-1'],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
for line in proc.stdout:
    if 'adobe_images.pipeline_completed' not in line:continue
    try:
        at=line.split(' ',1)[0];d=json.loads(line[line.index('{'):]);ts(at)
    except (ValueError,IndexError):continue
    for item in (d.get('timings') or {}).get('images') or []:
        diag=item.get('request_diagnostics') or {}
        if not diag.get('credential_fingerprint'):continue
        success.append({'at':at,'account':d.get('account_id'),'model':d.get('model'),'upstream_model':d.get('upstream_model'),'quality':d.get('requested_quality'),'size':d.get('size'),'processing_ms':d.get('processing_ms'),'credential_fp':diag.get('credential_fingerprint'),'bearer_fp':diag.get('bearer_fingerprint'),'session_fp':diag.get('session_fingerprint')})
assert proc.wait(timeout=20)==0
out['success_diagnostics_count']=len(success)
observations=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT proxy_id,proxy_updated_at,account_id,scope,started_at,checked_at FROM adobe_route_observations WHERE success AND checked_at>='"+start.isoformat()+"' AND checked_at<'"+end.isoformat()+"')t")
obs_by_account=collections.defaultdict(list)
for o in observations:obs_by_account[o['account_id']].append(o)
matched_success_routes=0
for s in success:
    candidates=[o for o in obs_by_account[s['account']] if 0<=(ts(s['at'])-ts(o['checked_at'])).total_seconds()<2]
    if len(candidates)==1:
        o=candidates[0];s['proxy']=o['proxy_id'];s['proxy_revision']=o['proxy_updated_at'];s['generation_started_at']=o['started_at'];matched_success_routes+=1
out['success_unique_route_matches']=matched_success_routes
by_exact=collections.defaultdict(list)
for s in success:by_exact[(s['account'],s['credential_fp'],s['model'])].append(s)
matched=[];same_proxy=[]
for f in failures:
    key=(f['account'],f['diagnostics'].get('credential_fingerprint'),f['model'])
    candidates=[s for s in by_exact.get(key,[]) if 0<(ts(s['at'])-ts(f['at'])).total_seconds()<=600]
    if not candidates:continue
    matched.append(f)
    exact=[s for s in candidates if s.get('proxy')==f['proxy'] and f['proxy_revision'] and s.get('proxy_revision') and ts(s['proxy_revision'])==ts(f['proxy_revision']) and ts(s['generation_started_at'])>ts(f['at'])]
    if exact:
        s=min(exact,key=lambda s:ts(s['at']))
        same_proxy.append({'failed_record_id':f['id'],'failed_at':f['at'],'success_at':s['at'],'success_started_at':s['generation_started_at'],'account':f['account'],'proxy':f['proxy'],'model':f['model'],'quality_comparison':'not_available','upstream_error_code':f['diagnostics'].get('upstream_error_code')})
out['natural_pairs']={'same_account_credentials_public_model_then_success':len(matched),'accounts':len({f['account'] for f in matched}),'also_same_proxy_revision_and_new_generation_after_failure':len(same_proxy),'same_proxy_accounts':len({p['account'] for p in same_proxy}),'examples':same_proxy[:12],'limitation':'Exact bearer/session fingerprints; prompts, quality and actual mapped model are not compared. Configured route is known, actual request egress IP is not observed.'}
out['latest_failures']=failures[:12]
out['failure_fingerprints']=failures
print(json.dumps(out,ensure_ascii=False,indent=2))
