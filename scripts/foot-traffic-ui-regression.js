// Run in the local demo browser. Imports synthetic samples; never changes budgets.
(async () => {
  const checks = [];
  const check = (condition, message) => {
    if (!condition) throw new Error(message);
    checks.push(message);
  };
  const wait = async (condition) => {
    for (let n = 0; n < 160; n++) {
      if (condition()) return;
      await new Promise(resolve => setTimeout(resolve, 50));
    }
    throw new Error("Foot-traffic UI did not finish the operation");
  };
  const ready = () => document.querySelector("[data-traffic-load]")?.disabled === false;
  location.hash = "foot-traffic";
  await wait(ready);
  const select = async (id) => {
    const field = document.querySelector("#traffic-sample");
    field.value = id;
    field.dispatchEvent(new Event("change", { bubbles: true }));
    await wait(ready);
  };
  const load = async () => {
    document.querySelector("[data-traffic-load]").click();
    await wait(ready);
    check(!document.querySelector("#traffic-report .error-banner"), "Sample import returns a report");
  };
  const metrics = () => document.querySelector("#traffic-report .metrics").textContent;
  for (const id of ["baseline", "busy", "gaps", "commuter", "retail", "threshold", "interruption", "quiet"]) {
    await select(id);
    await load();
    const before = metrics();
    check(!document.querySelector("[data-traffic-replay]").disabled, `${id}: replay enabled after import`);
    document.querySelector("[data-traffic-replay]").click();
    await wait(ready);
    check(document.querySelector("#traffic-notice").textContent.includes("Replay verified"), `${id}: replay confirmed`);
    check(metrics() === before, `${id}: replay leaves reported counts unchanged`);
    document.querySelector(".traffic-zone details").open = true;
    document.querySelector("[data-traffic-refresh]").click();
    await wait(ready);
    check(metrics() === before && document.querySelector("#traffic-notice").textContent.includes("Report refreshed"), `${id}: refresh confirmed without adding data`);
    check(document.querySelector(".traffic-zone details").open, `${id}: refresh preserves expanded table`);
    if (id === "busy") check(document.querySelectorAll(".traffic-bar.missing, .traffic-bar.suppressed").length === 0, "High activity has fully reportable pairs");
    if (id === "gaps") check(document.querySelectorAll(".traffic-bar.missing").length === 18, "Missing coverage remains missing across both days");
    if (id === "quiet") check(document.querySelectorAll(".traffic-bar.suppressed").length === 36, "Small windows remain suppressed across both days");
    if (id === "threshold") check(document.querySelectorAll(".traffic-bar.suppressed").length === 24, "Boundary sample hides 19 and releases 20");
    if (id === "interruption") check(document.querySelectorAll(".traffic-bar.missing").length === 16, "Interrupted hours remain missing across both days");
  }
  location.hash = "overview";
  await wait(() => !document.querySelector("#foot-traffic"));
  location.hash = "foot-traffic";
  await wait(ready);
  check(!document.querySelector("[data-traffic-replay]").disabled, "Replay survives navigating away and returning");
  check(document.querySelector("#traffic-sample").value === "quiet", "Selection survives navigation");

  const originalFetch = window.fetch;
  let failReport = true, loseImportResponse = true, reportCalls = 0;
  window.fetch = async (...args) => {
    const url = String(args[0]);
    if (url.includes("/foot-traffic/report")) {
      reportCalls++;
      if (failReport) {
        failReport = false;
        return new Response(JSON.stringify({ detail: "Simulated report interruption" }), { status: 503 });
      }
    }
    const response = await originalFetch(...args);
    if (url.includes("/foot-traffic/batches") && loseImportResponse) {
      loseImportResponse = false;
      throw new TypeError("Simulated lost import response");
    }
    return response;
  };
  try {
    const before = metrics();
    document.querySelector("[data-traffic-refresh]").click();
    await wait(ready);
    check(!!document.querySelector("#traffic-report .error-banner") && metrics() === before, "Failed refresh keeps last successful report");
    document.querySelector("#refresh").click();
    await wait(() => ready() && !document.querySelector("#traffic-report .error-banner"));
    check(reportCalls >= 2, "Top-bar refresh requests the foot-traffic report");
    document.querySelector("[data-traffic-load]").click();
    await wait(ready);
    check(document.querySelector("[data-traffic-replay]").textContent === "Retry last batch", "Lost response offers exact-input retry");
    document.querySelector("[data-traffic-replay]").click();
    await wait(ready);
    check(metrics() === before && document.querySelector("#traffic-notice").textContent.includes("Replay verified"), "Retry after lost response does not double-count");
  } finally {
    window.fetch = originalFetch;
  }
  const quietMetrics = metrics();
  const saved = () => JSON.parse(sessionStorage.getItem("afterglow.foot-traffic.v2"));
  let lastRandomID;
  for (let n = 0; n < 2; n++) {
    document.querySelector("[data-traffic-random]").click();
    await wait(ready);
    check(document.querySelector("#traffic-sample").value === "random", "Generator selects its own sample");
    const id = saved().attempts.random.batch.batch_id;
    check(id !== lastRandomID, "Generation receives a new batch identity");
    lastRandomID = id;
    const before = metrics();
    document.querySelector("[data-traffic-replay]").click();
    await wait(ready);
    check(metrics() === before && document.querySelector("#traffic-notice").textContent.includes("Replay verified"), "Random replay preserves the generated data");
  }
  document.querySelector("[data-traffic-empty]").click();
  await wait(ready);
  check(!!document.querySelector(".traffic-empty") && document.querySelectorAll(".traffic-chart").length === 0, "Empty batch removes all plotted observations");
  check(document.querySelector("[data-traffic-replay]").disabled && !saved().attempts.random, "Empty batch clears replay state and its saved copy");
  document.querySelector("[data-traffic-refresh]").click();
  await wait(ready);
  check(!!document.querySelector(".traffic-empty"), "Refresh cannot restore an emptied batch");
  await select("quiet");
  check(metrics() === quietMetrics, "Emptying random leaves another sample unchanged");
  document.querySelector("[data-traffic-empty]").click();
  await wait(ready);
  check(!!document.querySelector(".traffic-empty"), "A fixed sample can also be emptied");
  await load();
  check(!document.querySelector(".traffic-empty") && !document.querySelector("[data-traffic-replay]").disabled, "Load works again after Empty batch");
  check(document.documentElement.scrollWidth <= innerWidth, "No horizontal page overflow");
  return { passed: true, checks };
})();
