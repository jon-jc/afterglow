// Local demo only. Creates one two-cent run, deliberately drops a committed
// batch response, then resumes the exact saved identities through recovery.
(async () => {
  const checks = [];
  const check = (value, text) => {
    if (!value) throw new Error(text);
    checks.push(text);
  };
  const wait = async (predicate) => {
    for (let i = 0; i < 300; i++) {
      if (predicate()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw new Error("Timed out waiting for proof");
  };
  const key = "afterglow.integration-proof.v1";
  const saved = () => JSON.parse(sessionStorage.getItem(key));
  const original = window.fetch;
  try {
    location.hash = "proof";
    await wait(() => document.querySelector("[data-proof-run]"));
    check(
      !document.querySelector('nav a[href="/interview-notes.html"]'),
      "Interview guide removed from navigation",
    );
    check(
      (await original("/interview-notes.html")).status === 404,
      "Study guide is not served by the product",
    );
    let lost = false;
    window.fetch = async (url, options) => {
      const response = await original(url, options);
      if (url === "/api/v1/receipts/batch" && !lost) {
        lost = true;
        await response.clone().json();
        throw new TypeError("Simulated response lost after server acceptance");
      }
      return response;
    };
    document.querySelector("[data-proof-run]").click();
    await wait(
      () =>
        !document.querySelector("[data-proof-run]").disabled &&
        document
          .querySelector(".proof-status")
          .textContent.includes("Simulated response lost"),
    );
    const interrupted = saved();
    check(
      interrupted.steps[0].done && !interrupted.steps[1].done,
      "Unknown response does not claim success",
    );
    check(
      interrupted.payload.receipts.length === 4,
      "Payload persisted before mutation",
    );
    window.fetch = original;
    document.querySelector("[data-proof-run]").click();
    await wait(() => saved()?.steps.every((s) => s.done));
    const done = saved();
    check(
      done.id === interrupted.id &&
        JSON.stringify(done.plan) === JSON.stringify(interrupted.plan),
      "Resume preserves run and idempotency keys",
    );
    check(
      done.batch.replayed === 3,
      "Committed but lost batch recovered without new receipts",
    );
    check(
      done.replay.replayed === 3,
      "Intentional retry replays all accepted events",
    );
    check(
      done.final.filter((e) => e.delivery.status === "settled").length === 2,
      "Exactly two settlement decisions",
    );
    check(
      done.final[2].delivery.schema_version === 99 &&
        done.final[2].delivery.status === "quarantined",
      "Recovery preserves original bad evidence",
    );
    check(
      new Set(done.final.map((e) => e.reservation.id)).size === 2,
      "Four events refer to exactly two reservations",
    );
    document.querySelector('[data-proof-evidence="2"]').click();
    check(
      document.querySelector("#detail-dialog").open &&
        document
          .querySelector("#dialog-content")
          .textContent.includes("unsupported_schema"),
      "Receipt evidence dialog shows actual stored record",
    );
    document.querySelector("#detail-dialog").close();
    location.hash = "overview";
    await wait(() => !document.querySelector("#integration-proof"));
    location.hash = "proof";
    await wait(() => document.querySelector("#integration-proof"));
    check(
      document.querySelectorAll(".proof-step.complete").length === 6,
      "Verified run survives navigation",
    );
    let exported;
    const createURL = URL.createObjectURL;
    try {
      URL.createObjectURL = (blob) => {
        exported = blob;
        return createURL(blob);
      };
      document.querySelector("[data-proof-export]").click();
    } finally {
      URL.createObjectURL = createURL;
    }
    const report = JSON.parse(await exported.text());
    check(
      report.id === done.id &&
        report.final.length === 4 &&
        report.disclaimer.includes("Synthetic"),
      "Evidence export includes observed records and scope",
    );
    check(
      document.documentElement.scrollWidth <= window.innerWidth,
      "No viewport overflow",
    );
    return { passed: checks.length, checks, run_id: done.id };
  } finally {
    window.fetch = original;
  }
})();
