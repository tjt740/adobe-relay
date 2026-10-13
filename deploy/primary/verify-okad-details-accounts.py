import hashlib,json,pathlib,re,subprocess
root=pathlib.Path('/opt/adobe-relay/releases/okad-details-display-20261013')
backup=pathlib.Path('/opt/adobe-relay/backups/20261013T092251Z-f28f14480dc2a90fdd0e98ff7d85569a5b6649fd/database.dump')
tables=['accounts','proxy_auto_allocation','proxy_failover_events']
args=['docker','exec','-i','adobe-relay-postgres-1','pg_restore','--data-only','-f','-']+['--table='+t for t in tables]
p=subprocess.run(args,input=backup.read_bytes(),capture_output=True,check=True,timeout=60)
sql=p.stdout.decode()
def table(name):
    copy=re.search(r'COPY public\.'+name+r' \(([^)]+)\) FROM stdin;\n(.*?)^\\\.\n',sql,re.S|re.M)
    assert copy,'Missing backup table: '+name
    columns=copy[1].split(', ')
    return [dict(zip(columns,line.split('\t'))) for line in copy[2].splitlines()]
rows=[r for r in table('accounts') if r['deleted_at']==r'\N']
bindings={int(r['id']):0 if r['proxy_id']==r'\N' else int(r['proxy_id']) for r in rows}
last_event=max([int(r['id']) for r in table('proxy_failover_events')]+[0])
policy=table('proxy_auto_allocation')[0]
query="select json_build_object('accounts',(select json_agg(json_build_object('id',id,'proxy_id',coalesce(proxy_id,0))) from accounts where deleted_at is null),'events',(select coalesce(json_agg(e order by id),'[]') from (select id,account_id,from_proxy_id,to_proxy_id,created_at from proxy_failover_events where id>"+str(last_event)+") e),'policy',(select json_build_object('enabled',enabled,'revision',revision) from proxy_auto_allocation where id=1));"
live=json.loads(subprocess.check_output(['docker','exec','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atc',query],text=True))
current={r['id']:r['proxy_id'] for r in live['accounts']}
changed=[i for i in bindings if bindings[i]!=current.get(i)]
chains=[]
for account in changed:
    cursor=bindings[account];valid=True;events=[]
    for event in live['events']:
        if event['account_id']!=account:continue
        if event['from_proxy_id']!=cursor:valid=False
        cursor=event['to_proxy_id'];events.append(event)
    chains.append({'account_id':account,'before':bindings[account],'current':current.get(account),'events':events,'verified':valid and cursor==current.get(account) and bool(events)})
report={'backup':str(backup),'same_account_ids':set(bindings)==set(current),'account_count':len(current),'changed_proxy_accounts':len(changed),'proxy_policy_unchanged':live['policy']=={'enabled':policy['enabled']=='t','revision':int(policy['revision'])},'matches_actual_cutover':bindings==current,'explained_by_existing_failover':bool(changed) and all(c['verified'] for c in chains),'chains':chains}
(root/'proxy-audit.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
assert report['same_account_ids'] and report['proxy_policy_unchanged']
assert report['matches_actual_cutover'] or report['explained_by_existing_failover'],'Unexplained proxy binding changes'
