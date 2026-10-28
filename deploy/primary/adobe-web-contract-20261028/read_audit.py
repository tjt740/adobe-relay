import datetime, hashlib, json, os, pathlib, subprocess, sys, shutil

ROOT = pathlib.Path('/opt/adobe-relay/releases/adobe-web-contract-20261028')
EXPECTED = 'adobe-relay:df07b90d7e2ce8e27fd7a9deb11336eaf5077e3c'
CHROME = pathlib.Path('/opt/adobe-relay/releases/three-arm-20261027/chrome/chrome-linux64/chrome')
PRIVATE = pathlib.Path('/dev/shm/adobe-web-contract-20261028')
WEB_IMAGE = 'adobe-team-cookie:79382b0f79d559575956e023dc9f52fbf07dbfde'
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

def capture():
    assert not (ROOT/'capture-done.json').exists(), 'do not replay capture'
    before=preflight()
    assert before['chrome_exists'] and before['mem_available_kb']>10*1024**2
    assert not before['temporary_containers'] and not PRIVATE.exists()
    PRIVATE.mkdir(mode=0o700)
    save('capture-before.json',before)
    # A bounded cleanup watchdog contains no login, generation or production write.
    watch="import time,pathlib,subprocess,shutil;time.sleep(1200);p=pathlib.Path('/dev/shm/adobe-web-contract-20261028');subprocess.run(['docker','rm','-f','adobe-web-contract-browser-1892','adobe-web-contract-browser-1738'],capture_output=True);shutil.rmtree(p,ignore_errors=True)"
    subprocess.Popen(['python3','-c',watch],stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,start_new_session=True)
    accounts=sql("""SELECT json_build_object('id',a.id,'email',a.name,'credentials',a.credentials,
      'proxy_id',a.proxy_id,'proxy',CASE WHEN p.id IS NULL THEN NULL ELSE json_build_object('host',p.host,
      'port',p.port,'protocol',p.protocol,'username',p.username,'password',p.password) END)
      FROM accounts a LEFT JOIN proxies p ON p.id=a.proxy_id
      WHERE a.deleted_at IS NULL AND a.id IN (1892,1738) ORDER BY a.id DESC""")
    results=[]
    for a in accounts:
        c=a['credentials'];source=c.get('cookie') or ''
        session=c.get('adobe_session_cookie')
        use_runtime=isinstance(source,str) and session and c.get('adobe_session_cookie_source_hash')==hashlib.sha256(source.strip().encode()).hexdigest()
        cfg={'id':a['id'],'email':a['email'],'proxy_id':a['proxy_id'],'proxy':a['proxy'],
            'cookie':session if use_runtime else source,'expected_token':c.get('access_token') or ''}
        name='adobe-web-contract-browser-'+str(a['id'])
        args=['docker','run','--rm','-i','--name',name,'--network','adobe-relay_default','--memory','3g','--cpus','2',
            '--read-only','--tmpfs','/root/.config:rw,size=32m','--tmpfs','/root/.cache:rw,size=32m','--tmpfs','/tmp:rw,size=768m','--shm-size','256m',
            '--mount','type=bind,src='+str(CHROME.parent.parent)+',dst=/chrome,readonly',
            '--mount','type=bind,src='+str(PRIVATE)+',dst=/private',
            '--entrypoint','python',WEB_IMAGE,'-c',BROWSER_SOURCE]
        try:
            raw=command(args,json.dumps(cfg),timeout=240)
            result=json.loads(raw)
        except Exception as e:
            result={'account_id':a['id'],'error_class':type(e).__name__}
        finally:
            subprocess.run(['docker','rm','-f',name],capture_output=True,timeout=20)
        result['credential_selection']='runtime_cookie_matching_source' if use_runtime else 'source_cookie'
        save('browser-'+str(a['id'])+'.json',result)
        results.append(result)
    after=preflight()
    save('capture-after.json',after)
    save('capture-done.json',{'at':now(),'account_ids':[r['account_id'] for r in results],'production_writes':0,'generation_requests':0})
    return {'at':now(),'phase':'capture','results':results,'containers_unchanged':before['containers']==after['containers']}

def main():
    ROOT.mkdir(mode=0o700, exist_ok=True)
    phase = sys.argv[1] if len(sys.argv)>1 else 'preflight'
    assert phase in ['preflight','capture']
    result = preflight() if phase=='preflight' else capture()
    save(phase + '.json', result)
    print(json.dumps(result, ensure_ascii=False))



