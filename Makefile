# Quality gates. `make check` is the single gate run by the pre-push hook.
GOLANGCI_VERSION := v2.13.2
GOFUMPT_VERSION  := v0.12.0
RULES_VERSION    := v1.2.3

GOBIN := $(shell go env GOPATH)/bin

.PHONY: all check fmt fmt-fix vet lint test build run tools hooks bench e2e clean

all: check

## check: the full gate. Anything that fails here blocks a push.
check: fmt vet lint test build

## fmt: formatting must already be clean; never rewrites files in the gate.
fmt:
	@out=$$(gofmt -l . ); \
	if [ -n "$$out" ]; then \
		echo "gofmt: these files are not formatted:"; echo "$$out"; \
		echo "run 'make fmt-fix'"; exit 1; \
	fi
	@bin=$$(command -v gofumpt || echo "$(GOBIN)/gofumpt"); \
	if [ -x "$$bin" ]; then \
		out=$$($$bin -l . ); \
		if [ -n "$$out" ]; then \
			echo "gofumpt: these files are not formatted:"; echo "$$out"; \
			echo "run 'make fmt-fix'"; exit 1; \
		fi; \
	else \
		echo "gofumpt not installed - run 'make tools' (gofmt check passed)"; \
	fi

## fmt-fix: rewrite files to satisfy the formatters.
fmt-fix:
	gofmt -w .
	@bin=$$(command -v gofumpt || echo "$(GOBIN)/gofumpt"); \
	if [ -x "$$bin" ]; then $$bin -w . ; else echo "gofumpt not installed - run 'make tools'"; fi

vet:
	go vet ./...

## lint: fails loudly when the linter is absent. A gate that cannot run is not a gate.
lint:
	@bin=$$(command -v golangci-lint || echo "$(GOBIN)/golangci-lint"); \
	if [ ! -x "$$bin" ]; then \
		echo "golangci-lint not installed."; \
		echo "run 'make tools' to install $(GOLANGCI_VERSION)"; \
		exit 1; \
	fi; \
	$$bin run ./...

test:
	go test -race -cover ./...

build:
	go build ./...

run:
	go run ./cmd/typhon

## tools: install the pinned versions the gate depends on.
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
	go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	go install github.com/BattlesnakeOfficial/rules/cli/battlesnake@$(RULES_VERSION)

## hooks: .git/hooks is not versioned, so install our tracked hook into it.
hooks:
	install -m 0755 scripts/pre-push .git/hooks/pre-push
	@echo "installed .git/hooks/pre-push -> make check"

## bench: Go microbenchmarks. The tournament harness is `cmd/typhon-bench`.
bench:
	go test -run '^$$' -bench . -benchmem ./...

## e2e: one local game per supported ruleset against a running server.
##   Requires 'make tools' and './typhon' listening on PORT.
E2E_RULES ?= standard royale constrictor wrapped
e2e:
	@bin=$$(command -v battlesnake || echo "$(GOBIN)/battlesnake"); \
	if [ ! -x "$$bin" ]; then echo "battlesnake CLI not installed - run 'make tools'"; exit 1; fi; \
	for g in $(E2E_RULES); do \
		echo "=== $$g ==="; \
		$$bin play -W 11 -H 11 --name typhon --url http://localhost:8080 -g $$g || exit 1; \
	done

clean:
	go clean
	rm -f coverage.out typhon typhon-bench app
