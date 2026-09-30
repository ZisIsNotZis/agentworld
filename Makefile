GO ?= go
GOFMT ?= gofmt

.PHONY: fmt fmt-check fmt-check-failure-probe test race vet build bench check

fmt:
	$(GO) fmt ./...

fmt-check:
	@files="$$(find . -path './.git' -prune -o -path './.tmp' -prune -o -type f -name '*.go' -print)"; \
	output="$$($(GOFMT) -l $$files)" || { status=$$?; echo "gofmt failed" >&2; exit $$status; }; \
	test -z "$$output" || { printf '%s\n' "$$output"; exit 1; }

fmt-check-failure-probe:
	@if $(MAKE) --no-print-directory GOFMT=false fmt-check >/dev/null 2>&1; then \
		echo "fmt-check accepted a failing formatter" >&2; exit 1; \
	fi

test:
	$(GO) test ./...

race:
	$(GO) test -race -timeout 30m ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

bench:
	$(GO) test ./internal/component -run '^$$' -bench 'Benchmark(PointRead|Scan1000|Project1000)$$' -benchmem

check: fmt-check fmt-check-failure-probe test race vet build
