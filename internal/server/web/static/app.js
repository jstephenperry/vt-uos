"use strict";

const cfg = window.VTUOS || { heartbeat: 5 };
let token = localStorage.getItem("vtuos_token") || "";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s == null ? "" : s).replace(/[&<>"]/g, (c) =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

function authHeaders() {
  const h = { "Content-Type": "application/json" };
  if (token) h["X-Admin-Token"] = token;
  return h;
}

async function post(path, body) {
  const r = await fetch(path, { method: "POST", headers: authHeaders(), body: JSON.stringify(body || {}) });
  if (!r.ok) {
    const e = await r.json().catch(() => ({ error: r.statusText }));
    throw new Error(e.error || ("HTTP " + r.status));
  }
  return r.json().catch(() => ({}));
}

function setMsg(t, isErr) {
  const m = $("simMsg");
  if (!m) return;
  m.textContent = t || "";
  m.style.color = isErr ? "var(--red)" : "var(--amber)";
}

// ---- Token bar ----
$("token").value = token;
$("saveToken").onclick = () => {
  token = $("token").value.trim();
  localStorage.setItem("vtuos_token", token);
  setMsg("Operator token set.");
};

// ---- Tabs ----
document.querySelectorAll(".tab").forEach((t) => {
  t.onclick = () => {
    document.querySelectorAll(".tab").forEach((x) => x.classList.remove("active"));
    document.querySelectorAll(".panel").forEach((x) => x.classList.remove("active"));
    t.classList.add("active");
    $(t.dataset.tab).classList.add("active");
    if (t.dataset.tab === "clients") refreshClients();
    if (t.dataset.tab === "events") refreshEvents();
  };
});

// ---- Simulation controls ----
document.querySelectorAll("[data-act]").forEach((b) => {
  b.onclick = async () => {
    try {
      const act = b.dataset.act;
      if (act === "pause") await post("/api/v1/sim/pause");
      else if (act === "resume") await post("/api/v1/sim/resume");
      else if (act === "step") await post("/api/v1/sim/step", { hours: parseInt(b.dataset.hours, 10) });
      else if (act === "snapshot") { const r = await post("/api/v1/sim/snapshot"); setMsg("Snapshot saved: " + r.snapshot); return; }
      else if (act === "scale") await post("/api/v1/sim/scale", { time_scale: parseFloat($("scaleSel").value) });
      setMsg(act + " ok.");
    } catch (e) { setMsg(e.message, true); }
  };
});

// ---- Live state via SSE ----
function pct(v) { return Math.max(0, Math.min(100, v || 0)); }
function statusClass(s) { return s === "CRITICAL" ? "crit" : s === "WARNING" ? "warn" : s === "WATCH" ? "warn" : "ok"; }
function bar(v) { return `<span class="bar2"><span style="width:${pct(v)}%"></span></span>`; }

function renderState(st) {
  $("simStatus").textContent = st.status;
  $("simStatus").className = "status " + st.status;
  $("vaultTime").textContent = (st.vault_time || "").replace("T", " ").slice(0, 16);
  $("vaultName").textContent = st.vault_designation || "";

  const p = st.population || {};
  $("popCard").innerHTML =
    kv("Active", `${p.active} / ${p.capacity} (${(p.load_pct || 0).toFixed(0)}%)`) +
    kv("Births / Deaths", `${p.births} / ${p.deaths}`) +
    kv("Quarantine", p.quarantined) +
    kv("Average age", (p.average_age || 0).toFixed(1));

  const pw = st.power || {};
  const balCls = pw.balance_kw < 0 ? "crit" : (pw.reserve_pct < 10 ? "warn" : "ok");
  $("powerCard").innerHTML =
    kv("Generation", (pw.generation_kw || 0).toFixed(0) + " kW") +
    kv("Consumption", (pw.consumption_kw || 0).toFixed(0) + " kW") +
    `<div class="kv"><span class="k">Balance</span><span class="${balCls}">${(pw.balance_kw||0).toFixed(0)} kW (${(pw.reserve_pct||0).toFixed(0)}%)</span></div>`;

  const sy = st.systems || {};
  $("sysSummary").innerHTML =
    kv("Operational", sy.operational) + kv("Degraded", sy.degraded) +
    kv("Failed", sy.failed) + kv("Avg efficiency", (sy.avg_efficiency || 0).toFixed(0) + "%") +
    kv("Overdue PM", sy.overdue_maintenance);

  $("resCard").innerHTML = (st.resources || []).filter((r) => r.daily_use > 0).map((r) =>
    `<div class="kv"><span class="k">${esc(r.label)}</span><span class="${statusClass(r.status)}">${r.runway_days < 0 ? "∞" : r.runway_days + "d"} (${r.status})</span></div>`
  ).join("") || '<span class="hint">No metered consumption yet.</span>';

  $("alertsCard").innerHTML = (st.alerts || []).length === 0
    ? '<span class="ok">✓ No active alerts — all systems nominal</span>'
    : st.alerts.map((a) => `<div class="ev"><span class="${a.level === "CRITICAL" ? "crit" : "warn"}">[${a.level}]</span> ${esc(a.message)}${a.acknowledged ? ' <span class="hint">[ack]</span>' : ''}</div>`).join("");

  $("simInfo").innerHTML =
    kv("Status", st.status) + kv("Time scale", (st.time_scale || 0) + "×") +
    kv("Vault time", (st.vault_time || "").replace("T", " ").slice(0, 16)) +
    kv("Elapsed", `${st.elapsed_years}y ${st.elapsed_days % 365}d`) +
    kv("Ticks", st.tick_count);
  $("scaleSel").value = String(st.time_scale);

  const tb = document.querySelector("#sysTable tbody");
  tb.innerHTML = (st.system_list || []).map((s) => {
    const cls = s.efficiency < 50 ? "crit" : s.efficiency < 80 ? "warn" : "ok";
    return `<tr><td>${esc(s.code)}</td><td>${esc(s.name)}</td><td>${esc(s.category)}</td>
      <td class="${cls}">${esc(s.status)}</td><td>${bar(s.efficiency)} ${(s.efficiency||0).toFixed(0)}%</td></tr>`;
  }).join("");
}

function kv(k, v) { return `<div class="kv"><span class="k">${esc(k)}</span><span>${esc(v)}</span></div>`; }

function connectStream() {
  const es = new EventSource("/api/v1/stream");
  es.addEventListener("state", (e) => { try { renderState(JSON.parse(e.data)); } catch (_) {} });
  es.onopen = () => { $("conn").className = "conn on"; $("conn").textContent = "LIVE"; };
  es.onerror = () => { $("conn").className = "conn off"; $("conn").textContent = "RECONNECTING"; };
}

// ---- Events ----
async function refreshEvents() {
  try {
    const r = await fetch("/api/v1/events?limit=80");
    const evs = await r.json();
    $("eventList").innerHTML = (evs || []).map((e) => {
      const cls = e.severity === "CRITICAL" ? "crit" : e.severity === "WARNING" ? "warn" : "";
      return `<div class="ev"><span class="t">${(e.vault_time||"").replace("T"," ").slice(5,16)}</span><span class="c">${esc(e.category)}</span><span class="${cls}">${esc(e.summary)}</span></div>`;
    }).join("") || '<span class="hint">No events yet.</span>';
  } catch (_) {}
}

// ---- Clients / remote ops ----
async function refreshClients() {
  try {
    const r = await fetch("/api/v1/clients", { headers: authHeaders() });
    if (!r.ok) { document.querySelector("#cliTable tbody").innerHTML = `<tr><td colspan="7" class="warn">Operator token required to view terminals.</td></tr>`; return; }
    const cs = await r.json();
    const tb = document.querySelector("#cliTable tbody");
    tb.innerHTML = (cs || []).map((c) => {
      const state = c.online ? '<span class="ok">ONLINE</span>' : '<span class="crit">OFFLINE</span>';
      const ops = `
        <button onclick="op('${c.id}','SWITCH_VIEW')">VIEW</button>
        <button onclick="op('${c.id}','IDENTIFY')">IDENTIFY</button>
        <button onclick="op('${c.id}','SET_KIOSK')">KIOSK</button>
        <button onclick="op('${c.id}','MESSAGE')">MSG</button>
        <button onclick="op('${c.id}','REBOOT')">REBOOT</button>`;
      return `<tr><td>${esc(c.name)}</td><td>${esc(c.kind)}</td><td>${state}</td>
        <td>${esc((c.telemetry||{}).current_view||"—")}</td><td>${esc(c.address)}</td>
        <td>${(c.last_seen||"").replace("T"," ").slice(11,19)}</td><td>${ops}</td></tr>`;
    }).join("") || `<tr><td colspan="7" class="hint">No terminals connected.</td></tr>`;
  } catch (_) {}
}

window.op = async function (id, type) {
  const args = {};
  if (type === "SWITCH_VIEW") { const v = prompt("View (dashboard, population, resources, facilities, simulation, security):", "dashboard"); if (!v) return; args.view = v; }
  if (type === "MESSAGE") { const t = prompt("Message to display:"); if (!t) return; args.text = t; }
  if (type === "SET_KIOSK") { args.enabled = confirm("Enable kiosk mode? OK = enable, Cancel = disable") ? "true" : "false"; }
  try { await post(`/api/v1/clients/${id}/command`, { type, args }); setMsg(`Queued ${type} for terminal.`); refreshClients(); }
  catch (e) { setMsg(e.message, true); }
};

// ---- Boot ----
connectStream();
refreshEvents();
setInterval(() => {
  if ($("clients").classList.contains("active")) refreshClients();
  if ($("events").classList.contains("active")) refreshEvents();
}, (cfg.heartbeat || 5) * 1000);
