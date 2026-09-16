"use strict";
const $ = (s, root = document) => root.querySelector(s);
const esc = (v) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const money = (n) =>
  new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  }).format((n || 0) / 1e6);
const num = (n) => new Intl.NumberFormat("en-US").format(n || 0);
const time = (n) =>
  new Date(n).toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
const title = (s) => s.replace(/_/g, " ").replace(/^./, (c) => c.toUpperCase());
const views = {
  proof: "Live integration proof",
  intake: "Partner intake",
  overview: "Overview",
  campaigns: "Campaigns",
  ledger: "Delivery ledger",
  recovery: "Recovery queue",
  lab: "Failure lab",
  architecture: "Architecture",
};
let state = null,
  runtime = {},
  active = "overview",
  filter = "all",
  search = "",
  busy = false,
  toastTimer,
  refreshing = false,
  livePaused = false,
  lastUpdated = null,
  connectionLost = false,
  ledgerPage = 0,
  ledgerSort = "newest";
async function request(path, body, headers = {}) {
  const response = await fetch(path, {
    signal: AbortSignal.timeout(12000),
    method: body === undefined ? "GET" : "POST",
    headers: {
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json().catch(() => null);
  if (!response.ok)
    throw new Error(
      data?.detail || `Request failed (${response.status}). Please try again.`,
    );
  if (data === null)
    throw new Error(
      "The server returned an unreadable response. Please retry.",
    );
  return data;
}
function toast(message, error = false) {
  if ($("#detail-dialog").open) {
    $("#dialog-notice").textContent = message;
    $("#dialog-notice").className = error ? "form-error" : "";
    return;
  }
  const el = $("#toast");
  el.textContent = message;
  el.className = `toast show${error ? " error" : ""}`;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("show"), 8000);
}
async function refresh() {
  if (refreshing) return;
  refreshing = true;
  try {
    [state, runtime] = await Promise.all([
      request("/api/v1/snapshot"),
      request("/api/v1/runtime"),
    ]);
    $("#connection-label").textContent = "Engine connected";
    $("#connection-dot").classList.remove("bad");
    $("#storage-label").textContent =
      runtime.storage +
      " · " +
      (runtime.transport === "pubsub" ? "GCP Pub/Sub" : "Local transport");
    $("#recovery-badge").textContent = num(
      (state.counts.quarantined || 0) + (state.counts.failed || 0),
    );
    lastUpdated = new Date();
    connectionLost = false;
    freshness();
    render();
  } catch (e) {
    connectionLost = true;
    freshness();
    $("#connection-label").textContent = "Connection lost";
    $("#connection-dot").classList.add("bad");
    if (!state)
      $("#main").innerHTML =
        `<div class="error-banner">${esc(e.message)}. Start the Go service and refresh. Protected mode requires API credentials; this console is intended for local demo mode.</div>`;
  } finally {
    refreshing = false;
  }
}
function freshness() {
  const banner = $("#freshness-banner");
  banner.hidden = !connectionLost && !livePaused;
  banner.textContent = `${connectionLost ? "Connection interrupted. Showing the last successful snapshot." : livePaused ? "Automatic updates paused. The engine continues processing; refresh manually or resume updates." : ""}${lastUpdated && (connectionLost || livePaused) ? ` Last updated ${lastUpdated.toLocaleTimeString()}.` : ""}`;
  $("#refresh").title = lastUpdated
    ? `Refresh data · last updated ${lastUpdated.toLocaleTimeString()}`
    : "Refresh data";
}
$("#live-toggle").addEventListener("click", () => {
  livePaused = !livePaused;
  $("#live-toggle").setAttribute("aria-pressed", String(livePaused));
  $("#live-toggle").innerHTML = livePaused
    ? "▶ <span>Resume updates</span>"
    : "Ⅱ <span>Pause updates</span>";
  $("#live-toggle").title = livePaused
    ? "Resume automatic updates"
    : "Pause automatic updates";
  $("#live-toggle").setAttribute("aria-label", $("#live-toggle").title);
  freshness();
  if (!livePaused) refresh();
});
document.addEventListener("change", (e) => {
  if (e.target.id === "ledger-sort") {
    ledgerSort = e.target.value;
    ledgerPage = 0;
    render();
  }
});
document.addEventListener("click", (e) => {
  const page = e.target.closest("[data-ledger-page]");
  if (page && !page.disabled) {
    ledgerPage += page.dataset.ledgerPage === "next" ? 1 : -1;
    render();
  }
  if (e.target.closest("[data-clear-filters]")) {
    search = "";
    filter = "all";
    ledgerPage = 0;
    render();
    $("#ledger-search")?.focus();
  }
  if (e.target.closest("[data-export-receipts]")) {
    const report = {
      scope: "Matches within the latest 100 receipts, not the full ledger",
      observed_at: lastUpdated?.toISOString(),
      filter,
      search,
      receipts: filtered(),
    };
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(report, null, 2)], { type: "application/json" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "afterglow-receipts.json";
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    toast(`Exported ${report.receipts.length} matching receipts.`);
  }
});
function badge(status) {
  return `<span class="badge ${esc(status)}">${status === "settled" ? "✓ " : status === "duplicate" ? "↪ " : status === "quarantined" || status === "failed" ? "! " : ""}${esc(title(status))}</span>`;
}
function button(text, action, primary = false, extra = "") {
  return `<button class="button${primary ? " primary" : ""}" data-action="${action}" ${extra}>${text}</button>`;
}
function heading(
  name,
  description,
  actions = "",
  eyebrow = "DELIVERY INTELLIGENCE",
) {
  return `<div class="page-header"><div><div class="eyebrow">${eyebrow}</div><h1>${name}</h1><p>${description}</p></div><div class="actions">${actions}</div></div>`;
}
function total(field) {
  return state.campaigns.reduce((n, c) => n + c[field], 0);
}
function progress(c) {
  return `<div class="budget-progress" title="Settled ${money(c.spent_micros)} · Held ${money(c.reserved_micros)}"><i class="spent" style="width:${(c.spent_micros / c.budget_micros) * 100}%"></i><i class="held" style="width:${(c.reserved_micros / c.budget_micros) * 100}%"></i></div>`;
}
function metric(label, value, note, icon, amber = false) {
  return `<article class="metric"><div class="metric-name">${label}<span class="metric-icon">${icon}</span></div><div class="metric-value">${value}</div><div class="metric-note${amber ? " amber" : ""}">${note}</div></article>`;
}
function metrics() {
  return `<div class="metrics">${metric("Settled spend", money(total("spent_micros")), `<strong>✓</strong> Backed by matched playback receipts`, "↗")}${metric("Verified plays", num(state.counts.settled), "One settled reservation per play", "◷")}${metric("Duplicates suppressed", num(state.counts.duplicate), "<strong>$0.00</strong> Additional spend from duplicate plays", "⧉")}${metric("Needs attention", num((state.counts.failed || 0) + (state.counts.quarantined || 0)), `${num(state.counts.accepted)} receipts awaiting a decision`, "◌", true)}</div>`;
}
const point = (lng, lat) => [
  ((lng + 126) / 60) * 590 + 15,
  ((50 - lat) / 26) * 255 + 20,
];
function network() {
  const pts = state.screens.map((s) => ({ ...s, p: point(s.lng, s.lat) }));
  const outline = [
    [-124.7, 48.5],
    [-123, 48.3],
    [-122.9, 49],
    [-95, 49],
    [-95, 48],
    [-89, 48],
    [-85, 46.7],
    [-83, 46],
    [-82.3, 43],
    [-79.8, 43.2],
    [-76.5, 44],
    [-71.5, 45],
    [-70, 47],
    [-67, 45],
    [-69.8, 43],
    [-70.8, 41.5],
    [-74, 40.5],
    [-75.5, 38],
    [-75.7, 35.5],
    [-78, 34],
    [-80.5, 32],
    [-81, 29],
    [-80.2, 25.5],
    [-81, 25],
    [-82, 27],
    [-82.6, 28],
    [-83, 29.5],
    [-85, 30],
    [-87.5, 30.2],
    [-89.7, 29.3],
    [-91, 29.4],
    [-93.8, 29.8],
    [-96.5, 28],
    [-97.1, 26],
    [-99, 26.5],
    [-101, 29],
    [-103, 29],
    [-106.5, 32],
    [-111, 31.3],
    [-114.7, 32.7],
    [-117, 32.5],
    [-118.5, 34],
    [-120.5, 35],
    [-122.5, 38],
    [-124.3, 42],
    [-124, 46],
  ];
  const path =
    outline
      .map((p, i) => `${i ? "L" : "M"}${point(...p).join(",")}`)
      .join(" ") + "Z";
  const lines = pts
    .slice(1)
    .map(
      (s) =>
        `<path class="map-line" d="M${pts[0].p} Q${(pts[0].p[0] + s.p[0]) / 2},${Math.min(pts[0].p[1], s.p[1]) - 40} ${s.p}"/>`,
    )
    .join("");
  return `<section class="panel network-panel"><div class="panel-header"><div><h2>Demo advertising screens</h2><p>Playback reconciliation across the demo network</p></div><span class="panel-tag">${pts.length} SCREENS / US</span></div><div class="map-wrap"><svg viewBox="0 0 650 295" role="img" aria-label="Illustrative US map of eight synthetic screens. Select a screen for its delivery details."><defs><pattern id="grid" width="24" height="24" patternUnits="userSpaceOnUse"><path d="M24 0H0V24" fill="none" class="map-grid"/></pattern></defs><rect width="650" height="295" fill="url(#grid)"/><path class="us-outline" d="${path}"/>${lines}${pts
    .map((s) => {
      const label = s.id === "sea-02" ? "" : s.id.split("-")[0].toUpperCase();
      return `<g class="screen-dot" data-screen="${s.id}" tabindex="0" role="button" aria-label="${esc(s.name)}" transform="translate(${s.p})"><circle class="halo" r="12"/><circle class="core" r="3"/><text class="map-label" x="${s.id === "sea-01" ? -31 : 10}" y="${s.id === "sea-01" ? -10 : -6}">${label}</text></g>`;
    })
    .join(
      "",
    )}</svg><div class="map-caption">ILLUSTRATIVE GEOGRAPHY · SYNTHETIC INVENTORY</div></div><div class="map-footer"><div class="legend"><span><i></i>Demo screen</span><span><i class="dim"></i>${new Set(pts.map((s) => s.market)).size} markets</span></div><a class="text-link" href="#intake">Inspect partner intake ↗</a></div></section>`;
}
function flow() {
  const c = state.counts,
    accepted = Object.values(c).reduce((a, n) => a + n, 0),
    terminal = accepted - (c.accepted || 0);
  const warning = runtime.paused || runtime.breaker_open;
  return `<section class="panel"><div class="panel-header"><div><h2>From receipt to certainty</h2><p>Durable at every handoff</p></div><span class="panel-tag">${runtime.transport === "pubsub" ? "PUB/SUB" : "LOCAL"}</span></div><div class="flow-body">${[
    ["↓", "Receipt API", "Validated envelope · persisted intent", accepted],
    [
      "⇢",
      "Transactional outbox",
      "Accepted work survives a restart",
      c.accepted || 0,
    ],
    [
      "◇",
      "Reconciliation worker",
      "Screen + time window + reservation",
      terminal,
    ],
    [
      "✓",
      "Settlement ledger",
      "Atomic spend + immutable decision",
      c.settled || 0,
    ],
  ]
    .map(
      (r, i) =>
        `<div class="flow-row"><span class="flow-icon">${r[0]}</span><div class="flow-text">${r[1]}<small>${r[2]}</small></div><div class="flow-count">${num(r[3])}<small>${i === 1 ? "pending" : ""}</small></div></div>`,
    )
    .join(
      "",
    )}</div><div class="flow-bottom${warning ? " warning" : ""}"><span>${warning ? "Ⅱ" : "✓"}</span>${runtime.paused ? "Worker paused. Accepted receipts remain durable." : runtime.breaker_open ? "Retry cooldown active. Work is retained." : "No duplicate charges. Every decision is auditable."}</div></section>`;
}
function campaignsTable() {
  return `<section class="panel"><div class="panel-header"><div><h2>Campaign pacing</h2><p>Settled and held against a hard budget cap</p></div><a href="#campaigns" class="button quiet">All campaigns ↗</a></div><div class="table-wrap"><table><thead><tr><th>CAMPAIGN</th><th>COMMITTED / BUDGET</th><th>STATE</th></tr></thead><tbody>${state.campaigns.map((c, i) => `<tr><td><div class="campaign-name"><span class="campaign-mark ${i === 1 ? "blue" : i === 2 ? "purple" : ""}">${["C", "N", "S"][i] || "A"}</span><div>${esc(c.name.split(" / ")[0])}<small>${esc(c.name.split(" / ")[1])}</small></div></div></td><td>${money(c.spent_micros + c.reserved_micros)} <span class="muted">/ ${money(c.budget_micros)}</span>${progress(c)}</td><td><span class="badge">${c.spent_micros + c.reserved_micros === c.budget_micros ? "At cap" : "Active"}</span></td></tr>`).join("")}</tbody></table></div><div class="table-foot"><span>Lime: settled · Amber: held</span><span>USD micros · No floating point</span></div></section>`;
}
function ledgerTable(data, compact = false) {
  if (
    !data.length &&
    (search || (!compact && active === "ledger" && filter !== "all"))
  )
    return '<div class="empty"><strong>No matching receipts.</strong>Try a different search or <button class="text-link" data-clear-filters>clear your filters</button>.</div>';
  if (!data.length)
    return `<div class="empty"><strong>${active === "recovery" ? "Nothing needs recovery." : "Your next play starts here."}</strong>${active === "recovery" ? "Run the schema or retry scenario in the failure lab to inspect a rejected delivery." : "Simulate traffic to create real reservations and reconcile playback receipts."}</div>`;
  return `<div class="table-wrap"><table><thead><tr><th>RECEIPT / SCREEN</th>${compact ? "" : "<th>RESERVATION</th><th>PLAYED AT</th>"}<th>DECISION</th><th>${compact ? "RECEIVED" : "ATTEMPTS"}</th></tr></thead><tbody>${data.map((d) => `<tr><td data-label="Receipt / screen"><button class="text-link mono" data-delivery="${d.id}">${esc(d.event_id.slice(0, 8))}<span class="muted">… ↗</span></button><div class="muted" style="font-size:9px;margin-top:5px">${esc(d.screen_id.toUpperCase())}</div></td>${compact ? "" : `<td data-label="Reservation" class="mono muted">${esc(d.reservation_id.slice(0, 12))}…</td><td data-label="Played at" class="mono" title="${esc(new Date(d.played_at).toLocaleString())}">${time(d.played_at)}</td>`}<td data-label="Decision">${badge(d.status)}${!compact && ["quarantined", "failed"].includes(d.status) ? `<small class="decision-reason">${esc(title(d.reason || "Inspect receipt for details"))}</small>` : ""}</td><td data-label="${compact ? "Received" : "Attempts"}" class="mono muted">${compact ? time(d.received_at) : d.attempts}</td></tr>`).join("")}</tbody></table></div>`;
}
function recent() {
  return `<section class="panel"><div class="panel-header"><div><h2>Latest decisions</h2><p>Follow the evidence behind every play</p></div><a class="button quiet" href="#ledger">Open ledger ↗</a></div>${ledgerTable(state.deliveries.slice(0, 3), true)}<div class="table-foot"><span>Automatic updates · every 2 seconds when live</span><span>Persisted decisions, live view <span class="status-dot"></span></span></div></section>`;
}
function overview() {
  return (
    heading(
      "Every play. Accounted for.",
      "A clear view of campaign delivery, from reservation to reconciliation.",
      `<a href="#lab" class="button">⌘ Failure lab</a><a href="#proof" class="button primary">◎ Run live proof</a>`,
    ) +
    metrics() +
    `<div class="overview-next"><span><strong>${runtime.paused ? "Dispatcher paused" : (state.counts.accepted || 0) > 0 ? `${num(state.counts.accepted)} receipts awaiting a decision` : "Delivery queue is clear"}</strong> · ${(state.counts.quarantined || 0) + (state.counts.failed || 0) > 0 ? `${num((state.counts.quarantined || 0) + (state.counts.failed || 0))} exceptions recorded. Inspect the latest evidence and choose the next step.` : "Start a partner batch or verify the integration end to end."}</span><a href="#${runtime.paused ? "lab" : (state.counts.quarantined || 0) + (state.counts.failed || 0) > 0 ? "recovery" : "intake"}">${runtime.paused ? "Open controls" : (state.counts.quarantined || 0) + (state.counts.failed || 0) > 0 ? "Review exceptions" : "Open intake"} ↗</a></div>` +
    `<div class="network-grid">${network()}${flow()}</div><div class="bottom-grid">${campaignsTable()}${recent()}</div>`
  );
}
function campaigns() {
  return (
    heading(
      "Budgets with boundaries.",
      "Reserve first. Settle on evidence. Never spend the same budget twice.",
      button("+ Reserve a play", "reserve", true),
    ) +
    `<div class="balance-cards">${state.campaigns.map((c) => `<article class="panel balance-card"><span class="eyebrow">${esc(c.id)}</span><h2>${esc(c.name)}</h2><div class="amount">${money(c.budget_micros - c.spent_micros - c.reserved_micros)} <small class="muted" style="font-size:12px">available</small></div>${progress(c)}<div class="balance-breakdown"><div><span>Settled</span><br>${money(c.spent_micros)}</div><div><span>Held</span><br>${money(c.reserved_micros)}</div><div><span>Budget</span><br>${money(c.budget_micros)}</div></div></article>`).join("")}</div><section class="panel"><div class="panel-header"><div><h2>Reservations</h2><p>Newest 100 holds · 15-minute play window + 15-minute receipt grace period</p></div></div>${!state.reservations.length ? '<div class="empty">No reservations yet.</div>' : `<div class="table-wrap"><table><thead><tr><th>RESERVATION</th><th>CAMPAIGN</th><th>SCREEN</th><th>AMOUNT</th><th>STATE</th><th>ACTION</th></tr></thead><tbody>${state.reservations.map((r) => `<tr><td class="mono">${r.id.slice(0, 12)}…</td><td>${esc(r.campaign_id)}</td><td>${esc(r.screen_id)}</td><td>${money(r.cost_micros)}</td><td>${badge(r.state)}</td><td>${r.state === "held" ? `<button class="button small" data-proof="${r.id}">Record playback ↗</button>` : '<span class="muted">Final</span>'}</td></tr>`).join("")}</tbody></table></div>`}</section>`
  );
}
function filtered() {
  const rows = state.deliveries.filter(
    (d) =>
      (active === "recovery"
        ? ["quarantined", "failed"].includes(d.status) &&
          (filter === "all" || d.status === filter)
        : filter === "all" || d.status === filter) &&
      [d.id, d.event_id, d.screen_id, d.reservation_id, d.reason].some((v) =>
        String(v || "")
          .toLowerCase()
          .includes(search.toLowerCase()),
      ),
  );
  return rows.sort((a, b) =>
    ledgerSort === "oldest"
      ? a.received_at - b.received_at || a.id.localeCompare(b.id)
      : b.received_at - a.received_at || a.id.localeCompare(b.id),
  );
}
function ledgerResults() {
  const rows = filtered(),
    pages = Math.max(1, Math.ceil(rows.length / 20));
  ledgerPage = Math.min(ledgerPage, pages - 1);
  const start = ledgerPage * 20;
  return (
    ledgerTable(rows.slice(start, start + 20)) +
    `<div class="table-foot ledger-pagination"><span>${rows.length ? `${start + 1}–${Math.min(start + 20, rows.length)} of ${rows.length}` : "0 matches"} · newest 100 receipts searched</span><div><button class="button small" data-ledger-page="previous" ${ledgerPage === 0 ? "disabled" : ""} aria-label="Previous receipts page">← Previous</button><span>Page ${ledgerPage + 1} / ${pages}</span><button class="button small" data-ledger-page="next" ${ledgerPage >= pages - 1 ? "disabled" : ""} aria-label="Next receipts page">Next →</button></div></div>`
  );
}
function ledger() {
  const recovery = active === "recovery";
  const options = recovery
    ? ["all", "quarantined", "failed"]
    : ["all", "settled", "duplicate", "accepted", "quarantined", "failed"];
  return (
    heading(
      recovery ? "Exceptions, explained." : "The evidence behind every play.",
      recovery
        ? "Find the cause. Choose the next step. Preserve the original evidence."
        : "Search receipts, follow decisions, and inspect the evidence behind settlement.",
      `<button class="button" data-export-receipts>Export matches ↓</button><a href="#${recovery ? "lab" : "intake"}" class="button primary">${recovery ? "Open failure lab" : "Partner intake"} ↗</a>`,
    ) +
    (recovery
      ? `<div class="recovery-guidance"><article><span class="badge quarantined">Quarantined</span><h3>Evidence needs correction</h3><p>Inspect the schema, screen, or play window. A corrected payload needs a new event ID.</p></article><article><span class="badge failed">Failed</span><h3>Processing exhausted its retries</h3><p>Resolve the operational cause, then replay the original receipt. Charge protection still applies.</p></article></div>`
      : "") +
    `<div class="ledger-toolbar"><label class="ledger-search-label"><span>Search receipts</span><input class="search" id="ledger-search" type="search" placeholder="Event, receipt, screen, reservation or reason…" aria-label="Search deliveries" value="${esc(search)}"></label><label class="ledger-sort-label"><span>Sort by</span><select id="ledger-sort"><option value="newest" ${ledgerSort === "newest" ? "selected" : ""}>Newest first</option><option value="oldest" ${ledgerSort === "oldest" ? "selected" : ""}>Oldest first</option></select></label><button class="button quiet" data-clear-filters>Clear filters</button></div><div class="filters" aria-label="Filter by decision">${options.map((v) => `<button class="filter ${filter === v ? "active" : ""}" data-filter="${v}" aria-pressed="${filter === v}">${v === "all" ? "All decisions" : title(v)} <span>${state.deliveries.filter((d) => (!recovery || ["quarantined", "failed"].includes(d.status)) && (v === "all" || d.status === v)).length}</span></button>`).join("")}<span class="filter-scope">Counts within latest 100</span></div><section class="panel" id="ledger-results">${ledgerResults()}</section>`
  );
}
function lab() {
  const scenarios = [
    [
      "01",
      "Duplicate delivery",
      "Five distinct event IDs report the same reservation. The first settles; the other four are suppressed.",
      "duplicate",
      "Send five receipts",
    ],
    [
      "02",
      "Crash after commit",
      "The next playback commits its charge, then loses the acknowledgement. The retry must leave spend unchanged.",
      "crash",
      "Lose the acknowledgement",
    ],
    [
      "03",
      "Unsupported schema",
      "A partner sends schema v99. Keep the evidence, quarantine the event, and leave the budget held.",
      "poison",
      "Send schema v99",
    ],
    [
      "04",
      "Transient failure",
      "Fail the next five processing attempts. With an otherwise empty queue, the receipt reaches retry exhaustion.",
      "retry",
      "Inject five failures",
    ],
    [
      "05",
      "Worker interruption",
      "Pause the local dispatcher, simulate traffic, then resume. Watch durable backlog drain without losing receipts.",
      "pause",
      runtime.paused ? "Resume worker" : "Pause worker",
    ],
    [
      "06",
      "Concurrent budget pressure",
      "32 requests compete to reserve $10 each from Solstice. Successful holds persist; the hard cap rejects the rest.",
      "budget-race",
      "Race 32 reservations",
    ],
  ];
  return (
    heading(
      "Make it fail. Watch it recover.",
      "Controlled faults against the real reconciliation engine.",
      button("▷ Simulate traffic", "traffic", true),
      "RELIABILITY LAB",
    ) +
    `<div class="view-intro"><span class="big">⌘</span><div>These controls mutate the local demo database. Faults are shared across this process. Run one scenario at a time with an empty queue for a clear result. Demo controls are disabled outside demo mode.</div></div><div class="lab-grid">${scenarios.map((s) => `<article class="panel lab-card"><div class="lab-number">SCENARIO / ${s[0]}</div><h2>${s[1]}</h2><p>${s[2]}</p>${button(s[4] + " ↗", s[3])}</article>`).join("")}</div><section class="panel" style="margin-top:22px"><div class="panel-header"><div><h2>Decision trail</h2><p>Newest 80 persisted audit records</p></div><span class="panel-tag">APPEND-ONLY APPLICATION LOG</span></div><div class="audit-list">${state.audit.map((a) => `<div class="audit-item"><time>${time(a.created_at)}</time><div>${badge(a.kind)} <span class="mono muted">${a.resource_id.slice(0, 10)}</span><small>${esc(a.detail)}</small></div></div>`).join("") || '<div class="empty">Run a scenario to begin the trail.</div>'}</div></section>`
  );
}
function architecture() {
  const decisions = [
    [
      "01 / Durability is a contract",
      "<code>202 Accepted</code> means receipt + outbox committed to SQL. A broker outage delays processing without losing already accepted intent. It does not mean the play has been billed.",
    ],
    [
      "02 / Business identity beats message identity",
      "An event ID deduplicates a retry. The reservation state prevents a second charge even when a partner sends a brand-new event ID for the same play.",
    ],
    [
      "03 / One transaction, one decision",
      "Receipt status, reservation transition, integer-micros budget and audit decision commit together. A crash before commit rolls back; a crash after commit makes redelivery a no-op.",
    ],
    [
      "04 / Retry the recoverable failures",
      "Dispatch uses leased ownership, backoff with jitter and bounded retries. Invalid evidence is quarantined. Replays use the original payload and the same validation rules.",
    ],
    [
      "05 / Trust boundaries stay explicit",
      "Production API credentials bind a tenant. No request can choose another tenant. Shared screen metadata contains no people, device IDs or mobile-location histories.",
    ],
    [
      "06 / Know what has been demonstrated",
      "This console runs on " +
        esc(runtime.storage) +
        " and " +
        (runtime.transport === "pubsub"
          ? "the Google Pub/Sub client."
          : "a local SQL-backed transport.") +
        " Cloud Run infrastructure is supplied separately. This is a production-oriented demo, not a certified Vistar integration.",
    ],
  ];
  return (
    heading(
      "Small surface. Strong guarantees.",
      "An engineering demo designed around the failure boundaries in digital out-of-home.",
      `<a href="https://github.com/jon-jc/afterglow" class="button" target="_blank" rel="noreferrer">Private repository ↗</a>`,
      "SYSTEM DESIGN",
    ) +
    `<section class="panel"><div class="arch-flow">${[
      ["REST API", "Validate + reserve"],
      ["SQL transaction", "Receipt + outbox"],
      ["Pub/Sub", "At-least-once delivery"],
      ["Go worker", "Reconcile + settle"],
    ]
      .map(
        (s, i) =>
          `${i ? '<span class="arch-arrow">→</span>' : ""}<div class="arch-node">${s[0]}<small>${s[1]}</small></div>`,
      )
      .join(
        "",
      )}</div></section><div class="decision-grid">${decisions.map((d) => `<article class="panel decision"><h3>${d[0]}</h3><p>${d[1]}</p></article>`).join("")}</div><div class="arch-note">Independent demo inspired by public <a href="https://clearchanneloutdoor.com/programmatic-advertising/" target="_blank" rel="noreferrer">Clear Channel Outdoor programmatic material</a> and <a href="https://www.vistarmedia.com/news/clear-channel-outdoor-selects-vistar-media-as-dooh-technology-partner" target="_blank" rel="noreferrer">the CCO/Vistar partnership announcement</a>. Campaigns, costs and screen inventory are synthetic. Playback evidence is not a measured audience impression or attribution claim.</div>`
  );
}
function render() {
  if (!state) return;
  const focus = document.activeElement;
  const next = Object.hasOwn(views, location.hash.slice(1))
    ? location.hash.slice(1)
    : "overview";
  if (next === "proof" && active === "proof" && $("#integration-proof")) return;
  if (next === "intake" && active === "intake" && $("#partner-intake")) return;
  if (["ledger-search", "ledger-sort"].includes(focus?.id) && next === active) {
    $("#ledger-results").innerHTML = ledgerResults();
    return;
  }
  const focusedAttribute = [
    "data-filter",
    "data-action",
    "data-delivery",
    "data-proof",
    "data-screen",
    "data-ledger-page",
    "data-clear-filters",
    "data-export-receipts",
  ].find((a) => focus?.hasAttribute(a));
  const focusSelector =
    focusedAttribute && $("#main").contains(focus)
      ? `[${focusedAttribute}="${CSS.escape(focus.getAttribute(focusedAttribute))}"]`
      : null;
  active = location.hash.slice(1);
  if (!Object.hasOwn(views, active)) active = "overview";
  document.querySelectorAll("[data-view]").forEach((a) => {
    a.classList.toggle("active", a.dataset.view === active);
    if (a.dataset.view === active) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  });
  $("#breadcrumb-view").textContent = views[active];
  document.title = `${views[active]} — Afterglow`;
  $(".synthetic-pill").textContent = "SYNTHETIC DATA";
  $("#main").innerHTML = {
    proof: integrationProof.render,
    intake: partnerIntake.render,
    overview,
    campaigns,
    ledger,
    recovery: ledger,
    lab,
    architecture,
  }[active]();
  syncActions();
  if (focusSelector)
    ($(focusSelector) || $("#main")).focus({ preventScroll: true });
}
function syncActions() {
  document.querySelectorAll("[data-action]").forEach((b) => {
    b.disabled = busy || !runtime.demo;
    b.setAttribute("aria-busy", String(busy));
  });
}
function openDialog(html) {
  $("#dialog-notice").textContent = "";
  $("#dialog-content").innerHTML = html;
  const heading = $("h2", $("#dialog-content"));
  if (heading) heading.id = "dialog-title";
  $("#detail-dialog").showModal();
}
function details(id) {
  const d = state.deliveries.find((x) => x.id === id);
  if (!d) return;
  const r = state.reservations.find((x) => x.id === d.reservation_id);
  openDialog(
    `<div class="dialog-eyebrow">DELIVERY / ${esc(d.id.slice(0, 12))}</div><h2>${d.status === "settled" ? "A play, accounted for." : d.status === "duplicate" ? "Same play. No second charge." : d.status === "accepted" ? "Durable. Awaiting a decision." : "Evidence that needs attention."}</h2>${badge(d.status)}<dl class="detail-grid"><div><dt>DECISION</dt><dd>${esc(title(d.reason || "Waiting for consumer"))}</dd></div><div><dt>RESERVATION PRICE</dt><dd>${r ? money(r.cost_micros) : "Outside latest reservation window"}</dd></div><div><dt>SCREEN</dt><dd>${esc(d.screen_id)}</dd></div><div><dt>PROCESSING ATTEMPTS</dt><dd>${d.attempts}</dd></div></dl><div class="code-block">${esc(JSON.stringify({ schema_version: d.schema_version, event_id: d.event_id, reservation_id: d.reservation_id, screen_id: d.screen_id, played_at: d.played_at, duration_ms: d.duration_ms }, null, 2))}</div><div class="dialog-actions">${["quarantined", "failed"].includes(d.status) ? `<button class="button primary" data-replay="${d.id}">↻ Replay original receipt</button>` : ""}</div><p>${d.status === "duplicate" ? "The reservation already settled. A different message ID cannot create another financial effect." : d.status === "quarantined" ? "Replaying preserves the original payload. Resolve the underlying cause first; unsupported schemas are rejected again." : "The reservation price comes from the server, never from the playback receipt."}</p>`,
  );
}
async function scenario(kind) {
  if (busy) return;
  busy = true;
  syncActions();
  try {
    if (kind === "reserve") {
      openDialog(
        `<div class="dialog-eyebrow">CAMPAIGN OPERATIONS</div><h2>Reserve a play.</h2><form id="reserve-form"><label>Campaign<select name="campaign">${state.campaigns.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("")}</select></label><label>Screen<select name="screen">${state.screens.map((s) => `<option value="${s.id}">${esc(s.name)}</option>`).join("")}</select></label><label>Price in USD<input name="price" value="1.50" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,2})?" required></label><p id="reserve-error" class="form-error" role="alert"></p><button class="button primary" type="submit">Reserve budget →</button></form><p>The hold expires 15 minutes from now, followed by a 15-minute receipt grace period.</p>`,
      );
      return;
    }
    if (kind === "pause") {
      const result = await request("/api/demo/control", {
        paused: !runtime.paused,
      });
      toast(
        result.paused
          ? "Dispatcher paused. New receipts remain in the durable queue."
          : "Dispatcher resumed. Retained receipts can now settle.",
      );
    } else {
      const result = await request("/api/demo/scenario", { kind });
      toast(
        result.message +
          (result.accepted !== undefined
            ? ` Accepted: ${result.accepted}; rejected: ${result.rejected}.`
            : ""),
      );
    }
    await refresh();
  } catch (e) {
    toast(e.message, true);
  } finally {
    busy = false;
    syncActions();
  }
}
document.addEventListener("click", async (event) => {
  const b = event.target.closest(
    "[data-action],[data-delivery],[data-filter],[data-replay],[data-proof],[data-screen]",
  );
  if (!b) return;
  if (b.dataset.action) {
    await scenario(b.dataset.action);
    return;
  }
  if (b.dataset.delivery) {
    details(b.dataset.delivery);
    return;
  }
  if (b.dataset.filter) {
    ledgerPage = 0;
    filter = b.dataset.filter;
    render();
    return;
  }
  if (b.dataset.screen) {
    const s = state.screens.find((x) => x.id === b.dataset.screen);
    const ds = state.deliveries.filter((d) => d.screen_id === s.id);
    openDialog(
      `<div class="dialog-eyebrow">SYNTHETIC SCREEN / ${s.id}</div><h2>${esc(s.name)}</h2><p>${esc(s.market)} · ${esc(s.format)}<br>Illustrative inventory. Not a real CCO screen identifier.</p><dl class="detail-grid"><div><dt>RECENT RECEIPTS</dt><dd>${ds.length}</dd></div><div><dt>RECENT SETTLED PLAYS</dt><dd>${ds.filter((d) => d.status === "settled").length}</dd></div></dl><p>Counts reflect this screen within the newest 100 receipts.</p>`,
    );
    return;
  }
  try {
    b.disabled = true;
    if (b.dataset.replay) {
      await request(`/api/v1/deliveries/${b.dataset.replay}/replay`, {});
      $("#detail-dialog").close();
      toast(
        "Original payload requeued. Validation and charge protection still apply.",
      );
    }
    if (b.dataset.proof) {
      const r = state.reservations.find((x) => x.id === b.dataset.proof);
      await request("/api/v1/receipts", {
        schema_version: 1,
        event_id: crypto.randomUUID(),
        reservation_id: r.id,
        screen_id: r.screen_id,
        played_at: Date.now(),
        duration_ms: 10000,
      });
      toast("Playback receipt accepted durably. The worker will reconcile it.");
    }
    await refresh();
  } catch (e) {
    toast(e.message, true);
  } finally {
    b.disabled = false;
  }
});
document.addEventListener("keydown", (e) => {
  if (
    (e.key === "Enter" || e.key === " ") &&
    e.target.matches("[data-screen]")
  ) {
    e.preventDefault();
    e.target.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  }
});
document.addEventListener("input", (e) => {
  if (e.target.id === "ledger-search") {
    search = e.target.value;
    ledgerPage = 0;
    $("#ledger-results").innerHTML = ledgerResults();
  }
});
document.addEventListener("submit", async (e) => {
  if (e.target.id !== "reserve-form") return;
  e.preventDefault();
  const form = e.target,
    b = $("button", form);
  if (b.disabled) return;
  try {
    b.disabled = true;
    $("#reserve-error", form).textContent = "";
    const fd = new FormData(form),
      raw = String(fd.get("price"));
    if (!/^\d+(\.\d{1,2})?$/.test(raw))
      throw new Error(
        "Use a positive dollar amount with at most two decimals.",
      );
    const [whole, cents = ""] = raw.split(".");
    const cost = Number(whole) * 1000000 + Number(cents.padEnd(2, "0")) * 10000;
    if (!Number.isSafeInteger(cost) || cost <= 0 || cost > 1000000000)
      throw new Error("Choose a price from $0.01 to $1,000.");
    const payload = {
      campaign_id: fd.get("campaign"),
      screen_id: fd.get("screen"),
      cost_micros: cost,
    };
    // An ambiguous response must retry the same operation, not create another hold.
    const fingerprint = JSON.stringify(payload);
    if (form.dataset.fingerprint !== fingerprint) {
      form.dataset.fingerprint = fingerprint;
      form.dataset.requestKey = crypto.randomUUID();
    }
    await request("/api/v1/reservations", payload, {
      "Idempotency-Key": form.dataset.requestKey,
    });
    if (form.isConnected) $("#detail-dialog").close();
    toast("Budget reserved. Record its playback from the reservations table.");
    await refresh();
  } catch (err) {
    if (form.isConnected) {
      $("#reserve-error", form).textContent =
        `${err.message} Retrying the unchanged form uses the same request key.`;
    } else {
      toast(
        `${err.message} Check reservations before creating another hold.`,
        true,
      );
    }
  } finally {
    b.disabled = false;
  }
});
$("#close-dialog").addEventListener("click", () => $("#detail-dialog").close());
$("#detail-dialog").addEventListener("click", (e) => {
  if (e.target === $("#detail-dialog")) {
    const r = e.target.getBoundingClientRect();
    if (
      e.clientX < r.left ||
      e.clientX > r.right ||
      e.clientY < r.top ||
      e.clientY > r.bottom
    )
      e.target.close();
  }
});
$("#refresh").addEventListener("click", refresh);
$(".skip-link").addEventListener("click", (e) => {
  e.preventDefault();
  $("#main").focus();
});
window.addEventListener("hashchange", () => {
  search = "";
  filter = "all";
  ledgerPage = 0;
  render();
  window.scrollTo(0, 0);
});
refresh();
setInterval(() => {
  if (!livePaused && !document.hidden) refresh();
}, 2000);
