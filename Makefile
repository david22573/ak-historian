.PHONY: fmt test vet build verify verify-source-integrity build-prospective build-r1p5 build-r1p5r verify-r1p5 verify-r1p5r run-example preflight-backfill backfill-core backfill-expansion backfill-all prove-backfill

fmt:
	go fmt ./...

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o ./bin/ak-historian ./cmd/ak-historian

build-prospective:
	GOWORK=off go build -buildvcs=false -trimpath -ldflags "-X github.com/david22573/ak-historian/internal/prospective.CollectorSourceCommit=$$(git rev-parse HEAD)" -o ./bin/ak-historian ./cmd/ak-historian

build-r1p5:
	GOWORK=off go build -buildvcs=false -trimpath -ldflags "-X github.com/david22573/ak-historian/internal/r1p5.BackfillSourceCommit=$${R1P5_SOURCE_COMMIT:-$$(git rev-parse HEAD)}" -o ./bin/ak-historian-r1p5 ./cmd/ak-historian

build-r1p5r:
	GOWORK=off go build -buildvcs=false -trimpath -ldflags "-X github.com/david22573/ak-historian/internal/r1p5r.SourceSealCommit=$${R1P5R_SOURCE_SEAL_COMMIT:-$$(git rev-parse HEAD)}" -o ./bin/ak-historian-r1p5r ./cmd/ak-historian

verify-r1p5:
	./scripts/verify_pr4b0_r1p5_source.sh

verify-r1p5r:
	./scripts/verify_pr4b0_r1p5r_source.sh

verify:
	./scripts/verify.sh

verify-source-integrity:
	./scripts/verify_source_integrity.sh
	./scripts/test_source_integrity.sh

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
