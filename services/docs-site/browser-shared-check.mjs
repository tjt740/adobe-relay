import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {pathToFileURL,fileURLToPath} from 'node:url';
const pw=await import(process.env.PLAYWRIGHT_MODULE_PATH?pathToFileURL(process.env.PLAYWRIGHT_MODULE_PATH).href:'playwright');
const cwd=path.dirname(fileURLToPath(import.meta.url));
const tmp=await fs.mkdtemp(path.join(os.tmpdir(),'docs-shared-browser-'));
const ready=path.join(tmp,'ready');
const out=process.env.DOCS_BROWSER_OUTPUT||path.join(cwd,'.browser-output');await fs.mkdir(out,{recursive:true});
const fixture=spawn('go',['test','-run','^TestBrowserFixture$','-count=1','-timeout','6m'],{cwd,detached:true,env:{...process.env,DOCS_BROWSER_READY_FILE:ready},stdio:['ignore','pipe','pipe']});
let fixtureLog='';fixture.stdout.on('data',b=>fixtureLog+=b);fixture.stderr.on('data',b=>fixtureLog+=b);
const results=[];
let browser;
try {
 let url;
 for(let i=0;i<300;i++){try{url=await fs.readFile(ready,'utf8');break}catch{};if(fixture.exitCode!==null)throw Error(fixtureLog);await new Promise(r=>setTimeout(r,100))}
 assert(url,'fixture failed to start');
 browser=await pw.chromium.launch({headless:true,...(process.env.DOCS_CHROME_CHANNEL?{channel:process.env.DOCS_CHROME_CHANNEL}:{})});
 const expected=await fs.readFile(path.join(cwd,'guide.md'));
 async function setup(width=1440){
  const ctx=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width,height:950}});const page=await ctx.newPage();
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(url+'/docs/login?manual=1');
  return {ctx,page,errors};
 }
 async function store(page,access,refresh='browser-refresh',id=8){
  await page.evaluate(({access,refresh,id})=>{
   localStorage.setItem('auth_token',access);localStorage.setItem('auth_user',JSON.stringify({id,role:'admin',status:'active'}));
   localStorage.setItem('token_expires_at',String(Date.now()-1000));localStorage.setItem('refresh_token',refresh);
  },{access,refresh,id});
 }
 async function visible(page){await page.waitForURL(url+'/docs/');await page.locator('article h1').waitFor();assert.equal(await page.locator('body').getAttribute('data-shared'),'true')}
 for(const width of [1440,390]){
  for(const role of ['admin','user','2fa']){
   const {ctx,page,errors}=await setup(width);
   let logins=0,refreshes=0;const requestURLs=[];
   ctx.on('request',r=>{requestURLs.push(r.url());if(r.url().endsWith('/api/v1/auth/login'))logins++;if(r.url().endsWith('/api/v1/auth/refresh'))refreshes++});
   // A completed upstream browser login, using its actual response/storage contract.
   await page.evaluate(async role=>{
    const r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:role+'@example.com',password:'fixture-password'})});
    let {data}=await r.json();
    if(data.requires_2fa){const verified=await fetch('/api/v1/auth/login/2fa',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({temp_token:data.temp_token,totp_code:'123456'})});data=(await verified.json()).data}
    localStorage.setItem('auth_token',data.access_token);localStorage.setItem('auth_user',JSON.stringify(data.user));
    localStorage.setItem('refresh_token',data.refresh_token);localStorage.setItem('token_expires_at',String(Date.now()+data.expires_in*1000));
   },role);
   const state=await page.evaluate(()=>({...localStorage}));
   const raw=await ctx.request.get(url+'/docs/');assert(!(await raw.text()).includes('frame_duration_ms'),'anonymous shell leaked guide');
   await page.goto(url+'/docs/');await visible(page);
   assert.equal(logins,1);assert.equal(refreshes,0);assert.deepEqual(await page.evaluate(()=>({...localStorage})),state);
   assert(!(await ctx.cookies()).some(c=>c.name==='__Host-docs_session'),'shared access must not persist docs cookie');
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
   assert.equal(await page.locator('.copy').count(),18);
   if(role==='admin')await page.screenshot({path:path.join(out,`shared-${width}.png`)});
   await page.setViewportSize({width:1440,height:950});
   const downloading=page.waitForEvent('download');await page.getByRole('link',{name:'下载 Markdown'}).click();
   assert.deepEqual(await fs.readFile(await (await downloading).path()),expected);
   const section=await page.locator('article h2').nth(1).getAttribute('id');
   await page.getByRole('button',{name:'退出登录'}).click();await page.waitForURL(url+'/docs/login?manual=1');
   assert(await page.locator('[name=email]').isVisible());assert.deepEqual(await page.evaluate(()=>({...localStorage})),state);
   await page.goto(url+'/docs/#'+encodeURIComponent(section));
   await page.locator('article h1').waitFor();
   await page.waitForFunction(id=>document.getElementById(id)?.getBoundingClientRect().top<=128,section);
   // Main-site logout in another tab removes automatic access and cannot be restored by a docs cookie.
   const peer=await ctx.newPage();await peer.goto(url+'/docs/login?manual=1');await peer.evaluate(()=>localStorage.clear());
   await page.waitForURL(url+'/docs/login?manual=1');await page.goto(url+'/docs/');await page.waitForURL(url+'/docs/login');
   assert.equal(await page.locator('article').count(),0);assert(await page.locator('[name=email]').isVisible());
   assert(!requestURLs.some(u=>u.includes('access_token=')||u.includes('refresh_token=')||u.includes('Bearer')));
   assert.deepEqual(errors,[]);await ctx.close();results.push({case:'existing-'+role,width,no_second_login:true,no_shared_cookie:true,download:true,docs_logout_keeps_sub2:true,sub2_logout_closes_docs:true});
  }
 }
 for(const access of ['forged-token','fixture-challenge','inactive-token','outage-token']){
  const {ctx,page,errors}=await setup();await store(page,access,'invalid-refresh');
  await page.goto(url+'/docs/');await page.locator('.login-main:not([hidden]) [name=email]').waitFor();
  assert.equal(await page.locator('article').count(),0);assert.deepEqual(errors,[]);await ctx.close();results.push({case:access,denied:true});
 }
 {
  const {ctx,page}=await setup();await store(page,'expired','valid-refresh-concurrent');
  const other=await ctx.newPage();let refreshes=0;ctx.on('request',r=>{if(r.url().endsWith('/api/v1/auth/refresh'))refreshes++});
  await Promise.all([page.goto(url+'/docs/'),other.goto(url+'/docs/')]);await Promise.all([visible(page),visible(other)]);
  assert.equal(refreshes,1,'two docs tabs must share Sub2 refresh lock');
  assert.equal(await page.evaluate(()=>localStorage.getItem('refresh_token')),'rotated-valid-refresh-concurrent');
  await ctx.close();results.push({case:'concurrent-expired-session',single_refresh:true,automatic_entry:true});
 }
 {
  const {ctx,page}=await setup();await store(page,'expired','valid-refresh-main-peer');
  const peer=await ctx.newPage();await peer.goto(url+'/docs/login?manual=1');
  await peer.evaluate(()=>{window.peerLocked=false;window.peerTask=navigator.locks.request('sub2api-auth-token-refresh',async()=>{window.peerLocked=true;await new Promise(resolve=>window.finishPeer=resolve)})});
  await peer.waitForFunction(()=>window.peerLocked);
  let refreshes=0;ctx.on('request',r=>{if(r.url().endsWith('/api/v1/auth/refresh'))refreshes++});
  const rejected=page.waitForResponse(r=>r.url().endsWith('/docs/session')&&r.status()===401);
  await page.goto(url+'/docs/');await rejected;
  await peer.evaluate(async()=>{
   const r=await fetch('/api/v1/auth/refresh',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({refresh_token:localStorage.getItem('refresh_token')})});const {data}=await r.json();
   localStorage.setItem('auth_token',data.access_token);localStorage.setItem('token_expires_at',String(Date.now()+data.expires_in*1000));localStorage.setItem('refresh_token',data.refresh_token);window.finishPeer();
  });
  await visible(page);assert.equal(refreshes,1);await ctx.close();results.push({case:'main-tab-refresh-lock',adopts_peer:true,no_duplicate_refresh:true});
 }
 {
  const {ctx,page}=await setup();await store(page,'expired','valid-refresh-no-lock');
  await ctx.addInitScript(()=>Object.defineProperty(navigator,'locks',{value:undefined}));
  await ctx.route('**/api/v1/auth/refresh',async route=>{const response=await route.fetch();if(response.status()===200)await new Promise(r=>setTimeout(r,1500));await route.fulfill({response})});
  const other=await ctx.newPage();let refreshes=0;ctx.on('request',r=>{if(r.url().endsWith('/api/v1/auth/refresh'))refreshes++});
  await Promise.all([page.goto(url+'/docs/'),other.goto(url+'/docs/')]);await Promise.all([visible(page),visible(other)]);
  assert.equal(refreshes,2);assert.equal(await page.evaluate(()=>localStorage.getItem('refresh_token')),'rotated-valid-refresh-no-lock');
  await ctx.close();results.push({case:'no-web-locks-refresh-race',adopts_delayed_winner:true,no_session_loss:true});
 }
 for(const change of ['logout','account-switch']){
  const {ctx,page}=await setup();await store(page,'expired','valid-refresh-late-'+change);
  let release;let held;const seen=new Promise(resolve=>held=resolve);
  await page.route('**/api/v1/auth/refresh',async route=>{const response=await route.fetch();await new Promise(resolve=>{release=resolve;held()});await route.fulfill({response})});
  await page.goto(url+'/docs/');await seen;
  if(change==='logout')await page.evaluate(()=>localStorage.clear());else await store(page,'admin','replacement-refresh',99);
  const state=await page.evaluate(()=>({...localStorage}));release();
  await page.locator('.login-main:not([hidden]) [name=email]').waitFor();
  assert.deepEqual(await page.evaluate(()=>({...localStorage})),state);assert.equal(await page.locator('article').count(),0);
  await ctx.close();results.push({case:'late-refresh-'+change,no_session_restore:true});
 }
 {
  const {ctx,page}=await setup();await store(page,'user');
  const downloading=page.waitForEvent('download');await page.goto(url+'/docs/guide.md');
  assert.deepEqual(await fs.readFile(await (await downloading).path()),expected);await visible(page);
  await ctx.close();results.push({case:'direct-markdown-entry',authenticated_download:true});
 }
 {
  const {ctx,page,errors}=await setup();await page.addInitScript(()=>Object.defineProperty(window,'localStorage',{get(){throw new DOMException('Blocked','SecurityError')}}));
  await page.goto(url+'/docs/');await page.locator('[name=email]').waitFor();assert.deepEqual(errors,[]);await ctx.close();results.push({case:'blocked-storage',manual_login_available:true});
 }
 await fs.writeFile(path.join(out,'shared-results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results));
} finally {if(browser)await browser.close();try{process.kill(-fixture.pid,'SIGTERM')}catch{};await fs.rm(tmp,{recursive:true,force:true})}
