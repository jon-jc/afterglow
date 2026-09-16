"use strict";
const campaignAssurance = (() => {
  let selected = "",
    report = null,
    loading = false,
    error = "",
    generation = 0;
  function recommendations(r) {
    const items = [];
    if (!r.budget_consistent)
      items.push([
        "Budget mismatch",
        "Campaign balances disagree with reservation totals. Investigate before treating this report as financially reconciled.",
        "campaigns",
      ]);
    if (r.awaiting_evidence)
      items.push([
        "Follow up on missing valid playback",
        `${num(r.awaiting_evidence)} held plays are past their play window. Valid evidence may still arrive during the receipt grace period; expired holds are released by the worker.`,
        "ledger",
      ]);
    if (r.decisions.quarantined)
      items.push([
        "Review partner evidence",
        `${num(r.decisions.quarantined)} linked receipts are quarantined. Inspect their reasons; corrected content requires a new event identity. Historical quarantines remain even after a correction settles.`,
        "recovery",
      ]);
    if (r.decisions.failed)
      items.push([
        "Recover processing failures",
        `${num(r.decisions.failed)} linked receipts exhausted dispatch retries. Resolve the operational cause before replaying.`,
        "recovery",
      ]);
    if (r.decisions.accepted)
      items.push([
        "Watch processing progress",
        `${num(r.decisions.accepted)} receipts are durable but not yet reconciled. Acceptance is not a settlement.`,
        "ledger",
      ]);
    if (!items.length)
      items.push([
        "No exceptions in this report",
        "The observed balances agree and no linked receipt exceptions or overdue held plays were found. This is an operational check, not a campaign performance forecast.",
        "campaigns",
      ]);
    return items
      .map(
        (i) =>
          `<article class="assurance-action"><div><h3>${i[0]}</h3><p>${i[1]}</p></div><a class="button small" href="#${i[2]}">Inspect ↗</a></article>`,
      )
      .join("");
  }
  function content() {
    if (loading)
      return '<div class="empty">Reading the complete campaign ledger…</div>';
    if (error)
      return `<div class="error-banner" role="alert">${esc(error)} <button class="button" data-assurance-refresh>Retry report</button></div>`;
    if (!report)
      return '<div class="empty">Choose a campaign to inspect its delivery evidence.</div>';
    const r = report,
      c = r.campaign,
      available = c.budget_micros - c.spent_micros - c.reserved_micros;
    return `<div class="assurance-verdict ${r.budget_consistent ? "" : "mismatch"}"><span>${r.budget_consistent ? "✓" : "!"}</span><div><strong>${r.budget_consistent ? "Budget reconciles to the reservation ledger" : "Budget reconciliation needs investigation"}</strong><small>One consistent database snapshot · all retained campaign history · observed ${esc(new Date(r.observed_at).toLocaleString())}</small></div></div><div class="metrics">${metric("Settled spend", money(c.spent_micros), `${num(r.settled_plays)} distinct plays settled`, "↗")}${metric("Held budget", money(c.reserved_micros), `${num(r.held_plays)} plays awaiting settlement`, "◷")}${metric("Available budget", money(available), `Of ${money(c.budget_micros)} total budget`, "◫")}${metric("Duplicate reports", num(r.decisions.duplicate), "Repeated plays did not add another charge", "⧉")}</div><section class="panel"><div class="panel-header"><div><h2>Delivery by screen</h2><p>Spend comes from reservations. Receipt counts are aggregated separately.</p></div><span class="panel-tag">FULL RETAINED HISTORY</span></div>${r.screens.length ? `<div class="table-wrap"><table><thead><tr><th>SCREEN / MARKET</th><th>SETTLED PLAYS</th><th>SETTLED SPEND</th><th>HELD BUDGET</th><th>EXCEPTIONS</th></tr></thead><tbody>${r.screens.map((s) => `<tr><td><strong>${esc(s.name)}</strong><small class="proof-id">${esc(s.market)} · ${esc(s.format)} · ${esc(s.screen_id)}</small></td><td>${num(s.settled_plays)}</td><td>${money(s.settled_micros)}</td><td>${money(s.held_micros)}</td><td>${(s.decisions.quarantined || 0) + (s.decisions.failed || 0) ? `<span class="badge quarantined">${num((s.decisions.quarantined || 0) + (s.decisions.failed || 0))} receipts</span>` : '<span class="muted">None</span>'}</td></tr>`).join("")}</tbody></table></div>` : '<div class="empty">No reservations for this campaign yet. Reserve a play in Campaigns to begin.</div>'}</section><section class="panel assurance-actions"><div class="panel-header"><div><h2>Recommended operational follow-up</h2><p>Explainable rules from observed evidence. These links open workspace-wide investigation views.</p></div></div>${recommendations(r)}</section><p class="assurance-scope">${esc(r.scope)} Screen attribution uses the reserved inventory. Playback is not a measured audience impression, store visit, conversion, or proof of campaign lift.</p>`;
  }
  function update() {
    if ($("#assurance-report")) $("#assurance-report").innerHTML = content();
    const b = $("[data-assurance-export]");
    if (b) b.disabled = !report || loading || !!error;
  }
  async function fetchReport() {
    if (!selected) return;
    const ticket = ++generation;
    loading = true;
    error = "";
    report = null;
    update();
    try {
      const data = await request(
        "/api/v1/campaigns/" + encodeURIComponent(selected) + "/assurance",
      );
      if (ticket === generation) report = data;
    } catch (e) {
      if (ticket === generation) error = e.message;
    } finally {
      if (ticket === generation) {
        loading = false;
        update();
      }
    }
  }
  function render() {
    if (!selected) selected = state.campaigns[0]?.id || "";
    if (!report && !loading && !error) queueMicrotask(fetchReport);
    return (
      heading(
        "Delivery you can stand behind.",
        "Connect campaign budgets to verified plays, screen-level delivery and operational follow-up.",
        `<button class="button" data-assurance-export ${!report || loading || error ? 'disabled' : ''}>Export report ↓</button><button class="button primary" data-assurance-refresh>Refresh report ↻</button>`,
        "CAMPAIGN ASSURANCE",
      ) +
      `<div id="campaign-assurance"><div class="assurance-selector"><label for="assurance-campaign">Campaign</label><select id="assurance-campaign">${state.campaigns.map((c) => `<option value="${esc(c.id)}" ${c.id === selected ? "selected" : ""}>${esc(c.name)}</option>`).join("")}</select><span>Snapshot report · refresh to update</span></div><div id="assurance-report">${content()}</div></div>`
    );
  }
  document.addEventListener("change", (e) => {
    if (e.target.id === "assurance-campaign") {
      selected = e.target.value;
      fetchReport();
    }
  });
  document.addEventListener("click", (e) => {
    if (e.target.closest("[data-assurance-refresh]")) fetchReport();
    if (
      e.target.closest("[data-assurance-export]") &&
      report &&
      !loading &&
      !error
    ) {
      const url = URL.createObjectURL(
        new Blob([JSON.stringify(report, null, 2)], {
          type: "application/json",
        }),
      );
      const a = document.createElement("a");
      a.href = url;
      a.download = `afterglow-${report.campaign.id}-assurance.json`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
  });
  return { render };
})();
