.PHONY: run test test-unit test-integration fmt vet build docker-build

run:
	go run ./cmd/server

test: test-unit test-integration

test-unit:
	go test ./...

test-integration:
	go test ./... -run Integration

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

build:
	go build -o bin/nordikcsaaapi ./cmd/server

docker-build:
	docker build -t nordikcsaaapi:local .
