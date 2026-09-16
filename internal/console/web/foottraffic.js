"use strict";
const footTraffic = (() => {
  const samples = {
    baseline: ["Balanced activity", "Four zones with a mix of reportable and small counts. Compare matched hours across two days."],
    busy: ["High activity", "All 24 current windows are reportable. Explore larger counts without treating activity as campaign lift."],
    gaps: ["Missing coverage", "The transit feed and alternate retail hours are missing. Missing observations must not become zeros."],
    quiet: ["Low counts", "Three zones fall below the reporting threshold. Only the downtown counts are released."],
    commuter: ["Commuter peaks", "Transit activity rises around illustrative morning and evening UTC peaks. Inspect how zone patterns differ."],
    retail: ["Retail activity", "The retail district has the largest counts. Compare zones without interpreting activity as verified ad exposure."],
    threshold: ["Reporting threshold", "Counts alternate between 19 and 20. Values below 20 stay hidden; values at 20 are reportable."],
    interruption: ["Feed interruption", "Two hours out of each six-hour period are absent across every zone. A gap in delivery is not zero activity."],
    random: ["Random mix", "Generate a fresh mix of activity, missing hours and small counts. Each new generation replaces only this random sample."],
  };
  const storageKey = "afterglow.foot-traffic.v2";
  let selected = "baseline", attempts = {};
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey) || "null");
    if (saved && Object.hasOwn(samples, saved.selected)) selected = saved.selected;
    for (const id of Object.keys(samples)) {
      const attempt = saved?.attempts?.[id], b = attempt?.batch;
      if (b?.source === "synthetic-partner-v1" && typeof b.batch_id === "string" &&
          Array.isArray(b.windows) && b.windows.length > 0 && b.windows.length <= 96 &&
          b.windows.every(w => Number.isSafeInteger(w.window_start) && w.window_start >= Date.now() - 7 * 86400000))
        attempts[id] = { batch: b, confirmed: attempt.confirmed === true };
    }
  } catch { /* Storage may be unavailable; the page still works in memory. */ }
  function remember() {
    if (!demoSession.active()) return;
    try { sessionStorage.setItem(storageKey, JSON.stringify({ selected, attempts })); } catch {}
  }
  const path = route => `/api/v1/foot-traffic/${route}?scenario=${encodeURIComponent(selected)}`;
  let report = null,
    loading = false,
    importing = false,
    emptying = false,
    error = "",
    notice = "",
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
    if (loading && !report)
      return '<div class="empty" role="status">Reading aggregate observation windows…</div>';
    const failure = error ? `<div class="error-banner" role="alert">${esc(error)}${report ? " Showing the last successful report." : ""} <button class="button" data-traffic-refresh>Retry report</button></div>` : "";
    if (!report)
      return failure || '<div class="empty">Load this sample feed to explore its measurement results.</div>';
    const r = report;
    const empty = r.zones.every(z => [...z.current, ...z.previous].every(c => c.status === "missing"));
    return `${failure}<p class="traffic-freshness" role="status">${loading ? "Refreshing report…" : `Report checked ${esc(new Date(r.as_of).toLocaleTimeString())}`}${r.missing_windows === 24 ? " · No current observations. Load this sample feed to begin." : ""}</p><div class="metrics">${metric("Reported observations", num(r.published_observations), "Released counts only · not unique people", "◫")}${metric("Window coverage", `${r.released_windows + r.suppressed_windows}/24`, "Four zones × six completed hours", "◷")}${metric("Suppressed windows", num(r.suppressed_windows), `Counts below ${r.minimum_reportable_count} stay hidden`, "◇")}${metric("Missing windows", num(r.missing_windows), "Missing data is not a count of zero", "↗")}</div>
    <div class="traffic-period"><span>${esc(new Date(r.window_start).toLocaleDateString("en-US", { timeZone: "UTC" }))} · ${hour(r.window_start)}–${hour(r.window_end)} UTC</span><span><i class="traffic-key"></i> Current window <i class="traffic-key previous"></i> Same hours, previous day</span></div>
    ${empty ? '<section class="panel empty traffic-empty"><h2>No batch loaded</h2><p>Load a sample feed or generate a random one to fill this report.</p></section>' : `<div class="traffic-grid">${r.zones.map((z) => `<article class="panel traffic-zone" data-traffic-zone="${esc(z.id)}"><div class="panel-header"><div><h2>${esc(z.name)}</h2><p>${esc(z.context)}</p></div><span class="panel-tag">${z.change_percent === null ? "NO COMPARISON" : `${z.change_percent >= 0 ? "+" : ""}${z.change_percent.toFixed(1)}%`}</span></div>${bars(z)}<p class="traffic-comparison">${z.comparable_windows}/6 paired windows are reportable. Change compares only those pairs; it does not measure campaign lift.</p><details><summary>Inspect hourly counts</summary><div class="table-wrap"><table><thead><tr><th>HOUR (UTC)</th><th>CURRENT</th><th>PREVIOUS DAY</th></tr></thead><tbody>${z.current.map((c, i) => `<tr><td>${hour(c.window_start)}</td><td>${value(c)}</td><td>${value(z.previous[i])}</td></tr>`).join("")}</tbody></table></div></details></article>`).join("")}</div>`}
    <section class="panel traffic-method"><div class="panel-header"><div><h2>Understand the measurement</h2><p>Inspect the assumptions before interpreting the chart.</p></div><span class="panel-tag">SYNTHETIC PARTNER FEED</span></div><div class="traffic-method-grid"><div><h3>Coarse observations</h3><p>Fixed zones and hourly counts. The API rejects device identifiers, precise coordinates and other unknown fields.</p></div><div><h3>Safe repeated batches</h3><p>A batch replay returns the original result. Repeated windows do not add twice; changed content for an existing window is a conflict.</p></div><div><h3>Limited conclusions</h3><p>A change in sample activity can reflect coverage, seasonality or other factors. It is not proof that an advertisement caused visits.</p></div></div><p class="traffic-scope">${esc(r.scope)}</p></section>`;
  }
  function update() {
    const target = $("#traffic-report");
    if (target) {
      const expanded = [...target.querySelectorAll("[data-traffic-zone] details[open]")].map(d => d.closest("[data-traffic-zone]").dataset.trafficZone);
      const focusZone = target.contains(document.activeElement) ? document.activeElement.closest("[data-traffic-zone]")?.dataset.trafficZone : null;
      target.innerHTML = content();
      target.setAttribute("aria-busy", String(loading));
      for (const id of expanded) target.querySelector(`[data-traffic-zone="${CSS.escape(id)}"] details`)?.setAttribute("open", "");
      if (focusZone) target.querySelector(`[data-traffic-zone="${CSS.escape(focusZone)}"] summary`)?.focus({ preventScroll: true });
    }
    document
      .querySelectorAll(
        "[data-traffic-load], [data-traffic-replay], [data-traffic-refresh], [data-traffic-empty], [data-traffic-random]",
      )
      .forEach((b) => {
        b.disabled =
          loading ||
          importing ||
          emptying ||
          (b.hasAttribute("data-traffic-replay") && !attempts[selected]);
        b.setAttribute("aria-busy", String(b.hasAttribute("data-traffic-refresh") ? loading : b.hasAttribute("data-traffic-empty") ? emptying : importing));
        if (b.hasAttribute("data-traffic-replay")) b.textContent = attempts[selected]?.confirmed !== false ? "Replay last batch" : "Retry last batch";
        if (b.hasAttribute("data-traffic-load")) b.textContent = selected === "random" ? "Generate another feed" : "Load sample feed";
      });
    const select = $("#traffic-sample");
    if (select) { select.disabled = loading || importing || emptying; select.value = selected; }
    const description = $("#traffic-sample-description");
    if (description) description.textContent = samples[selected][1];
    const n = $("#traffic-notice");
    if (n) n.textContent = notice;
  }
  async function refresh(manual = true) {
    if (loading) return;
    const ticket = ++generation;
    loading = true;
    error = "";
    update();
    try {
      const data = await request(path("report"));
      if (ticket === generation) {
        report = data;
        if (manual) notice = "Report refreshed. This reads saved observations; use Load sample feed to import data.";
      }
    } catch (e) {
      if (ticket === generation) {
        error = e.message;
      }
    } finally {
      if (ticket === generation) {
        loading = false;
        update();
      }
    }
  }
  async function ingest(replay) {
    if (importing || loading || emptying) return;
    if (replay && !attempts[selected]) {
      notice = "Load a sample feed before replaying it.";
      update();
      return;
    }
    importing = true;
    notice = replay ? "Checking the previous batch again…" : "Importing synthetic aggregate windows…";
    update();
    try {
      const batch = replay
        ? attempts[selected].batch
        : await request(path("example"));
      // Keep the exact input before sending: a lost response has an unknown
      // outcome, and retrying must reuse the same identity and payload.
      attempts[selected] = { batch, confirmed: false };
      remember();
      const result = await request(path(selected === "random" ? "random-batches" : "batches"), batch);
      attempts[selected].confirmed = true;
      remember();
      notice = result.replayed
        ? "Replay verified: the original batch result was returned. No counts were added."
        : `Batch committed: ${result.inserted_windows} new windows. Overlapping identical windows were retained once.`;
      await refresh(false);
    } catch (e) {
      notice = `${e.message}${attempts[selected]?.confirmed === false ? " Use Retry last batch to resend the same input safely." : ""}`;
    } finally {
      importing = false;
      update();
    }
  }
  async function emptyBatch() {
    if (loading || importing || emptying) return;
    emptying = true;
    notice = "Emptying this sample…";
    update();
    try {
      await request(path("empty"), {});
      delete attempts[selected];
      remember();
      report = null;
      error = "";
      notice = `${samples[selected][0]} emptied. Load a feed when you are ready to start again.`;
      await refresh(false);
    } catch (e) {
      notice = `Could not confirm the sample was emptied. ${e.message} You can retry Empty batch safely.`;
    } finally {
      emptying = false;
      update();
    }
  }
  function randomFeed() {
    if (loading || importing || emptying) return;
    if (selected !== "random") report = null;
    selected = "random";
    error = "";
    remember();
    ingest(false);
  }
  function render() {
    queueMicrotask(() => {
      if (!report && !loading && !error && !importing && !emptying) refresh(false);
      else update();
    });
    return (
      heading(
        "A clearer view of activity.",
        "Explore coarse foot-traffic observations, data coverage and change over time. All observations are synthetic.",
        `<button class="button" data-traffic-empty title="Clear this sample’s saved observations and replay history" ${loading || importing || emptying ? "disabled" : ""}>Empty batch</button><button class="button" data-traffic-replay ${!attempts[selected] || loading || importing || emptying ? "disabled" : ""}>Replay last batch</button><button class="button" data-traffic-refresh ${loading || importing || emptying ? "disabled" : ""}>Refresh report ↻</button><button class="button primary" data-traffic-load ${loading || importing || emptying ? "disabled" : ""}>${selected === "random" ? "Generate another feed" : "Load sample feed"}</button>`,
        "FOOT TRAFFIC",
      ) +
      `<div id="foot-traffic"><section class="panel traffic-samples"><div><label for="traffic-sample">Choose a sample feed</label><select id="traffic-sample" aria-describedby="traffic-sample-description" ${loading || importing || emptying ? "disabled" : ""}>${Object.entries(samples).map(([id, sample]) => `<option value="${id}" ${id === selected ? "selected" : ""}>${esc(sample[0])}</option>`).join("")}</select></div><div><p id="traffic-sample-description">${esc(samples[selected][1])}</p><small>Each sample keeps its own observations. Empty batch clears only the selected sample.</small><div class="traffic-generate"><button class="button" data-traffic-random ${loading || importing || emptying ? "disabled" : ""}>Generate random feed ↗</button></div></div></section><p id="traffic-notice" class="traffic-notice" role="status" aria-live="polite">${esc(notice)}</p><div id="traffic-report">${content()}</div></div>`
    );
  }
  document.addEventListener("click", (e) => {
    if (e.target.closest("[data-traffic-load]")) ingest(false);
    if (e.target.closest("[data-traffic-replay]")) ingest(true);
    if (e.target.closest("[data-traffic-empty]")) emptyBatch();
    if (e.target.closest("[data-traffic-random]")) randomFeed();
    if (e.target.closest("[data-traffic-refresh]") && !loading && !importing && !emptying)
      refresh();
  });
  document.addEventListener("change", (e) => {
    if (e.target.id !== "traffic-sample" || loading || importing || emptying || !Object.hasOwn(samples, e.target.value)) return;
    selected = e.target.value;
    report = null;
    error = "";
    notice = "";
    remember();
    refresh(false);
  });
  return { render, refresh: () => { if (!importing && !emptying) return refresh(); } };
})();
