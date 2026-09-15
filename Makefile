.PHONY: run test verify build infra
run:
	go run ./cmd/afterglow
test:
	go test -race -count=1 ./...
verify:
	go vet ./...
	go test ./...
	node scripts/smoke.mjs
build:
	go build -trimpath -o bin/afterglow ./cmd/afterglow
infra:
	docker compose up -d
