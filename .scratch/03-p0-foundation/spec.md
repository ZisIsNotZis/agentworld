# P0 foundation implementation

Status: done

## Issue

The P0 architecture is documented but untested. The first implementation must validate that built-in and dynamic components can share one logical protocol without losing type safety, authority boundaries, numerical projection, deterministic behavior, or practical hot-path access.

## Objective

Implement slices S0–S2: repository quality gates, foundational IDs/time/version/value types, schema registry, one built-in component, one dynamic component, a unified read/scan/project protocol, and a shared conformance suite with initial benchmarks.

## Acceptance criteria

- The implementation follows `docs/P0-SPEC.md` and ADR 0005.
- Go is treated as a provisional implementation choice and its version is recorded.
- Built-in and dynamic components pass the same behavior conformance suite.
- Missing, zero, unknown, and inapplicable states are distinguishable where the slice exposes them.
- Consumers do not branch on physical storage implementation.
- Batch access and metric projection are supported.
- Tests, race checks where applicable, static analysis, build, and benchmarks have reproducible commands.
- The implementation plan and measured limitations are documented without overstating P0 completion.

Need-review: yes
Need-test-cases: yes

## Blockers

None.

## Comments

- 2026-09-17 — assistant: Opened after the operator authorized autonomous continuation and delegated implementation. One sequential writer owns the shared checkout; fresh review follows implementation.
- 2026-09-17 — worker: Implemented S0–S2 on the uncommitted working tree. Go 1.24.13 linux/amd64 was obtained without system installation by downloading and extracting Ubuntu packages under ignored `.tmp/go-toolchain`; the module baseline is Go 1.24.0 and has no third-party dependencies.

## Validation evidence

Commands use `GO="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin/go"` because Go is not installed on the ambient `PATH`.

- `"$GO" version` — passed: `go version go1.24.13 linux/amd64`.
- `gofmt -l internal` with the extracted toolchain on `PATH` — passed with no output.
- `"$GO" test ./...` — passed: `agentworld/internal/component` and `agentworld/internal/sim`.
- `"$GO" test -race ./...` — passed both packages.
- `"$GO" vet ./...` — passed with no diagnostics.
- `"$GO" build ./...` — passed with no diagnostics.
- `make GO="$GO" GOFMT="$(dirname "$GO")/gofmt" check` — passed all non-benchmark Make gates.
- No required validation was skipped.

Focused benchmark command:

```sh
"$GO" test ./internal/component -run '^$' -bench 'Benchmark(PointRead|Scan1000|Project1000)$' -benchmem -count=3
```

Baseline on Linux amd64, Intel Core i7-6850K at 3.60 GHz, 12 logical CPUs:

| Benchmark | Three measurements | Memory |
|---|---|---|
| Point read / energy | 1600, 1745, 1629 ns/op | 576 B/op, 9 allocs/op |
| Point read / fatigue | 1682, 1683, 1424 ns/op | 576 B/op, 9 allocs/op |
| 1,000-row scan / energy | 130321, 160158, 185966 ns/op | 206769 B/op, 16 allocs/op |
| 1,000-row scan / fatigue | 286079, 275292, 284739 ns/op | 206769 B/op, 16 allocs/op |
| 1,000-row projection / energy | 1360628, 1288908, 1396108 ns/op | 865493–865494 B/op, 2016 allocs/op |
| 1,000-row projection / fatigue | 1492454, 1454349, 1441276 ns/op | 865493–865494 B/op, 2016 allocs/op |

These are initial measurements, not thresholds. Projection currently prioritizes contract clarity over allocation reduction; optimization remains evidence-driven later work.

## Review follow-up

- 2026-09-28 — assistant: Independent fresh review after fixes found no remaining issues (review run `a6864c13-ca24-4c36-8470-f1f4d615d1e5`; verdict OK with notes). Its scope was current untracked source because this repository has no HEAD baseline. Parent reran `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check && git diff --check` successfully. The governed host `gate: "go test ./..."` rejected both worker runs solely because ambient Go is absent; those failures are not claimed as successful host acceptance. Subsequent workflow gates must use an explicit toolchain PATH or omit the host gate and validate separately.

## Review follow-up evidence

- 2026-09-20 — worker: Addressed all five fresh-review findings. `Reader.Describe` now redacts physical storage metadata and built-in/dynamic descriptors are tested for logical equality; `Has` now requires the snapshot authority and rejects absent or cross-reader capabilities; `fmt-check` preserves formatter failures and `check` runs a failure probe; all floating-point value constructors canonicalize signed zero where applicable; nested record schemas reject duplicate field names. Updated the P0 protocol signature accordingly.
- `PATH="$PWD/.tmp/go-toolchain/root/usr/lib/go-1.24/bin:$PATH" timeout 180s make check` — passed: format check and its failure probe, unit tests, race tests, vet, and build.
- `go test -count=1 ./internal/component -run 'Test(ReadProtocolConformance|AuthorityIsBoundToItsSnapshot|DynamicAndBuiltinEnergyReadProtocolEquivalence|RegistryRejectsInvalidDescriptors)$'` — passed.
- `go test -count=1 ./internal/sim -run '^TestSignedZeroCanonicalization$'` — passed.
- `make --no-print-directory GOFMT=false fmt-check` — failed as expected; the shell probe treated that failure as success evidence.
- `git diff --check` — passed with no output. Because this initial repository has no commits and its files are untracked, an additional per-file `git diff --no-index --check /dev/null <file>` sweep was run and passed.
- No review-fix validation was skipped; no files were staged.
