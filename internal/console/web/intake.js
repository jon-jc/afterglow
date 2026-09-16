"use strict";
const partnerIntake = (() => {
  const examples = {
    partial: [
      "Partial success",
      "Two events for one play plus a malformed item. Expect HTTP 207, then one settlement and one duplicate.",
    ],
    clean: [
      "Valid playback",
      "One valid receipt. Expect HTTP 202, followed by settlement.",
    ],
    duplicate: [
      "Different IDs, same play",
      "Five event IDs report one reservation. Expect one settlement and four duplicates.",
    ],
    replay: [
      "Same event repeated",
      "The identical event appears twice. Expect the second item to replay the first receipt identity.",
    ],
    schema: [
      "Unsupported schema",
      "Schema v99 is accepted durably, then quarantined by the worker. No settlement.",
    ],
    window: [
      "Outside the play window",
      "Playback predates the reservation. Expect durable acceptance, then quarantine.",
    ],
  };
  let price = "0.01",
    priceError = "";
  function priceMicros() {
    const value = price.trim();
    if (!/^\d{1,9}(\.\d{1,2})?$/.test(value))
      throw new Error(
        "Enter a USD amount with up to two decimal places, at least $0.01.",
      );
    const [dollars, cents = ""] = value.split(".");
    const micros =
      (Number(dollars) * 100 + Number(cents.padEnd(2, "0"))) * 10000;
    if (micros > 1000000000)
      throw new Error("The maximum reservation is $1,000.00.");
    if (micros < 10000) throw new Error("The minimum reservation is $0.01.");
    return micros;
  }
  let exampleKind = "partial",
    examplePlan = null,
    preparing = false;

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
    $("#intake-example").disabled = pending;
    $("#intake-price-error").textContent = priceError;
    $("#intake-price").setAttribute("aria-invalid", String(!!priceError));
    $("#intake-price").disabled =
      pending || !!(examplePlan && !examplePlan.hold);
    $("#example-explanation").textContent = examples[exampleKind][1];
    $('[data-intake="example"]').textContent = preparing
      ? "Preparing…"
      : "Load example";
    document.querySelectorAll("[data-intake]").forEach((b) => {
      b.disabled = pending || !runtime.demo;
    });
    $("#intake-submit").textContent =
      pending && !preparing ? "Sending…" : "Send batch →";
  }
  function render() {
    return (
      heading(
        "Every receipt. An explicit outcome.",
        "Inspect the contract between a partner feed and the reconciliation engine.",
        '<a class="button" href="#ledger">Open delivery ledger ↗</a>',
        "PARTNER INTEGRATION",
      ) +
      `<div id="partner-intake"><div class="view-intro"><span class="big">⇄</span><div>Synthetic partner contract · Real Go API. Accepted receipts change the local demo database. This is not a connected or certified Vistar integration.</div></div><div class="intake-grid"><section class="panel intake-card"><h2>1. Prepare a playback batch</h2><p>Choose a scenario and load its request. Loading reserves your chosen amount of synthetic budget when needed; playback is sent only when you click Send batch. Repeated loads at the same price reuse the unsent hold. Changing the price creates a new hold; earlier holds remain reserved until settled or expired.</p><label for="intake-example">Example scenario</label><select id="intake-example" ${pending ? "disabled" : ""}>${Object.entries(
        examples,
      )
        .map(
          ([key, v]) =>
            `<option value="${key}" ${key === exampleKind ? "selected" : ""}>${v[0]}</option>`,
        )
        .join(
          "",
        )}</select><p id="example-explanation">${examples[exampleKind][1]}</p><label for="intake-price">Reservation price · USD</label><input id="intake-price" type="text" inputmode="decimal" value="${esc(price)}" aria-describedby="intake-price-help intake-price-error" aria-invalid="${!!priceError}" ${pending || (examplePlan && !examplePlan.hold) ? "disabled" : ""}><p id="intake-price-error" class="form-error" role="status">${esc(priceError)}</p><p id="intake-price-help">$0.01–$1,000.00, up to two decimal places. Applies when you load a new example. Existing reservations are unchanged.</p><div class="actions"><button class="button" data-intake="example" ${pending || !runtime.demo ? "disabled" : ""}>Load example</button></div><label for="intake-payload">Request body · up to 50 receipts</label><textarea id="intake-payload" spellcheck="false" ${pending ? "disabled" : ""} placeholder='{"receipts": [...]}' aria-describedby="intake-status">${esc(draft)}</textarea><div class="actions"><button id="intake-submit" class="button primary" data-intake="send" ${pending || !runtime.demo ? "disabled" : ""}>${pending && !preparing ? "Sending…" : "Send batch →"}</button></div><p id="intake-status" class="intake-status" role="status">${esc(notice)}</p></section><section class="panel intake-card"><h2>2. Inspect durable acceptance</h2><p>A 202 item means the receipt and outbox committed together. A 207 batch contains mixed results. Settlement happens asynchronously; acceptance alone is not a charge.</p><div id="intake-results" aria-live="polite">${results()}</div></section></div><div class="decision-grid"><article class="panel decision"><h3>Event identity</h3><p>An unchanged event ID and payload return the original receipt. Changing content under the same ID produces a conflict.</p></article><article class="panel decision"><h3>Business identity</h3><p>Two different events can report one play. The reservation transition prevents a second financial effect.</p></article><article class="panel decision"><h3>Recoverable failures</h3><p>On a timeout, the outcome may be unknown. Keep IDs and content unchanged. Honor Retry-After on 429; inspect quarantined evidence in Recovery queue.</p></article><article class="panel decision"><h3>Transport boundary</h3><p>Current transport: ${esc(runtime.transport)}. The SQL outbox separates API acceptance from delivery. The managed Pub/Sub path requires separate staging verification.</p></article></div></div>`
    );
  }
  document.addEventListener("change", (e) => {
    if (e.target.id === "intake-example") {
      exampleKind = e.target.value;
      notice =
        "Click Load example to prepare this scenario. The current request body is unchanged.";
      update();
    }
  });
  document.addEventListener("input", (e) => {
    if (e.target.id === "intake-payload") draft = e.target.value;
    if (e.target.id === "intake-price") {
      price = e.target.value;
      priceError = "";
      notice =
        "Click Load example to apply this price. The current request body and existing reservation are unchanged.";
      update();
    }
  });
  document.addEventListener("click", async (e) => {
    const b = e.target.closest("[data-intake]");
    if (!b || pending || !runtime.demo) return;
    if (b.dataset.intake === "example") {
      let cost;
      try {
        cost = priceMicros();
        priceError = "";
      } catch (e) {
        priceError = e.message;
        notice = e.message;
        update();
        $("#intake-price").focus();
        return;
      }
      pending = true;
      preparing = true;
      notice = "Preparing a real demo reservation…";
      update();
      try {
        if (
          !examplePlan ||
          (examplePlan.hold &&
            (examplePlan.hold.expires_at <= Date.now() ||
              examplePlan.input.cost_micros !== cost))
        ) {
          const snapshot = await request("/api/v1/snapshot");
          const campaign = snapshot.campaigns.find(
            (c) => c.budget_micros - c.spent_micros - c.reserved_micros >= cost,
          );
          if (!campaign || !snapshot.screens.length)
            throw new Error(
              `No campaign has ${money(cost)} available. Lower the price or check campaign budgets.`,
            );
          examplePlan = {
            key: crypto.randomUUID(),
            input: {
              campaign_id: campaign.id,
              screen_id: snapshot.screens[0].id,
              cost_micros: cost,
            },
          };
        }
        if (!examplePlan.hold)
          examplePlan.hold = await request(
            "/api/v1/reservations",
            examplePlan.input,
            { "Idempotency-Key": examplePlan.key },
          );
        const r = examplePlan.hold;
        if (r.state !== "held" || r.expires_at <= Date.now()) {
          examplePlan = null;
          throw new Error(
            "The recovered reservation is no longer available. Load again to prepare a fresh example.",
          );
        }
        const receipt = {
          schema_version: 1,
          event_id: crypto.randomUUID(),
          reservation_id: r.id,
          screen_id: r.screen_id,
          played_at: r.created_at + 1,
          duration_ms: 10000,
        };
        let receipts;
        switch (exampleKind) {
          case "clean":
            receipts = [receipt];
            break;
          case "duplicate":
            receipts = Array.from({ length: 5 }, () => ({
              ...receipt,
              event_id: crypto.randomUUID(),
            }));
            break;
          case "replay":
            receipts = [receipt, { ...receipt }];
            break;
          case "schema":
            receipts = [{ ...receipt, schema_version: 99 }];
            break;
          case "window":
            receipts = [{ ...receipt, played_at: r.created_at - 1000 }];
            break;
          default:
            receipts = [
              receipt,
              { ...receipt, event_id: crypto.randomUUID() },
              { unexpected_field: "deliberately malformed" },
            ];
        }
        draft = JSON.stringify({ receipts }, null, 2);
        if ($("#intake-payload")) $("#intake-payload").value = draft;
        result = null;
        notice =
          `Example ready. A ${money(r.cost_micros)} demo hold is reserved; no playback receipts have been sent. ` +
          examples[exampleKind][1];
      } catch (e) {
        // Only definitive input/auth/budget rejections discard the plan. Timeouts,
        // rate limits and server failures keep the original idempotency key.
        if (
          examplePlan &&
          !examplePlan.hold &&
          [400, 401, 403, 404, 409, 422].includes(e.status)
        )
          examplePlan = null;
        notice =
          e.message +
          (examplePlan && !examplePlan.hold
            ? " Click Load example to retry; an uncertain reservation keeps the same request key."
            : " Adjust the price if needed, then click Load example again.");
      } finally {
        pending = false;
        preparing = false;
        update();
      }
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
    examplePlan = null; // A submitted hold must not be reused for a new example, even after a lost response.
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
      examplePlan = null;
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
