import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027');IDS=[1739,1741,1743];MEMBERS={1742:247,1744:245,1749:240,1758:231};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def log(kind,**kw):
 with (ROOT/'controller.jsonl').open('a') as f:f.write(json.dumps({'at':now(),'event':kind,**kw},ensure_ascii=False)+'\n')
def sql(q,raw=False):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 if r.returncode:raise RuntimeError('database_read_failed')
 return r.stdout.strip() if raw else [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]


TARGETS=['TangaroTamsyn54@outlook.com', 'SharleyHinish96@outlook.com', 'BehrmannConatser45@outlook.com', 'DonaldSimril48@outlook.com', 'MaltbaKinsolving20@outlook.com', 'DelaportillaSchneiter096@outlook.com', 'BergesenVillafranca062@outlook.com', 'DieffenbacherFencl096@outlook.com', 'HelfinstineSulit6602@outlook.com', 'YoungbergSchoonmaker63@outlook.com']
T=','.join("'"+e.lower()+"'" for e in TARGETS)
def emit(kind,rs):
 for i in range(0,len(rs),10):print(json.dumps({'at':now(),'kind':kind,'data':rs[i:i+10]},ensure_ascii=False))
emit('accounts',sql("SELECT json_build_object('id',a.id,'email',a.name,'status',a.status,'schedulable',a.schedulable,'manual_pause',a.extra->'okad_manual_pause','manual_proxy',a.extra->'manual_proxy_enabled','proxy_id',a.proxy_id,'other_bindings',(SELECT count(*) FROM accounts b WHERE b.deleted_at IS NULL AND b.proxy_id=a.proxy_id AND b.id<>a.id),'active_lease',EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW()),'test_state',a.extra->'okad_connection_test'->'state','version',a.extra->'okad'->'version') FROM accounts a WHERE a.deleted_at IS NULL AND lower(a.name) IN ("+T+") ORDER BY a.id"))
emit('flare_success',sql("SELECT row_to_json(t) FROM (SELECT a.id AS account_id,a.name AS email,count(*) AS successful_requests,sum(u.image_count) AS images,min(u.created_at) AS first_at,max(u.created_at) AS last_at,json_agg(DISTINCT u.upstream_model) AS upstream_models FROM usage_logs u JOIN accounts a ON a.id=u.account_id WHERE lower(a.name) IN ("+T+") AND u.created_at>='2026-10-27T00:00:00+08:00' AND u.model='gpt-image-2.5-flare' AND u.image_count>0 GROUP BY a.id,a.name ORDER BY a.id) t"))
emit('latest_flare_success',sql("SELECT row_to_json(t) FROM (SELECT DISTINCT ON (a.id) a.id AS account_id,u.created_at,u.model,u.requested_model,u.upstream_model,u.image_count,u.image_size,u.duration_ms FROM usage_logs u JOIN accounts a ON a.id=u.account_id WHERE lower(a.name) IN ("+T+") AND u.created_at>='2026-10-27T00:00:00+08:00' AND u.model='gpt-image-2.5-flare' AND u.image_count>0 ORDER BY a.id,u.created_at DESC) t"))
emit('latest_flare_401',sql("SELECT row_to_json(t) FROM (SELECT DISTINCT ON (a.id) a.id AS account_id,e.created_at,e.model,e.status_code,e.upstream_status_code,e.error_phase,left(e.upstream_error_message,220) AS upstream FROM ops_error_logs e JOIN accounts a ON a.id=e.account_id WHERE lower(a.name) IN ("+T+") AND e.created_at>='2026-10-27T00:00:00+08:00' AND e.model='gpt-image-2.5-flare' AND (e.status_code=401 OR e.upstream_status_code=401) ORDER BY a.id,e.created_at DESC) t"))
print(json.dumps({'at':now(),'diagnostic_containers':[x for x in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if x.startswith(('adobe-expanded-','adobe-native-','adobe-flare-confirmed-'))]}))
