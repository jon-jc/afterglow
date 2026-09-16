// Browser regression: creates six one-cent synthetic holds; exercises the real worker.
(async () => {
  const check = (ok, message) => {
    if (!ok) throw new Error(message);
  };
  const wait = async (fn) => {
    for (let i = 0; i < 200; i++) {
      if (await fn()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Timed out");
  };
  location.hash = "intake";
  await wait(() => document.querySelector("#intake-example"));
  const load = async (kind) => {
    const select = document.querySelector("#intake-example");
    select.value = kind;
    select.dispatchEvent(new Event("change", { bubbles: true }));
    document.querySelector('[data-intake="example"]').click();
    await wait(
      () => !document.querySelector('[data-intake="example"]').disabled,
    );
    check(
      document
        .querySelector("#intake-status")
        .textContent.includes("Example ready"),
      document.querySelector("#intake-status").textContent,
    );
    return JSON.parse(document.querySelector("#intake-payload").value);
  };
  const original = window.fetch;
  const reservationCalls = [];
  let drop = true;
  window.fetch = async (...args) => {
    const response = await original(...args);
    if (String(args[0]) === "/api/v1/reservations") {
      reservationCalls.push(args[1]);
      if (drop) {
        drop = false;
        throw new TypeError("Simulated lost reservation response");
      }
    }
    return response;
  };
  try {
    document.querySelector('[data-intake="example"]').click();
    await wait(
      () => !document.querySelector('[data-intake="example"]').disabled,
    );
    check(
      document
        .querySelector("#intake-status")
        .textContent.includes("same request key"),
      "Retry guidance",
    );
    const first = await load("clean");
    check(
      reservationCalls.length === 2 &&
        reservationCalls[0].headers["Idempotency-Key"] ===
          reservationCalls[1].headers["Idempotency-Key"],
      "Lost response reused key",
    );
    const second = await load("clean");
    check(
      second.receipts[0].reservation_id === first.receipts[0].reservation_id &&
        reservationCalls.length === 2,
      "Repeated loads reuse hold",
    );
  } finally {
    window.fetch = original;
  }
  const outcomes = [];
  for (const kind of [
    "clean",
    "partial",
    "duplicate",
    "replay",
    "schema",
    "window",
  ]) {
    const payload = await load(kind);
    document.querySelector("#intake-submit").click();
    await wait(() => !document.querySelector("#intake-submit").disabled);
    check(
      document
        .querySelector("#intake-status")
        .textContent.includes("Response received"),
      "Batch accepted " + kind,
    );
    if (kind === "partial")
      check(
        document
          .querySelector("#intake-results")
          .textContent.includes("HTTP 207"),
        "Partial status",
      );
    if (kind === "replay")
      check(
        document
          .querySelector("#intake-results")
          .textContent.includes("1 replayed"),
        "Identity replay",
      );
    const ids = [
      ...new Set(payload.receipts.map((r) => r.event_id).filter(Boolean)),
    ];
    let decisions;
    await wait(async () => {
      const snapshot = await (await fetch("/api/v1/snapshot")).json();
      decisions = snapshot.deliveries.filter((d) => ids.includes(d.event_id));
      return (
        decisions.length === ids.length &&
        decisions.every((d) =>
          ["settled", "duplicate", "quarantined"].includes(d.status),
        )
      );
    });
    const counts = (status) =>
      decisions.filter((d) => d.status === status).length;
    if (["schema", "window"].includes(kind))
      check(counts("quarantined") === 1, "Quarantine " + kind);
    else
      check(
        counts("settled") === 1 &&
          counts("duplicate") ===
            (kind === "partial" ? 1 : kind === "duplicate" ? 4 : 0),
        "Single settlement " + kind,
      );
    outcomes.push({ kind, decisions: decisions.map((d) => d.status) });
  }
  return { passed: true, outcomes, reservationRetry: true, holdReuse: true };
})();
