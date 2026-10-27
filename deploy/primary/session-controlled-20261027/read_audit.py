import pathlib,json,datetime,subprocess,sqlite3,hashlib
base=pathlib.Path('/opt/adobe-relay/releases/408-causal-20261028')
def emit(k,d):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':k,'data':d},ensure_ascii=False))
names=subprocess.check_output(['docker','ps','-a','--filter','name=adobe408-diag','--format','{{.Names}}'],text=True).splitlines()
private=[str(x) for x in pathlib.Path('/dev/shm').glob('adobe408-*')]
emit('diagnostic_cleanup',{'containers':names,'private_paths':private})
assert not names and not private,'bounded trials must be cleaned before final audit'
trials=[]
for tag in ['berg-a','berg-a2','berg-pa','craw-b','craw-a','donald-pa','donald-pb','donald-pa2']:
 root=base/tag
 if not root.exists():continue
 item={'tag':tag,'cleaned':json.loads((root/'cleaned.json').read_text()) if (root/'cleaned.json').exists() else None,'auth_failed':(root/'first-auth-failure.json').exists(),'trial_error':(root/'trial-error.json').exists()}
 if (root/'production-before.json').exists() and (root/'production-after.json').exists():
  a=json.loads((root/'production-before.json').read_text());z=json.loads((root/'production-after.json').read_text());item['account_and_sub2_hashes_preserved']=a['accounts_and_settings']==z['accounts_and_settings'];item['okad_selected_preserved']=a['okad_selected']==z['okad_selected'];item['production_containers_preserved']=a['containers']==z['containers']
  if a.get('okad_settings_by_key') and z.get('okad_settings_by_key'):
   ak=a['okad_settings_by_key'];zk=z['okad_settings_by_key'];item['okad_changed_keys']=[k for k in ak.keys()|zk.keys() if ak.get(k)!=zk.get(k)]
 trials.append(item)
emit('trial_invariants',trials)
cmd=['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1']
q="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; SELECT json_build_object('id',id,'status',status,'schedulable',schedulable,'proxy_id',proxy_id,'source_cookie_md5',md5(credentials->>'cookie'),'runtime_cookie_md5',md5(credentials->>'adobe_session_cookie'),'token_present',NULLIF(credentials->>'access_token','') IS NOT NULL,'manual_proxy',extra->'manual_proxy_enabled') FROM accounts WHERE id IN (1738,1815,1818) ORDER BY id; COMMIT;"
r=subprocess.run(cmd,input=q,capture_output=True,text=True,check=True,timeout=20);emit('production_selected_final',[json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')])
emit('production_runtime',[{'name':n,'image':(x:=json.loads(subprocess.check_output(['docker','inspect',n],text=True))[0])['Config']['Image'],'started':x['State']['StartedAt'],'restarts':x['RestartCount'],'memory':x['HostConfig']['Memory']} for n in ['adobe-relay-app-1','adobe-team-cookie-app-1']])
