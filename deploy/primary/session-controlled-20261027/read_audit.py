import json,datetime,subprocess,sqlite3,hashlib,pathlib,shutil

def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 if r.returncode:raise RuntimeError('read query failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

emit('same_email_or_parent',sql("SELECT json_build_object('id',id,'name',name,'platform',platform,'parent_id',parent_account_id,'status',status,'schedulable',schedulable,'proxy_id',proxy_id,'cookie_md5',md5(credentials->>'cookie'),'runtime_md5',md5(credentials->>'adobe_session_cookie'),'token_md5',md5(credentials->>'access_token'),'groups',(SELECT json_agg(group_id) FROM account_groups WHERE account_id=accounts.id)) FROM accounts WHERE deleted_at IS NULL AND (id=1738 OR parent_account_id=1738 OR id=(SELECT parent_account_id FROM accounts WHERE id=1738) OR lower(coalesce(credentials->>'email',name))='crawfordheather6275@hotmail.com' OR lower(name) LIKE '%crawfordheather6275%') ORDER BY id"))
emit('same_cookie',sql("SELECT json_build_object('id',a.id,'parent_id',a.parent_account_id,'status',a.status,'schedulable',a.schedulable,'proxy_id',a.proxy_id) FROM accounts a JOIN accounts c ON c.id=1738 WHERE a.deleted_at IS NULL AND (a.credentials->>'cookie'=c.credentials->>'cookie' OR (NULLIF(a.credentials->>'adobe_session_cookie','') IS NOT NULL AND a.credentials->>'adobe_session_cookie'=c.credentials->>'adobe_session_cookie'))"))
import base64
rs=sql("SELECT json_build_object('id',id,'name',name,'token',credentials->>'access_token','parent_id',parent_account_id,'proxy_id',proxy_id) FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND NULLIF(credentials->>'access_token','') IS NOT NULL")
matched=[]
for r in rs:
 try:
  p=r.pop('token').split('.')[1];c=json.loads(base64.urlsafe_b64decode(p+'='*(-len(p)%4)));h=hashlib.sha256(str(c.get('user_id','')).encode()).hexdigest()
  if h=='d3d3b9f2952f6c5be3ca36fb9302c5a2d1ec4f3178750bad86394e070cd623f0':matched.append(r)
 except Exception:pass
emit('same_adobe_identity',{'scanned_live_adobe_tokens':len(rs),'matches':matched})
