// Desktop roots get separate immutable servers; the existing editor owns
// save-before-navigation and per-root session restoration.
export function initProjects(leave: (action: () => Promise<void>) => Promise<void>) {
  const dialog = document.querySelector<HTMLDialogElement>('#projects-dialog')!;
  const input = document.querySelector<HTMLInputElement>('#project-path')!;
  const error = document.querySelector<HTMLElement>('#projects-error')!;
  const recent = document.querySelector<HTMLElement>('#recent-projects')!;
  const toggle = document.querySelector<HTMLButtonElement>('#projects-toggle')!;
  const submit = document.querySelector<HTMLButtonElement>('#project-open')!;
  if (new URL(location.href).searchParams.has('projects-warning')) {
    const notice = document.createElement('p'); notice.id = 'projects-warning'; notice.setAttribute('role', 'status');
    notice.textContent = 'Project opened, but recent projects could not be remembered on this Mac.';
    document.querySelector('header')!.after(notice);
  }
  let opening = false, generation = 0;
  let pending: AbortController | undefined;
  function busy(value: boolean) {
    opening = value; submit.disabled = value; input.disabled = value;
    submit.textContent = value ? 'Opening…' : 'Open project';
    recent.querySelectorAll('button').forEach(button => button.disabled = value);
  }
  async function open(path: string) {
    if (opening) return;
    const version = generation;
    pending = new AbortController();
    const signal = AbortSignal.any([pending.signal, AbortSignal.timeout(10000)]);
    busy(true);
    error.textContent = '';
    let switched = false;
    try {
      await leave(async () => {
        if (version !== generation || !dialog.open) return;
        let response: Response;
        try {
          response = await fetch('/projects', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({path}), signal});
        } catch {
          if (version === generation) error.textContent = 'Could not open the project. Please try again.';
          return;
        }
        if (version !== generation || !dialog.open) return;
        if (!response.ok) { error.textContent = await response.text(); return; }
        const result: {url: string} = await response.json();
        if (version !== generation || !dialog.open) return;
        switched = true;
        location.href = result.url;
      });
      if (version === generation && !switched && !error.textContent) error.textContent = 'Project not opened. Resolve the current file’s save message, then try again.';
    } catch { if (version === generation) error.textContent = 'Could not open the project. Please try again.'; }
    finally { busy(false); }
  }
  toggle.addEventListener('click', async () => {
    if (document.querySelector('dialog[open]') || opening) return;
    const version = ++generation;
    error.textContent = '';
    recent.replaceChildren();
    input.value = '';
    dialog.showModal();
    input.focus();
    try {
      const response = await fetch('/projects', {signal:AbortSignal.timeout(5000)});
      if (!response.ok) throw new Error();
      const data: {current: string; recent: string[]} = await response.json();
      if (version !== generation || !dialog.open) return;
      if (!input.value) { input.value = data.current; input.select(); }
      for (const path of data.recent) {
        const button = document.createElement('button');
        button.type = 'button'; button.textContent = path; button.title = path;
        button.disabled = opening;
        button.addEventListener('click', () => { input.value = path; void open(path); });
        const row = document.createElement('p'); row.append(button); recent.append(row);
      }
      if (!data.recent.length) recent.textContent = 'Projects you open here will appear here.';
    } catch { if (version === generation) error.textContent = 'Could not load recent projects. You can still enter a folder path.'; }
  });
  dialog.addEventListener('close', () => { generation++; pending?.abort(); toggle.focus(); });
  document.querySelector('#projects-cancel')!.addEventListener('click', () => dialog.close());
  document.querySelector('#projects-form')!.addEventListener('submit', event => { event.preventDefault(); void open(input.value.trim()); });
}
