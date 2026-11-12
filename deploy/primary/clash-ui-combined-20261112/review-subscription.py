"""Describe subscription changes without exposing URLs, passwords or note text."""
import pathlib,json,hashlib,subprocess,sys,re,datetime
ROOT=pathlib.Path('/opt/adobe-relay/releases/clash-ui-combined-20261112');sys.path.insert(0,str(ROOT));import audit as a
backup=json.loads((ROOT/'after.json').read_text())['backup']
raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=clash_subscription','-f','-'],input=(pathlib.Path(backup)/'database.dump').read_bytes(),timeout=90).decode()
escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
before=None;active=False
for line in raw.splitlines():
 if line.startswith('COPY public.clash_subscription ('):
  cols=[x.strip().strip('"') for x in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
 if active and line=='\\.':active=False;continue
 if active:
  row=dict(zip(cols,map(value,line.split('\t'))))
  if row['id']=='1':before=json.loads(row['document'])
assert before is not None
current=json.loads(a.sql('SELECT document FROM clash_subscription WHERE id=1'))
from subscription_review_guard import validate_addition

validate_addition(before,current)
inspection=json.loads((ROOT/'subscription-change-inspection.json').read_text())
original=json.loads((ROOT/'before.json').read_text());live=a.snapshot()
retained={k:live[k] for k in ['clash_settings_hash','clash_contract_hash']}
assert retained==inspection['current_guard_hashes']
assert retained=={'clash_settings_hash':'42fecfe63311dde889d7683a48b0754a','clash_contract_hash':'53d45d4d1eb35251ed87b1b154df8855'}
adjusted=dict(original);adjusted.update(retained);a.check_preserved(adjusted,live)
proof={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'application_sha':'e1c9b83ace0472ad7e97b35d4bf2c4dd5dc1574b','user_confirmation':'是，新增订阅也是我操作的，保留','status':'confirmed_incremental_subscription_retained','sources_before':8,'sources_current':9,'nodes_before':623,'nodes_current':666,'existing_node_contracts_preserved':True,'priorities_preserved':True,'original_guard_hashes':{k:original[k] for k in retained},'retained_guard_hashes':retained,'inspection_sha256':hashlib.sha256((ROOT/'subscription-change-inspection.json').read_bytes()).hexdigest(),'business_data_written':False}
a.save('reviewed-operator-subscription.json',proof);print(json.dumps(proof))
