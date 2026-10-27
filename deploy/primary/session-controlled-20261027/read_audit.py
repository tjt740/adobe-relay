import pathlib,json,datetime,subprocess,time
root_wait=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028-r6')
deadline=time.time()+480
while not (root_wait/'cleaned.json').exists() and time.time()<deadline:time.sleep(5)
time.sleep(15)
assert (root_wait/'cleaned.json').exists(),'bounded wait expired; inspect running trial, do not replay login'

names=['prepared.json','ready.json','login-started.json','login-result.json','trial-started.json','trial-finished.json','trial-error.json','first-auth-failure.json','latest-state.json','controller.jsonl','events.jsonl','database-journal.json','usage.json','errors.json','route-events.json','final-state.json','credential-observations.jsonl','egress-samples.jsonl','proxy-hash-map.json','production-before.json','production-after.json','cleaned.json','handoff.json','egress-continuous.jsonl','egress-observer-started.json','egress-observer-done.json','continuation-started.json','continuation-error.json']
for suffix in ['-r6']:
 root=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028'+suffix)
 for name in names:
  f=root/name
  if not f.exists():continue
  data=[json.loads(x) for x in f.read_text().splitlines()] if name.endswith('.jsonl') else json.loads(f.read_text())
  print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'export','root':root.name,'file':name,'data':data},ensure_ascii=False))
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'containers','data':subprocess.check_output(['docker','ps','-a','--filter','name=crawford-fullchain','--format','{{.Names}} {{.Status}}'],text=True).splitlines()}))
