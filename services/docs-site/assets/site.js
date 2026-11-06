'use strict';

// Highlight text nodes only: examples keep their exact text and are never HTML.
function highlightCode(code, language) {
  if (!['json', 'bash', 'python'].includes(language)) return;
  const source = code.textContent;
  const tokens = /#[^\n]*|"(?:\\[\s\S]|[^"\\])*"|'(?:\\[\s\S]|[^'\\])*'|\$\{[^}\n]+\}|\$[A-Za-z_][\w]*|--?[A-Za-z][\w-]*|\b\d+(?:\.\d+)?\b|\b[A-Za-z_]\w*\b/g;
  const keywords = new Set((language === 'python'
    ? 'import from as def return if elif else for in while with try except finally raise and or not is None True False'
    : language === 'json' ? 'true false null' : 'export').split(' '));
  const fragment = document.createDocumentFragment();
  let cursor = 0;
  for (const match of source.matchAll(tokens)) {
    const value = match[0];
    let kind = '';
    if (value[0] === '#' && language !== 'json') kind = 'comment';
    else if (value[0] === '"' || value[0] === "'") {
      kind = language === 'json' && /^\s*:/.test(source.slice(match.index + value.length)) ? 'key' : 'string';
    } else if (/^\d/.test(value)) kind = 'number';
    else if (keywords.has(value)) kind = 'keyword';
    else if (language === 'bash' && value[0] === '$') kind = 'variable';
    else if (language === 'bash' && value[0] === '-') kind = 'flag';
    else if ((language === 'bash' && value === 'curl') || (language === 'python' && /^\s*\(/.test(source.slice(match.index + value.length)))) kind = 'function';
    fragment.append(document.createTextNode(source.slice(cursor, match.index)));
    if (kind) {
      const token = document.createElement('span');
      token.className = 'token-' + kind;
      token.textContent = value;
      // Curl examples carry JSON inside a shell string; retain both quote marks.
      if (language === 'bash' && value.startsWith("'{")) {
        try {
          JSON.parse(value.slice(1, -1));
          const json = document.createElement('span');
          json.textContent = value.slice(1, -1);
          highlightCode(json, 'json');
          token.replaceChildren(document.createTextNode("'"), json, document.createTextNode("'"));
        } catch { /* Non-JSON shell strings keep their string styling. */ }
      }
      fragment.append(token);
    } else fragment.append(document.createTextNode(value));
    cursor = match.index + value.length;
  }
  fragment.append(document.createTextNode(source.slice(cursor)));
  code.replaceChildren(fragment);
}

const article = document.querySelector('article');
if (article) {
  const toc = document.querySelector('#toc');
  const headings = [...article.querySelectorAll('h2,h3')];
  const links = headings.map(h => {
    const a = document.createElement('a');
    a.href = '#' + h.id;
    a.textContent = h.textContent;
    a.className = 'level-' + h.tagName.slice(1);
    toc.append(a);
    return a;
  });
  let queued = false;
  function updateCurrentSection() {
    let active = 0;
    headings.forEach((h, i) => { if (h.getBoundingClientRect().top <= 128) active = i; });
    links.forEach((a, i) => { if (i === active) a.setAttribute('aria-current', 'location'); else a.removeAttribute('aria-current'); });
    queued = false;
  }
  window.addEventListener('scroll', () => {
    if (!queued) { queued = true; requestAnimationFrame(updateCurrentSection); }
  }, {passive: true});
  window.addEventListener('resize', updateCurrentSection);
  updateCurrentSection();

  for (const table of article.querySelectorAll('table')) {
    const wrap = document.createElement('div');
    wrap.className = 'table-wrap';
    table.before(wrap);
    wrap.append(table);
    for (const cell of table.querySelectorAll('td')) {
      const state = {'支持': 'yes', '不支持': 'no', '本地明确拒绝': 'limited'}[cell.textContent];
      if (!state || cell.children.length) continue;
      const badge = document.createElement('span');
      badge.className = 'capability capability-' + state;
      badge.textContent = cell.textContent;
      cell.replaceChildren(badge);
    }
  }
  for (const pre of article.querySelectorAll('pre')) {
    const code = pre.querySelector('code');
    if (!code) continue;
    const language = [...code.classList].find(c => c.startsWith('language-'))?.slice(9) || '';
    const source = code.textContent;
    highlightCode(code, language);
    const wrap = document.createElement('div');
    wrap.className = 'code-block';
    const toolbar = document.createElement('div');
    toolbar.className = 'code-toolbar';
    const label = document.createElement('span');
    label.className = 'code-language';
    label.textContent = {bash: 'Bash', python: 'Python', json: 'JSON'}[language] || '代码';
    const button = document.createElement('button');
    button.className = 'copy';
    button.textContent = '复制';
    button.type = 'button';
    button.setAttribute('aria-label', '复制' + label.textContent + '代码');
    button.setAttribute('aria-live', 'polite');
    button.addEventListener('click', async () => {
      try { await navigator.clipboard.writeText(source); button.textContent = '已复制'; }
      catch { button.textContent = '请选择代码复制'; }
      setTimeout(() => button.textContent = '复制', 1800);
    });
    toolbar.append(label, button);
    pre.before(wrap);
    wrap.append(toolbar, pre);
  }
  document.querySelector('#search').addEventListener('input', e => {
    const q = e.target.value.trim().toLocaleLowerCase();
    let count = 0;
    for (const a of links) {
      a.hidden = !a.textContent.toLocaleLowerCase().includes(q);
      if (!a.hidden) count++;
    }
    document.querySelector('#search-empty').hidden = count > 0;
  });
}
for (const form of document.querySelectorAll('form')) form.addEventListener('submit', () => {
  const b = form.querySelector('button');
  if (b) { b.disabled = true; b.textContent = '请稍候…'; }
});
window.addEventListener('pageshow', e => { if (e.persisted) location.reload(); });
