import pathlib,json,datetime,re
root=pathlib.Path('/opt/adobe-relay/releases/flare-confirmed-20261027')
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def clean(x):
 if x is None or isinstance(x,(bool,int,float)):return x
 if isinstance(x,str):
  x=re.sub(r'https?://[^\s]+','[url]',x);x=re.sub(r'[A-Za-z0-9_.+-]+@[A-Za-z0-9.-]+','[email]',x);x=re.sub(r'[A-Za-z0-9_./+=-]{32,}','[opaque]',x)
  return x[:350]
 if isinstance(x,dict):return {'keys':list(x)}
 return {'type':type(x).__name__}
for i in [1815,1813]:
 f=pathlib.Path('/dev/shm/adobe-flare-confirmed-20261027-'+str(i)+'.json')
 if not f.exists():print(json.dumps({'at':now(),'account_id':i,'private_session_present':False}));continue
 d=json.loads(f.read_text());r=d.get('record',{});v={k:clean(r.get(k)) for k in ['credits','credits_total','credits_error','account_profile','expires_at']}
 v['cookie_domains']=sorted(set(c['domain'] for c in d.get('cookies',[]) if 'domain' in c))
 print(json.dumps({'at':now(),'account_id':i,'session_metadata':v},ensure_ascii=False))
