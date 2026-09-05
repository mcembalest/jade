interface Result { file: string; line: number; excerpt: string }

export function initSearch(openFile: (file: string, line: number) => Promise<void>) {
  const dialog = document.querySelector<HTMLDialogElement>('#search-dialog')!;
  const input = document.querySelector<HTMLInputElement>('#search-input')!;
  const results = document.querySelector<HTMLElement>('#search-results')!;
  const status = document.querySelector<HTMLElement>('#search-status')!;
  const toggle = document.querySelector<HTMLButtonElement>('#search-toggle')!;
  let timer = 0, generation = 0;
  let pending: AbortController | undefined;
  let returnFocus: HTMLElement | null = null;
  const open = (source = document.activeElement as HTMLElement) => {
    if (document.querySelector('dialog[open]')) return;
    returnFocus = source;
    dialog.showModal(); input.focus(); input.select();
    if (input.value.trim()) search();
  };
  toggle.addEventListener('click', () => open(toggle));
  document.querySelector('#search-close')!.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => {
    clearTimeout(timer); pending?.abort(); generation++;
    if (returnFocus?.isConnected) returnFocus.focus(); else toggle.focus();
  });
  async function search() {
    const version = ++generation;
    pending?.abort(); results.replaceChildren();
    const query = input.value.trim();
    if (!query) { status.textContent = 'Search filenames and saved text in this folder.'; return; }
    pending = new AbortController();
    status.textContent = 'Searching…';
    const url = new URL('/search', location.origin);
    url.searchParams.set('jade', document.body.dataset.jade!); url.searchParams.set('q', query);
    try {
      const response = await fetch(url, {signal:AbortSignal.any([pending.signal, AbortSignal.timeout(5000)])});
      if (!response.ok) throw new Error((await response.text()).trim());
      const data: {results: Result[]; incomplete: boolean} = await response.json();
      if (version !== generation || !dialog.open) return;
      status.textContent = (data.results.length ? `${data.results.length} result${data.results.length === 1 ? '' : 's'}.` : 'No matches.') +
        (data.incomplete ? ' Some results may be missing; the search limit was reached.' : '');
      for (const result of data.results) {
        const button = document.createElement('button'); button.type = 'button'; button.dataset.previewFile = result.file;
        const name = document.createElement('strong'); name.textContent = result.file + (result.line ? ` · ${result.line}` : '');
        const excerpt = document.createElement('span'); excerpt.textContent = result.excerpt || 'Filename match';
        button.append(name, excerpt);
        button.addEventListener('click', () => { dialog.close(); void openFile(result.file, result.line); });
        results.append(button);
      }
    } catch (error) {
      if (version === generation && dialog.open) status.textContent = 'Search unavailable: ' + (error instanceof Error ? error.message : String(error));
    }
  }
  input.addEventListener('input', () => {
    clearTimeout(timer); generation++; pending?.abort(); results.replaceChildren();
    status.textContent = input.value.trim() ? 'Searching…' : 'Search filenames and saved text in this folder.';
    timer = window.setTimeout(search, 180);
  });
  dialog.addEventListener('keydown', event => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); dialog.close(); return; }
    const buttons = [...results.querySelectorAll('button')];
    if (event.key === 'Enter' && document.activeElement === input) { event.preventDefault(); buttons[0]?.click(); }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === 'ArrowUp' && index <= 0) input.focus();
    else buttons[Math.min(buttons.length-1, index + (event.key === 'ArrowDown' ? 1 : -1))]?.focus();
  });
  addEventListener('keydown', event => {
    if ((event.metaKey || event.ctrlKey) && event.shiftKey && event.key.toLowerCase() === 'f') { event.preventDefault(); open(); }
  });
}
