GO ?= go
STATICCHECK ?= staticcheck

.PHONY: build fmt fmt-check mod-check test test-race vet lint check terminal

build:
	$(GO) build -o bin/lingcell ./cmd/lingcell
fmt:
	gofmt -w cmd internal
fmt-check:
	@files="$$(gofmt -l cmd internal)"; \
	if [ -n "$$files" ]; then printf 'Run make fmt for:\n%s\n' "$$files"; exit 1; fi
mod-check:
	$(GO) mod tidy -diff
test:
	$(GO) test ./...
test-race:
	$(GO) test -race ./...
vet:
	$(GO) vet ./...
lint:
	$(STATICCHECK) ./...
check: fmt-check mod-check vet test-race
terminal: build
	python3 scripts/verify_terminal.py --binary bin/lingcell
	python3 scripts/verify_tasks.py --binary bin/lingcell
	python3 scripts/verify_runtime.py --binary bin/lingcell
