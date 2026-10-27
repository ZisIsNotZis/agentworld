# Contributing to Agent World

Agent World is a research codebase: a deterministic civilization simulator plus the experiment
runners that test claims about it. Contributions are welcome when they keep that property — a change
has to be runnable, recorded and reproducible, not just plausible.

## Scope

Useful contributions:

- bug fixes, determinism breaks, replay or checkpoint mismatches, conservation violations;
- performance work with a measured before/after on this machine (the Makefile records where the
  compute bounds come from);
- new tests that pin a behaviour the frozen specs already declare;
- documentation that corrects a fact or removes a duplicate, closer to its source.

Ask first (open an issue) before changing a frozen band, fixture, metric, policy ref or mechanism
axis: those are run identity, and changing one invalidates every recorded comparison. Tuning a band
so a run passes is not a fix; report the falsified band instead.

## How to run the checks

Go 1.24 or newer, standard library only.

```sh
make check   # fmt-check + fmt probe + test + race + vet + build
make bench   # component benchmarks, not part of check
```

`make test` and `make race` carry compute bounds derived from measured wall time, so a full run is
long (the race suite is over an hour on the gate machine). Run the package you touched while
iterating, e.g. `go test ./internal/world/ -run TestName`, and the full gates before opening a pull
request. Every report is expected to show its own measurements; a timing or byte-count claim without
a captured run is not evidence.

## Pull requests

- Small, single-purpose commits, Conventional Commit subjects (`fix:`, `test:`, `docs:`, `feat:`).
- Say what was measured and what the result was, including a negative or falsified outcome. If an
  acceptance criterion is not met, say so plainly; a documented failure is an accepted outcome here,
  a quietly relaxed criterion is not.
- Keep generated output under the ignored `.tmp/` directory. Do not commit bundles, run logs or
  build artifacts.
- Do not add CI configuration: checks are run locally and their evidence is attached to the change.

## Issues

State the revision, the exact command, the seed and the observed versus expected output. A
reproduction beats a description. Claims about behaviour under a real language model need the
authorised spend gate; an unconfigured run is hermetic by design and its results are labelled as
such.

## License

Agent World is released under the GNU Affero General Public License v3.0. By contributing you agree
that your contribution is licensed under the same terms.
