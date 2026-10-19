"""Verify the deployed UIs using ten-minute sessions and inert release fixtures."""
import asyncio
import json
from pathlib import Path
from urllib.parse import urlparse,parse_qs
import sys

from playwright.async_api import async_playwright, expect

ROOT=Path(__file__).resolve().parent
BASE='https://creative.cheap'


async def naive(page, selector, label):
    await page.locator(selector).locator('.n-base-selection').click()
    option=page.locator('.n-base-select-option:visible').filter(has=page.get_by_text(label,exact=True))
    await expect(option).to_have_count(1)
    await option.click()


async def choose(page, selector, label):
    await page.locator(selector).click()
    await page.get_by_role('option',name=label,exact=True).click()


async def main():
    secret=ROOT/'browser-auth.json'
    data=json.loads(secret.read_text())
    fixture=data['fixture'];member=fixture['member_id'];email=fixture['email'];a,b=fixture['groups'][:2]
    outputs=ROOT/'live-preview';outputs.mkdir(exist_ok=True)
    errors=[];writes=[];calls=[];created=[]
    async with async_playwright() as p:
        browser=await p.chromium.launch()
        context=await browser.new_context(viewport={'width':1440,'height':1000},locale='zh-CN')
        await context.add_init_script('''const data='''+json.dumps(data,ensure_ascii=False)+''';
if(location.origin==='https://creative.cheap'){
 localStorage.setItem('okad_token',data.okad_token);
 localStorage.setItem('auth_token',data.sub2_token);
 localStorage.setItem('auth_user',JSON.stringify(data.sub2_user));
 localStorage.setItem('sub2api_locale','zh');
 for(const key of ['admin_guide','user_guide','onboarding_tour'])localStorage.setItem(key+'_'+data.sub2_user.id+'_'+data.sub2_user.role+'_v4_interactive','true');
}''')
        page=await context.new_page();page.set_default_timeout(20000)
        page.on('pageerror',lambda error:errors.append(str(error)))
        async def response(value):
            path=urlparse(value.url).path
            if path.endswith('/api/external/groups') and value.request.method=='POST' and value.status==201:
                created.append((await value.json())['id'])
                (ROOT/'browser-created-groups.json').write_text(json.dumps(created))
        page.on('response',response)
        def request(req):
            url=urlparse(req.url)
            if url.hostname=='creative.cheap' and url.path.endswith('/members/group'):
                writes.append(req.post_data_json)
            if url.hostname=='creative.cheap' and url.path=='/api/v1/admin/accounts':
                calls.append(parse_qs(url.query))
        page.on('request',request)
        await page.goto(BASE+'/okad/external-members')
        await expect(page.locator('#extm-f-group')).to_be_visible()
        await page.locator('#extm-f-kw').fill(email)
        await naive(page,'#extm-f-company','越南账号')
        await naive(page,'#extm-f-group','发布验证-20261020-B')
        await expect(page.locator('#extm-total')).to_have_text('1')
        await expect(page.locator(f'[data-member="{member}"] .extm-group-tag')).to_have_text('发布验证-20261020-B')
        await page.locator('#extm-open-import').click()
        await page.locator('#extm-group-name').fill('发布验证-20261020-C')
        await page.locator('#extm-create-group').click()
        await expect(page.locator('#extm-import-group')).to_contain_text('发布验证-20261020-C')
        await page.locator('#extm-import-done').click()
        await page.locator(f'[data-member="{member}"] .extm-rowck').check()
        await page.locator('#extm-batch-group').scroll_into_view_if_needed()
        await naive(page,'#extm-batch-group','发布验证-20261020-A')
        await expect(page.locator('#extm-total')).to_have_text('0')
        await naive(page,'#extm-f-group','发布验证-20261020-A')
        await expect(page.locator('#extm-total')).to_have_text('1')
        await expect(page.locator(f'[data-member="{member}"] .extm-group-tag')).to_have_text('发布验证-20261020-A')
        assert writes[-1]=={'ids':[member],'group_id':a}
        await page.screenshot(path=str(outputs/'okad-desktop.png'))
        await page.set_viewport_size({'width':390,'height':950})
        await expect(page.locator('#extm-f-group')).to_be_visible()
        await naive(page,'#extm-f-group','发布验证-20261020-B')
        await expect(page.locator('#extm-total')).to_have_text('0')
        await naive(page,'#extm-f-group','发布验证-20261020-A')
        await expect(page.locator('#extm-total')).to_have_text('1')
        await page.screenshot(path=str(outputs/'okad-mobile.png'))
        assert created,'UI group creation was not observed'
        (ROOT/'browser-created-groups.json').write_text(json.dumps(created))
        # All following reads hit deployed Sub2 APIs; no API or auth responses are mocked.
        await page.set_viewport_size({'width':1440,'height':1000})
        await page.goto(BASE+'/admin/accounts')
        await expect(page.locator('#okad-group-filter')).to_be_visible()
        search=page.locator('input[placeholder="搜索账号..."]')
        if not await search.count():
            search=page.locator('input').filter(has_not=page.locator('[type=checkbox]'))
            # Use the table filter SearchInput, whose input is immediately before the platform selector.
            search=page.locator('input[placeholder]').filter(has_not=page.locator('[type=checkbox]'))
            candidates=await search.evaluate_all('(els)=>els.map(e=>({p:e.placeholder,type:e.type}))')
            raise RuntimeError('Resolve deployed account search placeholder: '+str(candidates))
        await search.fill(email)
        await choose(page,'#okad-group-filter','发布验证-20261020-A')
        kind=page.locator('button.select-trigger').filter(has_text='全部账号归属')
        await kind.click();await page.get_by_role('option',name='Okad · 越南账号',exact=True).click()
        await expect(page.get_by_text(email,exact=False).first).to_be_visible()
        for attempt in range(100):
            if any(q.get('okad_group')==[data['issuer']+':'+str(a)] and q.get('okad_account_type')==['vietnam'] and q.get('search')==[email] for q in calls):break
            await asyncio.sleep(.1)
        else:raise AssertionError('Combined Sub2 group/type/search request was not observed')
        await page.screenshot(path=str(outputs/'sub2-desktop.png'))
        await page.set_viewport_size({'width':390,'height':950})
        await expect(page.locator('#okad-group-filter')).to_be_visible()
        await choose(page,'#okad-group-filter','Okad · 未分组')
        await expect(page.get_by_text(email,exact=False)).to_have_count(0)
        await choose(page,'#okad-group-filter','发布验证-20261020-A')
        await expect(page.get_by_text(email,exact=False).first).to_be_visible()
        await page.screenshot(path=str(outputs/'sub2-mobile.png'))
        assert not errors,errors
        await context.close();await browser.close()
    result={'okad_ui_create':True,'okad_ui_regroup':True,'okad_type_group_filter':True,'sub2_type_group_filter':True,'desktop_and_mobile':True,'created_groups':created,'javascript_errors':errors,'mocked_responses':False}
    (ROOT/'browser-verification.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
    print(json.dumps(result,ensure_ascii=False))


if __name__=='__main__':
    asyncio.run(main())
