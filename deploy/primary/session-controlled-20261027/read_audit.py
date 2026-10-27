import pathlib,json,datetime,subprocess
p=pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027/control')
print(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'browser_running':'adobe-three-arm-web-20261027' in subprocess.check_output(['docker','ps','--format','{{.Names}}'],text=True).splitlines(),'process_log':(p/'web-process.log').read_text()[-4500:],'events':(p/'web-events.jsonl').read_text().splitlines()[-8:],'stop_marker':(p/'stop-web').exists()},ensure_ascii=False))
