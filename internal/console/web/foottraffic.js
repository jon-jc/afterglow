"use strict";
const footTraffic = (() => {
  let report = null,
    loading = false,
    importing = false,
    error = "",
    notice = "",
    lastBatch = null,
    generation = 0;
  const hour = (n) =>
    new Date(n).toLocaleTimeString("en-US", {
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "UTC",
      hour12: false,
    });
  function bars(zone) {
    const top = Math.max(
      1,
      ...zone.current.concat(zone.previous).map((c) => c.observations || 0),
    );
    return `<div class="traffic-chart" role="img" aria-label="${esc(zone.name)} hourly sample counts. Exact values are in the table below.">${zone.current.map((c, i) => `<div class="traffic-hour"><div class="traffic-pair"><span class="traffic-bar previous ${zone.previous[i].status}" style="height:${Math.max(3, (100 * (zone.previous[i].observations || 0)) / top)}%"></span><span class="traffic-bar ${c.status}" style="height:${Math.max(3, (100 * (c.observations || 0)) / top)}%"></span></div><small>${hour(c.window_start)}</small></div>`).join("")}</div>`;
  }
  function value(c) {
    return c.status === "reported"
      ? num(c.observations)
      : c.status === "suppressed"
        ? "Suppressed"
        : "No data";
  }
  function content() {
    if (loading)
      return '<div class="empty" role="status">Reading aggregate observation windows…</div>';
    if (error)
      return `<div class="error-banner" role="alert">${esc(error)} <button class="button" data-traffic-refresh>Retry report</button></div>`;
    if (!report)
      return '<div class="empty">Load a sample feed to explore the measurement pipeline.</div>';
    const r = report;
    return `<div class="metrics">${metric("Reported observations", num(r.published_observations), "Released counts only · not unique people", "◫")}${metric("Window coverage", `${r.released_windows + r.suppressed_windows}/24`, "Four zones × six completed hours", "◷")}${metric("Suppressed windows", num(r.suppressed_windows), `Counts below ${r.minimum_reportable_count} stay hidden`, "◇")}${metric("Missing windows", num(r.missing_windows), "Missing data is not a count of zero", "↗")}</div>
    <div class="traffic-period"><span>${esc(new Date(r.window_start).toLocaleDateString("en-US", { timeZone: "UTC" }))} · ${hour(r.window_start)}–${hour(r.window_end)} UTC</span><span><i class="traffic-key"></i> Current window <i class="traffic-key previous"></i> Same hours, previous day</span></div>
    <div class="traffic-grid">${r.zones.map((z) => `<article class="panel traffic-zone"><div class="panel-header"><div><h2>${esc(z.name)}</h2><p>${esc(z.context)}</p></div><span class="panel-tag">${z.change_percent === null ? "NO COMPARISON" : `${z.change_percent >= 0 ? "+" : ""}${z.change_percent.toFixed(1)}%`}</span></div>${bars(z)}<p class="traffic-comparison">${z.comparable_windows}/6 paired windows are reportable. Change compares only those pairs; it does not measure campaign lift.</p><details><summary>Inspect hourly counts</summary><div class="table-wrap"><table><thead><tr><th>HOUR (UTC)</th><th>CURRENT</th><th>PREVIOUS DAY</th></tr></thead><tbody>${z.current.map((c, i) => `<tr><td>${hour(c.window_start)}</td><td>${value(c)}</td><td>${value(z.previous[i])}</td></tr>`).join("")}</tbody></table></div></details></article>`).join("")}</div>
    <section class="panel traffic-method"><div class="panel-header"><div><h2>Understand the measurement</h2><p>Inspect the assumptions before interpreting the chart.</p></div><span class="panel-tag">SYNTHETIC PARTNER FEED</span></div><div class="traffic-method-grid"><div><h3>Coarse observations</h3><p>Fixed zones and hourly counts. The API rejects device identifiers, precise coordinates and other unknown fields.</p></div><div><h3>Safe repeated batches</h3><p>A batch replay returns the original result. Repeated windows do not add twice; changed content for an existing window is a conflict.</p></div><div><h3>Limited conclusions</h3><p>A change in sample activity can reflect coverage, seasonality or other factors. It is not proof that an advertisement caused visits.</p></div></div><p class="traffic-scope">${esc(r.scope)}</p></section>`;
  }
  function update() {
    const target = $("#traffic-report");
    if (target) target.innerHTML = content();
    document
      .querySelectorAll(
        "[data-traffic-load], [data-traffic-replay], [data-traffic-refresh]",
      )
      .forEach((b) => {
        b.disabled =
          loading ||
          importing ||
          (b.hasAttribute("data-traffic-replay") && !lastBatch);
      });
    const n = $("#traffic-notice");
    if (n) n.textContent = notice;
  }
  async function refresh() {
    const ticket = ++generation;
    loading = true;
    error = "";
    update();
    try {
      const data = await request("/api/v1/foot-traffic/report");
      if (ticket === generation) report = data;
    } catch (e) {
      if (ticket === generation) {
        error = e.message;
        report = null;
      }
    } finally {
      if (ticket === generation) {
        loading = false;
        update();
      }
    }
  }
  async function ingest(replay) {
    if (importing || loading) return;
    importing = true;
    notice = "Importing synthetic aggregate windows…";
    update();
    try {
      const batch = replay
        ? lastBatch
        : await request("/api/v1/foot-traffic/example");
      const result = await request("/api/v1/foot-traffic/batches", batch);
      lastBatch = batch;
      notice = result.replayed
        ? "Replay verified: the original batch result was returned. No counts were added."
        : `Batch committed: ${result.inserted_windows} new windows. Overlapping identical windows were retained once.`;
      await refresh();
    } catch (e) {
      notice = e.message;
    } finally {
      importing = false;
      update();
    }
  }
  function render() {
    if (!report && !loading && !error) queueMicrotask(refresh);
    return (
      heading(
        "A clearer view of activity.",
        "Explore coarse foot-traffic observations, data coverage and change over time. All observations are synthetic.",
        '<button class="button" data-traffic-replay disabled>Replay last batch</button><button class="button" data-traffic-refresh>Refresh report ↻</button><button class="button primary" data-traffic-load>Load sample feed</button>',
        "FOOT TRAFFIC",
      ) +
      `<div id="foot-traffic"><p id="traffic-notice" class="traffic-notice" role="status" aria-live="polite">${esc(notice)}</p><div id="traffic-report">${content()}</div></div>`
    );
  }
  document.addEventListener("click", (e) => {
    if (e.target.closest("[data-traffic-load]")) ingest(false);
    if (e.target.closest("[data-traffic-replay]")) ingest(true);
    if (e.target.closest("[data-traffic-refresh]") && !loading && !importing)
      refresh();
  });
  return { render };
})();
