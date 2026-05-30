.PHONY: fmt test vet build run-example

fmt:
	go fmt ./...

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o ./bin/ak-historian ./cmd/ak-historian

run-example:
	go run ./cmd/ak-historian fetch \
		--market futures-um \
		--symbols BTCUSDT \
		--interval 1m \
		--period monthly \
		--start 2024-01 \
		--end 2024-01 \
		--dry-run
