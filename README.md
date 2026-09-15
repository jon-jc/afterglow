# Afterglow

A digital out-of-home delivery and reconciliation engineering demo.

Built with Go. Synthetic campaigns and inventory only. Independent portfolio project; not affiliated with Clear Channel Outdoor or Vistar Media.

## Milestones

1. Transactional campaign budget and playback reconciliation.
2. Durable event delivery, Pub/Sub transport, and failure recovery.
3. Operations console, deployment assets, and interview walkthrough.

## Run

Requires Go 1.25 or later. Open this directory in GoLand and select the **Afterglow** run configuration, or:

```sh
go run ./cmd/afterglow
```

The local API listens on `http://127.0.0.1:8090`. SQLite persists in `data/afterglow.db`. Seed campaigns and screens are synthetic. Demo mode is restricted to a loopback listener.

```sh
go test ./...
go vet ./...
```

[Architecture decisions](docs/architecture.md)
