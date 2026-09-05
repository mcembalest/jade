export function initTerminals(body, status) {
  const terminalToggle = document.querySelector("#terminal-toggle");
  const terminalSelect = document.querySelector("#terminal-select");
  const notice = document.querySelector('#terminal-notice');
  function report(message) { status.textContent = message; notice.hidden = !message; }
  document.querySelector('#dismiss-terminal').addEventListener('click', () => report(''));
  const request = (url, options) => fetch(url, {...options, signal:AbortSignal.timeout(10000)});
  function showTerminals(result) {
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
      const result = await response.json();
      if (!response.ok) throw new Error(result.error || "Could not save terminal preference");
      showTerminals(result);
    } catch (error) {
      report(error.message);
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
      const data = new FormData(); data.set("jade", body.dataset.jade);
      const response = await request("/terminal", {method:"POST", body:data});
      const result = await response.json();
      report(response.ok ? result.message : (result.error || "Could not open terminal"));
    } catch (error) {
      report("Could not open terminal: " + error.message);
    } finally {
      terminalToggle.disabled = false;
      terminalToggle.textContent = "Open terminal";
    }
  }
  terminalToggle.addEventListener("click", openTerminal);
  loadTerminals();
  return openTerminal;
}
