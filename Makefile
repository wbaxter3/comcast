.PHONY: lint fmt test

lint:
	golangci-lint run ./...
	golangci-lint fmt --diff --diff-colored=false

fmt:
	golangci-lint run --fix ./...
	golangci-lint fmt

test:
	go test ./...
