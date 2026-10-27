import pathlib,json,datetime,collections,subprocess
R=pathlib.Path('/opt/adobe-relay/releases/crawford-login-egress-observe-20261027');rows=[]
f=R/'sdk-1738.jsonl'
if f.exists():
 for l in f.read_text().splitlines():
  try:rows.append(json.loads(l))
  except ValueError:pass
valid=[x for x in rows if x.get('event')=='validate']
jobs=[x for x in rows if x.get('event')=='job_finish']
http=[x for x in rows if x.get('event')=='http_finish']
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'success_jobs':sum(x.get('success') is True for x in jobs),'failed_jobs':sum(x.get('success') is False for x in jobs),'submit_status_counts':dict(collections.Counter(str(x['status']) for x in http if x.get('stage')=='submit')),'all_http_401':sum(x.get('status')==401 for x in http),'validation_checks':valid,'finished':(R/'finished.json').exists(),'restored':(R/'restored.json').exists(),'owned_containers':[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-crawford-login-egress-observe-')]},ensure_ascii=False))
