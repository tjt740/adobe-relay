import hashlib,json,pathlib,re,subprocess
root=pathlib.Path('/opt/adobe-relay/releases/okad-details-display-20261013')
backup=pathlib.Path('/opt/adobe-relay/backups/20261013T092251Z-f28f14480dc2a90fdd0e98ff7d85569a5b6649fd/database.dump')
p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','--table=accounts','-f','-'],input=backup.read_bytes(),capture_output=True,check=True,timeout=60)
sql=p.stdout.decode()
copy=re.search(r'COPY public\.accounts \(([^)]+)\) FROM stdin;\n(.*?)\n\\\.\n',sql,re.S)
assert copy,'Account table not found in deployment backup'
columns=copy[1].split(', ')
rows=[line.split('\t') for line in copy[2].splitlines()]
account_id=columns.index('id');proxy_id=columns.index('proxy_id');deleted_at=columns.index('deleted_at')
rows=sorted((r for r in rows if r[deleted_at]=='\\N'),key=lambda r:int(r[account_id]))
signature=','.join(r[account_id]+':'+('' if r[proxy_id]=='\\N' else r[proxy_id]) for r in rows)
cutover={'count':len(rows),'proxy_digest':hashlib.md5(signature.encode()).hexdigest()}
query="select json_build_object('count',count(*),'proxy_digest',md5(coalesce(string_agg(id::text||':'||coalesce(proxy_id::text,''),',' order by id),''))) from accounts where deleted_at is null;"
current=json.loads(subprocess.check_output(['docker','exec','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atc',query],text=True))
before=json.loads((root/'before.json').read_text())['sub2_accounts']
report={'backup':str(backup),'preflight':before,'cutover':cutover,'current':current,
        'changed_before_deploy':before!=cutover,'matches_actual_cutover':cutover==current}
(root/'proxy-audit.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
assert report['matches_actual_cutover'],'Current proxy bindings differ from cutover backup; further review needed'
