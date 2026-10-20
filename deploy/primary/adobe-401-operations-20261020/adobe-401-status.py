"""Aggregate operation status only. No credentials or customer identifiers in output."""
import collections,importlib.util,json,pathlib
old=pathlib.Path('/opt/adobe-relay/releases/adobe-quota-diagnostic-20261020');new=pathlib.Path('/opt/adobe-relay/releases/adobe-401-repair-20261020')
s=importlib.util.spec_from_file_location('audit',old/'audit.py');a=importlib.util.module_from_spec(s);s.loader.exec_module(a)
batch=old/('401-manual-batch-v2'if(old/'401-manual-batch-v2').exists()else'401-manual-batch');out={}
for name in ['started','progress','finished','process-exit']:
 p=batch/(name+'.json')
 if p.exists():out['batch_'+name]=json.loads(p.read_text())
ids=[int(r['id'])for r in json.loads((old/'401-preflight-private.json').read_text())['accounts']if not r.get('manual_pause')]
rows=json.loads(a.sql("SELECT coalesce(json_agg(json_build_object('source_key',extra->'okad'->>'source_key','status',status,'schedulable',schedulable,'paused',extra->'okad_manual_pause','token',COALESCE(credentials->>'access_token','')<>'','error_401',error_message LIKE '%401%','verification',extra->'okad'->>'verification','latest_code',extra->'adobe_health'->'latest'->>'code')),'[]') FROM accounts WHERE deleted_at IS NULL AND id IN ("+','.join(map(str,ids or[0]))+")"))
out['cohort']={'count':len(rows),'current_401_messages':sum(bool(r['error_401'])for r in rows),'with_token':sum(bool(r['token'])for r in rows),'paused':sum(r['paused']is True for r in rows),'verifications':dict(collections.Counter(r['verification']for r in rows)),'latest_image_codes':dict(collections.Counter(r['latest_code']for r in rows))}
keys=sorted(set(r.get('source_key')for r in rows if str(r.get('source_key')or'').startswith('external:')))
if keys:
 import subprocess
 program="""import collections,json,sys
from sqlalchemy import select
from app.db.session import SessionLocal
from app.models.external_member import ExternalMember
ids=[int(k.split(':')[1])for k in json.loads(sys.stdin.readline())]
with SessionLocal()as db:rows=db.execute(select(ExternalMember.login_status,ExternalMember.credits_error,ExternalMember.account_profile).where(ExternalMember.id.in_(ids))).all()
print(json.dumps({'sources':len(rows),'login_statuses':dict(collections.Counter(r[0]for r in rows)),'credit_401_warnings':sum('401'in(r[1]or'')for r in rows),'profile_kinds':dict(collections.Counter((r[2]or{}).get('current_kind','unknown')for r in rows))}))
"""
 result=subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-c',program],input=json.dumps(keys)+'\n',capture_output=True,text=True,timeout=35)
 if result.returncode==0:out['okad_authentication']=json.loads(result.stdout)
 else:out['okad_authentication']={'read_failed':True}
for name in ['progress','rolling-route-canary','rolling-route-canonical','rolling-finished','final-verification']:
 p=new/(name+'.json')
 if p.exists():
  d=json.loads(p.read_text());out['release_'+name]={k:v for k,v in d.items()if k in ['at','step','state','error','application_sha','account_count','manual_pauses_preserved','passive_sample_count']}
p=new/'after.json'
if p.exists():
 d=json.loads(p.read_text());out['release_audit']={'backup':d.get('backup'),'preserved':d.get('preserved')}
p=new/'business-verification.json'
if p.exists():
 d=json.loads(p.read_text());out['business_comparison']={'before_count':d.get('before_count'),'current_count':d.get('current_count'),'missing_ids':len(d.get('missing_ids',[])),'manual_pause_preserved':d.get('manual_pause_preserved'),'proxy_changes':sum('proxy_id'in r.get('fields',[])for r in d.get('changes',[])),'all_proxy_chains_match':all(r.get('proxy_chain_matches')for r in d.get('changes',[]))}
for name in ['business-verify.py','review-credentials.py']:
 p=new/(name+'.private.log')
 if p.exists():
  log=p.read_text();out[name+'_failure']={'proxy_chain_assertion':'Review unmatched live proxy changes'in log,'account_or_pause_assertion':'Review account changes'in log,'contains_traceback':'Traceback'in log}
p=new/'credential-change-review.json'
if p.exists():
 d=json.loads(p.read_text());out['credential_comparison']={'source_cookie_changes':len(d.get('cookie_changed_accounts',[])),'runtime_or_token_only_changes':len(d.get('token_fields_only_accounts',[]))}
p=new/'original-primary-quota-private.jsonl'
if p.exists():
 safe=[]
 for line in p.read_text().splitlines():
  if line.startswith('{'):
   d=json.loads(line);safe.append({k:v for k,v in d.items()if k in['phase','http','error_code','needs_reauth','has_error','type']})
 out['original_primary_quota_check']=safe
out['original_primary_quota_helper_exists']=(old.parent/'adobe-session-renewal-20261020'/'verify-native-host.py').exists()
for phase in ['old','canary']:
 p=new/('rolling-'+phase+'-drain-progress.json')
 if p.exists():out['drain_'+phase]=json.loads(p.read_text())['samples'][-1]
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
 d=a.inspect(name);out[name]={'image':d['Config']['Image'],'health':d['State'].get('Health',{}).get('Status'),'restarts':d['RestartCount']}
print(json.dumps(out),flush=True)
