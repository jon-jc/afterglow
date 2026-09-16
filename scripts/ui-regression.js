// Run in the local demo browser using agent-browser eval --stdin.
// Reservation responses are mocked: this does not create real holds.
(async () => {
  const checks = [];
  const check = (ok, name) => {
    if (!ok) throw new Error(name);
    checks.push(name);
  };
  const waitFor = async (fn) => {
    for (let i = 0; i < 100; i++) {
      if (fn()) return;
      await new Promise((r) => setTimeout(r, 50));
    }
    throw new Error("Timed out waiting for UI");
  };
  const originalFetch = window.fetch;
  const requests = [];
  try {
    location.hash = "campaigns";
    await waitFor(() => document.querySelector('[data-action="reserve"]'));
    document.querySelector('[data-action="reserve"]').click();
    const form = document.querySelector("#reserve-form");
    const price = form.elements.price;
    price.value = "1x2";
    check(
      !price.checkValidity(),
      "Price pattern rejects a non-decimal separator",
    );
    price.value = "1.50";
    window.fetch = async (url, options) => {
      if (url !== "/api/v1/reservations") return originalFetch(url, options);
      requests.push({
        key: options.headers["Idempotency-Key"],
        body: options.body,
      });
      if (requests.length < 4) throw new TypeError("Simulated lost response");
      return new Response("{}", { status: 200 });
    };
    const submit = async () => {
      form.requestSubmit();
      await waitFor(() => !form.querySelector("button").disabled);
    };
    await submit();
    check(
      document
        .querySelector("#reserve-error")
        .textContent.includes("lost response"),
      "Request errors appear inside the dialog",
    );
    await submit();
    check(
      requests.length === 2 &&
        requests[0].key === requests[1].key &&
        requests[0].body === requests[1].body,
      "Ambiguous reservation retries preserve identity and payload",
    );
    price.value = "2.00";
    await submit();
    check(
      requests[2].key !== requests[1].key,
      "Changed reservation input gets a new identity",
    );
    await submit();
    check(
      requests[3].key === requests[2].key &&
        !document.querySelector("#detail-dialog").open,
      "Successful retry closes the dialog without changing identity",
    );
    document.querySelector('[data-action="reserve"]').click();
    const pendingForm = document.querySelector("#reserve-form");
    let finishRequest;
    window.fetch = (url, options) =>
      url === "/api/v1/reservations"
        ? new Promise((resolve) => {
            finishRequest = resolve;
          })
        : originalFetch(url, options);
    pendingForm.requestSubmit();
    await waitFor(() => finishRequest);
    document.querySelector("#detail-dialog").close();
    document.querySelector('[data-action="reserve"]').click();
    const replacement = document.querySelector("#reserve-form");
    finishRequest(new Response("{}", { status: 200 }));
    await waitFor(() => !pendingForm.querySelector("button").disabled);
    check(
      replacement !== pendingForm &&
        document.querySelector("#detail-dialog").open,
      "An old request cannot close a newly opened dialog",
    );
    document.querySelector("#detail-dialog").close();
    window.fetch = originalFetch;
    location.hash = "ledger";
    await waitFor(() => document.querySelector("#ledger-search"));
    document.querySelector('[data-filter="settled"]').focus();
    await new Promise((r) => setTimeout(r, 2300));
    check(
      document.activeElement.dataset.filter === "settled",
      "Polling preserves keyboard focus on controls",
    );
    const search = document.querySelector("#ledger-search");
    search.value = "no-such-receipt-ux-test";
    search.dispatchEvent(new Event("input", { bubbles: true }));
    search.focus();
    check(
      document
        .querySelector("#ledger-results")
        .textContent.includes("No matching receipts"),
      "Search has a truthful empty state",
    );
    location.hash = "constructor";
    await waitFor(
      () =>
        document.querySelector("h1")?.textContent ===
        "Every play. Accounted for.",
    );
    check(
      true,
      "Invalid prototype-like routes recover even with search focused",
    );
    location.hash = "lab";
    await waitFor(
      () =>
        document
          .querySelector('[data-view="lab"]')
          .getAttribute("aria-current") === "page",
    );
    document.querySelector(".skip-link").click();
    check(
      location.hash === "#lab" && document.activeElement.id === "main",
      "Skip link moves focus without changing the current view",
    );
    return { passed: checks.length, checks };
  } finally {
    window.fetch = originalFetch;
    document.querySelector("#detail-dialog").close();
  }
})();
