import datetime,fcntl,hashlib,json,pathlib,re,sqlite3,subprocess,sys
from urllib.parse import quote
RID='okad-company-mobile-mail-20261014'
ROOT=pathlib.Path('/opt/adobe-relay/releases')/RID
ROOT.mkdir(parents=True,exist_ok=True);ROOT.chmod(0o700)
phase=sys.argv[1]
assert phase in ('before','after-sub2','after','okad-cutover-before')
outfile=ROOT/(phase+'.json')
assert not (phase=='before' and outfile.exists()), 'Preflight already saved; do not overwrite rollback evidence'
locks=[]
for name in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock','/opt/adobe-team-cookie/.local/deploy-login-batches.lock']:
 f=open(name,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);locks.append(f)
def sql(q):
 return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SET LOCAL statement_timeout='10s'; "+q+'; COMMIT;',text=True,capture_output=True,check=True,timeout=20).stdout.strip()
def digest(v):return hashlib.sha256(v).hexdigest()
def inspect(name):return json.loads(subprocess.check_output(['docker','inspect',name]))[0]
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'containers':{},'config_hashes':{}}
for name in ['adobe-relay-app-1','adobe-team-cookie-app-1','adobe-relay-postgres-1','adobe-relay-redis-1','adobe-relay-mihomo-1']:
 d=inspect(name);out['containers'][name]={'id':d['Id'],'image':d['Config']['Image'],'image_id':d['Image'],'started':d['State']['StartedAt'],'health':d['State'].get('Health',{}).get('Status'),'restarts':d['RestartCount']}
for path in ['/opt/adobe-relay/shared/.env','/opt/adobe-relay/shared/compose.yml','/opt/adobe-relay/shared/data/config.yaml','/opt/adobe-relay/shared/clash.yml','/opt/adobe-relay/shared/resources.yml','/opt/adobe-team-cookie/.env','/opt/adobe-team-cookie/compose.yml']:
 out['config_hashes'][path]=digest(pathlib.Path(path).read_bytes())
out['sub2_env_without_image']=digest('\n'.join(line for line in pathlib.Path('/opt/adobe-relay/shared/.env').read_text().splitlines() if not line.startswith('SUB2API_IMAGE=')).encode())
out['okad_compose_without_image']=digest(re.sub(r'(?m)^(    image:)\s*adobe-team-cookie:[^\s]+[ \t]*$',r'\1 <image>',pathlib.Path('/opt/adobe-team-cookie/compose.yml').read_text()).encode())
okad=inspect('adobe-team-cookie-app-1');url=next(v.split('=',1)[1] for v in okad['Config']['Env'] if v.startswith('DATABASE_URL='));assert url.startswith('sqlite:///')
p=pathlib.PurePosixPath(url[len('sqlite:///'):]);mount=next(m for m in sorted(okad['Mounts'],key=lambda m:len(m['Destination']),reverse=True) if p.is_relative_to(m['Destination']))
host=pathlib.Path(mount['Source'])/str(p.relative_to(mount['Destination']))
with sqlite3.connect('file:'+quote(str(host))+'?mode=ro',uri=True,timeout=5) as db:
 settings=db.execute('SELECT key,value FROM settings ORDER BY key').fetchall()
 out['okad_settings_hash']=digest(json.dumps(settings,ensure_ascii=False).encode())
 out['okad_company_counts']=dict(db.execute("SELECT CASE WHEN otp_mode!='manual' AND COALESCE(mail_url,'')='' AND NOT(COALESCE(refresh_token,'')!='' AND COALESCE(client_id,'')!='') THEN 'company' ELSE 'non_company' END,count(*) FROM external_members GROUP BY 1").fetchall())
 out['okad_account_credentials_hash']=digest(json.dumps(db.execute('SELECT id,email,cookie FROM external_members ORDER BY id').fetchall(),ensure_ascii=False).encode())
