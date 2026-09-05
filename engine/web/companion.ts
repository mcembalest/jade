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
  type Message = { id: string; role: string; text: string; sources?: {title: string; url: string}[]; proactive?: boolean };
  type State = { messages: Message[]; enabled: boolean; next: number; seen: string };
  const chat = document.querySelector<HTMLElement>('#companion-chat')!;
  const input = document.querySelector<HTMLTextAreaElement>('#companion-input')!;
  const status = document.querySelector<HTMLElement>('#companion-status')!;
  const send = document.querySelector<HTMLButtonElement>('#companion-send')!;
  const stop = document.querySelector<HTMLButtonElement>('#companion-stop')!;
  const bubble = document.querySelector<HTMLButtonElement>('#companion-bubble')!;
  let state: State | undefined;
  let active: AbortController | undefined;
  let checking = false;
  let rendered = '';
  let seenPending = '';
  let visibilityVersion = 0;

  async function api(body?: object, signal?: AbortSignal): Promise<State> {
    const response = await fetch('/companion', body ? {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body), signal} : {signal, headers:{'X-JaDE-Companion-Hidden':String(hidden)}});
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }
  function render(next: State) {
    state = next;
    // Server state is shared across ports and JaDE processes, including hide/restore.
    if (hidden === next.enabled) {
      hidden = !next.enabled;
      save('jade.companion.hidden', String(hidden));
      visibility();
      if (hidden) card.hidePopover();
    }
    const serialized = JSON.stringify(next.messages);
    if (serialized !== rendered) {
      rendered = serialized;
      chat.replaceChildren();
      for (const message of next.messages) {
        const row = document.createElement('div'); row.className = 'companion-message'; row.dataset.role = message.role;
        const author = document.createElement('strong'); author.textContent = message.role === 'user' ? 'You' : 'Sanjana';
        const text = document.createElement('p'); text.textContent = message.text;
        row.append(author, text);
        for (const source of message.sources || []) {
          try {
            const url = new URL(source.url);
            if (!['https:', 'http:'].includes(url.protocol)) continue;
            const link = document.createElement('a'); link.href = url.href; link.textContent = source.title || url.hostname; link.target = '_blank'; link.rel = 'noopener noreferrer'; row.append(link);
          } catch { /* Ignore malformed source links. */ }
        }
        chat.append(row);
      }
      if (!next.messages.length) chat.textContent = 'Tell me what you’re in the mood for, or ask me to search.';
      chat.scrollTop = chat.scrollHeight;
    }
    const latest = [...next.messages].reverse().find(message => message.role === 'assistant');
    bubble.hidden = hidden || !latest?.proactive || latest.id === next.seen || card.matches(':popover-open');
    bubble.textContent = latest?.text.slice(0,140) || '';
    if (card.matches(':popover-open') && latest && latest.id !== next.seen && seenPending !== latest.id) {
      seenPending = latest.id;
      void api({action:'seen', seen:latest.id}).then(updated => { if (state) state.seen = updated.seen; }).catch(() => {}).finally(() => { seenPending = ''; });
    }
  }
  async function refresh() {
    if (checking || active) return;
    checking = true;
    const version = visibilityVersion;
    try {
      const next = await api();
      if (version !== visibilityVersion) return;
      render(next);
      if (!hidden && state && Date.now() >= state.next) await talk(true);
    } catch (error) { status.textContent = (error as Error).message; }
    finally { checking = false; }
  }
  async function talk(proactive: boolean) {
    if (active || hidden) return;
    const message = input.value.trim();
    if (!proactive && !message) return;
    const controller = new AbortController(); active = controller;
    send.disabled = true; stop.hidden = false;
    status.textContent = proactive ? 'Exploring a little…' : 'Thinking…';
    try {
      render(await api({action:proactive ? 'discover' : 'chat', message}, controller.signal));
      if (!proactive && input.value.trim() === message) input.value = '';
      status.textContent = '';
    } catch (error) {
      status.textContent = controller.signal.aborted ? 'Stopped.' : (error as Error).message;
    } finally { active = undefined; send.disabled = false; stop.hidden = true; }
  }
  async function setEnabled(enabled: boolean) {
    const version = ++visibilityVersion;
    active?.abort();
    if (state) state.enabled = enabled;
    bubble.hidden = true;
    try { const next = await api({action:'enabled', enabled}); if (version === visibilityVersion) render(next); }
    catch (error) { status.textContent = (error as Error).message; }
  }
  document.querySelector('#companion-form')!.addEventListener('submit', event => { event.preventDefault(); void talk(false); });
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); void talk(false); }
  });
  stop.addEventListener('click', () => active?.abort());
  document.querySelector('#companion-hide')!.addEventListener('click', () => { void setEnabled(false); });
  restore.addEventListener('click', () => { void setEnabled(true); });
  card.addEventListener('toggle', () => { if (state) render(state); });
  window.addEventListener('pagehide', () => active?.abort());
  document.addEventListener('visibilitychange', () => { if (!document.hidden) void refresh(); });
  window.setInterval(() => { void refresh(); }, 15_000);
  visibility();
  void refresh();
}
