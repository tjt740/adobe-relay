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




(ROOT/'web_owner.py').write_text('import json,sys,datetime,hashlib,base64,pathlib\nfrom urllib.parse import urlparse\nfrom playwright.sync_api import sync_playwright\ncfg=json.load(sys.stdin);seen={}\ndef digest(v):return hashlib.sha256(str(v).encode()).hexdigest()[:24]\ndef claims(token):\n try:\n  p=token.split(\'.\')[1];return json.loads(base64.urlsafe_b64decode(p+\'=\'*(-len(p)%4)))\n except Exception:return {}\ndef onrequest(r):\n h=r.headers;auth=h.get(\'authorization\',\'\')\n if not auth.lower().startswith(\'bearer \'):return\n token=auth[7:];c=claims(token)\n if c.get(\'client_id\')==\'clio-playground-web\':seen[digest(token)]={\'token\':token,\'claims\':c}\nwith sync_playwright() as pw:\n p=cfg[\'proxy_config\'];proxy={\'server\':p[\'protocol\']+\'://\'+p[\'host\']+\':\'+str(p[\'port\'])}\n if p.get(\'username\'):proxy.update(username=p[\'username\'],password=p.get(\'password\') or \'\')\n browser=pw.chromium.launch(executable_path=\'/experiment/chrome/chrome-linux64/chrome\',headless=True,proxy=proxy,args=[\'--remote-debugging-port=9222\',\'--remote-debugging-address=127.0.0.1\'])\n ctx=browser.new_context(viewport={\'width\':1365,\'height\':950},locale=\'en-US\');ctx.on(\'request\',onrequest)\n ctx.add_cookies(cfg[\'cookies\']);page=ctx.new_page()\n page.goto(\'https://firefly.adobe.com/generate/image\',wait_until=\'domcontentloaded\',timeout=60000)\n page.wait_for_timeout(30000)\n matched=[];original=claims(cfg[\'record\'][\'access_token\'])\n for key,value in list(seen.items())[-4:]:\n  tok=value[\'token\'];c=value[\'claims\']\n  item={\'hash\':key,\'client_id\':c.get(\'client_id\'),\'account_type\':c.get(\'account_type\'),\'user_id_hash\':digest(c.get(\'user_id\',\'\')),\'same_user_as_okad\':c.get(\'user_id\')==original.get(\'user_id\'),\'same_sid_as_okad\':c.get(\'sid\')==original.get(\'sid\')}\n  try:\n   profile=page.evaluate("""async token => {const r=await fetch(\'https://ims-na1.adobelogin.com/ims/profile/v1\',{headers:{Authorization:\'Bearer \'+token,\'X-Client-Id\':\'clio-playground-web\'}});const d=await r.json();return {status:r.status,email:d.email||\'\'}}""",tok)\n   item[\'profile_status\']=profile[\'status\'];item[\'email_matches\']=str(profile.get(\'email\',\'\')).lower()==cfg[\'email\'].lower()\n   if item[\'profile_status\']==200 and item[\'email_matches\']:\n    f=pathlib.Path(\'/tmp/selected-web-token.json\');f.write_text(json.dumps({\'token\':tok,\'account_id\':cfg[\'account_id\']}));f.chmod(0o600)\n  except Exception as e:item[\'profile_error\']=type(e).__name__\n  matched.append(item)\n out={\'at\':datetime.datetime.now(datetime.timezone.utc).isoformat(),\'url\':urlparse(page.url)._replace(query=\'\',fragment=\'\').geturl(),\'text\':page.locator(\'body\').inner_text(timeout=10000)[:14000],\'native_tokens\':matched,\'imported_cookie_names\':sorted({c[\'name\'] for c in cfg[\'cookies\']})}\n out[\'controls\']=page.locator(\'button,input,textarea,select,[role="combobox"],[role="textbox"]\').evaluate_all(\'(els)=>els.map(e=>({tag:e.tagName,role:e.getAttribute("role"),type:e.type,id:e.id,name:e.getAttribute("name"),text:e.innerText?.slice(0,160),placeholder:e.getAttribute("placeholder"),aria:e.getAttribute("aria-label")}))\')\n pathlib.Path(\'/control/web-auth-ready.json\').write_text(json.dumps(out,ensure_ascii=False))\n import time\n deadline=time.monotonic()+2700\n while time.monotonic()<deadline and not pathlib.Path(\'/control/stop-web\').exists():\n  try:\n   if page.is_closed():page=ctx.pages[0] if ctx.pages else ctx.new_page()\n   page.wait_for_timeout(500)\n  except Exception:\n   if not browser.is_connected():break\n browser.close()\n')

private=pathlib.Path('/dev/shm/adobe-three-arm-20261027-private.json');assert private.exists()
byid={a['account_id']:a for a in json.loads(private.read_text())}
a=next(a for a in rows(True) if a['id']==1739);assert a['manual_pause'] and a['manual_proxy'] and a['proxy_id']==1398
config=byid[1739];config['proxy_config']=a['proxy']
subprocess.run(['docker','stop','-t','5','adobe-three-arm-web-20261027'],capture_output=True,timeout=15)
control=ROOT/'control'
assert not (control/'stop-web').exists() and not (control/'web-auth-ready.json').exists()
cmd=['docker','run','--rm','-i','--name','adobe-three-arm-web-20261027','--network','adobe-relay_default','--memory','2g','--cpus','1.5','--read-only','--tmpfs','/root/.config:rw,size=32m','--tmpfs','/root/.cache:rw,size=32m','--tmpfs','/tmp:rw,size=512m','--shm-size','256m','--mount',f'type=bind,src={ROOT},dst=/experiment,readonly','--mount',f'type=bind,src={control},dst=/control','--entrypoint','python','adobe-team-cookie:79382b0f79d559575956e023dc9f52fbf07dbfde','/experiment/web_owner.py']
f=open(control/'web-owner-process.log','a');proc=subprocess.Popen(cmd,stdin=subprocess.PIPE,stdout=f,stderr=f,text=True,start_new_session=True);proc.stdin.write(json.dumps(config));proc.stdin.close()
print(json.dumps({'at':now(),'web_owner_started':True,'pid':proc.pid,'auth_results':json.loads((ROOT/'auth-results.json').read_text())}))
