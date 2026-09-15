# Production startup and release hardening

Protected mode now requires explicit PostgreSQL, tenant identity and API credentials. It performs a read-only schema-version/checksum check instead of startup DDL. Missing metadata, modified migration source or a newer schema prevents service startup. Local demo mode retains automatic setup.

Adds the explicit `cmd/migrate` deployment command and includes it in the container. Version 1 adopts the known existing schema transactionally without clearing business data, records a line-ending-normalized checksum, and serializes PostgreSQL migration ownership with an advisory lock. Startup and migration database operations have bounded deadlines. Baseline adoption is not an arbitrary schema-drift detector.

Adds optional previous-key support for a controlled credential rotation window; tests prove acceptance during overlap and rejection after removal. Updates gRPC to 1.83.2 and x/crypto to 0.56.0 for available advisory fixes. Aligns the module minimum and container builder to the scanned Go 1.27.0 toolchain. Adds pinned govulncheck to CI.

## Verification

- Local Go tests and vet passed; full Linux race suite passed after dependency upgrades.
- PostgreSQL core race tests passed, including schema verification on a connection configured read-only and refusal to migrate through that connection.
- Migration tests cover missing metadata, preservation of seeded records, repeated application, checksum mismatch and newer schema refusal.
- Authentication tests cover current/previous keys, missing/wrong credentials and previous-key revocation.
- All 13 isolated HTTP/SQL/worker smoke checks passed after changes, exact final spend 43,650,000 micros.
- govulncheck v1.8.0 reports no reachable vulnerabilities and no advisories in imported packages. One module-only advisory remains for unused x/crypto/openpgp, which has no available fix. This is a known-vulnerability analysis, not a security certification.

See `docs/production-readiness.md` for the deployment sequence, controls and remaining release gates. No cloud deployment, production traffic/SLO result, real partner certification, backup restore drill or image build is claimed. Hosted CI remains affected by the account billing/spending-limit restriction.
