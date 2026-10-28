import datetime, hashlib, json, os, pathlib, subprocess, sys

ROOT = pathlib.Path('/opt/adobe-relay/releases/adobe-web-contract-20261028')
EXPECTED = 'adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c'
CHROME = pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027/chrome/chrome-linux64/chrome')
os.umask(0o077)

def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def command(args, data=None, timeout=30):
    r = subprocess.run(args, input=data, capture_output=True, text=True, timeout=timeout)
    if r.returncode:
        raise RuntimeError('command_failed:' + args[0] + ':' + str(r.returncode))
    return r.stdout

def sql(q):
    raw = command(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],
        "BEGIN READ ONLY;SET LOCAL statement_timeout='12s';" + q + ';COMMIT;')
    return [json.loads(line) for line in raw.splitlines() if line.startswith('{')]

def save(name, value):
    p = ROOT / name
    tmp = p.with_suffix(p.suffix + '.tmp')
    tmp.write_text(json.dumps(value, ensure_ascii=False, indent=2))
    tmp.replace(p)

def preflight():
    accounts = sql("""SELECT json_build_object('id',a.id,'name',a.name,'status',a.status,
      'schedulable',a.schedulable,'proxy_id',a.proxy_id,'manual_proxy',a.extra->'manual_proxy_enabled',
      'manual_pause',a.extra->'okad_manual_pause','verification',a.extra->'okad'->>'verification',
      'cookie_hash',md5(a.credentials->>'cookie'),'runtime_cookie_hash',md5(a.credentials->>'adobe_session_cookie'),
      'token_present',coalesce(a.credentials->>'access_token','')<>'','expires_at',a.credentials->>'expires_at',
      'arp_present',coalesce(a.credentials->>'arp_session_id','')<>'','proxy_active',p.status='active')
      FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id
      WHERE a.deleted_at IS NULL AND a.id IN (1738,1849,1892,1898,1910) ORDER BY a.id""")
    containers = []
    for name in ['adobe-relay-app-1','adobe-team-cookie-app-1']:
        x = json.loads(command(['docker','inspect',name]))[0]
        containers.append({'name':name,'image':x['Config']['Image'],'started':x['State']['StartedAt'],
            'restarts':x['RestartCount'],'memory':x['HostConfig']['Memory']})
    assert containers[0]['image'] == EXPECTED
    probe = "import pathlib,json,importlib.metadata;from playwright.sync_api import sync_playwright;print(json.dumps({'playwright':importlib.metadata.version('playwright')}))"
    runtime = json.loads(command(['docker','exec','adobe-team-cookie-app-1','python','-c',probe]))
    return {'at':now(),'accounts':accounts,'containers':containers,'chrome_exists':CHROME.exists(),
        'runtime':runtime,'temporary_containers':[n for n in command(['docker','ps','-a','--format','{{.Names}}']).splitlines() if n.startswith('adobe-web-contract-')],
        'mem_available_kb':next(int(x.split()[1]) for x in pathlib.Path('/proc/meminfo').read_text().splitlines() if x.startswith('MemAvailable:'))}

def main():
    ROOT.mkdir(mode=0o700, exist_ok=True)
    phase = sys.argv[1] if len(sys.argv)>1 else 'preflight'
    assert phase in ['preflight']
    result = preflight()
    save(phase + '.json', result)
    print(json.dumps(result, ensure_ascii=False))

if __name__ == '__main__':
    main()
