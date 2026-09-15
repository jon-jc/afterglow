// Exercise the full HTTP -> SQL outbox -> worker -> read API flow on an isolated database.
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import net from "node:net";
import assert from "node:assert/strict";

const root = process.cwd(),
  dir = mkdtempSync(path.join(tmpdir(), "afterglow-e2e-"));
const binary = path.join(
  dir,
  process.platform === "win32" ? "afterglow.exe" : "afterglow",
);
execFileSync("go", ["build", "-o", binary, "./cmd/afterglow"], {
  stdio: "inherit",
});
const socket = net.createServer();
await new Promise((r) => socket.listen(0, "127.0.0.1", r));
const port = socket.address().port;
await new Promise((r) => socket.close(r));
const child = spawn(binary, [], {
  cwd: root,
  windowsHide: true,
  env: {
    ...process.env,
    ADDR: `127.0.0.1:${port}`,
    DEMO_MODE: "true",
    DATABASE_URL: path.join(dir, "smoke.db"),
    TRANSPORT: "local",
    ROLE: "all",
  },
});
let logs = "";
child.stdout.on("data", (b) => {
  logs = (logs + b).slice(-20000);
});
child.stderr.on("data", (b) => {
  logs = (logs + b).slice(-20000);
});
const url = `http://127.0.0.1:${port}`,
  checks = [];
async function req(route, body, headers = {}) {
  const r = await fetch(url + route, {
    method: body === undefined ? "GET" : "POST",
    headers: { "Content-Type": "application/json", ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return { status: r.status, data: await r.json() };
}
async function until(check, label, timeout = 25000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try {
      const v = await req("/api/v1/snapshot");
      if (v.status === 200 && check(v.data)) {
        checks.push(label);
        return v.data;
      }
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`Timed out: ${label}`);
}
async function scenario(kind) {
  const r = await req("/api/demo/scenario", { kind });
  assert.equal(r.status, 202, JSON.stringify(r));
  return r.data;
}
const spent = (s) => s.campaigns.reduce((n, c) => n + c.spent_micros, 0);
try {
  await until((s) => s.campaigns.length === 3, "boot and seed");
  const bad = await req("/api/v1/reservations", { tenant: "attacker" });
  assert.equal(bad.status, 400);
  checks.push("strict schema blocks tenant injection");
  await scenario("traffic");
  let s = await until(
    (s) => s.counts.settled === 12,
    "12 HTTP receipts settle asynchronously",
  );
  assert.equal(spent(s), 19950000);
  await scenario("duplicate");
  s = await until(
    (s) => s.counts.duplicate === 4,
    "new event IDs cannot double-charge a reservation",
  );
  assert.equal(spent(s), 21200000);
  const poison = await scenario("poison");
  s = await until(
    (s) => s.counts.quarantined === 1,
    "unsupported schema quarantined",
  );
  assert.equal(spent(s), 21200000);
  assert.equal(
    (await req(`/api/v1/deliveries/${poison.delivery_ids[0]}/replay`, {}))
      .status,
    202,
  );
  await until(
    (s) =>
      s.counts.quarantined === 1 &&
      s.deliveries.find((d) => d.id === poison.delivery_ids[0]).attempts >= 2,
    "replay preserves validation",
  );
  const crash = await scenario("crash");
  s = await until(
    (s) =>
      s.deliveries.find((d) => d.id === crash.delivery_ids[0])?.attempts >= 2,
    "lost acknowledgement redelivered",
  );
  assert.equal(spent(s), 22450000);
  await req("/api/demo/control", { paused: true });
  await scenario("traffic");
  s = await until(
    (s) => s.counts.accepted === 12,
    "paused worker retains durable backlog",
  );
  assert.equal(spent(s), 22450000);
  await req("/api/demo/control", { paused: false });
  s = await until((s) => !s.counts.accepted, "resumed worker drains backlog");
  assert.equal(spent(s), 42400000);
  const retry = await scenario("retry");
  await until(
    (s) =>
      s.deliveries.find((d) => d.id === retry.delivery_ids[0])?.status ===
      "failed",
    "five transient failures exhaust into recovery queue",
  );
  await req(`/api/v1/deliveries/${retry.delivery_ids[0]}/replay`, {});
  s = await until(
    (s) =>
      s.deliveries.find((d) => d.id === retry.delivery_ids[0])?.status ===
      "settled",
    "operator replay recovers transient failure",
  );
  assert.equal(spent(s), 43650000);
  const race = await req("/api/demo/scenario", { kind: "budget-race" });
  assert.equal(race.status, 200);
  assert.equal(race.data.accepted + race.data.rejected, 32);
  assert.ok(race.data.rejected > 0);
  s = (await req("/api/v1/snapshot")).data;
  for (const c of s.campaigns)
    assert.ok(c.spent_micros + c.reserved_micros <= c.budget_micros);
  checks.push("HTTP concurrency scenario preserves campaign cap");
  const input = {
      campaign_id: "cmp-cascade",
      screen_id: "sea-01",
      cost_micros: 500000,
    },
    headers = { "Idempotency-Key": "smoke-idempotency" };
  const first = await req("/api/v1/reservations", input, headers),
    again = await req("/api/v1/reservations", input, headers),
    conflict = await req(
      "/api/v1/reservations",
      { ...input, cost_micros: 600000 },
      headers,
    );
  assert.equal(first.status, 201);
  assert.equal(again.data.id, first.data.id);
  assert.equal(conflict.status, 409);
  checks.push("HTTP idempotent replay and conflict");
  mkdirSync(path.join(root, "artifacts"), { recursive: true });
  const report = {
    passed: true,
    checks,
    counts: s.counts,
    spent_micros: spent(s),
    database: dir,
  };
  writeFileSync(
    path.join(root, "artifacts", "smoke.json"),
    JSON.stringify(report, null, 2),
  );
  console.log(JSON.stringify(report, null, 2));
} catch (e) {
  console.error(logs);
  throw e;
} finally {
  child.kill();
}
