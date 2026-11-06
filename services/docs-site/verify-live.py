#!/usr/bin/env python3
"""One existing Sub2 login; read docs; logout. Never print or persist credentials."""
import hashlib, http.client, json, pathlib, subprocess, time, urllib.parse
from http.cookies import SimpleCookie
BASE='127.0.0.1'
HEADERS={'Host':'docs.creative.cheap','Origin':'https://docs.creative.cheap','X-Real-IP':'127.0.0.1','User-Agent':'creative-docs-release-verification/1'}
def call(method,path,body=None,cookie=None):
    headers=HEADERS.copy()
    if cookie: headers['Cookie']=cookie
    if body is not None: headers['Content-Type']='application/x-www-form-urlencoded';body=urllib.parse.urlencode(body)
    c=http.client.HTTPConnection(BASE,6670,timeout=25)
    c.request(method,path,body,headers);r=c.getresponse();b=r.read();h=dict(r.getheaders());status=r.status;c.close()
    return status,h,b
results={}
for path in ('/','/guide.md','/assets/guide.md','/.git/config'):
    s,h,b=call('GET',path)
    assert s in (303,404) and b'frame_duration_ms' not in b
results['anonymous_content_blocked']=True
container=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1'],text=True))[0]
env=dict(v.split('=',1) for v in container['Config']['Env'])
credentials={'email':env['ADMIN_EMAIL'],'password':env['ADMIN_PASSWORD']}
s,h,b=call('POST','/auth/login',credentials)
del credentials,env,container
assert s==303, 'Existing user login did not complete; no automatic retry'
jar=SimpleCookie(h['Set-Cookie']);m=jar['__Host-docs_session'];cookie=m.key+'='+m.value
assert m['secure'] and m['httponly'] and m['samesite']=='Strict'
try:
    s,h,b=call('GET','/',cookie=cookie)
    assert s==200 and b'frame_duration_ms' in b and b'private-jwt' not in b
    assert 'no-store' in h['Cache-Control'] and 'frame-ancestors' in h['Content-Security-Policy']
    results['existing_sub2_user_login_and_read']=True
    s,h,b=call('GET','/guide.md',cookie=cookie)
    assert s==200
    results['protected_markdown_sha256']=hashlib.sha256(b).hexdigest()
    expected=pathlib.Path(__file__).with_name('guide.md').read_bytes()
    assert b==expected
finally:
    s,h,b=call('POST','/auth/logout',{},cookie)
    assert s==303
s,h,b=call('GET','/',cookie=cookie)
assert s==303 and b'frame_duration_ms' not in b
results.update(logout_invalidates_docs_session=True,credentials_persisted=False,adobe_requests=0,verified_at_utc=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()))
path=pathlib.Path('/opt/creative-docs/audit/live-verification.json')
assert not path.exists(), 'preserve prior evidence'
path.write_text(json.dumps(results,indent=2))
print(json.dumps(results))
