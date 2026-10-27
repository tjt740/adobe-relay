import pathlib,os,subprocess,json,datetime
os.umask(0o077)
src=pathlib.Path('/dev/shm/adobe-crawford-personal-20261027-1738.json')
dst=pathlib.Path('/dev/shm/adobe-crawford-followup-20261027-1738.json')
assert not dst.exists(),'do not overwrite an existing follow-up session'
if src.exists():
 d=json.loads(src.read_text());assert d.get('account_id')==1738 and d.get('ok') is True
 dst.write_bytes(src.read_bytes());dst.chmod(0o600)
 code="import time,pathlib;time.sleep(1800);pathlib.Path('/dev/shm/adobe-crawford-followup-20261027-1738.json').unlink(missing_ok=True)"
 subprocess.Popen(['python3','-c',code],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
 print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'temporary_followup_session_saved':True,'account_id':1738,'ttl_seconds':1800,'no_adobe_requests':True}))
else:print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'temporary_followup_session_saved':False,'reason':'source already cleaned; no login attempted'}))
