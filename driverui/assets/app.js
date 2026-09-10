const application = document.querySelector("#application");
const driverStatus = document.querySelector("#driver-status");
const sessionMatch = window.location.pathname.match(/^\/sessions\/(\d+)$/);
const workbenchMatch = window.location.pathname.match(/^\/workbench\/sessions\/(\d+)$/);

function escapeHTML(value) {
  const element = document.createElement("span");
  element.textContent = String(value);
  return element.innerHTML;
}

function setDriverStatus(state) {
  driverStatus.textContent = state === "running" ? "Driver online" : state;
  driverStatus.className = `status ${state === "running" ? "online" : "offline"}`;
}

function empty(message) {
  return `<div class="empty"><span class="empty-mark">⌁</span><p>${message}</p></div>`;
}

async function loadState() {
  const response = await fetch("/api/state", { cache: "no-store" });
  if (!response.ok) throw new Error(`state request failed: ${response.status}`);
  return response.json();
}

async function loadCapabilities() {
  const response = await fetch("/api/capabilities", { cache: "no-store" });
  if (!response.ok) throw new Error(`capabilities request failed: ${response.status}`);
  return response.json();
}

function renderIndex(state, capabilities) {
  setDriverStatus(state.state);
  const connectionRows = state.connections.map(connection => `
    <tr>
      <td><span class="identifier">${connection.id}</span></td>
      <td>${escapeHTML(connection.protocol)}</td>
      <td><code>${escapeHTML(connection.address)}</code></td>
    </tr>`).join("");
  const sessionRows = state.sessions.map(session => `
    <tr>
      <td><a class="session-link" href="/sessions/${session.id}">Session ${session.id}</a>${capabilities.workbench ? `<a class="workbench-link" href="/workbench/sessions/${session.id}">Workbench</a>` : ""}</td>
      <td>${session.connectionId}</td>
      <td>${escapeHTML(session.terminal)}</td>
      <td>${session.width} × ${session.height}</td>
      <td>${session.revision}</td>
      <td><span class="state ${escapeHTML(session.state)}">${escapeHTML(session.state)}</span></td>
    </tr>`).join("");
  application.innerHTML = `
    <section class="intro">
      <p class="eyebrow">Operational overview</p>
      <h1>Terminal sessions at a glance.</h1>
      <p>Read-only, live visibility into the connections managed by this driver.</p>
    </section>
    <section class="panel">
      <div class="panel-heading"><h2>Sessions</h2><span>${state.sessions.length} registered</span></div>
      ${sessionRows ? `<div class="table-wrap"><table><thead><tr><th>Session</th><th>Connection</th><th>Terminal</th><th>Size</th><th>Revision</th><th>State</th></tr></thead><tbody>${sessionRows}</tbody></table></div>` : empty("No sessions are currently registered.")}
    </section>
    <section class="panel">
      <div class="panel-heading"><h2>Connections</h2><span>${state.connections.length} open</span></div>
      ${connectionRows ? `<div class="table-wrap"><table><thead><tr><th>ID</th><th>Protocol</th><th>Address</th></tr></thead><tbody>${connectionRows}</tbody></table></div>` : empty("No connections are currently open.")}
    </section>`;
}

async function startIndex() {
  const refresh = async () => {
    try {
      const [state, capabilities] = await Promise.all([loadState(), loadCapabilities()]);
      renderIndex(state, capabilities);
    } catch (error) {
      setDriverStatus("Disconnected");
      application.innerHTML = empty("The driver state is unavailable.");
    }
  };
  await refresh();
  window.setInterval(refresh, 2000);
}

