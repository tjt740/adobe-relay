"""Snapshot scheduling ownership without credentials; never change business data."""
import datetime,importlib.util,json,sys,urllib.request
from pathlib import Path
ROOT=Path('/opt/adobe-relay/releases/adobe-408-timeout-scheduling-20261024')
spec=importlib.util.spec_from_file_location('audit',ROOT/'audit.py');a=importlib.util.module_from_spec(spec);spec.loader.exec_module(a)

def snapshot():
 rows=json.loads(a.sql("""SELECT json_agg(json_build_object('id',a.id,'status',status,'schedulable',schedulable,'proxy_id',proxy_id,
 'credentials_hash',md5(credentials::text),'manual_pause',extra->'okad_manual_pause','manual_proxy',extra->'manual_proxy_enabled',
 'verification',extra->'okad'->>'verification','code',extra->'last_check_status_code','success',extra->'last_check_success',
 'gate',extra->'okad_connection_test'->'gated','test_state',extra->'okad_connection_test'->>'state',
 'error_kind',CASE WHEN COALESCE(error_message,'')='' THEN 'none' WHEN error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）' THEN 'waiting_proxy' WHEN error_message='Okad credentials pending connection test' THEN 'pending_check' ELSE 'independent_error' END,
 'groups',COALESCE((SELECT json_agg(group_id ORDER BY group_id) FROM account_groups ag WHERE ag.account_id=a.id),'[]'),
 'intent_hash',md5(jsonb_build_object('concurrency',concurrency,'priority',priority,'expires_at',expires_at,'auto_pause_on_expired',auto_pause_on_expired,
 'platform',platform,'type',type,'model_mapping',credentials->'model_mapping','parent_account_id',parent_account_id,
 'groups',(SELECT jsonb_agg(group_id ORDER BY group_id) FROM account_groups ag WHERE ag.account_id=a.id))::text)) ORDER BY a.id)
 FROM accounts a WHERE deleted_at IS NULL"""))
 key=a.sql("SELECT value FROM settings WHERE key='admin_api_key'")
 req=urllib.request.Request('http://127.0.0.1:6666/api/v1/admin/accounts/proxy-allocation',headers={'x-api-key':key})
 with urllib.request.urlopen(req,timeout=25) as response:allocation=json.load(response)['data']
 groups=json.loads(a.sql("""SELECT json_agg(x ORDER BY id) FROM (SELECT g.id,count(a.id) AS members,
 count(a.id) FILTER(WHERE a.status='active' AND a.schedulable AND (a.auto_pause_on_expired IS NOT TRUE OR a.expires_at IS NULL OR a.expires_at>NOW())
 AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at<=NOW()) AND (a.overload_until IS NULL OR a.overload_until<=NOW())
 AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until<=NOW())) AS available
 FROM groups g LEFT JOIN account_groups ag ON ag.group_id=g.id LEFT JOIN accounts a ON a.id=ag.account_id AND a.deleted_at IS NULL WHERE g.deleted_at IS NULL GROUP BY g.id) x"""))
 return {'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'accounts':rows,'allocation':allocation,'groups':groups}

def compare(before,after,targets):
 old={r['id']:r for r in before['accounts']};new={r['id']:r for r in after['accounts']}
 assert set(old)<=set(new),'Accounts disappeared; review live operations, never restore old data'
 for i,b in old.items():
  n=new[i]
  assert b['manual_pause']==n['manual_pause'] and b['manual_proxy']==n['manual_proxy'],f'Operator state changed: {i}'
  assert b['intent_hash']==n['intent_hash'] and b['groups']==n['groups'],f'Account configuration/group changed: {i}'
  if b['manual_pause'] is True:assert not n['schedulable'],f'Manually paused account resumed: {i}'
  if b['verification']=='quota_blocked':assert n['verification']=='quota_blocked' and not n['schedulable'],f'Quota restriction changed: {i}'
  if b['manual_proxy'] is True:assert b['proxy_id']==n['proxy_id'],f'Fixed operator route changed: {i}'
 checked=[]
 for i in targets:
  assert i in old and i in new,'Target no longer exists; review current state'
  n=new[i]
  assert n['gate'] is False and n['test_state']=='failed' and str(n['code'])=='408' and n['success'] is False,f'Target gate/evidence changed: {i}'
  assert n['status']=='active' and n['schedulable'] and n['proxy_id'] is not None,f'Target has independent/new restrictions; inspect without forcing resume: {i}'
  checked.append({k:n[k] for k in ['id','status','schedulable','proxy_id','groups','gate','code','test_state']})
 return {'original_accounts':len(old),'current_accounts':len(new),'active_schedulable':sum(r['status']=='active' and r['schedulable'] for r in new.values()),
 'manual_pauses_preserved':True,'quota_restrictions_preserved':True,'manual_routes_preserved':True,'groups_preserved':True,'target_accounts':checked,
 'credential_hash_changes':[i for i in old if old[i]['credentials_hash']!=new[i]['credentials_hash']],
 'allocation':{k:after['allocation'].get(k) for k in ['enabled','healthy_nodes','available_slots','business_waiting','temporary_test_bindings','quarantined_408_accounts','pending_408_release']},'groups':after['groups']}

if __name__=='__main__':
 action=sys.argv[1];assert action in ('before','after')
 state=snapshot()
 if action=='before':a.save('account-state-before.json',state);print(json.dumps({'accounts':len(state['accounts']),'at':state['at']}))
 else:
  a.save('account-state-after.json',state)
  result=compare(json.loads((ROOT/'account-state-before.json').read_text()),state,json.loads((ROOT/'artifact/target-accounts.json').read_text()))
  assert state['allocation']['quarantined_408_accounts']==0 and state['allocation']['pending_408_release']==0
  assert a.sql('SELECT count(*) FROM adobe_proxy_408_blocks')=='0' and a.sql('SELECT count(*) FROM adobe_proxy_408_holds')=='0'
  assert a.sql('SELECT count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id=1290')=='0'
  assert a.sql('SELECT count(*) FROM adobe_proxy_test_leases WHERE proxy_id=1290')=='0'
  assert a.sql('SELECT EXISTS(SELECT 1 FROM adobe_route_exclusions WHERE proxy_id=1290)')=='t'
  assert a.sql('SELECT COALESCE(max(n),0) FROM (SELECT count(*) n FROM accounts WHERE deleted_at IS NULL AND proxy_id IS NOT NULL GROUP BY proxy_id)x') in ('0','1','2','3')
  a.save('account-state-verification.json',result);print(json.dumps(result,ensure_ascii=False))
