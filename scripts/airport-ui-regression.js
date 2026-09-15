// Run with agent-browser eval --stdin on a live Afterglow + airport-service pair.
// Read-only: this test never reserves budget or submits receipts.
(async () => {
  const checks = [];
  const check = (ok, name) => {
    if (!ok) throw new Error(name);
    checks.push(name);
  };
  const wait = async (fn) => {
    for (let i = 0; i < 150; i++) {
      if (fn()) return;
      await new Promise((r) => setTimeout(r, 50));
    }
    throw new Error("Timed out waiting for airport view");
  };
  location.hash = "airports";
  await wait(() => document.querySelectorAll(".airport-point").length > 100);
  const count = document.querySelectorAll(".airport-point").length;
  check(
    count > 100,
    "Full catalog renders map points, not only the first page",
  );
  const first = document.querySelector(".airport-directory tbody button")
    .dataset.airport;
  document.querySelector('[data-airport-page="next"]').click();
  await wait(
    () =>
      document.querySelector(".airport-directory tbody button")?.dataset
        .airport !== first,
  );
  check(
    document.querySelectorAll(".airport-point").length === count,
    "Pagination changes rows without dropping map locations",
  );
  const region = document.querySelector("#airport-region");
  region.value = "US-WA";
  region.dispatchEvent(new Event("change", { bubbles: true }));
  await wait(
    () =>
      document.querySelectorAll(".airport-point").length < count &&
      document.querySelector("#airport-loading").textContent === "",
  );
  check(
    [...document.querySelectorAll(".airport-directory tbody tr")].every((r) =>
      r.textContent.includes("US-WA"),
    ),
    "Region filter returns Washington airports",
  );
  const input = document.querySelector("#airport-search");
  input.value = "KSEA";
  input.dispatchEvent(new Event("input", { bubbles: true }));
  input.focus();
  await wait(() => document.querySelectorAll(".airport-point").length === 1);
  check(document.activeElement === input, "Searching preserves input focus");
  document
    .querySelector('.airport-directory button[data-airport="KSEA"]')
    .click();
  check(
    document.querySelector("dialog").open &&
      document.querySelector("#dialog-title").textContent.includes("Seattle"),
    "Airport detail opens with real source data",
  );
  document.querySelector("dialog").close();
  input.value = "no-airport-matches-this";
  input.dispatchEvent(new Event("input", { bubbles: true }));
  await wait(() =>
    document
      .querySelector("#airport-results")
      .textContent.includes("No airports match"),
  );
  check(true, "Empty search gives a useful recovery message");
  input.value = "";
  input.dispatchEvent(new Event("input", { bubbles: true }));
  region.value = "";
  region.dispatchEvent(new Event("change", { bubbles: true }));
  await wait(
    () => document.querySelectorAll(".airport-point").length === count,
  );
  return { passed: checks.length, airports: count, checks };
})();
