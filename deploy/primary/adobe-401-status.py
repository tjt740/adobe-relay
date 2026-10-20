"""Aggregate operation status only. No credentials or customer identifiers in output."""
import collections,importlib.util,json,pathlib
old=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');new=pathlib.Path('/opt/adobe-relay/releases/adobe-401-repair-20261020')
s=importlib.util.spec_from_file_location('audit',old/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
batch=old/'401-manual-batch';out={}
for name in ['started','progress','finished','process-exit']:
 p=batch/(name+'.json')
 if p.exists():out['batch_'+name]=json.loads(p.read_text())
ids=[int(r['id'])for r in json.loads((old/'401-preflight-private.json').read_text())['accounts']if not r.get('manual_pause')]
rows=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('status',status,'schedulable',schedulable,'paused',extra->'okad_manual_pause','token',COALESCE(credentials->>'access_token','')<>'','error_401',error_message LIKE '%401%','verification',extra->'okad'->>'verification','latest_code',extra->'adobe_health'->'latest'->>'code')),'[]') FROM accounts WHERE deleted_at IS NULL AND id IN ("+','.join(map(str,ids or[0]))+")"))
out['cohort']={'count':len(rows),'current_401_messages':sum(bool(r['error_401'])for r in rows),'with_token':sum(bool(r['token'])for r in rows),'paused':sum(r['paused']is True for r in rows),'verifications':dict(collections.Counter(r['verification']for r in rows)),'latest_image_codes':dict(collections.Counter(r['latest_code']for r in rows))}
for name in ['progress','rolling-route-canary','rolling-route-canonical','rolling-finished','final-verification']:
 p=new/(name+'.json')
 if p.exists():
  d=json.loads(p.read_text());out['release_'+name]={k:v for k,v in d.items()if k in ['at','step','state','error','application_sha','account_count','manual_pauses_preserved','passive_sample_count']}
for phase in ['old','canary']:
 p=new/('rolling-'+phase+'-drain-progress.json')
 if p.exists():out['drain_'+phase]=json.loads(p.read_text())['samples'][-1]
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
 d=a.inspect(name);out[name]={'image':d['Config']['Image'],'health':d['State'].get('Health',{}).get('Status'),'restarts':d['RestartCount']}
print(json.dumps(out),flush=True)
