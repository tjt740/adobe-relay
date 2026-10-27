import json,datetime,subprocess,sqlite3,hashlib,pathlib,shutil

def emit(kind,data):print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':kind,'data':data},ensure_ascii=False))
def sql(q):
 r=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY;SET LOCAL statement_timeout='12s';"+q+';COMMIT;',text=True,capture_output=True,timeout=20)
 if r.returncode:raise RuntimeError('read query failed')
 return [json.loads(x) for x in r.stdout.splitlines() if x.startswith('{')]

import importlib.util
emit('yaml_available',bool(importlib.util.find_spec('yaml')))
files=list(pathlib.Path('/opt/adobe-relay/shared/data').glob('*'));emit('data_files',[x.name for x in files if x.is_file()])
import yaml
f=pathlib.Path('/opt/adobe-relay/shared/data/config.yaml');d=yaml.safe_load(f.read_text());sensitive=('secret','password','key','token','url','host','email','dsn','endpoint','bucket','access')
def safe(v):
 if not isinstance(v,dict):return v
 return {k:('[redacted]' if any(w in k.lower() for w in sensitive) else safe(x)) for k,x in v.items()}
emit('config',safe(d))
emit('routing_config',sql("SELECT json_build_object('table','proxy_auto_allocation','value',to_jsonb(t)) FROM proxy_auto_allocation t UNION ALL SELECT json_build_object('table','account_proxy_failover','value',to_jsonb(t)) FROM account_proxy_failover t WHERE account_id=1738"))
emit('policy',sql("SELECT json_build_object('key',to_jsonb(k)-'key','user',to_jsonb(u)-'password_hash'-'totp_secret_encrypted'-'email'-'wechat','group',to_jsonb(g)) FROM api_keys k JOIN users u ON u.id=k.user_id JOIN groups g ON g.id=k.group_id WHERE k.id=62"))
emit('timeline',sql("SELECT json_build_object('at',created_at,'start',created_at-duration_ms*interval '1 millisecond','duration_ms',duration_ms,'endpoint',inbound_endpoint,'model',model,'input_size',image_input_size,'output_size',image_output_size,'size_source',image_size_source) FROM usage_logs WHERE account_id=1738 AND created_at BETWEEN '2026-10-27T18:14:00+08:00' AND '2026-10-27T18:18:00+08:00' ORDER BY created_at"))
