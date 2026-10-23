"""Back up before the candidate can apply the additive 408 state migration."""
import datetime,fcntl,hashlib,importlib.util,json,os,pathlib,shutil,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/account-manual-ip-20261023')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
os.umask(0o077)
held=[]
for p in ['/opt/adobe-relay/.release.lock','/opt/adobe-team-cookie/.release.lock','/opt/adobe-team-cookie/.incremental-release.lock']:
 f=open(p,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);held.append(f)
assert not (ROOT/'premigration-backup.json').exists()
assert a.inspect(a.APP)['Config']['Image']==a.OLD_IMAGE
assert not subprocess.check_output(['docker','exec','adobe-relay-redis-1','redis-cli','--raw','GET','concurrency:rolling_deployment'],text=True).strip()
backup=pathlib.Path('/opt/adobe-relay/backups')/(datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-before-adobe-route-health')
backup.mkdir(mode=0o700)
for name in ['.env','compose.yml','clash.yml','resources.yml']:
 shutil.copy2(pathlib.Path('/opt/adobe-relay/shared')/name,backup/('environment' if name=='.env' else name))
with (backup/'database.dump').open('wb') as f:subprocess.run(['docker','exec','adobe-relay-postgres-1','pg_dump','-U','sub2api','-d','sub2api','-Fc'],stdout=f,check=True,timeout=300)
with (backup/'database.dump').open('rb') as f:subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--list'],stdin=f,stdout=subprocess.DEVNULL,check=True,timeout=60)
with (backup/'app-data.tar.gz').open('wb') as f:subprocess.run(['docker','exec',a.APP,'tar','-C','/app','-czf','-','data'],stdout=f,check=True,timeout=300)
(backup/'previous-image').write_text(a.OLD_IMAGE+'\n')
installed=json.loads(a.sql("SELECT json_object_agg(filename,checksum) FROM schema_migrations"))
a.save('installed-migrations-before.json',installed)
contracts={}
for name,query in {
 'groups':"SELECT md5(coalesce(jsonb_agg(to_jsonb(g)-'updated_at'-'i18n' ORDER BY id)::text,'')) FROM groups g",
 'account_groups':"SELECT md5(coalesce(jsonb_agg(to_jsonb(g) ORDER BY account_id,group_id)::text,'')) FROM account_groups g",
 'apikey_binding':"SELECT md5(coalesce(jsonb_agg(jsonb_build_object('id',id,'key',key,'user_id',user_id,'group_id',group_id,'status',status,'deleted_at',deleted_at) ORDER BY id)::text,'')) FROM api_keys",
 'users_identity':"SELECT md5(coalesce(jsonb_agg(jsonb_build_object('id',id,'email',email,'password_hash',password_hash,'role',role,'status',status,'deleted_at',deleted_at) ORDER BY id)::text,'')) FROM users",
 'user_quotas':"SELECT md5(coalesce(jsonb_agg(to_jsonb(q) ORDER BY user_id,platform)::text,'')) FROM user_platform_quotas q"
}.items():contracts[name]=a.sql(query)
a.save('business-contracts-before.json',contracts)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'backup':str(backup),'database_bytes':(backup/'database.dump').stat().st_size,'database_sha256':hashlib.sha256((backup/'database.dump').read_bytes()).hexdigest(),'installed_migration_count':len(installed),'source_image':a.OLD_IMAGE,'dump_verified':True,'routing_changed':False}
a.save('premigration-backup.json',out);print(json.dumps(out))
subprocess.run(['/usr/bin/python3',str(ROOT/'capacity.py'),'before'],check=True,timeout=90)
