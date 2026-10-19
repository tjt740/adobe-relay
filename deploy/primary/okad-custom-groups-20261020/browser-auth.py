"""Mint short-lived verification sessions; redirect stdout to a private local file."""
import base64,hashlib,hmac,importlib.util,json,secrets,subprocess,time,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('live',root/'live-groups.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
context,key=m.credentials()
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
env=dict(item.split('=',1) for item in d['Config']['Env'])
secret=env.get('JWT_SECRET')
assert secret,'JWT secret must remain available only in the existing container environment'
user=json.loads(m.audit.sql("SELECT json_build_object('id',id,'email',email,'role',role,'password_hash',password_hash) FROM users WHERE role='admin' AND status='active' AND deleted_at IS NULL ORDER BY id LIMIT 1"))
version=int.from_bytes(hashlib.sha256((user['email'].strip().lower()+'\n'+user['password_hash']).encode()).digest()[:8],'big') & 0x7fffffffffffffff
now=int(time.time());claims={'user_id':user['id'],'email':user['email'],'role':user['role'],'token_version':version,'sid':secrets.token_hex(8),'exp':now+600,'iat':now,'nbf':now}
def b64(value):return base64.urlsafe_b64encode(value).decode().rstrip('=')
unsigned=b64(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())+'.'+b64(json.dumps(claims,separators=(',',':')).encode())
token=unsigned+'.'+b64(hmac.new(secret.encode(),unsigned.encode(),hashlib.sha256).digest())
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/auth/me',headers={'Authorization':'Bearer '+token})
with urllib.request.urlopen(req,timeout=15) as response:actual=json.load(response)['data']
assert actual['id']==user['id']
print(json.dumps({'okad_token':context['token'],'sub2_token':token,'sub2_user':actual,'issuer':context['issuer'],'fixture':json.loads((root/'group-test-fixture.json').read_text())}))
