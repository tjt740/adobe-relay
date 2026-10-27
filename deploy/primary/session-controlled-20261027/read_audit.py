import pathlib,json,datetime,subprocess
R=pathlib.Path('/opt/adobe-relay/releases/crawford-login-egress-trace2-20261027')
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
emit(owned_containers=[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-crawford-login-egress-trace2-')])

def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 assert r.returncode==0
 return [json.loads(l) for l in r.stdout.splitlines() if l.startswith('{')]
queries={'first_errors_after_login':"SELECT row_to_json(t) FROM (SELECT created_at,model,status_code,upstream_status_code,error_phase,left(upstream_error_message,260) AS upstream FROM ops_error_logs WHERE account_id=1738 AND created_at>='2026-10-27T18:14:01+08:00' AND (status_code=401 OR upstream_status_code=401) ORDER BY created_at LIMIT 8) t"}
for name,q in queries.items():emit(history=name,records=sql(q))

r=subprocess.run(['docker','exec','adobe-team-cookie-app-1','python','-c',"import json,hashlib,os;from curl_cffi import requests;s=requests.Session(impersonate='chrome',timeout=10);r=s.get('https://api.ipify.org?format=json');r.raise_for_status();print(json.dumps({'status':r.status_code,'ip_sha256':hashlib.sha256(r.json()['ip'].encode()).hexdigest(),'proxy_env_present':any(k.lower() in ['http_proxy','https_proxy','all_proxy'] for k in os.environ)}))"],capture_output=True,text=True,timeout=18)
if r.returncode==0:emit(production_okad_public_egress=json.loads(r.stdout))
else:emit(production_okad_public_egress_unknown=True,returncode=r.returncode)
