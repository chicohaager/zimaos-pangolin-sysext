'use strict';
// Pangolin tile. Helpers (api, esc, $, setStatus, describe, initTheme) come from common.js.

const API = '/pangolin';

function render(d) {
  const on = d.active === 'active';
  const configured = d.endpoint && d.site_id && d.secret_set;
  const pill = on ? '<span class="pill pill-ok">running</span>' : `<span class="pill pill-off">${esc(d.active || 'stopped')}</span>`;
  const boot = d.boot ? '<span class="pill pill-accent">starts at boot</span>' : '';
  let actions = '';
  if (on || d.boot) actions = '<button class="btn-secondary" data-act="disable">Turn off</button><button class="btn-secondary" data-act="restart">Restart</button>';
  else if (configured) actions = '<button class="btn-primary" data-act="enable">Turn on</button>';
  $('#main').innerHTML = `
  <section class="card">
    <div class="head">
      <h3>Site tunnel</h3>
      <div class="app-actions">${actions}</div>
    </div>
    <dl class="kv">
      <dt>State</dt><dd>${pill}${boot}</dd>
      <dt>Server</dt><dd class="mono">${esc(d.endpoint || '—')}</dd>
      <dt>Site ID</dt><dd class="mono">${esc(d.site_id || '—')}</dd>
      <dt>Secret</dt><dd>${d.secret_set ? 'stored on this host (not shown)' : '<span class="muted">not set</span>'}</dd>
      <dt>Pangolin CLI</dt><dd class="mono">${esc(d.cli || '?')}</dd>
    </dl>
    ${d.error ? `<p class="app-err">${esc(d.error)}</p>` : ''}
  </section>
  <section class="card">
    <h2 class="section-title">${configured ? 'Change the site' : 'Set up the site'}</h2>
    <p class="muted" style="font-size:12px">In your Pangolin dashboard: <b>Sites</b> → <b>Add Site</b> → type <b>Newt</b>. It shows the endpoint, the ID and the secret once. The secret stays on this host in <span class="mono">/DATA/AppData/pangolin/site.env</span> (readable by root only).</p>
    <form class="form" id="cfg-form" autocomplete="off">
      <label>Endpoint <input name="endpoint" class="mono" placeholder="https://pangolin.example.com" value="${esc(d.endpoint || '')}" required></label>
      <label>Site ID <input name="site_id" class="mono" value="${esc(d.site_id || '')}" required></label>
      <label>Secret <input name="secret" class="mono" type="password" placeholder="${d.secret_set ? 'empty = keep the stored secret' : ''}" ${d.secret_set ? '' : 'required'}></label>
      <div class="app-actions">
        <button class="btn-primary" type="submit">Save</button>
        ${configured ? '<button class="btn-secondary" type="button" data-act="forget">Forget this site</button>' : ''}
      </div>
    </form>
  </section>
  <section class="card">
    <h2 class="section-title">Log</h2>
    <pre class="log mono" id="log"></pre>
  </section>`;
  const log = $('#log');
  log.textContent = d.log || 'no log yet';
  log.scrollTop = log.scrollHeight; // the newest lines are at the end

}

async function load() {
  render(await api(`${API}/status`));
}

async function refresh() {
  try {
    await load();
    setStatus('');
  } catch (e) {
    $('#main').innerHTML = `<section class="card"><p class="app-err">${esc(describe(e))}</p></section>`;
    setStatus(describe(e), 'err');
  }
}

async function act(label, fn) {
  setStatus(`${label} …`);
  try {
    await fn();
    setStatus(`${label}: done.`, 'ok');
    await load();
  } catch (e) {
    setStatus(`${label}: ${describe(e)}`, 'err');
  }
}

const action = (a) => api(`${API}/action`, { method: 'POST', body: { action: a } });

document.addEventListener('submit', async (e) => {
  if (e.target.id !== 'cfg-form') return;
  e.preventDefault();
  const f = new FormData(e.target);
  await act('Saving the site', () => api(`${API}/config`, {
    method: 'POST',
    body: { endpoint: f.get('endpoint'), site_id: f.get('site_id'), secret: f.get('secret') },
  }));
});

document.addEventListener('click', async (e) => {
  const btn = e.target.closest('button[data-act]');
  if (!btn) return;
  switch (btn.dataset.act) {
    case 'enable': await act('Turning the site tunnel on', () => action('enable')); break;
    case 'disable': await act('Turning the site tunnel off', () => action('disable')); break;
    case 'restart': await act('Restarting the site tunnel', () => action('restart')); break;
    case 'forget':
      // eslint-disable-next-line no-alert
      if (!window.confirm('Turn the tunnel off and delete the stored site credentials on this host?')) return;
      await act('Forgetting the site', () => action('forget'));
      break;
    default:
  }
});

initTheme('pangolin.theme');
$('#refresh-btn').addEventListener('click', refresh);
refresh();
