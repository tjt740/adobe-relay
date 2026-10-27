import pathlib,subprocess,json,datetime,sys,hashlib
ROOT=pathlib.Path('/opt/adobe-relay/releases/408-causal-20261028/donald-pa')
assert ROOT.exists() and (ROOT/'ready.json').exists()
assert not any((ROOT/x).exists() for x in ['login-started.json','trial-started.json','cleaned.json']), 'never replay authentication or trial'
assert hashlib.sha256((ROOT/'sub2api').read_bytes()).hexdigest()=='e97b978b84ab96393338ac5d9906e5127b88de9099545e27245f60d9eddb6a29'
sys.path.insert(0,str(ROOT))
from replica import save,selected
assert selected()['proxy_id']==1335
raw_env=json.loads(subprocess.check_output(['docker','inspect','adobe408-diag-app'],text=True))[0]['Config']['Env']
diag_env={k:v for k,v in (x.split('=',1) for x in raw_env) if k in ('ADOBE_DIAG_FILE','ADOBE_DIAG_ACCOUNT_IDS','ADOBE_DIAG_SUBMIT_ONCE')}
assert diag_env.get('ADOBE_DIAG_ACCOUNT_IDS')==str(1818),'diagnostic account filter missing'
assert diag_env.get('ADOBE_DIAG_SUBMIT_ONCE')==('1' if {'tag': 'donald-pa', 'account_id': 1818, 'member_id': 69, 'email': 'DonaldSimril48@outlook.com', 'arm': 'production', 'root': '/opt/adobe-relay/releases/408-causal-20261028/donald-pa', 'comparison_jobs': 9, 'post_comparison_route_jobs_max': 2, 'source_proxy': 1335, 'alternative_proxy': 1438, 'concurrency_preserved': 3, 'models': ['gpt-image-2.5-sunburst', 'gpt-image-2.5-flare'], 'duration_after_ready_sec': 180, 'diagnostic_profile': 'personal', 'production_profile_preference': 'preserved'}['arm']=='single' else '0'),'submit control mismatch'
save('experiment-runtime.json',{'diagnostic_env':diag_env,'binary_sha256':'e97b978b84ab96393338ac5d9906e5127b88de9099545e27245f60d9eddb6a29'})
observer_log=open(ROOT/'egress-observer-process.log','a');subprocess.Popen(['python3',str(ROOT/'egress-observer.py')],stdout=observer_log,stderr=observer_log,start_new_session=True)
f=open(ROOT/'trial-process.log','a');proc=subprocess.Popen(['python3',str(ROOT/'run_trial.py')],stdout=f,stderr=f,start_new_session=True)
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'kind':'bounded_trial_launched','pid':proc.pid,'manifest':{'tag': 'donald-pa', 'account_id': 1818, 'member_id': 69, 'email': 'DonaldSimril48@outlook.com', 'arm': 'production', 'root': '/opt/adobe-relay/releases/408-causal-20261028/donald-pa', 'comparison_jobs': 9, 'post_comparison_route_jobs_max': 2, 'source_proxy': 1335, 'alternative_proxy': 1438, 'concurrency_preserved': 3, 'models': ['gpt-image-2.5-sunburst', 'gpt-image-2.5-flare'], 'duration_after_ready_sec': 180, 'diagnostic_profile': 'personal', 'production_profile_preference': 'preserved'},'production_unchanged':True}))
