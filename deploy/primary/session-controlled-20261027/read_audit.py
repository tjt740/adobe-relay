import pathlib,json,datetime,subprocess
R=pathlib.Path('/opt/adobe-relay/releases/crawford-20261027')
def emit(**kw):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),**kw},ensure_ascii=False))
for n in ['started.json','finished.json','restored.json','auth-1738.json']:
 f=R/n
 if f.exists():emit(file=n,document=json.loads(f.read_text()))
for n in ['controller.jsonl','web-1738.jsonl','sdk-1738.jsonl']:
 f=R/n
 if not f.exists():continue
 rs=[]
 for l in f.read_text().splitlines():
  try:d=json.loads(l)
  except ValueError:continue
  if d.get('event') in ['login_ready','login_failed','worker_failed','paired_phase_start','paired_phase_done','guard_stop','account_start','account_done','job_start','job_finish','web_ready','identity_egress','validate','fatal','stop','ui_after_click'] or (d.get('event')=='http_finish' and d.get('host')=='firefly-3p.ff.adobe.io'):rs.append(d)
 for i in range(0,len(rs),10):emit(file=n,offset=i,records=rs[i:i+10])
emit(owned_containers=[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-crawford-')])

def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 assert r.returncode==0
 return [json.loads(l) for l in r.stdout.splitlines() if l.startswith('{')]
queries={
 'first_errors_after_login':"SELECT row_to_json(t) FROM (SELECT created_at,model,status_code,upstream_status_code,error_phase,left(upstream_error_message,260) AS upstream FROM ops_error_logs WHERE account_id=1738 AND created_at>='2026-10-27T18:14:01+08:00' AND (status_code=401 OR upstream_status_code=401) ORDER BY created_at LIMIT 8) t",
 'proxy_events':"SELECT row_to_json(t) FROM (SELECT created_at,from_proxy_id,to_proxy_id,reason FROM proxy_failover_events WHERE account_id=1738 AND created_at>='2026-10-27T18:00:00+08:00' ORDER BY created_at LIMIT 30) t",
 'window_usage':"SELECT json_build_object('images',sum(image_count),'positive_usage_rows',count(*),'first_success',min(created_at),'last_success',max(created_at)) FROM usage_logs WHERE account_id=1738 AND created_at>='2026-10-27T18:14:01+08:00' AND image_count>0"
}
for name,q in queries.items():emit(history=name,records=sql(q))
