.PHONY: build test install

build:
	go build -o bin/tapioca ./cmd/tapioca

test:
	go test ./...

install:
	go install ./cmd/tapioca

