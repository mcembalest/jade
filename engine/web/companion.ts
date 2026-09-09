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
  type Message = { id: string; role: string; text: string; sources?: {title: string; url: string}[]; proactive?: boolean; foundAt?: number };
  type State = { messages: Message[]; enabled: boolean; next: number; seen: string; pending?: Message[]; researchNext?: number; researchChecked?: number; researchError?: string; paused?: boolean; providerStatus?: string; offline?: boolean };
  const chat = document.querySelector<HTMLElement>('#companion-chat')!;
  const input = document.querySelector<HTMLTextAreaElement>('#companion-input')!;
  const status = document.querySelector<HTMLElement>('#companion-status')!;
  const send = document.querySelector<HTMLButtonElement>('#companion-send')!;
  const stop = document.querySelector<HTMLButtonElement>('#companion-stop')!;
  const bubble = document.querySelector<HTMLButtonElement>('#companion-bubble')!;
  let state: State | undefined;
  let active: AbortController | undefined;
  const pause = document.querySelector<HTMLButtonElement>('#companion-pause')!;
  const research = document.querySelector<HTMLElement>('#companion-research')!;
  const researchStatus = document.querySelector<HTMLElement>('#companion-research-status')!;
  const researchCount = document.querySelector<HTMLElement>('#companion-research-count')!;
  let renderedResearch = '';
  let checking = false;
  let rendered = '';
  let seenPending = '';

  async function api(body?: object, signal?: AbortSignal): Promise<State> {
    const options: RequestInit = body ? {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body), signal} : {signal, headers:{'X-JaDE-Companion-Hidden':String(hidden)}};
    let response = await fetch('/companion', options);
    // Another explicit desktop chat may briefly hold the shared chat lock.
    if (response.status === 409 && (body as {action?: string})?.action === 'chat') {
      await new Promise(resolve => window.setTimeout(resolve, 250));
      response = await fetch('/companion', options);
    }
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }
  function messageRow(message: Message) {
    const row = document.createElement('div'); row.className = 'companion-message'; row.dataset.role = message.role;
    const author = document.createElement('strong'); author.textContent = message.foundAt ? new Date(message.foundAt).toLocaleString([], {month:'short', day:'numeric', hour:'numeric', minute:'2-digit'}) : message.role === 'user' ? 'You' : 'Sanjana';
    const text = document.createElement('p'); text.textContent = message.text;
    row.append(author, text);
    for (const source of message.sources || []) {
      try {
        const url = new URL(source.url);
        if (!['https:', 'http:'].includes(url.protocol)) continue;
        const link = document.createElement('a'); link.href = url.href; link.textContent = source.title || url.hostname; link.target = '_blank'; link.rel = 'noopener noreferrer'; row.append(link);
      } catch { /* Ignore malformed source links. */ }
    }
    return row;
  }
  function render(next: State) {
    state = next;
    pause.textContent = next.paused ? 'Resume research everywhere' : 'Pause research everywhere';
    if (next.offline) status.textContent = 'Offline · showing saved updates';
    const serialized = JSON.stringify(next.messages);
    if (serialized !== rendered) {
      rendered = serialized;
      chat.replaceChildren();
      for (const message of next.messages) {
        const row = messageRow(message);
        chat.append(row);
      }
      if (!next.messages.length) chat.textContent = 'Tell me what you’re in the mood for, or ask me to search.';
      chat.scrollTop = chat.scrollHeight;
    }
    const pending = next.pending || [];
    const pendingJSON = JSON.stringify(pending);
    if (pendingJSON !== renderedResearch) {
      renderedResearch = pendingJSON;
      research.replaceChildren(...pending.map(messageRow));
      if (!pending.length) research.textContent = 'No pending findings yet. New research will appear here as it is collected.';
    }
    researchCount.textContent = String(pending.length);
    researchStatus.textContent = next.paused ? 'Research and daily publication are paused on all devices.' :
      next.providerStatus ? next.providerStatus :
      next.researchError ? next.researchError :
      pending.length >= 24 ? '24 findings pending. Research resumes after the daily update.' :
      next.researchChecked ? 'Last checked ' + new Date(next.researchChecked).toLocaleString() : 'Cloudflare checks hourly, even with JaDE closed.';
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
    try {
      const next = await api();
      status.textContent = next.offline ? 'Offline · showing saved updates' : '';
      render(next);
    } catch (error) { status.textContent = (error as Error).message; }
    finally { checking = false; }
  }
  async function talk(action: 'chat') {
    if (active || hidden) return;
    const message = input.value.trim();
    if (!message) return;
    const controller = new AbortController(); active = controller;
    send.disabled = true; stop.hidden = false;
    status.textContent = 'Thinking…';
    try {
      render(await api({action, message}, controller.signal));
      if (input.value.trim() === message) input.value = '';
      status.textContent = '';
    } catch (error) {
      status.textContent = controller.signal.aborted ? 'Stopped.' : (error as Error).message;
    } finally { active = undefined; send.disabled = false; stop.hidden = true; if (state) render(state); }
  }
  pause.addEventListener('click', async () => {
    if (!state || pause.disabled) return;
    pause.disabled = true;
    try { render(await api({action:'settings',paused:!state.paused})); }
    catch (error) { status.textContent = (error as Error).message; }
    finally { pause.disabled = false; }
  });
  document.querySelector('#companion-form')!.addEventListener('submit', event => { event.preventDefault(); void talk('chat'); });
  input.addEventListener('keydown', event => {
    if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); void talk('chat'); }
  });
  stop.addEventListener('click', () => active?.abort());
  card.addEventListener('toggle', () => { if (state) render(state); if (card.matches(':popover-open')) void refresh(); });
  window.addEventListener('pagehide', () => active?.abort());
  document.addEventListener('visibilitychange', () => { if (!document.hidden) void refresh(); });
  window.setInterval(() => { void refresh(); }, 15_000);
  visibility();
  void refresh();
}
