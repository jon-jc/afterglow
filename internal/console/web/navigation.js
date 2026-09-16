"use strict";
(() => {
  const pages = [
    ["foot-traffic", "◈", "Foot traffic", "Aggregate observation windows, coverage and comparisons"],
    [
      "assurance",
      "✓",
      "Campaign assurance",
      "Full-history delivery and budget reconciliation",
    ],
    ["overview", "◫", "Overview", "Campaign performance and delivery health"],
    ["campaigns", "▤", "Campaigns", "Budgets, reservations and playback"],
    ["intake", "⇄", "Partner intake", "Submit and inspect playback batches"],
    [
      "ledger",
      "≋",
      "Delivery ledger",
      "Search receipts and settlement decisions",
    ],
    [
      "recovery",
      "↻",
      "Recovery queue",
      "Inspect failures and quarantined evidence",
    ],
    ["proof", "◎", "Live proof", "Verify the integration from end to end"],
    ["lab", "⌘", "Failure lab", "Exercise controlled faults and recovery"],
    [
      "architecture",
      "◇",
      "Architecture",
      "Understand guarantees and system design",
    ],
  ];
  const dialog = $("#navigation-dialog"),
    input = $("#navigation-search");
  function results() {
    const q = input.value.toLowerCase().trim();
    const matches = pages.filter((p) => p.join(" ").toLowerCase().includes(q));
    $("#navigation-results").innerHTML =
      matches
        .map(
          (p) =>
            `<a href="#${p[0]}" ${active === p[0] ? 'aria-current="page"' : ""}><span aria-hidden="true">${p[1]}</span><div><strong>${p[2]}</strong><small>${p[3]}</small></div><span aria-hidden="true">${active === p[0] ? "●" : "↗"}</span></a>`,
        )
        .join("") ||
      '<p class="empty">No pages match. Try “receipts” or “budget”.</p>';
  }
  function open() {
    if ($("#detail-dialog").open || dialog.open) return;
    input.value = "";
    results();
    dialog.showModal();
    input.focus();
  }
  $("#open-navigation").addEventListener("click", open);
  $("#close-navigation").addEventListener("click", () => dialog.close());
  input.addEventListener("input", results);
  dialog.addEventListener("click", (e) => {
    if (e.target.closest("a")) {
      dialog.close();
      setTimeout(() => $("#main").focus(), 0);
    }
    if (e.target === dialog) {
      const b = dialog.getBoundingClientRect();
      if (
        e.clientX < b.left ||
        e.clientX > b.right ||
        e.clientY < b.top ||
        e.clientY > b.bottom
      )
        dialog.close();
    }
  });
  dialog.addEventListener("keydown", (e) => {
    const links = [...$("#navigation-results").querySelectorAll("a")];
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const i = links.indexOf(document.activeElement);
      if (links.length)
        links[
          (i + (e.key === "ArrowDown" ? 1 : links.length - 1) + links.length) %
            links.length
        ].focus();
    }
    if (e.key === "Enter" && e.target === input && links.length) {
      e.preventDefault();
      links[0].click();
    }
  });
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      open();
    }
  });
})();
