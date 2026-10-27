import pathlib,sys,json,datetime,hashlib,urllib.parse
ROOT=pathlib.Path('/opt/adobe-relay/releases/crawford-full-chain-20261028');sys.path.insert(0,str(ROOT))
from replica import *
assert (ROOT/'prepared.json').exists() and not (ROOT/'cleaned.json').exists() and not (ROOT/'login-started.json').exists()
checks=crows("SELECT json_build_object('others_with_credentials',count(*) FILTER(WHERE id<>1738 AND credentials<>'{}'::jsonb),'other_schedulable',count(*) FILTER(WHERE id<>1738 AND schedulable)) FROM accounts")[0]
assert checks=={'others_with_credentials':0,'other_schedulable':0}
clone("CREATE OR REPLACE FUNCTION diag_hash(s text) RETURNS text LANGUAGE sql IMMUTABLE AS $$ SELECT CASE WHEN s IS NULL OR s='' THEN '' ELSE encode(sha256(convert_to(s,'UTF8')),'hex') END $$;CREATE TABLE diag_account_journal_sha(LIKE diag_account_journal INCLUDING ALL);")
s=clone("SELECT pg_get_functiondef('diag_account_record'::regproc)");assert 'diag_account_journal' in s and 'md5(' in s
clone(s.replace('diag_account_journal(','diag_account_journal_sha(').replace('md5(','diag_hash('))
secret=json.loads((PRIVATE/'secrets.json').read_text());status,body=request('GET','/v1/models',headers={'Authorization':'Bearer '+secret['api_key']},timeout=15)
assert status==200,'clone_api_key_auth_failed'
a=json.loads(body);models=[x['id'] for x in a.get('data',[])];assert 'gpt-image-2.5-sunburst' in models and 'gpt-image-2.5-flare' in models
proxies=crows("SELECT json_build_object('proxy',to_jsonb(p)) FROM proxies p WHERE deleted_at IS NULL")
mapping=[]
for x in proxies:
 p=x['proxy'];auth=(urllib.parse.quote(p.get('username') or '',safe='')+':'+urllib.parse.quote(p.get('password') or '',safe='')+'@') if p.get('username') else ''
 u=p['protocol']+'://'+auth+p['host']+':'+str(p['port']);mapping.append({'id':p['id'],'url_sha256':sha(u)})
save('proxy-hash-map.json',mapping)
command(['docker','exec','--user','1000',APP,'test','-w','/diagnostic/events.jsonl'])
checks.update(api_key_auth_http=status,models_present=True,selected=selected(),events_file_bytes=(ROOT/'events.jsonl').stat().st_size)
save('ready.json',{'at':now(),'checks':checks});print(json.dumps({'at':now(),'kind':'ready','data':checks},ensure_ascii=False))
