import pathlib,json,subprocess,datetime,collections
ROOT=pathlib.Path('/opt/adobe-relay/releases/native-20261027')
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
for name in ['started.json','finished.json','restored.json']:
 f=ROOT/name
 if f.exists():print(json.dumps({'at':now(),'file':name,'document':json.loads(f.read_text())},ensure_ascii=False))
for f in sorted(ROOT.glob('*.jsonl')):
 rs=[]
 for l in f.read_text().splitlines():
  try:rs.append(json.loads(l))
  except ValueError:pass
 status=collections.Counter(str(r.get('status')) for r in rs if r.get('event')=='http_finish' and r.get('host')=='firefly-3p.ff.adobe.io')
 print(json.dumps({'at':now(),'file':f.name,'count':len(rs),'statuses':status,'native_code_method_clicks':sum(r.get('event')=='native_code_method_selected' for r in rs),'native_email_submissions':sum(r.get('event')=='native_email_submitted' for r in rs)}))
 imp=[r for r in rs if r.get('event') in ['native_auth_ui','native_email_submitted','native_otp_submitted','native_login_completed','fatal','web_ready','account_start','account_done','stop','job_start','ui_after_click','identity_egress']]
 for r in imp[-5:]:print(json.dumps({'at':now(),'file':f.name,'event_record':r},ensure_ascii=False))
for f in list(ROOT.glob('*stderr.log'))+[ROOT/'controller-process.log']:
 if f.exists() and f.stat().st_size:print(json.dumps({'at':now(),'file':f.name,'error_tail':f.read_text()[-1600:]},ensure_ascii=False))
print(json.dumps({'at':now(),'containers':[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-native-')]}))
