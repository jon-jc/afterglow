// Read-only browser checks against the running local application.
(async () => {
  const checks = [];
  const check = (v, s) => {
    if (!v) throw new Error(s);
    checks.push(s);
  };
  const wait = async (f) => {
    for (let i = 0; i < 100; i++) {
      if (f()) return;
      await new Promise((r) => setTimeout(r, 50));
    }
    throw new Error("UI timeout");
  };
  const original = window.fetch;
  try {
    location.hash = "overview";
    await wait(() => document.querySelector(".overview-next"));
    document.dispatchEvent(
      new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true }),
    );
    const dialog = document.querySelector("#navigation-dialog"),
      input = document.querySelector("#navigation-search");
    check(
      dialog.open && document.activeElement === input,
      "Keyboard shortcut opens focused navigation",
    );
    input.value = "receipts";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    check(
      document.querySelectorAll("#navigation-results a").length === 1,
      "Navigation search filters by task",
    );
    input.dispatchEvent(
      new KeyboardEvent("keydown", {
        key: "Enter",
        bubbles: true,
        cancelable: true,
      }),
    );
    await wait(() => document.querySelector("#ledger-search"));
    check(
      !dialog.open && location.hash === "#ledger",
      "Enter opens the matching page",
    );
    check(
      document.querySelectorAll("#ledger-results tbody tr").length <= 20,
      "Ledger page contains at most 20 receipts",
    );
    const next = document.querySelector('[data-ledger-page="next"]');
    if (!next.disabled) {
      const first = document.querySelector("[data-delivery]").dataset.delivery;
      next.click();
      check(
        document.querySelector("[data-delivery]").dataset.delivery !== first,
        "Pagination advances to different receipts",
      );
    }
    const search = document.querySelector("#ledger-search");
    search.value = "definitely-absent";
    search.dispatchEvent(new Event("input", { bubbles: true }));
    check(
      document
        .querySelector("#ledger-results")
        .textContent.includes("No matching receipts"),
      "Search empty state is actionable",
    );
    document.querySelector("[data-clear-filters]").click();
    check(
      document.querySelector("#ledger-search").value === "" &&
        document.activeElement.id === "ledger-search",
      "Clear filters resets search and focuses input",
    );
    const sort = document.querySelector("#ledger-sort");
    sort.value = "oldest";
    sort.dispatchEvent(new Event("change", { bubbles: true }));
    const sorted = state.deliveries
      .slice()
      .sort(
        (a, b) => a.received_at - b.received_at || a.id.localeCompare(b.id),
      );
    check(
      document.querySelector("[data-delivery]").dataset.delivery ===
        sorted[0].id,
      "Oldest-first sort uses receipt time",
    );
    document.querySelector("#live-toggle").click();
    check(
      document.querySelector("#live-toggle").getAttribute("aria-pressed") ===
        "true" && !document.querySelector("#freshness-banner").hidden,
      "Paused updates are clearly labeled",
    );
    let reads = 0;
    window.fetch = (...args) => {
      if (args[0] === "/api/v1/snapshot") reads++;
      return original(...args);
    };
    await new Promise((r) => setTimeout(r, 2200));
    check(reads === 0, "Pause stops automatic snapshot fetches");
    document.querySelector("#refresh").click();
    await wait(() => reads > 0 && !refreshing);
    check(reads === 1, "Manual refresh still works while paused");
    window.fetch = async () => {
      throw new TypeError("Simulated connection interruption");
    };
    await refresh();
    check(
      document
        .querySelector("#freshness-banner")
        .textContent.includes("Connection interrupted") &&
        document.querySelectorAll("#ledger-results tbody tr").length > 0,
      "Connection failure labels retained data",
    );
    window.fetch = original;
    await refresh();
    document.querySelector("#live-toggle").click();
    await wait(() => !refreshing);
    check(
      document.querySelector("#freshness-banner").hidden,
      "Successful reconnection and resume clear stale warning",
    );
    location.hash = "recovery";
    await wait(() => document.querySelector(".recovery-guidance"));
    check(
      document.querySelector('[data-filter="failed"]') &&
        document.querySelector('[data-filter="quarantined"]'),
      "Recovery has distinct failure filters",
    );
    document.querySelector('[data-filter="failed"]').click();
    check(
      [...document.querySelectorAll("#ledger-results tbody tr")].every((r) =>
        r.querySelector(".badge.failed"),
      ),
      "Recovery filter limits decisions correctly",
    );
    check(
      document.title === "Recovery queue — Afterglow",
      "Page title follows navigation",
    );
    document.querySelector("[data-clear-filters]").click();
    check(
      document.documentElement.scrollWidth <= innerWidth,
      "Page stays inside viewport",
    );
    return { passed: checks.length, checks };
  } finally {
    window.fetch = original;
    if (livePaused) document.querySelector("#live-toggle").click();
  }
})();
