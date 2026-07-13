// sandbox-probe reporting page. All derivation is client-side from one concatenated
// file (see ADR 0001 / docs/reporting-site-plan.md). Falls back to sample-data.json
// for local dev until the CI aggregate job produces all-reports.json.
"use strict";

const CATEGORIES = [
  { key: "fs_read", label: "FS read" },
  { key: "fs_write", label: "FS write" },
  { key: "net_egress", label: "Net egress" },
  { key: "local_services", label: "Local svc" },
  { key: "ipc_sockets", label: "IPC sockets" },
  { key: "process_visibility", label: "Proc vis" },
  { key: "host_mounts", label: "Mounts" },
  { key: "privileged", label: "Privileged" },
];
const FT2CAT = {
  sensitive_readable_paths: "fs_read",
  writeable_paths: "fs_write",
  external_host_dns_resolution: "net_egress",
  external_host_connectivity: "net_egress",
  tcp_ports_open: "local_services",
  udp_ports_open: "local_services",
  unix_socket_detection: "ipc_sockets",
  process_detection: "process_visibility",
  parent_process_detection: "process_visibility",
  mounted_volumes_detections: "host_mounts",
};
// context-only finding types (not counted); everything else unmapped -> "other" column.
const CONTEXT_FT = new Set([
  "sandbox_detection", "user_context_detection", "hostname_detection",
  "environment_detection", "proxy_detection",
]);

const find = (r, ft) => r.report.findings.find((f) => f.findingType === ft);
const kernelOf = (r) => (find(r, "environment_detection")?.value?.kernelRelease) || "?";
const sandboxOf = (r) => (find(r, "sandbox_detection")?.value) || "none";
const isRoot = (r) => (find(r, "user_context_detection")?.value?.euid) === 0;
function harnessVersion(r) {
  const skip = new Set(["os", "harness", "sandbox", "runner", "mode"]);
  for (const t of r.report.metadata?.tags || []) {
    const [k, v] = t.split("=");
    if (!skip.has(k) && v) return v;
  }
  return "";
}
const fingerprint = (r) => [harnessVersion(r), r.report.probeBinary.commit, kernelOf(r), r.os].join("|");

// A finding only signals a real capability if it carries something: a task can
// run and find nothing, emitting the finding type with an empty value (e.g. DNS
// resolution blocked -> external_host_dns_resolution: []). Empty != leaked.
function hasSignal(f) {
  const v = f.value;
  if (Array.isArray(v)) return v.length > 0;
  if (v && typeof v === "object") return Object.keys(v).length > 0;
  return v != null && v !== "";
}

// which leak categories a report exhibits (a non-empty finding of that type).
function leakedCats(r) {
  const s = new Set();
  for (const f of r.report.findings) {
    if (!hasSignal(f)) continue;
    const cat = FT2CAT[f.findingType];
    if (cat) s.add(cat);
    else if (!CONTEXT_FT.has(f.findingType)) s.add("other");
  }
  return s;
}

// state per category for one harness row, normalized against its same-run same-os baseline.
function cellStates(row, baseline) {
  const leaks = leakedCats(row);
  const achievable = baseline ? leakedCats(baseline) : new Set();
  const out = {};
  for (const { key } of CATEGORIES) {
    if (key === "privileged") { out[key] = isRoot(row) ? "leaked" : "blocked"; continue; }
    if (!baseline) out[key] = "unprovable";
    else if (!achievable.has(key)) out[key] = "na";
    else out[key] = leaks.has(key) ? "leaked" : "blocked";
  }
  if (leaks.has("other")) out.other = "leaked";
  return out;
}

// ── build model: identities -> ordered collapsed points ────────────────────────
function build(rows) {
  // index baselines by os|runTimestamp
  const baselines = {};
  for (const r of rows) if (r.harness === "direct") baselines[`${r.os}|${r.runTimestamp}`] = r;

  const byIdentity = {};
  for (const r of rows) {
    const id = `${r.os}/${r.harness}`;
    (byIdentity[id] ||= []).push(r);
  }

  const identities = {};
  for (const [id, list] of Object.entries(byIdentity)) {
    list.sort((a, b) => a.runTimestamp.localeCompare(b.runTimestamp));
    // collapse by fingerprint: one point per distinct fingerprint, latest run wins, first-seen ts.
    const points = new Map(); // fp -> point
    for (const r of list) {
      const fp = fingerprint(r);
      const base = baselines[`${r.os}|${r.runTimestamp}`];
      const pt = {
        fp, ts: r.runTimestamp, harnessVersion: harnessVersion(r),
        probe: r.report.probeBinary.commit, kernel: kernelOf(r), os: r.os,
        sandbox: sandboxOf(r), root: isRoot(r), row: r,
        states: cellStates(r, base), hasBaseline: !!base,
      };
      if (!points.has(fp)) points.set(fp, pt);
      else { const p = points.get(fp); p.states = pt.states; p.row = r; } // latest wins, keep first ts
    }
    const arr = [...points.values()].sort((a, b) => a.ts.localeCompare(b.ts));
    for (const p of arr) p.exposure = CATEGORIES.filter((c) => p.states[c.key] === "leaked").length;
    identities[id] = arr;
  }
  return identities;
}

