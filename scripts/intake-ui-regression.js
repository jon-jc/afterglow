// Run in the local demo browser. Creates one $0.01 hold and settles it.
(async () => {
  const check = (ok, message) => {
    if (!ok) throw new Error(message);
  };
  const wait = async (fn) => {
    for (let i = 0; i < 100; i++) {
      if (fn()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Timed out");
  };
  location.hash = "intake";
  await wait(() => document.querySelector("#intake-payload"));
  check(
    !document.querySelector('[data-view="airports"]'),
    "Airport navigation removed",
  );
  document.querySelector('[data-intake="example"]').click();
  await wait(() =>
    document
      .querySelector("#intake-status")
      .textContent.includes("Example ready"),
  );
  const input = document.querySelector("#intake-payload");
  const payload = input.value;
  const sample = JSON.parse(payload);
  check(sample.receipts.length === 3, "Example generated");
  input.focus();
  await new Promise((r) => setTimeout(r, 2300));
  check(
    document.activeElement === input && input.value === payload,
    "Polling preserves editor",
  );
  document.querySelector("#intake-submit").click();
  await wait(() =>
    document
      .querySelector("#intake-status")
      .textContent.includes("Response received"),
  );
  check(
    document.querySelector("#intake-results").textContent.includes("HTTP 207"),
    "Partial success visible",
  );
  check(
    document
      .querySelector("#intake-results")
      .textContent.includes("2 accepted · 1 rejected"),
    "Per-item results",
  );
  document.querySelector("#intake-submit").click();
  await wait(() =>
    document
      .querySelector("#intake-status")
      .textContent.includes("Response received"),
  );
  check(
    document
      .querySelector("#intake-results")
      .textContent.includes("2 replayed"),
    "Unchanged retry deduplicated",
  );
  await new Promise((r) => setTimeout(r, 1500));
  const after = await (await fetch("/api/v1/snapshot")).json();
  const decisions = after.deliveries.filter((d) =>
    sample.receipts.slice(0, 2).some((r) => r.event_id === d.event_id),
  );
  check(
    decisions.length === 2 &&
      decisions.some((d) => d.status === "settled") &&
      decisions.some((d) => d.status === "duplicate"),
    "One settlement, one duplicate",
  );
  location.hash = "overview";
  await wait(() => !document.querySelector("#intake-payload"));
  location.hash = "intake";
  await wait(() => document.querySelector("#intake-payload"));
  check(
    document.querySelector("#intake-payload").value === payload,
    "Draft survives navigation",
  );
  return { passed: 8, decisions: decisions.map((d) => d.status) };
})();
