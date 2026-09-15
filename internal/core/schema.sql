CREATE TABLE IF NOT EXISTS campaigns (
 tenant TEXT NOT NULL, id TEXT NOT NULL, name TEXT NOT NULL,
 budget BIGINT NOT NULL CHECK (budget > 0), reserved BIGINT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
 spent BIGINT NOT NULL DEFAULT 0 CHECK (spent >= 0),
 PRIMARY KEY(tenant,id), CHECK (reserved + spent <= budget)
);
CREATE TABLE IF NOT EXISTS screens (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, market TEXT NOT NULL, format TEXT NOT NULL,
 lat DOUBLE PRECISION NOT NULL, lng DOUBLE PRECISION NOT NULL
);
CREATE TABLE IF NOT EXISTS reservations (
 tenant TEXT NOT NULL, id TEXT NOT NULL, campaign TEXT NOT NULL, screen TEXT NOT NULL REFERENCES screens(id),
 cost BIGINT NOT NULL CHECK(cost > 0), state TEXT NOT NULL CHECK(state IN ('held','settled','released')),
 idem_key TEXT NOT NULL, fingerprint TEXT NOT NULL, created BIGINT NOT NULL, expires BIGINT NOT NULL,
 PRIMARY KEY(tenant,id), UNIQUE(tenant,idem_key),
 FOREIGN KEY(tenant,campaign) REFERENCES campaigns(tenant,id)
);
CREATE TABLE IF NOT EXISTS deliveries (
 tenant TEXT NOT NULL, id TEXT NOT NULL, event_id TEXT NOT NULL, fingerprint TEXT NOT NULL,
 payload TEXT NOT NULL, status TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '',
 created BIGINT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(tenant,id), UNIQUE(tenant,event_id)
);
CREATE TABLE IF NOT EXISTS outbox (
 tenant TEXT NOT NULL, id TEXT NOT NULL, state TEXT NOT NULL DEFAULT 'pending',
 owner TEXT NOT NULL DEFAULT '', lease_until BIGINT NOT NULL DEFAULT 0,
 next_at BIGINT NOT NULL DEFAULT 0, attempts INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(tenant,id), FOREIGN KEY(tenant,id) REFERENCES deliveries(tenant,id)
);
CREATE INDEX IF NOT EXISTS outbox_due ON outbox(state,next_at,lease_until);
CREATE INDEX IF NOT EXISTS deliveries_recent ON deliveries(tenant,created DESC);
CREATE INDEX IF NOT EXISTS reservations_expiry ON reservations(state,expires);
CREATE TABLE IF NOT EXISTS audit (
 id TEXT PRIMARY KEY, tenant TEXT NOT NULL, kind TEXT NOT NULL, resource TEXT NOT NULL,
 detail TEXT NOT NULL, created BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS audit_recent ON audit(tenant,created DESC);
