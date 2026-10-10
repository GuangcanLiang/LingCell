.PHONY: build test check terminal
build:
	go build -o bin/lingcell ./cmd/lingcell
test:
	go test ./...
check:
	go vet ./...
	go test -race ./...
terminal: build
	python3 scripts/verify_terminal.py --binary bin/lingcell
