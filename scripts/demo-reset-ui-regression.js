// Run three times with agent-browser eval in an ISOLATED local demo. Pass one
// resets completed work, pass two resets an active proof, and pass three checks
// that no late work reappeared. Never use a dataset you want to retain.
(async () => {
  const key = "afterglow.reset-ui-check";
  const checks = [];
  const check = (condition, message) => {
    if (!condition) throw new Error(message);
    checks.push(message);
  };
  const wait = async condition => {
    for (let n = 0; n < 300; n++) {
      if (condition()) return;
      await new Promise(resolve => setTimeout(resolve, 50));
    }
    throw new Error("UI did not finish the operation");
  };
  const snapshot = () => request("/api/v1/snapshot");
  const proofDone = () => {
    const saved = JSON.parse(sessionStorage.getItem("afterglow.integration-proof.v1") || "null");
    return saved?.steps?.every(s => s.done);
  };
  await wait(() => !document.querySelector("#reset-demo").hidden);
  if (!sessionStorage.getItem(key)) {
    location.hash = "overview";
    await wait(() => document.querySelector("[data-overview-proof]"));
    document.querySelector("[data-overview-proof]").click();
    await wait(proofDone);
    check(proofDone(), "An actual six-stage proof is saved before reset");
    location.hash = "foot-traffic";
    await wait(() => document.querySelector("[data-traffic-load]")?.disabled === false);
    document.querySelector("[data-traffic-load]").click();
    await wait(() => document.querySelector("[data-traffic-replay]")?.disabled === false);
    check(!!sessionStorage.getItem("afterglow.foot-traffic.v2"), "A real sample batch is remembered before reset");
    const before = await snapshot();
    document.querySelector("#reset-demo").click();
    check(document.activeElement.id === "cancel-demo-reset", "Confirmation focuses the non-destructive choice");
    document.querySelector("#cancel-demo-reset").click();
    check(!document.querySelector("#reset-demo-dialog").open, "Cancel closes the confirmation");
    check((await snapshot()).deliveries.length === before.deliveries.length, "Cancel preserves saved receipts");
    sessionStorage.setItem(key, sessionStorage.getItem("afterglow.demo-generation.v1"));
    document.querySelector("#reset-demo").click();
    setTimeout(() => document.querySelector("#confirm-demo-reset").click(), 100);
    return { phase: "reset-scheduled", checks };
  }
  const previous = sessionStorage.getItem(key);
  if (previous.startsWith("active:")) {
    check(!sessionStorage.getItem("afterglow.integration-proof.v1"), "Interrupted proof cannot save stale evidence after reset");
    check(!sessionStorage.getItem("afterglow.foot-traffic.v2"), "Interrupted run leaves no remembered feed");
    const fresh = await snapshot();
    check(fresh.deliveries.length === 0 && fresh.reservations.length === 0 && fresh.audit.length === 0, "Active proof leaves no late work after reset");
    check(!(await request("/api/v1/runtime")).paused, "Reset resumes the paused worker");
    sessionStorage.removeItem(key);
    return { phase: "passed", checks };
  }
  check(sessionStorage.getItem("afterglow.demo-generation.v1") !== previous, "Reset changed the dataset generation");
  check(!sessionStorage.getItem("afterglow.integration-proof.v1"), "Saved proof evidence was removed");
  check(!sessionStorage.getItem("afterglow.foot-traffic.v2"), "Remembered batch inputs were removed");
  const empty = await snapshot();
  check(empty.deliveries.length === 0 && empty.reservations.length === 0 && empty.audit.length === 0, "All ledger activity is empty");
  check(empty.campaigns.every(c => c.spent_micros === 0 && c.reserved_micros === 0), "All campaign budgets are fully available");
  check(empty.campaigns.length === 3 && empty.screens.length === 8, "Seed inventory is still ready to use");
  await wait(() => document.querySelector("[data-traffic-replay]")?.disabled === true);
  check(document.querySelector("[data-traffic-replay]").disabled, "Stale batch replay is unavailable");
  check((await request("/api/v1/foot-traffic/report")).missing_windows === 24, "Measurement returns an empty report");
  location.hash = "overview";
  await wait(() => document.querySelector("[data-overview-proof]"));
  document.querySelector("[data-overview-proof]").click();
  await wait(proofDone);
  const fresh = await snapshot();
  check(fresh.counts.settled === 2 && fresh.counts.duplicate === 1 && fresh.counts.quarantined === 1, "A fresh live proof completes with exactly its own four receipts");
  check(fresh.campaigns.reduce((n, c) => n + c.spent_micros, 0) === 20000, "Fresh proof settles exactly two cents");
  await request("/api/demo/control", { paused: true });
  document.querySelector("[data-proof-run]").click();
  await wait(() => {
    const run = JSON.parse(sessionStorage.getItem("afterglow.integration-proof.v1") || "null");
    return run?.steps?.filter(s => s.done).length === 3;
  });
  check((await snapshot()).counts.accepted > 0, "A second proof has durable work waiting on the paused worker");
  sessionStorage.setItem(key, "active:" + sessionStorage.getItem("afterglow.demo-generation.v1"));
  document.querySelector("#reset-demo").click();
  setTimeout(() => document.querySelector("#confirm-demo-reset").click(), 100);
  return { phase: "active-reset-scheduled", checks };
})()
