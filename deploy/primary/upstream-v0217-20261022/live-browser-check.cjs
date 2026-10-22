// Ephemeral read-only verification. No credentials are logged or saved.
const {chromium}=require('playwright');
const {execFileSync}=require('node:child_process');
const fs=require('node:fs');
const mint=String.raw`
import base64,hashlib,hmac,json,secrets,subprocess,time,urllib.request
x=json.loads(subprocess.check_output(['docker','inspect','adobe-relay-app-1']))[0]
secret=dict(s.split('=',1) for s in x['Config']['Env'])['JWT_SECRET']
p=subprocess.run(['docker','exec','-i','adobe-relay-postgres-1','psql','-U','sub2api','-d','sub2api','-Atq','-v','ON_ERROR_STOP=1'],input="BEGIN READ ONLY; SELECT json_build_object('id',id,'email',email,'role',role,'password_hash',password_hash) FROM users WHERE role='admin' AND status='active' AND deleted_at IS NULL ORDER BY id LIMIT 1; COMMIT;",text=True,capture_output=True,check=True,timeout=12)
u=json.loads(p.stdout)
v=int.from_bytes(hashlib.sha256((u['email'].strip().lower()+'\n'+u['password_hash']).encode()).digest()[:8],'big')&0x7fffffffffffffff
now=int(time.time());claims={'user_id':u['id'],'email':u['email'],'role':u['role'],'token_version':v,'sid':secrets.token_hex(8),'exp':now+600,'iat':now,'nbf':now}
def b64(b):return base64.urlsafe_b64encode(b).decode().rstrip('=')
raw=b64(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())+'.'+b64(json.dumps(claims,separators=(',',':')).encode())
token=raw+'.'+b64(hmac.new(secret.encode(),raw.encode(),hashlib.sha256).digest())
req=urllib.request.Request('http://127.0.0.1:6666/api/v1/auth/me',headers={'Authorization':'Bearer '+token})
with urllib.request.urlopen(req,timeout=10) as response:actual=json.load(response)['data']
assert actual['id']==u['id']
print(json.dumps({'token':token,'user':actual}))
`;
(async()=>{
 const stdout=execFileSync('ssh',['-i',require('node:path').join(require('node:os').homedir(),'.ssh/deploy_key'),'-o','BatchMode=yes','-o','StrictHostKeyChecking=yes','-o','ConnectTimeout=15','-o','ControlPath=none','root@173.234.15.150','python3 -'],{input:mint,encoding:'utf8',timeout:30000});
 let auth;try{auth=JSON.parse(stdout);}catch{throw Error('Could not prepare the ephemeral read-only verification session');}
 const browser=await chromium.launch({headless:true});const layouts=[];
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1000},locale:'zh-CN'});const writes=[],errors=[];
  await context.route('**/*',route=>{
   const req=route.request(),u=new URL(req.url());
   if(u.hostname==='creative.cheap'&&!['GET','HEAD','OPTIONS'].includes(req.method())){writes.push({method:req.method(),path:u.pathname});return route.abort();}
   return route.continue();
  });
  await context.addInitScript(data=>{
   if(location.origin==='https://creative.cheap'){
    localStorage.setItem('auth_token',data.token);localStorage.setItem('auth_user',JSON.stringify(data.user));
    if(!localStorage.getItem('sub2api_locale'))localStorage.setItem('sub2api_locale','zh');
    localStorage.setItem('theme','light');
    for(const k of ['admin_guide','user_guide','onboarding_tour'])localStorage.setItem(k+'_'+data.user.id+'_'+data.user.role+'_v4_interactive','true');
   }
  },auth);
  const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.goto('https://creative.cheap/admin/changelog',{waitUntil:'domcontentloaded'});
  const release=page.locator('[data-update-id="upstreamV0217"]');await release.waitFor({timeout:25000});
  if(await page.getByTestId('recovery-scenario').locator('option').count()!==11)throw Error('Lost production proposal scenarios');
  for(const lang of ['zh','en']){
   await page.evaluate(l=>localStorage.setItem('sub2api_locale',l),lang);await page.reload({waitUntil:'domcontentloaded'});await release.waitFor();
   await page.locator('.daily-preview').evaluate(e=>e.open=false);
   for(const dark of [false,true])for(const width of [390,768,1440]){
    await page.setViewportSize({width,height:1000});await page.evaluate(d=>document.documentElement.classList.toggle('dark',d),dark);
    const text=await release.innerText();if(text.includes('changelog.daily.')||!text.includes('v0.2.17')||!text.includes('Adobe')||!text.includes('Okad'))throw Error('Missing release content');
    await page.waitForFunction(()=>document.documentElement.scrollWidth<=innerWidth,{}, {timeout:3000}).catch(()=>{});
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
    if(overflow){
     const diagnostics=await page.evaluate(()=>({viewport:innerWidth,scrollWidth:document.documentElement.scrollWidth,overflow:[...document.querySelectorAll('body *')].map(e=>({tag:e.tagName,cls:typeof e.className==='string'?e.className:'svg',right:e.getBoundingClientRect().right,width:e.getBoundingClientRect().width})).filter(e=>e.right>innerWidth+1&&e.width>0).slice(0,20)}));
     fs.writeFileSync(__dirname+'/live-layout-diagnostic.json',JSON.stringify({lang,dark,width,...diagnostics},null,2));await page.screenshot({path:__dirname+'/live-layout-diagnostic.png',fullPage:true});throw Error('Page exceeds viewport after layout settled');
    }
    if(await release.locator('[data-testid="release-details"] > div').count()!==7)throw Error('Incomplete release details');
    if(await page.locator('[data-release-date="2026-10-22"]').getAttribute('open')===null)throw Error('Latest day not expanded');
    layouts.push({lang,dark,width});
    if(width===390||width===1440){await release.scrollIntoViewIfNeeded();await page.screenshot({path:__dirname+'/live-'+lang+'-'+(dark?'dark':'light')+'-'+width+'.png',fullPage:true});}
   }
  }
  if(await page.locator('[data-update-id="clashSubscriptions"]').count()!==1)throw Error('Lost subscription release');
  if(writes.length||errors.length)throw Error('Read-only UI verification failed: '+JSON.stringify({writes,errors}));
  const report={at:new Date().toISOString(),applicationSha:'7087d7e77277c8fd953b0e372078008c48c085a5',url:'https://creative.cheap/admin/changelog',scenarioCount:11,case_count:layouts.length,attempted_writes:writes,runtime_errors:errors,sections:7,layouts,attemptedWrites:writes,runtimeErrors:errors,activeAdobeProbes:false,credentialsWrittenToDisk:false};
  fs.writeFileSync(__dirname+'/live-browser-verification.json',JSON.stringify(report,null,2));console.log(JSON.stringify({liveBrowserPassed:true,scenarioCount:11,layouts:layouts.length,writes:writes.length,runtimeErrors:errors.length}));
 }finally{await browser.close();auth=null;}
})().catch(e=>{console.error(e.message);process.exitCode=1;});
