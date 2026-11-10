import datetime,json,os,pathlib,re,subprocess,time
IDS=['7792ed46-03df-412f-9c90-f3a2a3eb0616','14a3ccc8-e8d7-4a0e-b721-62062bd73ee8','c4a2dfa3-c7b4-428b-aafb-3b1ca1adfc32','946668ca-f4fa-4668-917f-c40735e97662','c017d781-e63c-4855-b902-8188a1c9cd71','a5265452-0362-4947-af8d-d5268612b80a','381d8732-ac51-4088-b6be-f101624cc0ed','acb4bdfe-6219-4fa3-b30c-85a514ca5e78','8cc904c8-c79d-45b8-8d7f-708ae5ca812d']
def cmd(args,timeout=15):
 p=subprocess.run(args,capture_output=True,text=True,timeout=timeout)
 if p.returncode:raise RuntimeError('command_failed:'+args[0]+':'+str(p.returncode))
 return p.stdout
def sql(q):
 p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='12s'; "+q+'; COMMIT;',capture_output=True,text=True,timeout=20)
 if p.returncode:return {'error':p.stderr[:500]}
 return json.loads(p.stdout)
def redis(*args):return cmd(['docker','exec','adobe-relay-redis-1','redis-cli','--raw',*map(str,args)])
out={'at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'read_only':True}
app=json.loads(cmd(['docker','inspect','adobe-relay-app-1']))[0]
out['runtime']={'image':app['Config']['Image'],'started':app['State']['StartedAt'],'restarts':app['RestartCount']}
env=dict(x.split('=',1) for x in app['Config'].get('Env',[]) if '=' in x)
out['image_storage_environment']={k:(v if k in ('IMAGE_STORAGE_ENABLED',) else bool(v)) for k,v in env.items() if k.startswith('IMAGE_STORAGE_')}
out['storage_settings']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT key,(value::jsonb)->>'enabled' enabled,(value::jsonb)->>'reuse_backup_s3' reuse_backup_s3,updated_at FROM settings WHERE key='image_storage_config') t")
out['response_related_columns']=sql("SELECT coalesce(json_agg(t),'[]') FROM (SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='public' AND (column_name ~ '(response_body|response_data|result|image_url|b64)' OR table_name IN ('ops_system_logs','idempotency_records')) ORDER BY table_name,ordinal_position) t")
out['task_cache']={'exact_id_keys':[],'total_scanned':0,'scan_complete':False,'candidate_metadata':[]}
for ident in IDS:
 k='image_task:'+ident
 if redis('EXISTS',k).strip()=='1':out['task_cache']['exact_id_keys'].append(k)
cursor='0';keys=[];started=time.monotonic()
while time.monotonic()-started<15 and len(keys)<1000:
 parts=redis('SCAN',cursor,'MATCH','image_task:*','COUNT','200').splitlines()
 if not parts or not parts[0].isdigit():
  out['task_cache']['scan_error']='unexpected_redis_response';break
 cursor=parts[0];keys.extend(parts[1:])
 if cursor=='0':out['task_cache']['scan_complete']=True;break
keys=list(dict.fromkeys(keys));out['task_cache']['total_scanned']=len(keys)
for k in keys[:250]:
 raw=redis('GET',k)
 try:t=json.loads(raw)
 except:continue
 if t.get('user_id')!=152 or t.get('api_key_id')!=107:continue
 created=t.get('created_at',0)
 if not (1794294780<=created<=1794295320):continue
 result=t.get('result')
 if isinstance(result,str):
  try:result=json.loads(result)
  except:result=None
 data=result.get('data',[]) if isinstance(result,dict) else []
 out['task_cache']['candidate_metadata'].append({'key':k,'created_at':created,'status':t.get('status'),'completed_at':t.get('completed_at'),'has_result':bool(result),'images':[{'base64_chars':len(x.get('b64_json','')),'has_url':bool(x.get('url'))} for x in data if isinstance(x,dict)]})
out['task_cache']['values_examined']=min(len(keys),250)
root=pathlib.Path('/opt/adobe-relay/shared/data');walked=0;matches=[];dirs=[]
for current,subdirs,files in os.walk(root):
 subdirs[:]=[x for x in subdirs if x not in ('logs','.git','node_modules')]
 if current==str(root):dirs=subdirs[:]
 for name in files:
  walked+=1
  if any(x in name for x in IDS):
   p=pathlib.Path(current)/name;matches.append({'path':str(p),'bytes':p.stat().st_size})
 if walked>20000:break
out['persistent_data']={'top_level_dirs':dirs,'filenames_scanned':walked,'scan_complete':walked<=20000,'matching_filenames':matches}
config=root/'config.yaml'
out['config_file_storage']={'file_exists':config.exists(),'section_present':False}
if config.exists():
 lines=config.read_text().splitlines();inside=False
 for line in lines:
  if re.match(r'^image_storage\s*:',line):inside=True;out['config_file_storage']['section_present']=True;continue
  if inside and line and not line[0].isspace() and not line.lstrip().startswith('#'):break
  if inside:
   m=re.match(r'\s+(enabled|bucket|access_key_id|secret_access_key|endpoint)\s*:\s*(.*)',line)
   if m:out['config_file_storage'][m[1]]=m[2] if m[1]=='enabled' else bool(m[2].strip(' "\''))
try:
 p=subprocess.run(['nginx','-T'],capture_output=True,text=True,timeout=12)
 text=p.stdout+'\n'+p.stderr
 out['nginx_cache_directives']=[line.strip() for line in text.splitlines() if re.match(r'^\s*(proxy_cache\s|proxy_cache_path\s|proxy_buffering\s|proxy_store\s|proxy_temp_path\s)',line)]
except Exception as e:out['nginx_cache_directives']={'error':type(e).__name__}
out['temporary_file_metadata']={}
for path in [pathlib.Path('/proc')/str(app['State']['Pid'])/'root/tmp',pathlib.Path('/var/cache/nginx/proxy_temp'),pathlib.Path('/var/lib/nginx/proxy')]:
 if not path.exists():out['temporary_file_metadata'][str(path)]={'exists':False};continue
 count=0;matched=[]
 for current,ds,fs in os.walk(path):
  for name in fs:
   count+=1
   if any(x in name for x in IDS):matched.append(str(pathlib.Path(current)/name))
  if count>5000:break
 out['temporary_file_metadata'][str(path)]={'exists':True,'files_scanned':count,'matching_names':matched}
print(json.dumps(out,ensure_ascii=False,indent=2))
