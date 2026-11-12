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
def digest(v):return hashlib.sha256(json.dumps(v,sort_keys=True,ensure_ascii=False).encode()).hexdigest()
urls=before.get('url','').split();after_urls=current.get('url','').split()
changes=[]
for url in dict.fromkeys(urls+after_urls):
 oldnote=(before.get('notes') or {}).get(url,'');newnote=(current.get('notes') or {}).get(url,'')
 oldpri=(before.get('priorities') or {}).get(url,0);newpri=(current.get('priorities') or {}).get(url,0)
 if oldnote!=newnote or oldpri!=newpri:changes.append({'source_index':after_urls.index(url)+1 if url in after_urls else None,'source_fingerprint':digest(url),'note_changed':oldnote!=newnote,'priority_before':oldpri,'priority_current':newpri})
fields=[k for k in set(before)|set(current) if before.get(k)!=current.get(k)]
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'read_only':True,'cutover_backup':backup,'changed_fields':sorted(fields),'sources_before':len(urls),'sources_current':len(after_urls),'source_urls_preserved':urls==after_urls,'nodes_before':len(before.get('nodes') or []),'nodes_current':len(current.get('nodes') or []),'nodes_preserved':before.get('nodes')==current.get('nodes'),'settings_changes':changes,'revision_before':before.get('revision'),'revision_current':current.get('revision'),'settings_revision_before':before.get('settings_revision'),'settings_revision_current':current.get('settings_revision'),'field_hashes':{k:{'before':digest(before.get(k)),'current':digest(current.get(k))} for k in fields},'current_guard_hashes':{k:v for k,v in a.snapshot().items() if k in ['clash_settings_hash','clash_contract_hash']}}
a.save('subscription-change-inspection.json',out);print(json.dumps(out))
