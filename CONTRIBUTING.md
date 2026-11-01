# Contributing

Read [AGENTS.md](AGENTS.md) and [ADR 0001](doc/adr/0001-norwegian-calendar-design.adoc) first. CI, release and repository-settings material for maintainers is in [doc/maintainers.md](doc/maintainers.md).

## Checks

CI runs these on every pull request; run them locally before you push:

```sh
gofmt -l .                                   # must print nothing
go mod tidy -diff && go -C tools mod tidy -diff
go list -m all                               # must print only github.com/entur/go-calendar
go vet ./...
go test -race -shuffle=on ./...
GOOS=linux GOARCH=386 go vet ./...           # CI also runs the tests on 386
golangci-lint run                            # v2.15.0, .golangci.yaml
go -C tools tool govulncheck -C .. ./...
go test -run='^$' -fuzz='^FuzzParseDate$' -fuzztime=20s .   # also FuzzAddDays, FuzzNewRange, ./snapshot FuzzParse
actionlint && uvx zizmor@1.31.1 --offline .github/
shellcheck .github/scripts/*.sh && bash .github/scripts/tag-audit_test.sh
```

Tests never use the network.

## Commits and releases

Use [Conventional Commits](https://www.conventionalcommits.org/). release-please derives versions from them: `feat:` is a minor release, `fix:` a patch release, `feat!:`/`fix!:` a breaking change. Dependabot uses `ci(deps):` and `chore(deps):`, which do not release.

## Changing the rules

The answers of a released rules identifier never change silently. `rules_test.go` holds an append-only `rulesHistory`, and `testdata/golden/` holds the retained fixtures it pins: every observance from 2000–2099 and per-date workday membership for all 8 combinations of customary opt-ins.

- **Changed law or a fix that changes any answer:** bump `RulesVersion` in `calendar.go`, and append a `rulesHistory` record with a new identifier. Set its cutoff to the civil effective date (law) or the earliest intentionally corrected date (fix; record the issue, use a `fix!:` commit). Add new fixture files; never overwrite old ones.
- **Range extension only:** append a record with the same identifier, the new span and new fixture paths. Every answer on the overlap with earlier records must stay identical. This is a `feat:`.

There is no `-update` flag. When the live rules no longer reproduce a fixture, the test reports the first differing dates and writes the regenerated file to its artifact directory:

```sh
go test -run TestRulesHistory_Current -artifacts -outputdir /tmp/artifacts .
```

Review the regenerated files, copy them into `testdata/golden/` under new names, and record their SHA-256 in `rulesHistory`.

## External fixtures

`testdata/usno/` and `testdata/census/` are produced by `scripts/fetch_usno.go` and `scripts/fetch_census.go` and are never edited by hand. Their provenance and digests are in [testdata/SOURCES.md](testdata/SOURCES.md).
