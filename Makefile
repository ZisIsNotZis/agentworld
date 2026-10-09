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

# test/race timeouts are compute ceilings, so P15 requires them to come from measured
# compute, not from an arbitrary cap. Measured on the gate machine at revision c1d7810
# (12 logical CPUs, go1.24.13; log .tmp/vs1-gate-final.log):
#   non-race suite wall 992.11s, internal/world alone 985.44s;
#   race set wall 3552.14s (per-package, sequential), internal/world alone 2671.00s.
#
# test: 30m (1800s) = 1.81x the measured full-suite wall and 1.83x the slowest package, so it
# already meets the P15 >=1.5x-measured standard and is left at its measured-derived bound.
#
# race: the former 30m (1800s) pin was BELOW measured compute -- 0.51x the 3552.14s full race
# set and 0.67x its slowest package (internal/world, 2671.00s ~= 44.5 min) -- so it aborted the
# green suite with a false "test timed out" before it could finish. The bound below is derived
# from that measurement: 90m (5400s) = 1.52x the measured full-set wall and 2.02x the slowest
# package. See docs/VALIDATION.md, P15 findings.
test:
	$(GO) test -timeout 30m ./...

race:
	$(GO) test -race -timeout 90m ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

bench:
	$(GO) test ./internal/component -run '^$$' -bench 'Benchmark(PointRead|Scan1000|Project1000)$$' -benchmem

check: fmt-check fmt-check-failure-probe test race vet build