// ── flips ──────────────────────────────────────────────────────────────────────
function flips(identities) {
  const out = [];
  for (const [id, pts] of Object.entries(identities)) {
    if (id.endsWith("/direct")) continue;
    for (let i = 1; i < pts.length; i++) {
      const a = pts[i - 1], b = pts[i];
      const moved = [];
      if (a.harnessVersion !== b.harnessVersion) moved.push(`harness ${a.harnessVersion || "?"}→${b.harnessVersion || "?"}`);
      if (a.probe !== b.probe) moved.push(`probe ${a.probe}→${b.probe}`);
      if (a.kernel !== b.kernel) moved.push(`kernel ${a.kernel}→${b.kernel}`);
      const cause = moved.join(" · ") || "no config change";
      for (const c of CATEGORIES) {
        const s0 = a.states[c.key], s1 = b.states[c.key];
        if ((s0 === "leaked" || s0 === "blocked") && (s1 === "leaked" || s1 === "blocked") && s0 !== s1)
          out.push({ id, ts: b.ts, cat: c.label, from: s0, to: s1, cause, degraded: s1 === "leaked" });
      }
    }
  }
  return out.sort((a, b) => b.ts.localeCompare(a.ts));
}

// ── rendering ───────────────────────────────────────────────────────────────────
const GLYPH = { leaked: "●", blocked: "●", na: "—", unprovable: "?" };
let MODEL, OSFILTER = "";

function renderMeta(rows) {
  const runs = new Set(rows.map((r) => r.runTimestamp));
  const harnesses = new Set(rows.map((r) => r.harness).filter((h) => h !== "direct"));
  document.getElementById("meta").textContent =
    `${rows.length} reports · ${harnesses.size} harnesses · ${runs.size} runs · ${[...runs].sort().at(-1)?.slice(0, 10)} latest`;
}

function renderMatrix() {
  const ids = Object.keys(MODEL).filter((id) => !OSFILTER || id.startsWith(OSFILTER + "/")).sort();
  let h = "<table><thead><tr><th>identity</th><th>enforcement</th>";
  for (const c of CATEGORIES) h += `<th>${c.label}</th>`;
  h += "<th>exp.</th></tr></thead><tbody>";
  for (const id of ids) {
    const pts = MODEL[id], p = pts.at(-1), prev = pts.length > 1 ? pts.at(-2) : null;
    const baseline = id.endsWith("/direct");
    h += `<tr class="${baseline ? "baseline-row" : ""}"><td class="id">${id}${baseline ? ' <span class="tag">baseline</span>' : ""}</td>`;
    h += `<td>${baseline ? "—" : `<span class="tag enf">${p.sandbox}</span>${p.root ? ' <span class="tag root">root</span>' : ""}`}</td>`;
    for (const c of CATEGORIES) {
      const st = p.states[c.key] || "na";
      const changed = prev && !baseline && prev.states[c.key] !== st && (st === "leaked" || st === "blocked");
      const arrow = changed ? (st === "leaked" ? '<span class="up">▲</span>' : '<span class="down">▼</span>') : "";
      h += `<td class="cell ${st}" data-id="${id}" data-cat="${c.key}" title="${st}">${GLYPH[st] || ""}${arrow}</td>`;
    }
    h += `<td class="exp">${baseline ? "—" : p.exposure}</td></tr>`;
  }
  h += "</tbody></table>";
  document.getElementById("matrix").innerHTML = h;
  document.querySelectorAll("#matrix .cell").forEach((td) =>
    td.addEventListener("click", () => drill(td.dataset.id, td.dataset.cat)));
}

function drill(id, catKey) {
  const p = MODEL[id].at(-1);
  const cat = CATEGORIES.find((c) => c.key === catKey);
  const fts = Object.entries(FT2CAT).filter(([, v]) => v === catKey).map(([k]) => k);
  const items = [];
  for (const ft of fts) {
    const f = find(p.row, ft);
    if (f) items.push(...(Array.isArray(f.value) ? f.value : [f.value]).map((v) => typeof v === "object" ? JSON.stringify(v) : v));
  }
  document.getElementById("drill-title").textContent = `${id} · ${cat.label} · ${p.states[catKey]}`;
  document.getElementById("drill-body").innerHTML =
    `<div class="fp">fingerprint: ${p.harnessVersion || "—"} · probe ${p.probe} · ${p.kernel}</div>` +
    (items.length ? "<ul>" + items.map((i) => `<li>${i}</li>`).join("") + "</ul>"
      : `<p class="muted">No accessible items (${p.states[catKey]}).</p>`);
  document.getElementById("drill").classList.remove("hidden");
}

