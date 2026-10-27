import pathlib,json,datetime,subprocess,time
root=pathlib.Path('/opt/adobe-relay/releases/408-causal-20261028/craw-a')
deadline=time.time()+900
while not (root/'cleaned.json').exists() and time.time()<deadline:time.sleep(5)
assert (root/'cleaned.json').exists(),'bounded export wait expired; inspect trial, do not replay authentication'
time.sleep(15)
names=['experiment-manifest.json', 'prepared.json', 'ready.json', 'login-started.json', 'login-result.json', 'trial-started.json', 'trial-finished.json', 'trial-error.json', 'first-auth-failure.json', 'controller.jsonl', 'events.jsonl', 'database-journal.json', 'usage.json', 'errors.json', 'route-events.json', 'final-state.json', 'credential-observations.jsonl', 'proxy-hash-map.json', 'production-before.json', 'production-after.json', 'cleaned.json', 'egress-continuous.jsonl', 'egress-observer-done.json']
for name in names:
 f=root/name
 if not f.exists():continue
 data=[json.loads(x) for x in f.read_text().splitlines()] if name.endswith('.jsonl') else json.loads(f.read_text())
 print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'export','trial':'craw-a','file':name,'data':data},ensure_ascii=False))
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'containers','data':subprocess.check_output(['docker','ps','-a','--filter','name=adobe408-diag','--format','{{.Names}} {{.Status}}'],text=True).splitlines()}))
