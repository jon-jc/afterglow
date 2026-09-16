"use strict";
// A generation belongs to one saved demo dataset, not one browser tab. Keep it
// fixed for this document so a background proof cannot write into a newer run.
const demoSession = (() => {
  const generationKey = "afterglow.demo-generation.v1";
  const broadcastKey = "afterglow.demo-reset.v1";
  const runKeys = ["afterglow.integration-proof.v1", "afterglow.foot-traffic.v2"];
  let generation = null, invalidated = false;
  try { generation = sessionStorage.getItem(generationKey); } catch {}
  function remember(next) {
    generation = next;
    try { sessionStorage.setItem(generationKey, next); } catch {}
  }
  function clear(next) {
    for (const key of runKeys) {
      try { sessionStorage.removeItem(key); } catch {}
    }
    remember(next);
  }
  function reload(next, broadcast = false) {
    if (invalidated) return;
    invalidated = true;
    clear(next);
    if (broadcast) {
      try { localStorage.setItem(broadcastKey, next); } catch {}
    }
    location.reload();
  }
  async function send(path, options = {}, resetting = false) {
    if (invalidated) throw new Error("Demo data was cleared. Reloading this page.");
    const headers = new Headers(options.headers);
    if (generation) headers.set("X-Demo-Generation", generation);
    const response = await fetch(path, { ...options, headers });
    if (invalidated) throw new Error("Demo data was cleared. Reloading this page.");
    const next = response.headers.get("X-Demo-Generation");
    if (next && (!resetting || !response.ok)) {
      if (generation && generation !== next) {
        reload(next);
        throw new Error("Demo data was cleared. Reloading this page.");
      }
      remember(next);
    }
    return response;
  }
  window.addEventListener("storage", (e) => {
    if (e.key === broadcastKey && e.newValue && generation && e.newValue !== generation)
      reload(e.newValue);
  });
  return { fetch: send, active: () => !invalidated, reset: next => reload(next, true) };
})();

document.addEventListener("DOMContentLoaded", () => {
  const dialog = document.querySelector("#reset-demo-dialog");
  const open = document.querySelector("#reset-demo");
  const confirm = document.querySelector("#confirm-demo-reset");
  const cancel = document.querySelector("#cancel-demo-reset");
  const notice = document.querySelector("#reset-demo-notice");
  let pending = false;
  open.addEventListener("click", () => {
    notice.textContent = "";
    dialog.showModal();
    cancel.focus();
  });
  cancel.addEventListener("click", () => dialog.close());
  dialog.addEventListener("cancel", e => { if (pending) e.preventDefault(); });
  confirm.addEventListener("click", async () => {
    if (pending) return;
    pending = true;
    confirm.disabled = cancel.disabled = true;
    confirm.textContent = "Clearing…";
    notice.textContent = "Finishing active work and clearing the demo…";
    try {
      const response = await demoSession.fetch("/api/demo/reset", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: true }),
        signal: AbortSignal.timeout(15000),
      }, true);
      const result = await response.json();
      if (!response.ok || !result.cleared || !result.generation)
        throw new Error(result.detail || "The reset could not be confirmed.");
      demoSession.reset(result.generation);
    } catch (error) {
      notice.textContent = `${error.message} Refresh to check the current state before trying again.`;
      pending = false;
      confirm.disabled = cancel.disabled = false;
      confirm.textContent = "Clear all demo data";
    }
  });
});