function renderFlips() {
  const fl = flips(MODEL);
  document.getElementById("flips").innerHTML = fl.length
    ? "<ul class='flip-list'>" + fl.map((f) =>
        `<li class="${f.degraded ? "deg" : "imp"}"><span class="badge">${f.degraded ? "▲ regression" : "▼ improvement"}</span>
         <code>${f.id}</code> — <b>${f.cat}</b> ${f.from}→${f.to}
         <span class="muted">@ ${f.ts.slice(0, 10)} · ${f.cause}</span></li>`).join("") + "</ul>"
    : "<p class='muted'>No flips yet.</p>";
}

function renderCharts() {
  // Chart A — exposure over time, one line per non-baseline identity.
  const ex = echarts.init(document.getElementById("chart-exposure"));
  const series = [];
  const versionMarks = [];
  for (const [id, pts] of Object.entries(MODEL)) {
    if (id.endsWith("/direct")) continue;
    series.push({
      name: id, type: "line", step: "end", showSymbol: true, symbolSize: 6,
      data: pts.map((p) => [p.ts, p.exposure]),
    });
    for (let i = 1; i < pts.length; i++)
      if (pts[i].harnessVersion && pts[i].harnessVersion !== pts[i - 1].harnessVersion)
        versionMarks.push({ xAxis: pts[i].ts, label: { formatter: `${id.split("/")[1]} ${pts[i].harnessVersion}`, rotate: 90, fontSize: 9 } });
  }
  if (series.length) series[0].markLine = { symbol: "none", silent: true, lineStyle: { type: "dashed", color: "#999" }, data: versionMarks };
  ex.setOption({
    tooltip: { trigger: "axis" },
    legend: { type: "scroll", top: 0 },
    grid: { top: 40, left: 40, right: 20, bottom: 30 },
    xAxis: { type: "time" },
    yAxis: { type: "value", name: "leaked cats", min: 0, max: 8, minInterval: 1 },
    series,
  });

  renderHeatmap();
  window.addEventListener("resize", () => { ex.resize(); HM && HM.resize(); });
}

let HM;
function renderHeatmap() {
  const id = document.getElementById("hm-identity").value;
  const pts = MODEL[id] || [];
  HM = HM || echarts.init(document.getElementById("chart-heatmap"));
  const val = { leaked: 2, blocked: 1, na: 0, unprovable: 0 };
  const data = [];
  pts.forEach((p, x) => CATEGORIES.forEach((c, y) => data.push([x, y, val[p.states[c.key]] ?? 0])));
  HM.setOption({
    tooltip: { formatter: (o) => `${CATEGORIES[o.value[1]].label} @ ${pts[o.value[0]].ts.slice(0, 10)}: ${["n/a", "blocked", "leaked"][o.value[2]]}` },
    grid: { top: 10, left: 90, right: 20, bottom: 60 },
    xAxis: { type: "category", data: pts.map((p) => p.ts.slice(5, 10)), axisLabel: { rotate: 45 } },
    yAxis: { type: "category", data: CATEGORIES.map((c) => c.label) },
    visualMap: {
      type: "piecewise", show: true, orient: "horizontal", bottom: 0, left: "center",
      pieces: [{ value: 0, label: "n/a", color: "#d9d9d9" }, { value: 1, label: "blocked", color: "#3f8f4f" }, { value: 2, label: "leaked", color: "#c0392b" }],
    },
    series: [{ type: "heatmap", data, label: { show: false } }],
  }, true);
}

// ── boot ─────────────────────────────────────────────────────────────────────────
async function boot() {
  let rows;
  for (const src of ["all-reports.json", "sample-data.json"]) {
    try { const res = await fetch(src); if (res.ok) { rows = await res.json(); break; } } catch { /* next */ }
  }
  if (!rows) { document.getElementById("matrix").textContent = "No data."; return; }

  MODEL = build(rows);
  renderMeta(rows);

  // OS filter options
  const oses = [...new Set(rows.map((r) => r.os))].sort();
  const osSel = document.getElementById("os-filter");
  for (const o of oses) osSel.add(new Option(o, o));
  osSel.addEventListener("change", (e) => { OSFILTER = e.target.value; renderMatrix(); });

  // heatmap identity options (non-baseline)
  const hmSel = document.getElementById("hm-identity");
  const nonBaseline = Object.keys(MODEL).filter((id) => !id.endsWith("/direct")).sort();
  for (const id of nonBaseline) hmSel.add(new Option(id, id));
  hmSel.value = nonBaseline.find((id) => id.includes("claude")) || nonBaseline[0];
  hmSel.addEventListener("change", renderHeatmap);

  renderMatrix();
  renderFlips();
  renderCharts();

  document.getElementById("drill-close").addEventListener("click", () =>
    document.getElementById("drill").classList.add("hidden"));
}
boot();
