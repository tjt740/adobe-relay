'use strict';
const article=document.querySelector('article');
if(article){
 const toc=document.querySelector('#toc');
 for(const h of article.querySelectorAll('h2,h3')){const a=document.createElement('a');a.href='#'+h.id;a.textContent=h.textContent;a.className='level-'+h.tagName.slice(1);toc.append(a)}
 for(const table of article.querySelectorAll('table')){const wrap=document.createElement('div');wrap.className='table-wrap';table.before(wrap);wrap.append(table)}
 for(const pre of article.querySelectorAll('pre')){const code=pre.querySelector('code');if(!code)continue;const button=document.createElement('button');button.className='copy';button.textContent='复制';button.type='button';button.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(code.textContent);button.textContent='已复制'}catch{button.textContent='请选择代码复制'}setTimeout(()=>button.textContent='复制',1800)});pre.append(button)}
 document.querySelector('#search').addEventListener('input',e=>{const q=e.target.value.trim().toLocaleLowerCase();let count=0;for(const a of toc.querySelectorAll('a')){a.hidden=!a.textContent.toLocaleLowerCase().includes(q);if(!a.hidden)count++}document.querySelector('#search-empty').hidden=count>0});
}
for(const form of document.querySelectorAll('form'))form.addEventListener('submit',()=>{const b=form.querySelector('button');if(b){b.disabled=true;b.textContent='请稍候…'}});
window.addEventListener('pageshow',e=>{if(e.persisted)location.reload()});