async function startSession(id) {
  let metadata;
  try {
    const state = await loadState();
    setDriverStatus(state.state);
    metadata = state.sessions.find(session => session.id === Number(id));
  } catch (_) {
    setDriverStatus("Disconnected");
  }
  if (!metadata) {
    application.innerHTML = empty("This session is no longer registered.");
    return;
  }
  application.innerHTML = `
    <div class="page-actions"><a class="back" href="/">← All sessions</a><a id="open-workbench" class="button-link hidden" href="/workbench/sessions/${id}">Open Workbench</a></div>
    <section class="session-heading">
      <div><p class="eyebrow">Live terminal</p><h1>Session ${id}</h1></div>
      <dl><div><dt>Terminal</dt><dd>${escapeHTML(metadata.terminal)}</dd></div><div><dt>Size</dt><dd id="size">${metadata.width} × ${metadata.height}</dd></div><div><dt>Revision</dt><dd id="revision">${metadata.revision}</dd></div><div><dt>Stream</dt><dd id="stream-state" class="online-text">Connecting</dd></div></dl>
    </section>
    <section class="terminal-frame" aria-label="Session ${id} terminal screen"><pre id="screen">Waiting for screen…</pre></section>`;

  const streamState = document.querySelector("#stream-state");
  try {
    if ((await loadCapabilities()).workbench) document.querySelector("#open-workbench").classList.remove("hidden");
  } catch (_) {}
  const events = new EventSource(`/api/sessions/${id}/screens`);
  events.addEventListener("screen", event => {
    const screen = JSON.parse(event.data);
    document.querySelector("#screen").textContent = screen.text;
    document.querySelector("#size").textContent = `${screen.width} × ${screen.height}`;
    document.querySelector("#revision").textContent = screen.revision;
    streamState.textContent = "Connected";
    streamState.className = "online-text";
  });
  events.addEventListener("end", () => {
    streamState.textContent = "Ended";
    streamState.className = "offline-text";
    events.close();
  });
  events.onerror = () => {
    streamState.textContent = "Reconnecting";
    streamState.className = "offline-text";
  };
}

