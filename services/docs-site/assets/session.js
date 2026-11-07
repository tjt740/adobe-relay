'use strict';

window.DocsSession = (() => {
  const refreshLock = 'sub2api-auth-token-refresh';
  const refreshTimeout = 30000;
  let pendingRefresh;

  function snapshot() {
    let userID = null;
    try {
      const id = Number(JSON.parse(localStorage.getItem('auth_user'))?.id);
      if (Number.isFinite(id) && id > 0) userID = id;
    } catch { /* The server, never this browser profile, decides access. */ }
    return {
      access: localStorage.getItem('auth_token'),
      refresh: localStorage.getItem('refresh_token'),
      expires: Number(localStorage.getItem('token_expires_at')),
      userID
    };
  }
  function sameSession(before) {
    const now = snapshot();
    return !!now.access && now.access === before.access && now.userID === before.userID;
  }
  function peerToken(before, failedToken) {
    const now = snapshot();
    if (!now.access || !now.refresh || !Number.isFinite(now.expires) || now.expires <= Date.now() || now.userID !== before.userID) return null;
    if (now.refresh !== before.refresh || (failedToken && now.access !== failedToken)) return now.access;
    return null;
  }
  function failure(status = 0) { return Object.assign(new Error('Session check failed'), {status}); }
  async function fetchBounded(path, options, timeout = refreshTimeout) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);
    try {
      const response = await fetch(path, {...options, mode: 'same-origin', redirect: 'error', cache: 'no-store', signal: controller.signal});
      const bytes = await response.arrayBuffer();
      return {status: response.status, ok: response.ok, bytes, type: response.headers.get('content-type') || ''};
    } finally { clearTimeout(timer); }
  }
  async function waitForPeer(before, failedToken, deadline) {
    do {
      const token = peerToken(before, failedToken);
      if (token) return token;
      const now = snapshot();
      if (!now.access || !now.refresh || now.userID !== before.userID) return null;
      await new Promise(resolve => setTimeout(resolve, 25));
    } while (Date.now() < deadline);
    return peerToken(before, failedToken);
  }

  // Same lock, snapshot checks and refresh-token-last commit as the Sub2 client.
  // A late result must never restore a logged-out or replaced browser session.
  function refreshAccess(failedToken) {
    if (pendingRefresh) return pendingRefresh;
    const before = snapshot();
    if (!before.access || !before.refresh || !before.userID) return Promise.reject(failure(401));
    const run = async (uncoordinated = false) => {
      const peer = peerToken(before, failedToken);
      if (peer) return peer;
      if (!sameSession(before) || snapshot().refresh !== before.refresh) throw failure(401);
      const deadline = Date.now() + refreshTimeout + 1000;
      try {
        const response = await fetchBounded('/api/v1/auth/refresh', {
          method: 'POST', credentials: 'same-origin',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({refresh_token: before.refresh})
        });
        if (!response.ok) throw failure(response.status);
        const payload = JSON.parse(new TextDecoder().decode(response.bytes));
        const pair = payload.data;
        if (payload.code !== 0 || !pair || typeof pair.access_token !== 'string' || !pair.access_token || pair.access_token.length > 8192 ||
            typeof pair.refresh_token !== 'string' || !pair.refresh_token || !Number.isFinite(pair.expires_in) || pair.expires_in <= 0) throw failure(502);
        if (!sameSession(before) || snapshot().refresh !== before.refresh) {
          const winner = peerToken(before, failedToken);
          if (winner) return winner;
          throw failure(401);
        }
        localStorage.setItem('auth_token', pair.access_token);
        localStorage.setItem('token_expires_at', String(Date.now() + pair.expires_in * 1000));
        localStorage.setItem('refresh_token', pair.refresh_token);
        return pair.access_token;
      } catch (error) {
        const rejected = error.status >= 400 && error.status < 500;
        const winner = await waitForPeer(before, failedToken, rejected && uncoordinated ? deadline : Date.now() + 1000);
        if (winner) return winner;
        throw error;
      }
    };
    const task = navigator.locks ? navigator.locks.request(refreshLock, () => run()) : run(true);
    pendingRefresh = task;
    const clear = () => { if (pendingRefresh === task) pendingRefresh = null; };
    task.then(clear, clear);
    return task;
  }

  async function readDocument(download = false) {
    for (let attempt = 0; attempt < 2; attempt++) {
      const before = snapshot();
      if (!before.access || before.access.length > 8192) throw failure(401);
      const response = await fetchBounded(download ? '/docs/session/guide.md' : '/docs/session', {
        method: 'POST', credentials: 'omit', headers: {Authorization: 'Bearer ' + before.access}
      });
      if (!sameSession(before)) {
        if (attempt === 0 && snapshot().access) continue;
        throw failure(401);
      }
      if (response.status === 401 && attempt === 0 && before.refresh) {
        await refreshAccess(before.access);
        continue;
      }
      if (!response.ok) throw failure(response.status);
      if (!response.type.startsWith(download ? 'text/markdown' : 'text/html')) throw failure(502);
      return {...response, session: before};
    }
    throw failure(401);
  }

  function leave() {
    // Remove the already rendered document before returning to the login page.
    document.body.replaceChildren();
    location.replace('/docs/login?manual=1');
  }
  function watchSession(session) {
    const check = () => {
      try {
        const now = snapshot();
        if (!now.access || now.userID !== session.userID) leave();
      } catch { leave(); }
    };
    window.addEventListener('storage', check);
    window.addEventListener('focus', check);
    document.addEventListener('visibilitychange', () => { if (!document.hidden) check(); });
  }
  function enableDownload() {
    const link = document.querySelector('a[href="/docs/guide.md"]');
    let downloading = false;
    link?.addEventListener('click', async event => {
      event.preventDefault();
      if (downloading) return;
      downloading = true;
      link.setAttribute('aria-busy', 'true');
      try {
        const response = await readDocument(true);
        if (!sameSession(response.session)) throw failure(401);
        const url = URL.createObjectURL(new Blob([response.bytes], {type: 'text/markdown;charset=utf-8'}));
        const a = document.createElement('a');
        a.href = url; a.download = 'adobe-image-api.md';
        document.body.append(a); a.click(); a.remove();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
      } catch (error) {
        if (error.status === 401 || error.status === 403) leave();
        else {
          link.textContent = '下载失败，请重试';
          setTimeout(() => link.textContent = '下载 Markdown', 2500);
        }
      } finally { downloading = false; link.removeAttribute('aria-busy'); }
    });
    return link;
  }

  async function start(initializePage) {
    if (document.body.dataset.autoLogin !== 'true') return;
    try { if (!snapshot().access) return; } catch { return; }
    const form = document.querySelector('.login-main');
    const waiting = document.createElement('p');
    waiting.className = 'session-status'; waiting.textContent = '正在验证登录…'; waiting.setAttribute('role', 'status');
    form.hidden = true; form.before(waiting);
    const download = new URLSearchParams(location.search).get('download') === '1';
    try {
      const response = await readDocument();
      if (!sameSession(response.session)) throw failure(401);
      const page = new DOMParser().parseFromString(new TextDecoder().decode(response.bytes), 'text/html');
      if (page.body.dataset.shared !== 'true' || !page.querySelector('article')) throw failure(502);
      // Only the server-authenticated, same-origin template supplies this body.
      document.body.replaceWith(page.body);
      history.replaceState(null, '', '/docs/' + location.hash);
      initializePage();
      if (location.hash) {
        try { document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView({behavior: 'instant'}); }
        catch { /* Ignore malformed fragments, never treat them as a redirect. */ }
      }
      watchSession(response.session);
      const link = enableDownload();
      if (download) link?.click();
    } catch (error) {
      waiting.remove(); form.hidden = false;
      if (error.status !== 401 && error.status !== 403) {
        const message = document.createElement('p');
        message.className = 'error'; message.setAttribute('role', 'alert');
        message.textContent = '暂时无法验证登录，请稍后重试。';
        form.querySelector('h1').after(message);
      }
    }
  }
  return {start};
})();
