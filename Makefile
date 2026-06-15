.PHONY: build test fmt lint check image

build:
	go build ./cmd/leader-elector

test:
	go test ./...

fmt:
	goimports -w .

lint:
	golangci-lint run ./...

check: test lint
	go vet ./...

image:
	nix build .#image