async function postWorkbench(id, operation, body = {}) {
  const response = await fetch(`/api/workbench/sessions/${id}/${operation}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || `Workbench request failed: ${response.status}`);
  return result;
}

function describeStep(step) {
  if (step.action === "waitForText") return `<strong>Wait for text</strong><code>${escapeHTML(step.text)}</code>`;
  if (step.action === "type" && step.parameter) return `<strong>Type parameter</strong><code>${escapeHTML(step.parameter)}</code>`;
  if (step.action === "type") return `<strong>Type text</strong><code>${escapeHTML(step.text)}</code>`;
  const modifiers = step.modifiers?.length ? `${step.modifiers.join("+")}+` : "";
  return `<strong>Press key</strong><code>${escapeHTML(modifiers + step.key)}</code>`;
}

function renderRecording(result) {
  const recording = result.recording;
  const timeline = document.querySelector("#timeline");
  timeline.innerHTML = recording.steps.length
    ? recording.steps.map((step, index) => `<li><span>${index + 1}</span><div>${describeStep(step)}</div></li>`).join("")
    : `<li class="timeline-empty">Actions and assertions will appear here.</li>`;
  document.querySelector("#recording-state").textContent = result.active ? "Recording" : "Stopped";
  document.querySelector("#recording-state").className = result.active ? "recording" : "stopped";
  document.querySelectorAll("[data-requires-recording]").forEach(element => { element.disabled = !result.active; });
  document.querySelector("#start-recording").disabled = result.active;
  document.querySelector("#replay-recording").disabled = result.active || recording.steps.length === 0;
  document.querySelector("#download-recording").disabled = recording.steps.length === 0;
  window.currentRecording = recording;
}

function setWorkbenchMessage(message, error = false) {
  const element = document.querySelector("#workbench-message");
  element.textContent = message;
  element.className = error ? "message error" : "message";
}

async function startWorkbench(id) {
  const state = await loadState();
  const metadata = state.sessions.find(session => session.id === Number(id));
  if (!metadata) {
    application.innerHTML = empty("This session is no longer registered.");
    return;
  }
  setDriverStatus(state.state);
  application.className = "workbench-page";
  application.innerHTML = `
    <div class="page-actions"><a class="back" href="/sessions/${id}">← Session details</a><span id="recording-state" class="stopped">Not recording</span></div>
    <section class="workbench-heading"><div><p class="eyebrow">Test authoring</p><h1>TUICast Workbench</h1><p>Session ${id} · ${escapeHTML(metadata.terminal)} · ${metadata.width} × ${metadata.height}</p></div><div class="toolbar"><button id="start-recording" class="primary">Start recording</button><button id="stop-recording" data-requires-recording disabled>Stop</button><button id="replay-recording" disabled>Replay</button><button id="download-recording" disabled>Download JSON</button></div></section>
    <p id="workbench-message" class="message">Start recording to take exclusive control of this session.</p>
    <div class="workbench-grid">
      <section>
        <div class="terminal-frame workbench-terminal"><pre id="screen">Waiting for screen…</pre></div>
        <form id="type-form" class="control-card">
          <label for="type-text">Terminal input</label>
          <div class="input-row"><input id="type-text" autocomplete="off" placeholder="Text to type" data-requires-recording disabled><button class="primary" data-requires-recording disabled>Send</button></div>
          <div class="parameter-row"><label><input id="parameterize" type="checkbox" data-requires-recording disabled> Store as parameter</label><input id="parameter-name" placeholder="parameter-name" data-requires-recording disabled></div>
        </form>
        <div class="control-card"><span class="control-label">Named keys</span><div id="keypad" class="keypad">${["Enter", "Tab", "Escape", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "F1", "F2", "F3", "F4"].map(key => `<button data-key="${key}" data-requires-recording disabled>${key}</button>`).join("")}</div></div>
        <form id="assert-form" class="control-card"><label for="assert-text">Add assertion from the current screen</label><div class="input-row"><input id="assert-text" placeholder="Visible text" data-requires-recording disabled><button data-requires-recording disabled>Add wait</button></div></form>
      </section>
      <aside class="timeline-card"><div class="timeline-heading"><h2>Recording</h2><span>Trace v1</span></div><ol id="timeline"><li class="timeline-empty">Actions and assertions will appear here.</li></ol></aside>
    </div>`;

  const events = new EventSource(`/api/sessions/${id}/screens`);
  events.addEventListener("screen", event => { document.querySelector("#screen").textContent = JSON.parse(event.data).text; });
  events.addEventListener("end", () => { events.close(); setWorkbenchMessage("The terminal session ended.", true); });

  document.querySelector("#start-recording").onclick = async () => {
    try { renderRecording(await postWorkbench(id, "recording/start")); setWorkbenchMessage("Recording started. SDK input is paused until you stop."); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#stop-recording").onclick = async () => {
    try { renderRecording(await postWorkbench(id, "recording/stop")); setWorkbenchMessage("Recording stopped and session control released."); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#type-form").onsubmit = async event => {
    event.preventDefault();
    const input = document.querySelector("#type-text");
    const parameter = document.querySelector("#parameterize").checked ? document.querySelector("#parameter-name").value : "";
    try { renderRecording(await postWorkbench(id, "type", { text: input.value, parameter })); input.value = ""; setWorkbenchMessage("Input sent and recorded."); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#keypad").onclick = async event => {
    if (!event.target.dataset.key) return;
    try { renderRecording(await postWorkbench(id, "press", { key: event.target.dataset.key })); setWorkbenchMessage(`${event.target.dataset.key} sent and recorded.`); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#assert-form").onsubmit = async event => {
    event.preventDefault();
    const input = document.querySelector("#assert-text");
    try { renderRecording(await postWorkbench(id, "assert", { text: input.value, timeoutMilliseconds: 30000 })); input.value = ""; setWorkbenchMessage("Screen assertion added."); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#replay-recording").onclick = async () => {
    const names = [...new Set(window.currentRecording.steps.filter(step => step.parameter).map(step => step.parameter))];
    const parameters = {};
    for (const name of names) {
      const value = window.prompt(`Value for ${name}`);
      if (value === null) return;
      parameters[name] = value;
    }
    try { await postWorkbench(id, "replay", { parameters }); setWorkbenchMessage("Recording replayed successfully."); }
    catch (error) { setWorkbenchMessage(error.message, true); }
  };
  document.querySelector("#download-recording").onclick = () => {
    const blob = new Blob([JSON.stringify(window.currentRecording, null, 2) + "\n"], { type: "application/json" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob); link.download = `tuicast-session-${id}.json`; link.click(); URL.revokeObjectURL(link.href);
  };
  try {
    const response = await fetch(`/api/workbench/sessions/${id}/recording`);
    if (response.ok) {
      const result = await response.json();
      renderRecording(result);
      if (result.active) setWorkbenchMessage("Recording active. SDK input is paused until you stop.");
    }
  } catch (_) {}
}

if (workbenchMatch) {
  startWorkbench(workbenchMatch[1]).catch(error => { application.innerHTML = empty(escapeHTML(error.message)); });
} else if (sessionMatch) {
  startSession(sessionMatch[1]);
} else {
  startIndex();
}
