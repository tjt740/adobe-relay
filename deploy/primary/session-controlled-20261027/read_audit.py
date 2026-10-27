import json,datetime,subprocess,sqlite3,hashlib,pathlib,shutil

def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 if r.returncode:raise RuntimeError('read query failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

emit('group_constraints',sql("SELECT json_build_object('name',conname,'type',contype,'validated',convalidated,'def',pg_get_constraintdef(oid)) FROM pg_constraint WHERE conrelid='groups'::regclass ORDER BY conname"))
emit('group_columns',sql("SELECT json_build_object('column',column_name,'type',data_type,'generated',is_generated,'identity',is_identity) FROM information_schema.columns WHERE table_name='groups' ORDER BY ordinal_position"))
emit('group_roundtrip',sql("SELECT json_build_object('id',g.id,'equal',to_jsonb(g)=to_jsonb(x)) FROM groups g CROSS JOIN LATERAL jsonb_populate_record(NULL::groups,to_jsonb(g)) x ORDER BY g.id"))
