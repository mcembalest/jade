// Filter the rendered tree; opening a match uses its existing guarded navigation.
export function initFileFilter() {
  const input = document.querySelector<HTMLInputElement>('#file-filter')!;
  const clear = document.querySelector<HTMLButtonElement>('#clear-file-filter')!;
  const status = document.querySelector<HTMLElement>('#file-filter-status')!;
  const tree = document.querySelector<HTMLElement>('.tree')!;
  const links = [...tree.querySelectorAll<HTMLAnchorElement>('.file-link')];
  const folders = [...tree.querySelectorAll<HTMLDetailsElement>('details')];
  let previousExpansion: boolean[] | undefined;

  function filter() {
    const query = input.value.trim().toLocaleLowerCase();
    if (query && !previousExpansion) previousExpansion = folders.map(folder => folder.open);
    let count = 0;
    for (const link of links) {
      const path = `${link.dataset.jade}/${link.dataset.file}`.toLocaleLowerCase();
      const matches = !query || path.includes(query);
      link.parentElement!.hidden = !matches;
      if (matches) count++;
    }
    // Children are settled before their containing folders.
    for (let index = folders.length - 1; index >= 0; index--) {
      const folder = folders[index];
      const matches = [...folder.querySelector('ul')!.children].some(child => !(child as HTMLElement).hidden);
      folder.parentElement!.hidden = !!query && !matches;
      if (query) folder.open = matches;
      else if (previousExpansion) folder.open = previousExpansion[index];
    }
    if (!query) previousExpansion = undefined;
    clear.hidden = !input.value;
    status.hidden = !query;
    status.textContent = query ? (count ? `${count} matching file${count === 1 ? '' : 's'}.` : 'No matching filenames or paths.') : '';
  }
  function reset() { input.value = ''; filter(); input.focus(); }
  input.addEventListener('input', filter);
  clear.addEventListener('click', reset);
  input.addEventListener('keydown', event => {
    if (event.isComposing) return;
    if (event.key === 'Escape' && input.value) {
      event.preventDefault(); event.stopPropagation(); reset();
    }
    if (event.key === 'Enter' && input.value.trim()) {
      event.preventDefault();
      links.find(link => !link.parentElement!.hidden)?.click();
    }
  });
}
