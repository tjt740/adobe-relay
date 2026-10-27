import pathlib,json,datetime,subprocess,collections
ROOT=pathlib.Path('/opt/adobe-relay/releases/flare-confirmed-20261027')
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
for name in ['started.json','isolated.json','finished.json','restored.json','native-started.json','native-ui.json','native-ui-login-result.json']+[f'auth-{i}.json' for i in [1815,1813]]:
 f=ROOT/name
 if f.exists():print(json.dumps({'at':now(),'file':name,'document':json.loads(f.read_text())},ensure_ascii=False))
for f in sorted(ROOT.glob('*.jsonl')):
 rows=[]
 for l in f.read_text().splitlines():
  try:rows.append(json.loads(l))
  except:pass
 statuses=collections.Counter(str(x.get('status')) for x in rows if x.get('event')=='http_finish' and x.get('host')=='firefly-3p.ff.adobe.io')
 keep=[r for r in rows if r.get('event') in ['fatal','stop','account_done','account_start','login_ready','login_failed','guard_stop','worker_failed','resolved_model','web_ready','job_finish','paired_phase_start','paired_phase_done','paired_stop']]
 bad=[r for r in rows if r.get('status')==401 or r.get('valid') is False]
 print(json.dumps({'at':now(),'file':f.name,'record_count':len(rows),'statuses':statuses,'events':keep[-12:],'bad':bad,'tail':rows[-3:]},ensure_ascii=False))
for f in sorted(ROOT.glob('*stderr.log'))+[ROOT/'controller-process.log']:
 if f.exists() and f.stat().st_size:print(json.dumps({'at':now(),'file':f.name,'log_tail':f.read_text()[-1800:]},ensure_ascii=False))
print(json.dumps({'at':now(),'containers':[n for n in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines() if n.startswith('adobe-flare-confirmed-')]}))

import hashlib
for i in [1815,1813]:
 f=pathlib.Path('/dev/shm/adobe-flare-confirmed-20261027-'+str(i)+'.json')
 if not f.exists():continue
 r=json.loads(f.read_text()).get('record',{});v=r.get('account_profile') or {};out={k:v.get(k) for k in ['current_kind','account_type','profiles_complete','requested_preference','fallback_reason','personal_organizations_complete','fetched_at'] if k in v}
 for k in ['current_profile_id','current_org_id','credits_account_id']:
  if v.get(k):out[k+'_sha256']=hashlib.sha256(str(v[k]).encode()).hexdigest()
 out['profile_count']=len(v.get('profiles') or []);out['profile_summary']=[{k:x[k] for k in ['kind','type','account_type','has_subscription','subscription_active'] if k in x} for x in (v.get('profiles') or []) if isinstance(x,dict)]
 out['subscription_evidence_type']=type(v.get('subscription_evidence')).__name__
 print(json.dumps({'at':now(),'account_id':i,'native_login_profile':out},ensure_ascii=False))
