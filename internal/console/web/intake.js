"use strict";
const partnerIntake = (() => {
  let draft = "",
    pending = false,
    result = null,
    notice = "",
    httpStatus = 0;
  function results() {
    if (!result)
      return '<div class="empty">Send a batch to see the real API response.</div>';
    return `<p><strong>HTTP ${httpStatus}</strong> · ${num(result.accepted)} accepted · ${num(result.rejected)} rejected · ${num(result.replayed)} replayed</p>${result.results.map((r) => `<div class="intake-result"><strong>Item ${r.index + 1} · ${r.status}${r.replayed ? " · replayed" : ""}</strong><small>${esc(r.detail)}</small>${r.id ? `<small>Receipt: ${esc(r.id)}</small>` : ""}<small>${r.retryable ? "Transient failure: retry unchanged after backoff." : r.status === 202 ? "Durably accepted. Check the ledger for the processing decision." : "Resolve the input problem before retrying this item."}</small></div>`).join("")}`;
  }
  function update() {
    if (!$("#partner-intake")) return;
    $("#intake-status").textContent = notice;
    $("#intake-results").innerHTML = results();
    $("#intake-payload").disabled = pending;
    document.querySelectorAll("[data-intake]").forEach((b) => {
      b.disabled = pending || !runtime.demo;
    });
    $("#intake-submit").textContent = pending ? "Sending…" : "Send batch →";
  }
  function render() {
    return (
      heading(
        "Every receipt. An explicit outcome.",
        "Inspect the contract between a partner feed and the reconciliation engine.",
        '<a class="button" href="#ledger">Open delivery ledger ↗</a>',
        "PARTNER INTEGRATION",
      ) +
      `<div id="partner-intake"><div class="view-intro"><span class="big">⇄</span><div>Synthetic partner contract · Real Go API. Accepted receipts change the local demo database. This is not a connected or certified Vistar integration.</div></div><div class="intake-grid"><section class="panel intake-card"><h2>1. Prepare a playback batch</h2><p>Reserve a play in <a class="text-link" href="#campaigns">Campaigns</a>, then load an example: two events for that play and one malformed neighbor. Sending again preserves event IDs to demonstrate safe retries.</p><div class="actions"><button class="button" data-intake="example" ${pending || !runtime.demo ? "disabled" : ""}>Load example</button></div><label for="intake-payload">Request body · up to 50 receipts</label><textarea id="intake-payload" spellcheck="false" ${pending ? "disabled" : ""} placeholder='{"receipts": [...]}' aria-describedby="intake-status">${esc(draft)}</textarea><div class="actions"><button id="intake-submit" class="button primary" data-intake="send" ${pending || !runtime.demo ? "disabled" : ""}>${pending ? "Sending…" : "Send batch →"}</button></div><p id="intake-status" class="intake-status" role="status">${esc(notice)}</p></section><section class="panel intake-card"><h2>2. Inspect durable acceptance</h2><p>A 202 item means the receipt and outbox committed together. A 207 batch contains mixed results. Settlement happens asynchronously; acceptance alone is not a charge.</p><div id="intake-results" aria-live="polite">${results()}</div></section></div><div class="decision-grid"><article class="panel decision"><h3>Event identity</h3><p>An unchanged event ID and payload return the original receipt. Changing content under the same ID produces a conflict.</p></article><article class="panel decision"><h3>Business identity</h3><p>Two different events can report one play. The reservation transition prevents a second financial effect.</p></article><article class="panel decision"><h3>Recoverable failures</h3><p>On a timeout, the outcome may be unknown. Keep IDs and content unchanged. Honor Retry-After on 429; inspect quarantined evidence in Recovery queue.</p></article><article class="panel decision"><h3>Transport boundary</h3><p>Current transport: ${esc(runtime.transport)}. The SQL outbox separates API acceptance from delivery. The managed Pub/Sub path requires separate staging verification.</p></article></div></div>`
    );
  }
  document.addEventListener("input", (e) => {
    if (e.target.id === "intake-payload") draft = e.target.value;
  });
  document.addEventListener("click", async (e) => {
    const b = e.target.closest("[data-intake]");
    if (!b || pending || !runtime.demo) return;
    if (b.dataset.intake === "example") {
      const r = state.reservations.find(
        (r) => r.state === "held" && r.expires_at > Date.now(),
      );
      if (!r) {
        notice =
          "Reserve a new play in Campaigns first, then load the example.";
        update();
        return;
      }
      const receipt = {
        schema_version: 1,
        event_id: crypto.randomUUID(),
        reservation_id: r.id,
        screen_id: r.screen_id,
        played_at: Date.now(),
        duration_ms: 10000,
      };
      draft = JSON.stringify(
        {
          receipts: [
            receipt,
            { ...receipt, event_id: crypto.randomUUID() },
            { unexpected_field: "deliberately malformed" },
          ],
        },
        null,
        2,
      );
      $("#intake-payload").value = draft;
      result = null;
      notice = "Example prepared. Nothing has been sent yet.";
      update();
      return;
    }
    let body;
    try {
      body = JSON.parse(draft);
      if (
        !Array.isArray(body?.receipts) ||
        body.receipts.length < 1 ||
        body.receipts.length > 50
      )
        throw new Error("Use a receipts array with 1–50 items.");
    } catch (e) {
      notice = `Invalid batch: ${e.message}`;
      update();
      return;
    }
    pending = true;
    result = null;
    notice = "Sending to the Go intake API…";
    update();
    try {
      const response = await fetch("/api/v1/receipts/batch", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        signal: AbortSignal.timeout(12000),
      });
      const data = await response.json();
      if (!response.ok)
        throw new Error(
          `${data.detail || "Request failed"} (${response.status})${response.headers.get("Retry-After") ? ". Retry after " + response.headers.get("Retry-After") + " seconds" : ""}`,
        );
      if (!Array.isArray(data.results))
        throw new Error("Unreadable result; acceptance is unknown.");
      result = data;
      httpStatus = response.status;
      notice =
        "Response received. Original payload retained for an unchanged retry.";
      await refresh();
    } catch (e) {
      notice = `${e.message}. Payload retained. If the outcome is unknown, retry unchanged.`;
    } finally {
      pending = false;
      update();
    }
  });
  return { render };
})();
