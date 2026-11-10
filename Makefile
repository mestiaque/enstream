.PHONY: test vet race build run
GO ?= $(shell command -v go >/dev/null && echo go || echo ./scripts/go.sh)

test:
	$(GO) test ./...
vet:
	$(GO) vet ./...
race:
	CGO_ENABLED=1 $(GO) test -race ./...
build:
	./scripts/build.sh
run:
	./scripts/dev.sh
