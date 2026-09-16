// Run against a local demo. Creates synthetic reservations and verifies exact pricing.
(async () => {
  const check = (ok, m) => {
    if (!ok) throw new Error(m);
  };
  const wait = async (fn) => {
    for (let i = 0; i < 150; i++) {
      if (await fn()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Timed out");
  };
  await wait(() => document.querySelector("#intake-price"));
  const setPrice = (v) => {
    const el = document.querySelector("#intake-price");
    el.value = v;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  };
  const load = async () => {
    document.querySelector('[data-intake="example"]').click();
    await wait(
      () => !document.querySelector('[data-intake="example"]').disabled,
    );
  };
  const original = window.fetch;
  let calls = 0;
  window.fetch = (...args) => {
    if (String(args[0]) === "/api/v1/reservations") calls++;
    return original(...args);
  };
  try {
    for (const v of ["", "0", "-1", "1.001", "1e2", "1000.01"]) {
      setPrice(v);
      await load();
      check(calls === 0, "Invalid price created hold: " + v);
    }
    setPrice("0.29");
    await load();
    check(
      document.querySelector("#intake-status").textContent.includes("$0.29"),
      "Actual price in confirmation",
    );
    const first = JSON.parse(document.querySelector("#intake-payload").value)
      .receipts[0].reservation_id;
    const snapshot = await (await fetch("/api/v1/snapshot")).json();
    check(
      snapshot.reservations.find((r) => r.id === first).cost_micros === 290000,
      "Exact integer price",
    );
    await load();
    check(calls === 1, "Same price reuses hold");
    setPrice("0.37");
    await load();
    const second = JSON.parse(document.querySelector("#intake-payload").value)
      .receipts[0].reservation_id;
    check(second !== first && calls === 2, "Changed price creates new hold");
    location.hash = "overview";
    await wait(() => !document.querySelector("#intake-price"));
    location.hash = "intake";
    await wait(() => document.querySelector("#intake-price"));
    check(
      document.querySelector("#intake-price").value === "0.37",
      "Navigation preserves price",
    );
    document.querySelector("#intake-submit").click();
    await wait(() => !document.querySelector("#intake-submit").disabled);
    await wait(async () => {
      const s = await (await fetch("/api/v1/snapshot")).json();
      return s.reservations.some(
        (r) =>
          r.id === second && r.state === "settled" && r.cost_micros === 370000,
      );
    });
    return {
      passed: true,
      invalidPrices: 6,
      exactPrice: true,
      reuse: true,
      newPrice: true,
      persisted: true,
      settledMicros: 370000,
    };
  } finally {
    window.fetch = original;
  }
})();
