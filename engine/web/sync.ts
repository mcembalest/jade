export function initSync() {
  const panel = document.createElement('div'); panel.id = 'sync-status-panel'; panel.hidden = true;
  panel.style.cssText = 'padding:8px 16px;background:#edf5ef;border-bottom:1px solid #d5e2d8;font-size:12px';
  const label = document.createElement('span'); label.setAttribute('role', 'status');
  const button = document.createElement('button'); button.textContent = 'Sync now'; button.type = 'button';
  const both = document.createElement('button'); both.textContent = 'Keep both versions'; both.type = 'button'; both.hidden = true;
  button.style.marginLeft = both.style.marginLeft = '12px';
  panel.append(label, button, both); document.querySelector('header')!.after(panel);
  let pending = false;
  const path = () => [document.body.dataset.jade === '.' ? '' : document.body.dataset.jade, document.body.dataset.file].filter(Boolean).join('/');
  async function update() {
    if (pending || document.hidden) return; pending = true;
    try {
      const response = await fetch('/sync', {signal:AbortSignal.timeout(25000)});
      if (!response.ok) throw new Error('Sync status unavailable');
      const data = await response.json(); panel.hidden = !data.enabled;
      if (data.enabled) {
        const status = data.files?.[path()] || data.message;
        label.textContent = status + (data.message?.includes('pending') || data.message?.includes('failed') || data.message?.includes('unreachable') ? ' · ' + data.message : '');
        both.hidden = !status.startsWith('Conflict');
        label.title = data.lastSync ? 'Last server check: ' + new Date(data.lastSync).toLocaleString() : 'Not yet synced';
      }
    } catch { if (!panel.hidden) label.textContent = 'Sync status unavailable · local editor saves are separate'; }
    finally { pending = false; }
  }
  async function run(keepBoth = false) {
    button.disabled = both.disabled = true;
    try {
      const response = await fetch('/sync', {method:'POST',body:new URLSearchParams(keepBoth ? {keepBoth:path()} : {}),signal:AbortSignal.timeout(25000)});
      if (!response.ok) throw new Error(await response.text());
      label.textContent = 'Sync requested…';
    } catch (e) {label.textContent = String(e);}
    finally {button.disabled = both.disabled = false;}
    void update();
  }
  button.addEventListener('click', () => void run()); both.addEventListener('click', () => void run(true));
  setInterval(update,3000); addEventListener('focus',update); void update();
}
