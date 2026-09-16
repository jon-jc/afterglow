"use strict";
const integrationProof = (() => {
  const storageKey = "afterglow.integration-proof.v1";
  const stages = [
    [
      "Reserve",
      "Two server-priced holds",
      "Reserve two $0.01 plays with stable idempotency keys.",
    ],
    [
      "Accept",
      "Partial success is explicit",
      "Three valid envelopes commit. One malformed neighbor is rejected.",
    ],
    [
      "Retry",
      "Same events, same receipts",
      "Resend the exact batch and verify all three accepted identities replay.",
    ],
    [
      "Reconcile",
      "Protect the financial boundary",
      "Observe one settlement, one duplicate, and one quarantined schema.",
    ],
    [
      "Correct",
      "Keep original evidence intact",
      "Submit schema v1 under a new event ID for the quarantined play.",
    ],
    [
      "Verify",
      "Inspect persisted outcomes",
      "Confirm two settled reservations and retain the original quarantine.",
    ],
  ];
  let run = null,
    running = false,
    message = "",
    storageWarning = "";
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey));
    if (
      saved?.version === 1 &&
      saved.plan &&
      Array.isArray(saved.steps) &&
      saved.steps.length === 6
    )
      run = saved;
  } catch {
    /* A corrupt or unavailable cache must not block the console. */
  }
  function save() {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(run));
    } catch {
      storageWarning =
        "Browser storage unavailable. Export evidence before closing this page.";
    }
  }
  const assert = (condition, text) => {
    if (!condition) throw new Error(text);
  };
  function cards() {
    return stages
      .map(
        (s, i) =>
          `<li class="proof-step ${run?.steps[i]?.done ? "complete" : running && run?.current === i ? "executing" : ""}"><span class="proof-step-number">${run?.steps[i]?.done ? "✓" : String(i + 1).padStart(2, "0")}</span><div><strong>${s[0]}</strong><small>${s[1]}</small></div><span class="proof-step-state">${run?.steps[i]?.done ? "Verified" : running && run?.current === i ? "Running" : "Pending"}</span></li>`,
      )
      .join("");
  }
  function evidence() {
    const rows = run?.final || run?.initial || [];
    if (!rows.length)
      return '<div class="proof-empty"><span>◎</span><h3>Evidence, not a canned animation.</h3><p>The run will create real receipts and read their decisions back from the Go API. Every status below comes from persisted state.</p></div>';
    return `<div class="table-wrap"><table><thead><tr><th>EVENT</th><th>DECISION</th><th>REASON</th><th>EVIDENCE</th></tr></thead><tbody>${rows.map((e, i) => `<tr><td><strong>${["Play A", "Play A · second event", "Play B · schema v99", "Play B · corrected"][i]}</strong><small class="proof-id">${esc(e.delivery.event_id.slice(0, 12))}…</small></td><td>${badge(e.delivery.status)}</td><td>${esc(title(e.delivery.reason))}</td><td><button class="button small" data-proof-evidence="${i}">Inspect ↗</button></td></tr>`).join("")}</tbody></table></div><p class="proof-footnote">Observed ${esc(new Date(rows[rows.length - 1].observed_at).toLocaleString())}. Reservation state is current at that observation; receipt payloads remain unchanged.</p>`;
  }
  function contents() {
    const completed = run?.steps.filter((s) => s.done).length || 0;
    const done = completed === 6;
    return `<section class="proof-hero"><div><div class="eyebrow">LIVE INTEGRATION PROOF / GO + EVENT-DRIVEN SYSTEMS</div><h1>Trust is earned<br>at the failure boundary.</h1><p>Watch a partner batch become durable evidence, survive a retry, and settle each play once. Then correct bad evidence without rewriting history.</p><div class="proof-actions"><button class="button primary" data-proof-run ${running || !runtime.demo ? "disabled" : ""}>${running ? "Verifying live…" : done ? "Run again · $0.02 demo spend" : run ? "Resume this run" : "Start live proof · $0.02 demo spend"}</button><button class="button" data-proof-export ${!run ? "disabled" : ""}>Export evidence ↓</button></div><p class="proof-disclosure">Uses synthetic inventory and your local database. No real advertising purchase. No Vistar connection. A run creates two $0.01 reservations.</p></div><div class="proof-orbit" aria-label="${completed} of 6 stages verified"><div class="proof-ring" style="--progress:${(completed / 6) * 100}%"><div><strong>${String(completed).padStart(2, "0")}<span>/06</span></strong><small>${done ? "ALL CHECKS VERIFIED" : running ? "READING REAL OUTCOMES" : "VERIFIABLE STAGES"}</small></div></div><span class="proof-transport">${esc(runtime.storage)} · ${esc(runtime.transport)} transport</span></div></section><div class="proof-status" role="status">${esc(message || (done ? "Run verified. Export the evidence or inspect a receipt below." : run ? "Saved run loaded. Resume preserves its event IDs and reservation keys." : "Ready. Run one scenario at a time for a clear result."))}${storageWarning ? `<br>${esc(storageWarning)}` : ""}</div><div class="proof-layout"><section class="panel proof-journey"><div class="panel-header"><div><h2>The verification path</h2><p>Each stage checks an actual API result</p></div></div><ol>${cards()}</ol></section><section class="panel proof-explainer"><div class="eyebrow">${done ? "VERIFIED OUTCOME" : "WHAT THIS DEMONSTRATES"}</div><h2>${done ? "Four events. Two plays.<br>Two cents settled." : "Transport identity ≠<br>business identity."}</h2><p>An event ID prevents an unchanged retry from becoming new work. A reservation guard prevents different events for the same play from moving money twice.</p><div class="proof-facts"><div><strong>${run?.batch ? "207" : "—"}</strong><span>Mixed batch HTTP status</span></div><div><strong>${run?.replay ? num(run.replay.replayed) : "—"}</strong><span>Existing receipts replayed</span></div><div><strong>${done ? "$0.02" : "—"}</strong><span>Verified reservation total</span></div></div><a class="text-link" href="#architecture">Explore the system design ↗</a></section></div><section class="panel proof-evidence"><div class="panel-header"><div><h2>Evidence from the ledger</h2><p>Exact receipt reads · tenant-scoped · independent of recent traffic</p></div><a class="text-link" href="#recovery">Recovery queue ↗</a></div>${evidence()}</section><div class="decision-grid"><article class="panel decision"><h3>Why it matters for ad tech</h3><p>Partner feeds can repeat, arrive late, or change schema. Separate acceptance from reconciliation so a bad item does not silently discard good plays or charge a campaign twice.</p></article><article class="panel decision"><h3>What this run proves</h3><p>These particular requests passed the listed checks on the displayed transport. It is not a throughput benchmark, an availability guarantee, or managed Pub/Sub certification.</p></article></div>`;
  }
  function render() {
    return `<div id="integration-proof">${contents()}</div>`;
  }
  function update() {
    if (!$("#integration-proof")) return;
    const focus = document.activeElement;
    const restore = focus?.hasAttribute("data-proof-run")
      ? "[data-proof-run]"
      : focus?.hasAttribute("data-proof-export")
        ? "[data-proof-export]"
        : null;
    $("#integration-proof").innerHTML = contents();
    if (restore) $(restore)?.focus({ preventScroll: true });
  }
  async function batch(body) {
    const response = await fetch("/api/v1/receipts/batch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(12000),
    });
    const data = await response.json();
    assert(
      response.status === 207,
      `Expected partial success; received HTTP ${response.status}: ${data.detail || "Inspect the API response"}. Keep this run and retry unchanged.`,
    );
    assert(
      data.accepted === 3 &&
        data.rejected === 1 &&
        data.results[3].status === 400,
      "Unexpected batch outcome. Evidence retained for inspection.",
    );
    return data;
  }
  async function observe(ids, terminal = true) {
    const deadline = Date.now() + 20000;
    while (true) {
      const rows = await Promise.all(
        ids.map((id) =>
          request("/api/v1/deliveries/" + encodeURIComponent(id)),
        ),
      );
      if (!terminal || rows.every((e) => e.delivery.status !== "accepted"))
        return rows;
      if (Date.now() > deadline)
        throw new Error(
          "Worker has not completed within 20 seconds. Check the Failure lab and connection, then resume this run.",
        );
      await new Promise((r) => setTimeout(r, 350));
    }
  }
  async function execute() {
    if (running || !runtime.demo) return;
    running = true;
    try {
      if (!run || run.steps.every((s) => s.done)) {
        const fresh = await request("/api/v1/snapshot");
        const c = fresh.campaigns.find(
          (c) => c.budget_micros - c.spent_micros - c.reserved_micros >= 20000,
        );
        assert(
          c && fresh.screens.length,
          "No campaign has $0.02 available. Inspect campaign budgets first.",
        );
        run = {
          version: 1,
          id: crypto.randomUUID(),
          started_at: new Date().toISOString(),
          environment: {
            storage: runtime.storage,
            transport: runtime.transport,
          },
          plan: {
            campaign: c.id,
            screen: fresh.screens[0].id,
            keys: [crypto.randomUUID(), crypto.randomUUID()],
            events: Array.from({ length: 4 }, () => crypto.randomUUID()),
          },
          steps: stages.map(() => ({ done: false })),
          current: 0,
        };
        save();
      }
      const step = async (i, fn) => {
        if (run.steps[i].done) return;
        run.current = i;
        message = stages[i][2];
        update();
        await fn();
        run.steps[i] = { done: true, verified_at: new Date().toISOString() };
        save();
        update();
      };
      await step(0, async () => {
        run.holds = [];
        for (const key of run.plan.keys)
          run.holds.push(
            await request(
              "/api/v1/reservations",
              {
                campaign_id: run.plan.campaign,
                screen_id: run.plan.screen,
                cost_micros: 10000,
              },
              { "Idempotency-Key": key },
            ),
          );
        const receipt = (i, h, v = 1) => ({
          schema_version: v,
          event_id: run.plan.events[i],
          reservation_id: h.id,
          screen_id: h.screen_id,
          played_at: h.created_at + 1,
          duration_ms: 10000,
        });
        run.payload = {
          receipts: [
            receipt(0, run.holds[0]),
            receipt(1, run.holds[0]),
            receipt(2, run.holds[1], 99),
            { unexpected_field: "malformed_neighbor" },
          ],
        };
        run.correction = receipt(3, run.holds[1]);
      });
      await step(1, async () => {
        run.batch = await batch(run.payload);
      });
      await step(2, async () => {
        run.replay = await batch(run.payload);
        assert(
          run.replay.replayed === 3 &&
            run.replay.results
              .slice(0, 3)
              .every((r, i) => r.id === run.batch.results[i].id),
          "Retry did not preserve all receipt identities.",
        );
      });
      await step(3, async () => {
        run.initial = await observe(
          run.batch.results.slice(0, 3).map((r) => r.id),
        );
        const statuses = run.initial
          .slice(0, 2)
          .map((e) => e.delivery.status)
          .sort()
          .join(",");
        assert(
          statuses === "duplicate,settled",
          "Expected one settlement and one duplicate for play A.",
        );
        assert(
          run.initial[2].delivery.status === "quarantined" &&
            run.initial[2].delivery.reason === "unsupported_schema" &&
            run.initial[2].reservation?.state === "held",
          "Unsupported evidence did not preserve the held budget.",
        );
      });
      await step(4, async () => {
        run.corrected = await request("/api/v1/receipts", run.correction);
      });
      await step(5, async () => {
        run.final = await observe([
          ...run.batch.results.slice(0, 3).map((r) => r.id),
          run.corrected.id,
        ]);
        assert(
          run.final[3].delivery.status === "settled",
          "Corrected evidence did not settle play B.",
        );
        assert(
          run.final[2].delivery.schema_version === 99 &&
            run.final[2].delivery.status === "quarantined",
          "Original quarantine was changed.",
        );
        assert(
          run.final
            .slice(0, 2)
            .map((e) => e.delivery.status)
            .sort()
            .join(",") === "duplicate,settled",
          "Play A outcome changed.",
        );
        const reservations = new Map(
          run.final.map((e) => [e.reservation?.id, e.reservation]),
        );
        assert(
          reservations.size === 2 &&
            [...reservations.values()].every(
              (r) => r?.state === "settled" && r.cost_micros === 10000,
            ),
          "Expected exactly two settled one-cent reservations.",
        );
        run.completed_at = new Date().toISOString();
      });
      message =
        "All six stages verified against persisted receipt and reservation records.";
      await refresh();
    } catch (e) {
      message =
        e.message +
        " Resume keeps the same identities; existing work is not discarded.";
      if (run) {
        run.last_error = message;
        save();
      }
    } finally {
      running = false;
      update();
    }
  }
  document.addEventListener("click", async (e) => {
    if (e.target.closest("[data-proof-run]")) {
      await execute();
      return;
    }
    if (e.target.closest("[data-proof-export]") && run) {
      const blob = new Blob(
        [
          JSON.stringify(
            {
              disclaimer:
                "Synthetic local demonstration; not Vistar certification or a production SLO. Snapshots describe observations at their recorded times.",
              ...run,
            },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      );
      const url = URL.createObjectURL(blob),
        a = document.createElement("a");
      a.href = url;
      a.download = `afterglow-proof-${run.id}.json`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    }
    const b = e.target.closest("[data-proof-evidence]");
    if (b && run) {
      const row = (run.final || run.initial || [])[
        Number(b.dataset.proofEvidence)
      ];
      if (!row) return;
      openDialog(
        `<div class="dialog-eyebrow">PERSISTED EVIDENCE / ${esc(row.delivery.id)}</div><h2>${esc(title(row.delivery.status))}</h2><p>${esc(title(row.delivery.reason))}</p><dl class="detail-grid"><div><dt>RESERVATION STATE</dt><dd>${esc(row.reservation?.state || "Missing")}</dd></div><div><dt>SERVER PRICE</dt><dd>${money(row.reservation?.cost_micros)}</dd></div><div><dt>OUTBOX STATE</dt><dd>${esc(row.outbox_state)}</dd></div><div><dt>PROCESSING ATTEMPTS</dt><dd>${num(row.delivery.attempts)}</dd></div></dl><p>Observed ${esc(new Date(row.observed_at).toLocaleString())}. Dispatch acknowledgement can lag the committed decision.</p><div class="code-block">${esc(JSON.stringify(row, null, 2))}</div>`,
      );
    }
  });
  return { render };
})();
