// Inject reservation failure responses without creating holds.
(async () => {
  const check = (ok, m) => {
    if (!ok) throw new Error(m);
  };
  const wait = async (fn) => {
    for (let i = 0; i < 150; i++) {
      if (fn()) return;
      await new Promise((r) => setTimeout(r, 50));
    }
    throw new Error("Timed out");
  };
  location.hash = "intake";
  await wait(() => document.querySelector("#intake-price"));
  const original = window.fetch;
  let mode = 409;
  const keys = [];
  window.fetch = async (url, options) => {
    if (String(url) != "/api/v1/reservations") return original(url, options);
    keys.push(options.headers["Idempotency-Key"]);
    if (mode === "lost") throw new TypeError("Lost response");
    if (mode === "expired")
      return new Response(
        JSON.stringify({
          id: "expired-test",
          state: "released",
          expires_at: 1,
        }),
        { status: 200 },
      );
    return new Response(
      JSON.stringify({ detail: "Injected reservation rejection" }),
      { status: mode },
    );
  };
  const load = async () => {
    document.querySelector('[data-intake="example"]').click();
    await wait(
      () => !document.querySelector('[data-intake="example"]').disabled,
    );
  };
  try {
    await load();
    check(
      !document.querySelector("#intake-price").disabled,
      "Budget rejection unlocks price",
    );
    mode = "lost";
    await load();
    check(
      document.querySelector("#intake-price").disabled,
      "Unknown outcome locks original price",
    );
    check(keys[0] !== keys[1], "Definite rejection allows new request");
    mode = 503;
    await load();
    check(
      keys[1] === keys[2] && document.querySelector("#intake-price").disabled,
      "Server failure retains request",
    );
    mode = "expired";
    await load();
    check(keys[2] === keys[3], "Recovery reuses original key");
    check(
      !document.querySelector("#intake-price").disabled &&
        document
          .querySelector("#intake-status")
          .textContent.includes("no longer available"),
      "Expired recovery unlocks without generating invalid example",
    );
    const p = document.querySelector("#intake-price");
    p.value = "0";
    p.dispatchEvent(new Event("input", { bubbles: true }));
    await load();
    check(
      p.getAttribute("aria-invalid") === "true" &&
        document
          .querySelector("#intake-price-error")
          .textContent.includes("minimum"),
      "Accessible inline price error",
    );
    return { passed: 6 };
  } finally {
    window.fetch = original;
  }
})();
