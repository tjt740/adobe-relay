import json,subprocess,pathlib,datetime,time,sqlite3,hashlib,urllib.request,urllib.error,urllib.parse,os,sys,traceback,signal,threading
ROOT=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027');IDS=[1739,1741,1743];MEMBERS={1742:247,1744:245,1749:240,1758:231};EXPECTED='adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c';os.umask(0o077)
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def log(kind,**kw):
 with (ROOT/'controller.jsonl').open('a') as f:f.write(json.dumps({'at':now(),'event':kind,**kw},ensure_ascii=False)+'\n')
def sql(q,raw=False):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='8s';"+q+';COMMIT;',capture_output=True,text=True,timeout=15)
 if r.returncode:raise RuntimeError('database_read_failed')
 return r.stdout.strip() if raw else [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]
def api(method,path,body=None,timeout=35):
 assert path=='/api/v1/admin/accounts/bulk-update' or any(path==f'/api/v1/admin/accounts/{i}'+suffix for i in IDS for suffix in ('','/schedulable','/test'))
 if path.endswith('bulk-update'):assert set(body['account_ids'])<=set(IDS) and 'filters' not in body
 key=sql("SELECT value FROM settings WHERE key='admin_api_key'",True)
 req=urllib.request.Request('http://127.0.0.1:6666'+path,data=json.dumps(body).encode() if body is not None else None,headers={'x-api-key':key,'Content-Type':'application/json','X-Request-ID':'session-controlled-20261027'},method=method)
 with urllib.request.urlopen(req,timeout=timeout) as r:
  if path.endswith('/test'):
   result=[]
   for line in r:
    if line.startswith(b'data: '):
     d=json.loads(line[6:]);result.append({k:d[k] for k in ['type','success','status_code'] if k in d})
   return result
  d=json.load(r);return {'code':d.get('code'),'success_ids':(d.get('data') or {}).get('success_ids')} if isinstance(d,dict) else {}
def rows(private=False):
 q="SELECT json_build_object('id',a.id,'email',a.name,'status',a.status,'schedulable',a.schedulable,'concurrency',a.concurrency,'manual_pause',a.extra->'okad_manual_pause','manual_proxy',a.extra->'manual_proxy_enabled','proxy_id',a.proxy_id,'proxy_updated_at',p.updated_at,'proxy_status',p.status,'proxy_excluded',EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=a.proxy_id),'other_bindings',(SELECT count(*) FROM accounts b WHERE b.deleted_at IS NULL AND b.proxy_id=a.proxy_id AND b.id<>a.id),'active_lease',EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id AND l.expires_at>NOW()),'version',a.extra->'okad'->'version','verification',a.extra->'okad'->'verification','test',a.extra->'okad_connection_test','error',a.error_message,'cookie_hash',md5(a.credentials->>'cookie'),'runtime_cookie_hash',md5(a.credentials->>'adobe_session_cookie'),'token_hash',md5(a.credentials->>'access_token'),'token',a.credentials->>'access_token','arp_session_id',a.credentials->>'arp_session_id','proxy',json_build_object('host',p.host,'port',p.port,'protocol',p.protocol,'username',p.username,'password',p.password)) FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id WHERE a.deleted_at IS NULL AND a.id IN ("+','.join(map(str,IDS))+') ORDER BY a.id'
 data=sql(q)
 if not private:
  for a in data:
   for k in ['token','arp_session_id','proxy']:a.pop(k,None)
 return data
def save(name,data):
 p=ROOT/name;tmp=ROOT/(name+'.tmp');tmp.write_text(json.dumps(data,ensure_ascii=False,indent=2));tmp.replace(p)
def others():
 return sql("SELECT json_build_object('id',id,'config_hash',md5(json_build_object('concurrency',concurrency,'priority',priority,'groups',(SELECT json_agg(group_id ORDER BY group_id) FROM account_groups WHERE account_id=accounts.id),'manual_pause',extra->'okad_manual_pause','manual_proxy',extra->'manual_proxy_enabled')::text),'cookie_hash',md5(credentials->>'cookie')) FROM accounts WHERE deleted_at IS NULL AND id NOT IN ("+','.join(map(str,IDS))+') ORDER BY id')




