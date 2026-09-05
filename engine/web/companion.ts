export function initCompanion() {
  const dock = document.querySelector<HTMLElement>('#companion-dock')!;
  const sprite = document.querySelector<HTMLElement>('#companion-sprite')!;
  const toggle = document.querySelector<HTMLButtonElement>('#companion-toggle')!;
  const restore = document.querySelector<HTMLButtonElement>('#companion-restore')!;
  const card = document.querySelector<HTMLElement>('#companion-card')!;
  const motion = document.querySelector<HTMLInputElement>('#companion-motion')!;
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const read = (key: string) => { try { return localStorage.getItem(key); } catch { return null; } };
  const save = (key: string, value: string) => { try { localStorage.setItem(key, value); } catch { /* Preferences are optional. */ } };
  let hidden = read('jade.companion.hidden') === 'true';
  motion.checked = read('jade.companion.still') === 'true';
  let timer = 0, frame = 0, waving = false;
  const idle = [280, 110, 110, 140, 140, 320];
  const wave = [140, 140, 140, 280];
  function animate() {
    clearTimeout(timer);
    if (hidden || document.hidden || motion.checked || reduced.matches) {
      sprite.style.backgroundPosition = '0px 0px';
      return;
    }
    const durations = waving ? wave : idle;
    sprite.style.backgroundPosition = `${-frame * 72}px ${waving ? -234 : 0}px`;
    const delay = durations[frame];
    frame++;
    if (frame >= durations.length) { frame = 0; waving = false; }
    timer = window.setTimeout(animate, delay);
  }
  card.addEventListener('toggle', () => {
    if (card.matches(':popover-open')) { waving = true; frame = 0; animate(); }
  });
  // Safari restores the previously focused editor after a mouse-opened popover.
  // Explicit dismissal should return keyboard users to the companion button.
  const dismiss = () => { card.hidePopover(); toggle.focus(); };
  document.querySelector('#companion-close')!.addEventListener('click', dismiss);
  card.addEventListener('keydown', event => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); dismiss(); }
  });
  function visibility() {
    dock.hidden = hidden;
    restore.hidden = !hidden;
    frame = 0;
    animate();
  }
  document.querySelector('#companion-hide')!.addEventListener('click', () => {
    card.hidePopover(); hidden = true; save('jade.companion.hidden', 'true'); visibility(); restore.focus();
  });
  restore.addEventListener('click', () => {
    hidden = false; save('jade.companion.hidden', 'false'); visibility(); toggle.focus();
  });
  motion.addEventListener('change', () => { save('jade.companion.still', String(motion.checked)); frame = 0; animate(); });
  reduced.addEventListener('change', () => { frame = 0; animate(); });
  document.addEventListener('visibilitychange', () => { frame = 0; animate(); });
  const samples = [
    { category: 'News', title: 'A little good in the world', body: 'A place for a hopeful story, a small breakthrough, or something worth knowing about.' },
    { category: 'Culture', title: 'Something to fall in love with', body: 'A place for a film, a song, an exhibition, or a book worth making time for.' },
    { category: 'Web discovery', title: 'A lovely little rabbit hole', body: 'A place for an unexpected corner of the internet that makes an ordinary day more interesting.' },
  ];
  let current = 0;
  function showSample() {
    const sample = samples[current];
    document.querySelector('#companion-category')!.textContent = sample.category;
    document.querySelector('#companion-title')!.textContent = sample.title;
    document.querySelector('#companion-body')!.textContent = sample.body;
    document.querySelector('#companion-count')!.textContent = `${current + 1} of ${samples.length}`;
  }
  document.querySelector('#companion-next')!.addEventListener('click', () => {
    current = (current + 1) % samples.length; showSample();
  });
  showSample(); visibility();
}
