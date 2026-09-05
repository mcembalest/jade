interface Terminals { apps: {name: string; path: string}[]; selected: string; overridden: boolean }

export function initTerminals(body: HTMLElement, status: HTMLElement) {
  const terminalToggle = document.querySelector<HTMLButtonElement>("#terminal-toggle")!;
  const terminalSelect = document.querySelector<HTMLSelectElement>("#terminal-select")!;
  const notice = document.querySelector<HTMLElement>('#terminal-notice')!;
  function report(message: string) { status.textContent = message; notice.hidden = !message; }
  document.querySelector<HTMLButtonElement>('#dismiss-terminal')!.addEventListener('click', () => report(''));
  const request = (url: string, options?: RequestInit) => fetch(url, {...options, signal:AbortSignal.timeout(10000)});
  function showTerminals(result: Terminals) {
    terminalSelect.replaceChildren(...result.apps.map(app => new Option(app.name, app.path)));
    terminalSelect.value = result.selected;
    terminalSelect.disabled = result.overridden;
    terminalSelect.title = result.overridden ? "Set by JADE_TERMINAL" : "Terminal app";
  }
  async function loadTerminals() {
    try {
      const response = await request("/terminals");
      if (response.ok) showTerminals(await response.json());
    } catch (_) { /* Opening still uses the engine's default. */ }
  }
  terminalSelect.addEventListener("change", async () => {
    report('');
    terminalSelect.disabled = true;
    terminalToggle.disabled = true;
    try {
      const data = new FormData(); data.set("terminal", terminalSelect.value);
      const response = await request("/terminal/preference", {method:"POST", body:data});
      const result: Terminals & {error?: string} = await response.json();
      if (!response.ok) throw new Error(result.error || "Could not save terminal preference");
      showTerminals(result);
    } catch (error) {
      report((error instanceof Error ? error.message : String(error)));
      terminalSelect.disabled = false;
      await loadTerminals();
    } finally {
      terminalToggle.disabled = false;
    }
  });
  async function openTerminal() {
    if (terminalToggle.disabled) return;
    report('');
    terminalToggle.disabled = true;
    terminalToggle.textContent = "Opening…";
    try {
      const data = new FormData(); data.set("jade", body.dataset.jade!);
      const response = await request("/terminal", {method:"POST", body:data});
      const result: {message: string; error?: string} = await response.json();
      report(response.ok ? result.message : (result.error || "Could not open terminal"));
    } catch (error) {
      report("Could not open terminal: " + (error instanceof Error ? error.message : String(error)));
    } finally {
      terminalToggle.disabled = false;
      terminalToggle.textContent = "Open terminal";
    }
  }
  terminalToggle.addEventListener("click", openTerminal);
  loadTerminals();
  return openTerminal;
}
