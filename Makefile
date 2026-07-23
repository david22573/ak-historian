.PHONY: fmt test vet build verify run-example preflight-backfill backfill-core backfill-expansion backfill-all prove-backfill

fmt:
	go fmt ./...

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o ./bin/ak-historian ./cmd/ak-historian

verify:
	./scripts/verify.sh

run-example:
	go run ./cmd/ak-historian fetch \
		--market futures-um \
		--symbols BTCUSDT \
		--interval 1m \
		--period monthly \
		--start 2024-01 \
		--end 2024-01 \
		--dry-run

preflight-backfill:
	./scripts/preflight_backfill.sh

backfill-core:
	./scripts/backfill_core_futures_1m.sh

backfill-expansion:
	./scripts/backfill_expansion_futures_1m.sh

backfill-all:
	./scripts/backfill_all_futures_1m.sh

prove-backfill:
	./scripts/prove_backfill.sh

prove-link-dataset:
	./scripts/prove_link_research_dataset.sh
