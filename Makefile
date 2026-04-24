.PHONY: run test fmt vet build docker-build

run:
	go run ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

build:
	go build -o bin/nordikcsaaapi ./cmd/server

docker-build:
	docker build -t nordikcsaaapi:local .
