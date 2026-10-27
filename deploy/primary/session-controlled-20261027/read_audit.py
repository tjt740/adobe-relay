import json,datetime,subprocess,sqlite3,hashlib,pathlib,shutil

def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 if r.returncode:raise RuntimeError('read query failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

emit('tables',sql("SELECT json_build_object('table',table_name) FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"))
emit('usage_samples',sql("SELECT to_jsonb(t)-'user_agent'-'ip_address' FROM (SELECT * FROM usage_logs WHERE account_id=1738 ORDER BY created_at DESC LIMIT 8) t"))
emit('settings_keys',sql("SELECT json_build_object('key',key,'hash',md5(value),'len',length(value)) FROM settings ORDER BY key"))
emit('configs',sql("SELECT json_build_object('table',table_name,'columns',json_agg(column_name ORDER BY ordinal_position)) FROM information_schema.columns WHERE table_schema='public' AND (table_name LIKE '%proxy%' OR table_name LIKE '%okad%' OR table_name LIKE '%setting%' OR table_name LIKE '%adobe%' OR table_name LIKE '%subscription%') GROUP BY table_name ORDER BY table_name"))
emit('account_extra',sql("SELECT json_build_object('extra',extra,'model_mapping',credentials->'model_mapping') FROM accounts WHERE id=1738"))
d=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1'],text=True))[0]
emit('app_runtime',{'entrypoint':d['Config']['Entrypoint'],'cmd':d['Config']['Cmd'],'workingdir':d['Config']['WorkingDir'],'mounts':[{'source':x['Source'],'dest':x['Destination']} for x in d['Mounts']]})
