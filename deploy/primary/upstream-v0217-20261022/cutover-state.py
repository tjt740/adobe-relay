"""Read the exact cutover baseline; do not overwrite live state with early preflight."""
import importlib.util,json,pathlib,re,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/upstream-v0217-20261022')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)
backup=pathlib.Path(json.loads((ROOT/'after.json').read_text())['backup'])
raw=subprocess.check_output(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=(backup/'database.dump').read_bytes(),timeout=90).decode()
escapes={'b':'\b','f':'\f','n':'\n','r':'\r','t':'\t','v':'\v','\\':'\\'}
def value(s):return None if s=='\\N' else re.sub(r'\\(.)',lambda m:escapes.get(m[1],m[1]),s)
rows=[];active=False
for line in raw.splitlines():
 if line.startswith('COPY public.accounts ('):
  cols=[c.strip().strip('"') for c in line.split('(',1)[1].split(')',1)[0].split(',')];active=True;continue
 if active and line=='\\.':active=False;continue
 if active:
  r=dict(zip(cols,map(value,line.split('\t'))))
  if r['deleted_at'] is None:
   manual=json.loads(r['extra'] or '{}').get('okad_manual_pause')
   rows.append({'id':int(r['id']),'status':r['status'],'schedulable':r['schedulable']=='t','manual_pause':str(manual).lower() if manual is not None else None})
assert len(rows)==978
a.save('cutover-account-state.json',rows)
early={r['id']:r for r in json.loads((ROOT/'before.json').read_text())['account_state']}
changes=[{'id':r['id'],'fields':[k for k in ['status','schedulable','manual_pause'] if early[r['id']][k]!=r[k]],'preflight':{k:early[r['id']][k] for k in ['status','schedulable']},'cutover':{k:r[k] for k in ['status','schedulable']}} for r in rows if any(early[r['id']][k]!=r[k] for k in ['status','schedulable','manual_pause'])]
a.save('precutover-live-state-changes.json',{'cutover_backup':str(backup),'changes_between_early_preflight_and_actual_cutover':changes,'restored_old_data':False})
print(json.dumps({'cutover_baseline_recorded':True,'accounts':len(rows),'early_preflight_changed_states':len(changes),'restored_old_data':False}))
