"""Add only the four authorized site-only monitoring rules; preserve every existing rule."""
import datetime,fcntl,json,pathlib,subprocess
ROOT=pathlib.Path('/opt/adobe-relay/releases/proxy-reliability-20261112')
def sql(q):
 return subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input=q,text=True,capture_output=True,check=True,timeout=20).stdout.strip()
def rules():return json.loads(sql("SELECT coalesce(json_agg(to_jsonb(r)-'updated_at'-'last_triggered_at' ORDER BY id),'[]') FROM ops_alert_rules r"))
names=['Mihomo 内存预警 70%','Mihomo 内存紧急 85%','Mihomo 新增 OOM 或重启','Mihomo 监控中断']
locks=[]
for p in ['/opt/adobe-relay/shared/.adobe-deploy.lock','/opt/adobe-relay/.release.lock']:
 f=open(p,'a');fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB);locks.append(f)
assert not (ROOT/'alerts-installed.json').exists(),'Already applied; inspect receipt'
before=rules();assert not set(names).intersection(r['name']for r in before),'Inspect existing rules before retry'
p=ROOT/'alerts-before.json';assert not p.exists();p.write_text(json.dumps(before,ensure_ascii=False));p.chmod(0o600)
q="""BEGIN; SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='5s';
INSERT INTO ops_alert_rules(name,description,enabled,severity,metric_type,operator,threshold,window_minutes,sustained_minutes,cooldown_minutes,filters,notify_email) VALUES
('Mihomo 内存预警 70%','代理内存达到上限 70%，持续两分钟；仅站内告警。',true,'P2','mihomo_memory_percent','>=',70,1,2,15,'{}',false),
('Mihomo 内存紧急 85%','代理内存达到上限 85%，持续一分钟；仅站内告警。',true,'P1','mihomo_memory_percent','>=',85,1,1,15,'{}',false),
('Mihomo 新增 OOM 或重启','最近五分钟观察到新增内存耗尽或进程重启；历史计数不触发，仅站内告警。',true,'P1','mihomo_abnormal_events','>=',1,1,0,15,'{}',false),
('Mihomo 监控中断','连续一分钟没有有效且新鲜的代理运行采样；采样超过 90 秒视为失联，仅站内告警。',true,'P1','mihomo_monitor_unavailable','>=',1,1,1,15,'{}',false);
COMMIT;"""
sql(q);after=rules();old={r['id']:r for r in before};assert {r['id']:r for r in after if r['id']in old}==old
added=[r for r in after if r['id']not in old];assert len(added)==4 and all(not r['notify_email']and r['enabled']for r in added)
out={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'added':added,'existing_rules_unchanged':True,'existing_count':len(before),'mail_sent':False}
p=ROOT/'alerts-installed.json';p.write_text(json.dumps(out,ensure_ascii=False));p.chmod(0o600);print(json.dumps(out,ensure_ascii=False))
