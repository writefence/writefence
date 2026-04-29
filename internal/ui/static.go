package ui

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>WriteFence Operator</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #0c0f12;
      --panel: #15191e;
      --panel-2: #101418;
      --line: #28313a;
      --text: #edf2f7;
      --muted: #9aa7b3;
      --blue: #4d9fff;
      --green: #47c97f;
      --amber: #e7b84f;
      --violet: #a78bfa;
      --red: #ef6b73;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 13px/1.45 Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    .app { min-height: 100vh; display: grid; grid-template-columns: 220px minmax(0, 1fr); }
    nav { border-right: 1px solid var(--line); background: #0f1317; padding: 18px 14px; position: sticky; top: 0; height: 100vh; }
    .brand { font-size: 18px; font-weight: 700; margin-bottom: 4px; }
    .sub { color: var(--muted); font-size: 12px; margin-bottom: 20px; }
    .navbtn {
      width: 100%; display: flex; align-items: center; gap: 9px; border: 1px solid transparent;
      background: transparent; color: var(--muted); padding: 9px 10px; border-radius: 7px; cursor: pointer; text-align: left;
    }
    .navbtn.active, .navbtn:hover { background: #17202a; color: var(--text); border-color: #263342; }
    main { padding: 20px; min-width: 0; }
    .topline { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
    h1 { font-size: 22px; margin: 0; letter-spacing: 0; }
    .hint { color: var(--muted); }
    .section { margin-bottom: 18px; }
    .grid { display: grid; gap: 12px; }
    .grid.cols-4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .grid.cols-5 { grid-template-columns: repeat(5, minmax(0, 1fr)); }
    .panel { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; min-width: 0; }
    .panel.pad { padding: 14px; }
    .metric .label { color: var(--muted); font-size: 12px; text-transform: uppercase; }
    .metric .value { font-size: 28px; font-weight: 700; margin-top: 6px; }
    .strip { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
    .status { padding: 10px; border: 1px solid var(--line); border-radius: 7px; background: var(--panel-2); min-width: 0; }
    .status strong { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .status span { display: block; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .arch { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; color: #d7e3ee; white-space: pre; }
    .tablewrap { overflow-x: hidden; }
    table { width: 100%; border-collapse: collapse; table-layout: fixed; }
    th, td { border-bottom: 1px solid var(--line); padding: 9px 10px; vertical-align: top; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    th { color: var(--muted); font-weight: 600; text-align: left; background: #10151b; }
    tr:hover td { background: #111a22; }
    .events-table th:nth-child(1), .events-table td:nth-child(1) { width: 15%; }
    .events-table th:nth-child(2), .events-table td:nth-child(2) { width: 14%; }
    .events-table th:nth-child(3), .events-table td:nth-child(3) { width: 15%; }
    .events-table th:nth-child(4), .events-table td:nth-child(4) { width: 18%; }
    .events-table th:nth-child(5), .events-table td:nth-child(5) { width: 20%; }
    .events-table th:nth-child(6), .events-table td:nth-child(6) { width: 9%; }
    .events-table th:nth-child(7), .events-table td:nth-child(7) { width: 9%; }
    .quarantine-table th:nth-child(1), .quarantine-table td:nth-child(1) { width: 170px; }
    .quarantine-table th:nth-child(2), .quarantine-table td:nth-child(2) { width: 110px; }
    .quarantine-table th:nth-child(3), .quarantine-table td:nth-child(3) { width: 160px; }
    .quarantine-table th:nth-child(4), .quarantine-table td:nth-child(4) { width: 180px; }
    .quarantine-table th:nth-child(5), .quarantine-table td:nth-child(5) { width: auto; }
    .quarantine-table th:nth-child(6), .quarantine-table td:nth-child(6) { width: 120px; }
    .quarantine-table th:nth-child(7), .quarantine-table td:nth-child(7) { width: 180px; }
    .replay-table th:nth-child(1), .replay-table td:nth-child(1) { width: 12%; }
    .replay-table th:nth-child(2), .replay-table td:nth-child(2) { width: 12%; }
    .replay-table th:nth-child(3), .replay-table td:nth-child(3) { width: 12%; }
    .replay-table th:nth-child(4), .replay-table td:nth-child(4) { width: 14%; }
    .replay-table th:nth-child(5), .replay-table td:nth-child(5) { width: 20%; }
    .replay-table th:nth-child(6), .replay-table td:nth-child(6) { width: 30%; }
    code, pre, .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .decision { font-weight: 700; text-transform: uppercase; }
    .allowed { color: var(--green); }
    .warned { color: var(--amber); }
    .quarantined { color: var(--violet); }
    .blocked { color: var(--red); }
    .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
    button, select {
      border: 1px solid #344454; background: #16202a; color: var(--text); border-radius: 7px; padding: 7px 10px;
    }
    button { cursor: pointer; }
    button.primary { background: #12395f; border-color: #23659f; }
    button.danger { background: #3a171d; border-color: #70313a; }
    .layout-2 { display: grid; grid-template-columns: minmax(0, 1fr) 420px; gap: 12px; align-items: start; }
    .drawer { position: sticky; top: 20px; }
    .fix { border-left: 3px solid var(--blue); background: #101c28; padding: 10px; margin-bottom: 10px; color: #d7eaff; }
    pre { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; max-height: 460px; overflow: auto; color: #cbd7e2; }
    .empty { color: var(--muted); padding: 18px; }
    .changed td { background: rgba(77, 159, 255, 0.08); }
    .rules { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 8px; }
    .rule { padding: 10px; border: 1px solid var(--line); border-radius: 7px; background: var(--panel-2); }
    .rule .off { color: var(--muted); }
    @media (max-width: 980px) {
      .app { grid-template-columns: 1fr; }
      nav { height: auto; position: static; border-right: 0; border-bottom: 1px solid var(--line); }
      .grid.cols-4, .grid.cols-5, .strip, .layout-2 { grid-template-columns: 1fr; }
      main { padding: 14px; }
    }
  </style>
</head>
<body>
  <div class="app">
    <nav>
      <div class="brand">WriteFence</div>
      <div class="sub">Local operator/admin UI</div>
      <button class="navbtn active" data-view="overview">Overview</button>
      <button class="navbtn" data-view="violations">Violations</button>
      <button class="navbtn" data-view="quarantine">Quarantine</button>
      <button class="navbtn" data-view="replay">Replay</button>
      <button class="navbtn" data-view="config">Config</button>
    </nav>
    <main>
      <div class="topline">
        <div>
          <h1 id="title">Overview</h1>
          <div class="hint">Agent -> WriteFence :9622 -> Memory store</div>
        </div>
        <button class="primary" id="refresh">Refresh</button>
      </div>
      <div id="view"></div>
    </main>
  </div>
  <script>
    const state = { view: "overview", selected: null, violationsFilter: "all", replay: null };
    const view = document.querySelector("#view");
    const title = document.querySelector("#title");
    document.querySelectorAll(".navbtn").forEach(btn => btn.onclick = () => setView(btn.dataset.view));
    document.querySelector("#refresh").onclick = () => render();

    function setView(name) {
      state.view = name;
      state.selected = null;
      document.querySelectorAll(".navbtn").forEach(btn => btn.classList.toggle("active", btn.dataset.view === name));
      render();
    }
    async function api(path, options) {
      const res = await fetch("/_writefence/api/" + path, options || {});
      if (!res.ok) throw new Error(await res.text());
      return res.json();
    }
    function esc(value) {
      return String(value ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
    }
    function preview(entry) {
      const text = entry.doc?.text || "";
      return text.length > 110 ? text.slice(0, 110) + "..." : text;
    }
	    function decision(value) {
	      return '<span class="decision ' + esc(value) + '">' + esc(value || "-") + '</span>';
	    }
	    function row(entry, click) {
	      const attr = click ? ' onclick="selectTrace(&quot;' + esc(entry.trace_id) + '&quot;)"' : "";
	      return '<tr' + attr + '><td class="mono">' + esc(entry.ts) + '</td><td>' + decision(entry.result || entry.decision) + '</td>' +
	        '<td>' + esc(entry.rule || "-") + '</td><td>' + esc(entry.reason_code || "-") + '</td>' +
	        '<td>' + esc(preview(entry)) + '</td><td>' + esc(Boolean(entry.retryable)) + '</td>' +
	        '<td class="mono">' + esc(entry.trace_id || "-") + '</td></tr>';
	    }
    window.selectTrace = trace => { state.selected = trace; renderViolations(); };

    async function render() {
      title.textContent = state.view[0].toUpperCase() + state.view.slice(1);
      try {
        if (state.view === "overview") await renderOverview();
        if (state.view === "violations") await renderViolations();
        if (state.view === "quarantine") await renderQuarantine();
        if (state.view === "replay") await renderReplay();
        if (state.view === "config") await renderConfig();
	      } catch (err) {
	        view.innerHTML = '<div class="panel pad"><strong>Error</strong><pre>' + esc(err.message) + '</pre></div>';
	      }
	    }

    async function renderOverview() {
	      const data = await api("overview");
	      const counters = data.counters || {};
	      view.innerHTML =
	        '<div class="section strip">' + (data.status || []).map(s => '<div class="status"><strong>' + esc(s.name) + ': ' + esc(s.state) + '</strong><span>' + esc(s.detail) + '</span></div>').join("") + '</div>' +
	        '<div class="section grid cols-4">' + ["allowed","warned","quarantined","blocked"].map(k => '<div class="panel pad metric"><div class="label">' + k + '</div><div class="value ' + k + '">' + esc(counters[k] || 0) + '</div></div>').join("") + '</div>' +
	        '<div class="section panel pad"><div class="arch">Agent -> WriteFence :9622 -> Memory store\n              |-> WAL\n              |-> Quarantine</div></div>' +
	        '<div class="section panel tablewrap"><table class="events-table"><thead><tr><th>time</th><th>decision</th><th>rule</th><th>reason</th><th>preview</th><th>retryable</th><th>trace ID</th></tr></thead><tbody>' + (data.feed || []).map(e => row(e, false)).join("") + '</tbody></table></div>' +
	        '<div class="section panel pad"><div class="rules">' + (data.rules || []).map(r => '<div class="rule"><strong class="' + (r.enabled ? "" : "off") + '">' + esc(r.id) + '</strong><div class="hint">' + esc(r.detail) + '</div></div>').join("") + '</div></div>';
	    }
    async function renderViolations() {
      const data = await api("violations");
      const entries = (data.entries || []).filter(e => state.violationsFilter === "all" || e.result === state.violationsFilter);
      const selected = entries.find(e => e.trace_id === state.selected) || entries[0] || null;
      if (selected && !state.selected) state.selected = selected.trace_id;
	      view.innerHTML = '<div class="toolbar"><select id="vf"><option>all</option><option>blocked</option><option>quarantined</option><option>warned</option></select></div>' +
	        '<div class="layout-2"><div class="panel tablewrap"><table class="events-table"><thead><tr><th>time</th><th>decision</th><th>rule</th><th>reason</th><th>preview</th><th>retryable</th><th>trace ID</th></tr></thead><tbody>' + entries.map(e => row(e, true)).join("") + '</tbody></table></div>' +
	        '<aside class="panel pad drawer">' + (selected ? detail(selected) : '<div class="empty">No violations recorded.</div>') + '</aside></div>';
      document.querySelector("#vf").value = state.violationsFilter;
      document.querySelector("#vf").onchange = e => { state.violationsFilter = e.target.value; state.selected = null; renderViolations(); };
    }
    function detail(entry) {
	      const fix = entry.suggested_fix || entry.message || "No suggested fix recorded for this entry.";
	      return '<div class="fix"><strong>Suggested fix</strong><div>' + esc(fix) + '</div></div><pre>' + esc(JSON.stringify(entry, null, 2)) + '</pre>';
	    }
    async function renderQuarantine() {
	      const data = await api("quarantine");
	      const entries = data.entries || [];
	      if (!entries.length) { view.innerHTML = '<div class="panel empty">No writes need review.</div>'; return; }
	      view.innerHTML = '<div class="panel tablewrap"><table class="quarantine-table"><thead><tr><th>time</th><th>status</th><th>rule</th><th>reason</th><th>preview</th><th>trace ID</th><th>action</th></tr></thead><tbody>' + entries.map(e => {
	        const action = e.status === "pending" ? '<button onclick="q(&quot;' + esc(e.trace_id) + '&quot;,&quot;approve&quot;)">Approve</button> <button class="danger" onclick="q(&quot;' + esc(e.trace_id) + '&quot;,&quot;reject&quot;)">Reject</button>' : esc(e.reviewed_at || "");
	        return '<tr><td class="mono">' + esc(e.ts) + '</td><td>' + esc(e.status) + '</td><td>' + esc(e.rule || "-") + '</td><td>' + esc(e.reason_code || "-") + '</td><td>' + esc(preview(e)) + '</td><td class="mono">' + esc(e.trace_id) + '</td><td>' + action + '</td></tr>';
	      }).join("") + '</tbody></table></div>';
	    }
	    window.q = async (trace, action) => { await api("quarantine/" + trace + "/" + action, { method: "POST" }); renderQuarantine(); };
    async function renderReplay() {
	      if (!state.replay) state.replay = await api("replay", { method: "POST" });
	      const r = state.replay;
	      view.innerHTML = '<div class="toolbar"><button class="primary" id="runReplay">Run replay</button><span class="hint">Last run: ' + esc(r.last_run || "-") + ' | Evaluated: ' + esc(r.evaluated_count || 0) + ' | Changed: ' + esc(r.changed_decision_count || 0) + '</span></div>' +
	        '<div class="panel tablewrap"><table class="replay-table"><thead><tr><th>trace ID</th><th>old decision</th><th>new decision</th><th>rule</th><th>reason</th><th>preview</th></tr></thead><tbody>' + (r.results || []).map(x => '<tr class="' + (x.Changed || x.changed ? "changed" : "") + '"><td class="mono">' + esc(x.TraceID || x.trace_id) + '</td><td>' + decision(x.OrigDecision || x.orig_decision) + '</td><td>' + decision(x.NewDecision || x.new_decision) + '</td><td>' + esc(x.Rule || x.rule || "-") + '</td><td>' + esc(x.ReasonCode || x.reason_code || "-") + '</td><td>' + esc(x.TextPreview || x.text_preview || "") + '</td></tr>').join("") + '</tbody></table></div>';
      document.querySelector("#runReplay").onclick = async () => { state.replay = await api("replay", { method: "POST" }); renderReplay(); };
    }
    async function renderConfig() {
	      const data = await api("config");
	      view.innerHTML = '<div class="panel pad"><pre>' + esc(JSON.stringify(data, null, 2)) + '</pre></div>';
	    }
    render();
  </script>
</body>
</html>`
