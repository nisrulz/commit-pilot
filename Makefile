.PHONY: help build install uninstall clean setup setup-lmstudio setup-ollama vet test test-cover test-live

BINARY := commit-pilot
PROVIDER ?= lmstudio

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build                 Build the binary ($(BINARY))"
	@echo "  install               Build and copy to ~/go/bin"
	@echo "  uninstall             Remove from ~/go/bin"
	@echo "  clean                 Remove the binary"
	@echo "  setup-lmstudio        Setup LMStudio"
	@echo "  setup-ollama          Setup Ollama"
	@echo "  vet                   Run go vet (static analysis)"
	@echo "  test                  Run unit tests"
	@echo "  test-cover            Run tests with coverage report"
	@echo "  test-live             Run live integration test (requires AI provider)"

build:
	@go build -o $(BINARY) ./src/
	@printf "  \033[1;32m✓\033[0m Built $(BINARY)\n"

install: build
	@mkdir -p ~/go/bin
	@cp $(BINARY) ~/go/bin/$(BINARY)
	@echo "  ✓ Installed to ~/go/bin/$(BINARY)"
	@scripts/setup-path.sh

vet:
	@go vet ./...
	@echo "  ✓ go vet passed"

test:
	@go run ./scripts/testtable

test-cover:
	@go test -count=1 -coverprofile=/tmp/cov.out -coverpkg=./src/lib/,./src/lib/provider/ ./tests/...
	@go tool cover -func=/tmp/cov.out | tail -1
	@rm /tmp/cov.out

test-live:
	@go run ./scripts/livetable

setup:
	@scripts/setup-$(PROVIDER).sh

setup-lmstudio:
	@scripts/setup-lmstudio.sh

setup-ollama:
	@scripts/setup-ollama.sh

clean:
	@rm -f $(BINARY)
	@echo "  ✓ Removed $(BINARY)"

uninstall:
	@rm -f ~/go/bin/$(BINARY)
	@echo "  ✓ Removed $(BINARY) from ~/go/bin"