code='import json,sys,datetime,hashlib,base64,pathlib\nfrom urllib.parse import urlparse\nfrom playwright.sync_api import sync_playwright\ncfg=json.load(sys.stdin);seen={}\ndef digest(v):return hashlib.sha256(str(v).encode()).hexdigest()[:24]\ndef claims(token):\n try:\n  p=token.split(\'.\')[1];return json.loads(base64.urlsafe_b64decode(p+\'=\'*(-len(p)%4)))\n except Exception:return {}\ndef onrequest(r):\n h=r.headers;auth=h.get(\'authorization\',\'\')\n if not auth.lower().startswith(\'bearer \'):return\n token=auth[7:];c=claims(token)\n if c.get(\'client_id\')==\'clio-playground-web\':seen[digest(token)]={\'token\':token,\'claims\':c}\nwith sync_playwright() as pw:\n browser=pw.chromium.connect_over_cdp(\'http://127.0.0.1:9222\',timeout=10000)\n ctx=next(c for c in browser.contexts if c.pages);ctx.on(\'request\',onrequest)\n old=list(ctx.pages);page=ctx.new_page()\n for old_page in old:old_page.close()\n session=ctx.new_cdp_session(page)\n session.send(\'Storage.clearDataForOrigin\',{\'origin\':\'https://firefly.adobe.com\',\'storageTypes\':\'local_storage,cache_storage,indexeddb\'})\n ctx.clear_cookies();ctx.add_cookies(cfg[\'cookies\'])\n page.goto(\'https://firefly.adobe.com/generate/image\',wait_until=\'domcontentloaded\',timeout=60000)\n page.wait_for_timeout(30000)\n matched=[];original=claims(cfg[\'record\'][\'access_token\'])\n for key,value in list(seen.items())[-4:]:\n  tok=value[\'token\'];c=value[\'claims\']\n  item={\'hash\':key,\'client_id\':c.get(\'client_id\'),\'account_type\':c.get(\'account_type\'),\'user_id_hash\':digest(c.get(\'user_id\',\'\')),\'same_user_as_okad\':c.get(\'user_id\')==original.get(\'user_id\'),\'same_sid_as_okad\':c.get(\'sid\')==original.get(\'sid\')}\n  try:\n   profile=page.evaluate("""async token => {const r=await fetch(\'https://ims-na1.adobelogin.com/ims/profile/v1\',{headers:{Authorization:\'Bearer \'+token,\'X-Client-Id\':\'clio-playground-web\'}});const d=await r.json();return {status:r.status,email:d.email||\'\'}}""",tok)\n   item[\'profile_status\']=profile[\'status\'];item[\'email_matches\']=str(profile.get(\'email\',\'\')).lower()==cfg[\'email\'].lower()\n   if item[\'profile_status\']==200 and item[\'email_matches\']:\n    f=pathlib.Path(\'/tmp/selected-web-token.json\');f.write_text(json.dumps({\'token\':tok,\'account_id\':cfg[\'account_id\']}));f.chmod(0o600)\n  except Exception as e:item[\'profile_error\']=type(e).__name__\n  matched.append(item)\n out={\'at\':datetime.datetime.now(datetime.timezone.utc).isoformat(),\'url\':urlparse(page.url)._replace(query=\'\',fragment=\'\').geturl(),\'text\':page.locator(\'body\').inner_text(timeout=10000)[:14000],\'native_tokens\':matched,\'imported_cookie_names\':sorted({c[\'name\'] for c in cfg[\'cookies\']})}\n out[\'controls\']=page.locator(\'button,input,textarea,select,[role="combobox"],[role="textbox"]\').evaluate_all(\'(els)=>els.map(e=>({tag:e.tagName,role:e.getAttribute("role"),type:e.type,id:e.id,name:e.getAttribute("name"),text:e.innerText?.slice(0,160),placeholder:e.getAttribute("placeholder"),aria:e.getAttribute("aria-label")}))\')\n print(json.dumps(out,ensure_ascii=False))\n'
private=pathlib.Path('/dev/shm/adobe-three-arm-20261027-private.json')
for _ in range(10):
 if private.exists() or (ROOT/'auth-failed.json').exists():break
 time.sleep(5)
if not private.exists():
 out={'at':now(),'ready':False}
 for name in ['auth-results.json','auth-failed.json']:
  if (ROOT/name).exists():out[name]=json.loads((ROOT/name).read_text())
 print(json.dumps(out,ensure_ascii=False));raise SystemExit(0)
results=json.loads(private.read_text());a=next(a for a in results if a['account_id']==1739);assert a['ok']
r=subprocess.run(['docker','exec','-i','adobe-three-arm-web-20261027','python','-c',code],input=json.dumps(a),capture_output=True,text=True,timeout=150)
out={'at':now(),'returncode':r.returncode,'browser':json.loads(r.stdout) if r.returncode==0 else None,'error_tail':r.stderr[-300:] if r.returncode else None}
save('browser-auth.json',out);print(json.dumps(out,ensure_ascii=False))