out['mail_hash']=sql("SELECT md5(COALESCE(string_agg(key||':'||value,',' ORDER BY key),'')) FROM settings WHERE key LIKE 'smtp_%' OR key IN ('ops_adobe_alert_mail_private','ops_email_notification_config','email_verify_enabled','ops_alert_runtime_settings')")
out['rule']=json.loads(sql('SELECT row_to_json(r) FROM ops_alert_rules r WHERE id=9'))
out['pagination']=json.loads(sql("SELECT COALESCE(json_object_agg(key,value),'{}') FROM settings WHERE key IN ('table_default_page_size','table_page_size_options')"))
out['account_state']=json.loads(sql("SELECT COALESCE(json_agg(json_build_object('id',id,'credentials_hash',md5(credentials::text),'proxy_id',proxy_id,'status',status,'schedulable',schedulable,'manual_pause',extra->>'okad_manual_pause','cookie_version',extra->'okad'->>'version') ORDER BY id),'[]') FROM accounts WHERE deleted_at IS NULL"))
out['metadata_counts']=json.loads(sql("SELECT COALESCE(json_object_agg(kind,n),'{}') FROM (SELECT CASE WHEN extra->'okad_account'->>'is_company'='true' THEN 'company' WHEN extra->'okad_account'->>'is_company'='false' THEN 'non_company' ELSE 'unclassified' END kind,count(*) n FROM accounts WHERE deleted_at IS NULL AND platform='adobe' GROUP BY 1) q"))
script="""import hashlib,json,pathlib
root=pathlib.Path('/app');out={}
for p in root.rglob('*'):
 if p.is_file() and '__pycache__' not in str(p) and '.venv' not in str(p) and p.suffix in ('.py','.js','.css','.html'):
  out[str(p.relative_to(root))]=hashlib.sha256(p.read_bytes()).hexdigest()
print(json.dumps(out,sort_keys=True))
"""
out['okad_source_hashes']=json.loads(subprocess.run(['docker','exec','-i','adobe-team-cookie-app-1','python','-'],input=script,text=True,capture_output=True,check=True,timeout=30).stdout)
if phase=='before':
 assert out['containers']['adobe-relay-app-1']['image']=='adobe-relay:71c9b10e1c70c883cf3686d7b5ee5fe0a0c6583d','Sub2 version changed'
 subprocess.run(['docker','tag',out['containers']['adobe-relay-app-1']['image_id'],'adobe-relay:before-'+RID],check=True)
 # Preserve writable-layer Okad overlays, not only its historical image tag.
 subprocess.run(['docker','commit','--pause=false','adobe-team-cookie-app-1','adobe-team-cookie:before-'+RID],check=True,capture_output=True)
 out['okad_rollback_image_id']=json.loads(subprocess.check_output(['docker','image','inspect','adobe-team-cookie:before-'+RID]))[0]['Id']
else:
 before=json.loads((ROOT/'before.json').read_text())
 reviewed_settings=False
 if out['okad_settings_hash']!=before['okad_settings_hash'] and (ROOT/'reviewed-concurrent-settings.json').exists():
  # Keep independently saved settings; require the exact reviewed change and
  # anchor every other key to the immutable pre-release database snapshot.
  review=json.loads((ROOT/'reviewed-concurrent-settings.json').read_text())
  with sqlite3.connect('file:'+str(ROOT/'okad-pre-release/app.db')+'?mode=ro',uri=True) as db:
   old_settings=db.execute('SELECT key,value FROM settings ORDER BY key').fetchall()
  assert digest(json.dumps(old_settings,ensure_ascii=False).encode())==before['okad_settings_hash']
  old=dict(old_settings);current=dict(settings)
  changed={key for key in old.keys()|current.keys() if old.get(key)!=current.get(key)}
  assert changed==set(review['changes']), 'Unreviewed concurrent settings change'
  for key in changed:
   item=review['changes'][key]
   assert digest(str(old.get(key)).encode())==item['before_hash']
   assert digest(str(current.get(key)).encode())==item['after_hash']
   assert item['saved_at']<review['okad_deployed_at']
  out['reviewed_concurrent_settings']=review
  reviewed_settings=True
 out['preserved']={
  'other_containers':all(out['containers'][k]['id']==v['id'] for k,v in before['containers'].items() if k not in ('adobe-relay-app-1','adobe-team-cookie-app-1')),
  'config_files':all(out['config_hashes'][k]==v for k,v in before['config_hashes'].items() if k not in ('/opt/adobe-relay/shared/.env','/opt/adobe-team-cookie/compose.yml')),
  'sub2_env':out['sub2_env_without_image']==before['sub2_env_without_image'],
  'okad_compose':out['okad_compose_without_image']==before['okad_compose_without_image'],
  'okad_settings_preserved_or_reviewed':out['okad_settings_hash']==before['okad_settings_hash'] or reviewed_settings,
  'smtp':out['mail_hash']==before['mail_hash'],
  'rule':all(out['rule'][k]==v for k,v in before['rule'].items() if k not in ('updated_at','last_triggered_at')),
  'pagination':out['pagination']==before['pagination']}
 okad_before=json.loads((ROOT/'okad-cutover-before.json').read_text()) if (ROOT/'okad-cutover-before.json').exists() else before
 out['changed_okad_files']=[k for k in sorted(set(out['okad_source_hashes'])|set(okad_before['okad_source_hashes'])) if out['okad_source_hashes'].get(k)!=okad_before['okad_source_hashes'].get(k)]
 if phase=='after':
  assert set(out['changed_okad_files'])=={'backend/app/api/routes/external_member.py','backend/app/services/external_sub2.py','backend/app/services/reliable_integration.py','backend/app/services/okad_account_metadata.py'},out['changed_okad_files']
 assert all(out['preserved'].values()),out['preserved']
if outfile.exists():outfile=ROOT/(phase+'-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'.json')
outfile.write_text(json.dumps(out,ensure_ascii=False,indent=2));outfile.chmod(0o600)
print(json.dumps({'file':str(outfile),'at':out['at'],'containers':out['containers'],'account_count':len(out['account_state']),'metadata_counts':out['metadata_counts'],'okad_company_counts':out['okad_company_counts'],'preserved':out.get('preserved'),'changed_okad_files':out.get('changed_okad_files')},ensure_ascii=False))
