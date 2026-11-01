# AGENTS.md

Instructions for AI coding agents (and humans) working in this repository. Follow the Entur conventions in [entur/ai](https://github.com/entur/ai), in particular [CONVENTIONS.md](https://github.com/entur/ai/blob/main/CONVENTIONS.md) and [guides/reference/go.md](https://github.com/entur/ai/blob/main/guides/reference/go.md), with the repository rules below taking precedence. The design is [ADR 0001](doc/adr/0001-norwegian-calendar-design.adoc).

## Repository rules

- **Zero dependencies.** The root `go.mod` has no `require`, `replace`, `toolchain` or `tool` lines, also not for tests (no testify; a written deviation from the Entur Go guide). Development tools go in the nested `tools/` module.
- **No package state.** No exported or unexported package-level `var`, no `init`, no caches, no goroutines. Sentinel errors are constants of type `Error`. `gochecknoglobals` and `gochecknoinits` enforce this.
- **No clock, no implicit zone.** Never call `time.Now`, `time.LoadLocation` or use `time.Local` in library code, and never embed tzdata (tests may import `time/tzdata`).
- **Errors, not silent results.** Invalid input returns an error wrapping a sentinel; never a zero, empty or normalised result.
- **Rules are append-only.** Changing any answer needs a new `RulesVersion` and a new `rulesHistory` record (see CONTRIBUTING.md). Never edit or regenerate a file in `testdata/golden/`, `testdata/usno/` or `testdata/census/` in place, and never add an `-update` flag.
- **Snapshot format is a contract.** `entur-calendar-snapshot 1` and the observance slugs never change. A different format is a new version.
- **Workflows.** Pin every `uses:` to a full commit SHA with a `# vX.Y.Z` comment resolved from the upstream repository. Keep `permissions: {}` at the top, job-level grants only, `persist-credentials: false`, no caches, no secrets. Run `actionlint` and `zizmor` after every workflow change.
- **Commits.** Conventional Commits. Run the checks in CONTRIBUTING.md before you commit.

`CLAUDE.md` is a symlink to this file; edit `AGENTS.md`.
