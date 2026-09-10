const application = document.querySelector("#application");
const driverStatus = document.querySelector("#driver-status");
const sessionMatch = window.location.pathname.match(/^\/sessions\/(\d+)$/);

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

function renderIndex(state) {
  setDriverStatus(state.state);
  const connectionRows = state.connections.map(connection => `
    <tr>
      <td><span class="identifier">${connection.id}</span></td>
      <td>${escapeHTML(connection.protocol)}</td>
      <td><code>${escapeHTML(connection.address)}</code></td>
    </tr>`).join("");
  const sessionRows = state.sessions.map(session => `
    <tr>
      <td><a class="session-link" href="/sessions/${session.id}">Session ${session.id}</a></td>
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
      renderIndex(await loadState());
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
    <a class="back" href="/">← All sessions</a>
    <section class="session-heading">
      <div><p class="eyebrow">Live terminal</p><h1>Session ${id}</h1></div>
      <dl><div><dt>Terminal</dt><dd>${escapeHTML(metadata.terminal)}</dd></div><div><dt>Size</dt><dd id="size">${metadata.width} × ${metadata.height}</dd></div><div><dt>Revision</dt><dd id="revision">${metadata.revision}</dd></div><div><dt>Stream</dt><dd id="stream-state" class="online-text">Connecting</dd></div></dl>
    </section>
    <section class="terminal-frame" aria-label="Session ${id} terminal screen"><pre id="screen">Waiting for screen…</pre></section>`;

  const streamState = document.querySelector("#stream-state");
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

if (sessionMatch) {
  startSession(sessionMatch[1]);
} else {
  startIndex();
}
