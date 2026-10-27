import pathlib,json,datetime,subprocess,sys
ROOT=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028')
def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
for n in ['prepared.json','prepare-error.json','cleaned.json','login-started.json','login-result.json','trial-started.json','trial-finished.json','trial-error.json','first-auth-failure.json','latest-state.json']:
 if (ROOT/n).exists():emit(n,json.loads((ROOT/n).read_text()))
for n in ['controller.jsonl','events.jsonl']:
 if (ROOT/n).exists():
  a=[]
  for l in (ROOT/n).read_text().splitlines():
   try:a.append(json.loads(l))
   except:pass
  from collections import Counter
  emit(n,{'count':len(a),'events':dict(Counter(x.get('event') for x in a)),'http_statuses':dict(Counter(str(x.get('status')) for x in a if x.get('event')=='http_finish')),'tail':a[-8:]})
if (ROOT/'prepared.json').exists() and not (ROOT/'cleaned.json').exists():
 sys.path.insert(0,str(ROOT));from replica import clone,crows,selected,command
 emit('isolation_check',crows("SELECT json_build_object('other_accounts_with_credentials',count(*) FILTER(WHERE id<>1738 AND credentials<>'{}'::jsonb),'other_schedulable',count(*) FILTER(WHERE id<>1738 AND schedulable),'target',count(*) FILTER(WHERE id=1738)) FROM accounts"))
 emit('selected',selected())
 emit('migrations',crows("SELECT json_build_object('count',count(*),'digest',md5(string_agg(filename||checksum,',' ORDER BY filename))) FROM schema_migrations"))
emit('containers',subprocess.check_output(['docker','ps','-a','--filter','name=crawford-fullchain','--format','{{.Names}} {{.Status}}'],text=True).splitlines())
