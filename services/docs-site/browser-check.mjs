import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {pathToFileURL,fileURLToPath} from 'node:url';
const pw=await import(process.env.PLAYWRIGHT_MODULE_PATH?pathToFileURL(process.env.PLAYWRIGHT_MODULE_PATH).href:'playwright');
const cwd=path.dirname(fileURLToPath(import.meta.url));
const tmp=await fs.mkdtemp(path.join(os.tmpdir(),'docs-browser-'));
const ready=path.join(tmp,'ready');
const out=process.env.DOCS_BROWSER_OUTPUT||path.join(cwd,'.browser-output');await fs.mkdir(out,{recursive:true});
const fixture=spawn('go',['test','-run','^TestBrowserFixture$','-count=1','-timeout','6m'],{cwd,detached:true,env:{...process.env,DOCS_BROWSER_READY_FILE:ready},stdio:['ignore','pipe','pipe']});
let fixtureLog='';fixture.stdout.on('data',b=>fixtureLog+=b);fixture.stderr.on('data',b=>fixtureLog+=b);
const results=[];
try{
 let url;
 for(let i=0;i<300;i++){try{url=await fs.readFile(ready,'utf8');break}catch{};if(fixture.exitCode!==null)throw Error(fixtureLog);await new Promise(r=>setTimeout(r,100))}
 assert(url,'fixture failed to start');
 for(const kind of (process.env.DOCS_BROWSERS||'chromium,webkit').split(',')){
  const browser=await pw[kind].launch({headless:true,...(kind==='chromium'&&process.env.DOCS_CHROME_CHANNEL?{channel:process.env.DOCS_CHROME_CHANNEL}:{})});
  try{
   // Reproduce the old response policy using a real native form submission.
   const old=await browser.newContext({ignoreHTTPSErrors:true});const oldPage=await old.newPage();
   await oldPage.route('**/docs/login',async route=>{const r=await route.fetch();await route.fulfill({response:r,headers:{...r.headers(),'referrer-policy':'no-referrer'}})});
   await oldPage.goto(url+'/docs/login');await oldPage.locator('[name=email]').fill('admin@example.com');await oldPage.locator('[name=password]').fill('fixture-password');
   const failed=oldPage.waitForResponse(r=>new URL(r.url()).pathname==='/docs/auth/login');await oldPage.getByRole('button',{name:'登录',exact:false}).click();const fail=await failed;
   assert.equal(fail.status(),403);assert.equal((await fail.request().allHeaders()).origin,'null');
   results.push({browser:kind,case:'old-policy-native-form',status:403,origin:'null'});await old.close();
   for(const width of [1440,390]){
    const ctx=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width,height:950}});const page=await ctx.newPage();
    const errors=[];page.on('pageerror',e=>errors.push(e.message));
    for(const email of ['admin@example.com','user@example.com','2fa@example.com']){
     const r=await page.goto(url+'/docs/login');assert.equal(r.headers()['referrer-policy'],'same-origin');
     const text=await page.locator('body').innerText();assert(!text.includes('仅向已注册')&&!text.includes('与 Sub2 使用同一账号')&&!text.includes('查看模型参数'));
     if(email==='admin@example.com')await page.screenshot({path:path.join(out,`${kind}-login-${width}.png`),fullPage:true});
     await page.locator('[name=email]').fill(email);await page.locator('[name=password]').fill('fixture-password');
     const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/docs/auth/login');await page.getByRole('button',{name:'登录',exact:false}).click();const login=await response;
     assert.equal(login.status(),303);assert.equal((await login.request().allHeaders()).origin,url);
     if(email.startsWith('2fa')){await page.locator('[name=totp_code]').fill('123456');await page.getByRole('button',{name:'验证',exact:false}).click()}
     await page.waitForURL(url+'/docs/');await page.locator('article h1').waitFor();
     assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'page overflow');
     const md=await ctx.request.get(url+'/docs/guide.md');assert.equal(md.status(),200);assert((await md.text()).includes('frame_duration_ms'));
     await page.getByRole('button',{name:'退出登录'}).click();await page.waitForURL(url+'/docs/login');
     const denied=await ctx.request.get(url+'/docs/guide.md',{maxRedirects:0});assert.equal(denied.status(),303);
     results.push({browser:kind,width,case:email.split('@')[0],native_login:true,download:true,logout:true,overflow:false});
    }
    assert.deepEqual(errors,[]);await ctx.close();
   }
  }finally{await browser.close()}
 }
 await fs.writeFile(path.join(out,'results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results));
}finally{try{process.kill(-fixture.pid,'SIGTERM')}catch{};await fs.rm(tmp,{recursive:true,force:true})}