BROWSER_SOURCE = 'import base64, datetime, hashlib, json, pathlib, re, sys, time, urllib.parse\nfrom playwright.sync_api import sync_playwright\n\ncfg = json.load(sys.stdin)\nOUT = pathlib.Path(\'/evidence\')\nPRIVATE = pathlib.Path(\'/private\')\nevents = []\nprivate_requests = []\nprivate_tokens = {}\n\ndef digest(v):\n    return hashlib.sha256((v if isinstance(v, bytes) else str(v).encode())).hexdigest()\n\ndef token_info(token):\n    result = {\'sha256\':digest(token),\'bytes\':len(token)}\n    try:\n        p = token.split(\'.\')[1]\n        d = json.loads(base64.urlsafe_b64decode(p+\'=\'*(-len(p)%4)))\n        result.update(client_id=d.get(\'client_id\'),user_sha256=digest(d.get(\'user_id\',\'\')),\n            sid_sha256=digest(d.get(\'sid\',\'\')),guest=d.get(\'isGuest\') or d.get(\'guest\'),account_type=d.get(\'account_type\'),scope=d.get(\'scope\'))\n    except Exception:\n        pass\n    return result\n\ndef clean_url(url):\n    p = urllib.parse.urlsplit(url)\n    return p.scheme+\'://\'+p.netloc+p.path\n\ndef safe_body(data):\n    if not isinstance(data,dict):\n        return {\'type\':type(data).__name__}\n    out = {\'keys\':sorted(data)}\n    for k,v in data.items():\n        if k in [\'client_id\',\'scope\',\'grant_type\',\'guest_allowed\',\'user_type\',\'modelId\',\'modelVersion\',\'n\',\'size\',\'caiClaimVersion\',\'generationSettings\',\'output\',\'modelSpecificPayload\']:\n            out[k]=v\n        elif k == \'generationMetadata\' and isinstance(v,dict):\n            out[k]={x:y for x,y in v.items() if x in [\'module\',\'submodule\']}\n            out[k][\'keys\']=sorted(v)\n        elif k == \'referenceBlobs\' and isinstance(v,list):\n            out[k]=[{\'keys\':sorted(x),\'usage\':x.get(\'usage\')} for x in v if isinstance(x,dict)]\n        else:\n            out[k]={\'present\':bool(v),\'bytes\':len(str(v)),\'sha256\':digest(v)}\n    return out\n\ndef on_request(req):\n    p = urllib.parse.urlsplit(req.url)\n    relevant = any(s in p.path for s in [\'/ims/check/\',\'generate-async\',\'/v2/storage/image\',\'/sherlock\',\'/amsterdam/\'])\n    if not relevant:\n        return\n    h=req.all_headers()\n    raw=req.post_data or \'\'\n    try:\n        body=json.loads(raw) if \'json\' in h.get(\'content-type\',\'\') else dict(urllib.parse.parse_qsl(raw))\n    except Exception:\n        body={}\n    safe={\'at\':datetime.datetime.now(datetime.timezone.utc).isoformat(),\'event\':\'request\',\'url\':clean_url(req.url),\n        \'method\':req.method,\'query\':{k:v if k in [\'jslVersion\'] else \'[present]\' for k,v in urllib.parse.parse_qsl(p.query)},\n        \'header_names\':sorted(h),\'headers\':{k:v for k,v in h.items() if k in [\'user-agent\',\'x-api-key\',\'origin\',\'referer\',\'sec-ch-ua\',\'sec-ch-ua-platform\',\'sec-fetch-site\',\'content-type\']},\n        \'secret_headers\':{k:{\'bytes\':len(h[k]),\'sha256\':digest(h[k])} for k in [\'authorization\',\'cookie\',\'x-arp-session-id\',\'x-nonce\'] if k in h},\n        \'body\':safe_body(body)}\n    events.append(safe)\n    if \'/ims/check/\' in p.path or \'generate-async\' in p.path:\n        private_requests.append({\'url\':req.url,\'headers\':h,\'body\':body})\n\ndef on_response(resp):\n    p=urllib.parse.urlsplit(resp.url)\n    if not any(s in p.path for s in [\'/ims/check/\',\'generate-async\',\'/sherlock\',\'/amsterdam/\']):\n        return\n    safe={\'at\':datetime.datetime.now(datetime.timezone.utc).isoformat(),\'event\':\'response\',\'url\':clean_url(resp.url),\'status\':resp.status}\n    if \'/ims/check/\' in p.path:\n        try:\n            d=resp.json()\n            tok=d.get(\'access_token\',\'\')\n            safe.update(keys=sorted(d),error=d.get(\'error\'),issued_token=bool(tok))\n            if tok:\n                safe[\'token\']=token_info(tok)\n                private_tokens[digest(tok)]=tok\n        except Exception:\n            safe[\'json\']=False\n    events.append(safe)\n\ndef import_cookies(raw):\n    if isinstance(raw,str):\n        try:\n            d=json.loads(raw)\n        except Exception:\n            d=None\n        if d is not None:\n            return import_cookies(d)\n        pairs=[]\n        for part in raw.removeprefix(\'Cookie:\').split(\';\'):\n            if \'=\' not in part:\n                continue\n            k,v=part.strip().split(\'=\',1)\n            if k and v:\n                pairs.append((k,v))\n        # The flattened IMS request Cookie no longer has browser-domain metadata.\n        # Restore it only on Adobe-owned authentication/site domains and record\n        # this limitation. Do not grant cookies to any other domain.\n        return [{\'name\':k,\'value\':v,\'domain\':domain,\'path\':\'/\',\'secure\':True,\'sameSite\':\'None\'}\n            for domain in [\'.adobe.com\',\'.adobelogin.com\'] for k,v in pairs]\n    if isinstance(raw,dict):\n        return import_cookies(raw.get(\'cookies\') or raw.get(\'cookie\') or \'\')\n    if isinstance(raw,list):\n        return [{k:v for k,v in c.items() if k in [\'name\',\'value\',\'domain\',\'path\',\'expires\',\'httpOnly\',\'secure\',\'sameSite\']}\n            for c in raw if isinstance(c,dict) and c.get(\'name\') and c.get(\'domain\')]\n    return []\n\nproxy=cfg.get(\'proxy\')\nlaunch={\'headless\':True,\'executable_path\':\'/chrome/chrome-linux64/chrome\'}\nif proxy:\n    launch[\'proxy\']={\'server\':proxy[\'protocol\']+\'://\'+proxy[\'host\']+\':\'+str(proxy[\'port\'])}\n    if proxy.get(\'username\'):\n        launch[\'proxy\'].update(username=proxy[\'username\'],password=proxy.get(\'password\') or \'\')\n\nresult={\'at\':datetime.datetime.now(datetime.timezone.utc).isoformat(),\'account_id\':cfg[\'id\'],\n    \'proxy_id\':cfg.get(\'proxy_id\'),\'password_login\':False,\'production_writes\':0,\n    \'cookie_import_note\':\'flattened IMS Cookie restored to Adobe-owned domains; browser domain/path fidelity is not guaranteed\'}\nwith sync_playwright() as pw:\n    browser=pw.chromium.launch(**launch)\n    context=browser.new_context(locale=\'en-US\',viewport={\'width\':1365,\'height\':950})\n    cookies=import_cookies(cfg[\'cookie\'])\n    context.add_cookies(cookies)\n    result[\'imported_cookie_names\']=sorted({c[\'name\'] for c in cookies})\n    result[\'browser_version\']=browser.version\n    context.on(\'request\',on_request)\n    context.on(\'response\',on_response)\n    page=context.new_page()\n    page_errors=[]\n    page.on(\'pageerror\',lambda e:page_errors.append(type(e).__name__))\n    try:\n        page.goto(\'https://firefly.adobe.com/generate/image\',timeout=60000,wait_until=\'domcontentloaded\')\n        page.wait_for_timeout(30000)\n        result.update(url=clean_url(page.url),title=page.title())\n        text=page.locator(\'body\').inner_text(timeout=8000)\n        result[\'body_text\']=re.sub(r\'[\\w.+-]+@[\\w.-]+\\.[A-Za-z]{2,}\', \'[email]\',text[:1600])\n        result[\'controls\']=page.locator(\'button,input,textarea,select,[role="combobox"],[role="textbox"]\').evaluate_all(\n            \'(els)=>els.map(e=>({tag:e.tagName,role:e.getAttribute("role"),type:e.type,text:e.innerText?.slice(0,100),placeholder:e.getAttribute("placeholder"),aria:e.getAttribute("aria-label")})).slice(0,90)\')\n        result[\'ims_requests\']=sum(\'/ims/check/\' in e.get(\'url\',\'\') and e[\'event\']==\'request\' for e in events)\n        result[\'native_tokens\']=[token_info(t) for t in private_tokens.values()]\n        expected=token_info(cfg.get(\'expected_token\') or \'\')\n        result[\'expected_user_sha256\']=expected.get(\'user_sha256\')\n        for tok in result[\'native_tokens\']:\n            tok[\'same_user_as_production\']=bool(expected.get(\'user_sha256\')) and tok.get(\'user_sha256\')==expected[\'user_sha256\']\n        if private_requests:\n            observed=next((r for r in reversed(private_requests) if \'/ims/check/\' in r[\'url\']),None)\n            if observed:\n                old_scope=\'AdobeID,firefly_api,openid,pps.read,pps.write,additional_info.projectedProductContext,additional_info.ownerOrg,uds_read,uds_write,ab.manage,read_organizations,additional_info.roles,account_cluster.read,creative_production,tk_platform,tk_platform_sync,profile\'\n                new_scope=observed[\'body\'].get(\'scope\',\'\')\n                variants=[{\'name\':\'current_sdk_auth_contract\',\'url\':\'https://adobeid-na1.services.adobe.com/ims/check/v6/token?jslVersion=v2-v0.54.0-3-g58cfcb7\',\'body\':{\'client_id\':\'clio-playground-web\',\'scope\':old_scope}},\n                    {\'name\':\'current_web_scope_version_only\',\'url\':observed[\'url\'],\'body\':{\'client_id\':\'clio-playground-web\',\'scope\':new_scope}},\n                    {\'name\':\'observed_web_auth_contract\',\'url\':observed[\'url\'],\'body\':observed[\'body\']}]\n                raw=page.evaluate(\'\'\'async variants => Promise.all(variants.map(async v=>{try{const r=await fetch(v.url,{method:\'POST\',credentials:\'include\',headers:{\'Content-Type\':\'application/x-www-form-urlencoded;charset=UTF-8\'},body:new URLSearchParams(v.body)});return {name:v.name,status:r.status,data:await r.json()}}catch(e){return {name:v.name,error:e.name}}}))\'\'\',variants)\n                safe=[]\n                for row in raw:\n                    d=row.pop(\'data\',{})\n                    tok=d.get(\'access_token\',\'\')\n                    row.update(issued_token=bool(tok),error_code=d.get(\'error\'),response_keys=sorted(d))\n                    if tok:\n                        row[\'token\']=token_info(tok)\n                        private_tokens[digest(tok)]=tok\n                    safe.append(row)\n                result[\'same_browser_auth_contract_comparison\']=safe\n        matched=[]\n        for tok in list(private_tokens.values())[-3:]:\n            try:\n                profile=page.evaluate(\'\'\'async token=>{const r=await fetch(\'https://ims-na1.adobelogin.com/ims/profile/v1\',{headers:{Authorization:\'Bearer \'+token,\'X-Client-Id\':\'clio-playground-web\'}});const d=await r.json();return {status:r.status,email:d.email||\'\'}}\'\'\',tok)\n                matched.append({\'token_sha256\':digest(tok),\'profile_status\':profile[\'status\'],\n                    \'email_matches_account\':profile.get(\'email\',\'\').lower()==cfg[\'email\'].lower()})\n            except Exception as e:\n                matched.append({\'token_sha256\':digest(tok),\'profile_error\':type(e).__name__})\n        result[\'identity_verification\']=matched\n        context.storage_state(path=str(PRIVATE/(\'state-\'+str(cfg[\'id\'])+\'.json\')))\n        (PRIVATE/(\'requests-\'+str(cfg[\'id\'])+\'.json\')).write_text(json.dumps({\'requests\':private_requests,\'tokens\':private_tokens}))\n    except Exception as e:\n        result[\'error_class\']=type(e).__name__\n    result[\'events\']=events\n    result[\'page_errors\']=page_errors\n    browser.close()\nprint(json.dumps(result,ensure_ascii=False))\n'

if __name__ == '__main__':
    main()
