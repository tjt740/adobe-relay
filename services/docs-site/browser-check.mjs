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
    await page.addInitScript(()=>Object.defineProperty(navigator,'clipboard',{value:{writeText:async text=>{window.copiedExample=text}}}));
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
     if(email==='admin@example.com'){
      const original=await ctx.request.get(url+'/docs/');
      const expected=await page.evaluate(html=>{
       const doc=new DOMParser().parseFromString(html,'text/html');
       return {codes:[...doc.querySelectorAll('pre code')].map(n=>n.textContent),cells:[...doc.querySelectorAll('td')].map(n=>n.textContent)};
      },await original.text());
      assert.deepEqual(await page.locator('pre code').allTextContents(),expected.codes,'highlighting must preserve every example byte');
      assert.deepEqual(await page.locator('td').allTextContents(),expected.cells,'status styling must preserve table content');
      assert(await page.locator('.token-key').count()>0&&await page.locator('.token-number').count()>0);
      assert.equal(await page.locator('#toc [aria-current="location"]').count(),1);
      await page.screenshot({path:path.join(out,`${kind}-document-${width}.png`)});
      await page.locator('table').nth(1).scrollIntoViewIfNeeded();
      await page.screenshot({path:path.join(out,`${kind}-models-${width}.png`)});
      await page.locator('article pre').nth(1).scrollIntoViewIfNeeded();
      await page.screenshot({path:path.join(out,`${kind}-code-${width}.png`)});
      for(let i=0;i<expected.codes.length;i++){
       await page.locator('.copy').nth(i).click();
       assert.equal(await page.evaluate(()=>window.copiedExample),expected.codes[i],'copy must preserve exact example');
      }
      await page.locator('#search').fill('不存在的章节');
      assert.equal(await page.locator('#search-empty').isVisible(),true);
      await page.locator('#search').fill('GIF');
      assert(await page.locator('#toc a:visible').count()>0);
      await page.locator('#search').fill('');
      const link=page.locator('#toc a.level-2').nth(1);
      const target=await link.getAttribute('href');await link.click();
      await page.waitForFunction(id=>document.querySelector('#toc [aria-current="location"]')?.getAttribute('href')===id,target);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,'code/table overflow must stay inside containers');
      results.push({browser:kind,width,case:'document-ui',exact_code_and_copy:expected.codes.length,exact_table_content:true,search:true,current_section:true});
     }
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
