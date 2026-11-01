# go-calendar

Norwegian public holidays and conventional working days for Go. **Norway only.**

```go
import "github.com/entur/go-calendar"
```

- Statutory holidays from the law: the helligdager of helligdagsfredloven § 2 (every Sunday, nyttårsdag, skjærtorsdag, langfredag, første and annen påskedag, Kristi himmelfartsdag, første and annen pinsedag, første and annen juledag) and the høytidsdager 1 and 17 May (lov om 1. og 17. mai som høgtidsdager § 1).
- Customary days (onsdag før skjærtorsdag, julaften, nyttårsaften) only when you opt in.
- Easter by the Gregorian ecclesiastical computus, the way Norway observes it.
- A civil `Date` type with no time zone. Lookups never take a `time.Time`.
- Snapshots: canonical, hash-pinnable files that freeze a calendar so a library upgrade can never change a published statistic.
- Standard library only, no package state, no dependencies (tests included).

The design and its reasons are in [doc/adr/0001-norwegian-calendar-design.adoc](doc/adr/0001-norwegian-calendar-design.adoc).

## API tour

```go
var c calendar.Calendar // zero value: weekends and statutory holidays are days off

d, _ := calendar.ParseDate("2027-05-17")
ok, _ := c.IsWorkday(d)    // false
set, _ := c.Observances(d) // grunnlovsdag and annen pinsedag: a date can carry several
for o := range set.All() {
	fmt.Println(o, o.Kind(), o.Basis()) // "17. mai høytidsdag lov om 1. og 17. mai som høgtidsdager § 1", ...
}

// Opt in to customary days off.
c2, _ := calendar.Calendar{}.WithDaysOff(calendar.Julaften, calendar.Nyttarsaften)

// Half-open ranges [start, end). A 28-day window ending on (and including) D:
start, _ := D.AddDays(-27)
end, _ := D.AddDays(1)
window, _ := calendar.NewRange(start, end)
n, _ := c.CountWorkdays(window)
days, _ := c.Workdays(window) // iter.Seq[calendar.Date], validated up front
for d := range days { ... }

e, _ := calendar.Easter(2019) // 2019-04-21, not the astronomical 2019-03-24
```

What the package does not model: half days, work hours, fellesferie, bridge days ("inneklemte dager"), substitute days, and the legal word "virkedag", which means different things in ferieloven § 5 and domstolloven § 149. `IsWorkday` is the conventional office calendar: Monday–Friday without a day-off observance.

Every invalid input returns an error; nothing is turned into a silent zero or empty result. Errors wrap constant sentinels (`ErrZeroDate`, `ErrInvalidDate`, `ErrOutOfRange`, `ErrInvalidRange`, `ErrInvalidObservance`, `ErrInvalidLocation`, `ErrNonexistentMidnight`); test them with `errors.Is`.

Calendar queries support **2000–2099** (`2100-01-01` is allowed as an exclusive range end); other years return `ErrOutOfRange`. The rules project current law onto those years: this is not a historical-law service.

## Dates, time zones and the UTC-label caveat

`Date` is a civil date. Converting an instant needs an explicit location:

```go
calendar.DateAt(t, time.UTC)  // the UTC calendar date of t
calendar.DateAt(t, oslo)      // oslo loaded by you with time.LoadLocation("Europe/Oslo")
```

The package never reads the clock, never picks the process-local zone itself, never calls `time.LoadLocation` and never loads tzdata. `Date.In(loc)` returns midnight and reports `ErrNonexistentMidnight` instead of silently moving to another day when a zone skips midnight.

**Caveat for UTC statistics.** entur/kompass counts usage per UTC day and looks those UTC date labels up as if they were Norwegian dates. That is a deliberate convention, not an exact legal day boundary: usage between 00:00 and 02:00 Norwegian summer time on 17 May counts on the UTC date 16 May, which is a working day. Snapshots record the convention in their `basis` line (`utc-date-labels` or `oslo-civil-dates`).

## Freezing a calendar (for statistics and other released definitions)

`calendar.Calendar{}` follows the rules of the module version you build with (`calendar.RulesVersion`). A rule fix or a new law changes its answers after an upgrade. If answers must never change, freeze a snapshot:

```sh
go run github.com/entur/go-calendar/cmd/calendar-snapshot@vX.Y.Z \
  -start 2026-11-01 -end 2036-01-01 -basis utc-date-labels > calendar/v2.txt
```

The file lists every workday in the span, every Monday–Friday that is not a workday with its reasons, and the metadata:

```
entur-calendar-snapshot 1
rules no-public/v1
basis utc-date-labels
daysoff none
start 2026-11-01
end 2036-01-01
excluded 2026-12-25 forste-juledag
...
days
2026-11-02
...
```

The consumer then:

1. commits and embeds the exact file (`//go:embed`);
2. records the calendar identifier, span, basis, complete-file SHA-256, generator module version, Go version and command line in its own append-only manifest;
3. pins the approved SHA-256 in a test and verifies the embedded bytes against it at initialisation, rejecting a mismatch or a window outside the span;
4. reads membership with `snapshot.Parse` and `(*Frozen).CountWorkdays` (or its own few lines of code), never from the live rules;
5. never regenerates or overwrites a released file. A changed date set is a new consumer calendar version.

`-verify-sha256 HEX` makes the command write nothing and exit 1 unless the output matches, so a pipeline can only receive approved bytes. `snapshot.Parse` checks structure and canonical form; it does not evaluate holiday rules and accepts snapshots made under earlier rules identifiers.

## Security and releases

### What "signed release" means here

This is a source-only Go module. There are no binaries or other release assets.

- **Module content:** the Go checksum database (sum.golang.org) is the integrity anchor. After each release, the release workflow downloads the version through proxy.golang.org (which records it in the checksum database) and proves that its `h1:` hash equals the hash of the tagged tree computed under Go's module-zip rules.
- **Releases:** immutable GitHub releases add a platform release attestation; check one with `gh release verify vX.Y.Z`.
- **Commits** on `main` are verified (signed) commits.
- **Tags** are lightweight release-please tags and are not signed. No separate SLSA build provenance is generated, because nothing is built for distribution.

**OpenSSF Scorecard, Signed-Releases:** the check ignores source-only releases and may report an inconclusive or not-applicable result when there are no eligible release assets. That is expected for this module; the integrity guarantees above do not rely on release assets.

### Supply chain

- Zero module dependencies. `govulncheck` and the module-hash checker live in the nested `tools/` module, which is not part of the published module.
- Every GitHub Action and reusable workflow is pinned to a full commit SHA with a version comment. This deviates on purpose from the Entur golden path's major-version tags: tags can be moved, and a SHA pin of `entur/gha-security` also pins its `$/` local actions.
- Workflows: `permissions: {}` at the top and job-level grants only, `persist-credentials: false`, no caches, `GOTOOLCHAIN: local`, no repository secrets, no `secrets: inherit`, no `pull_request_target`. zizmor and CodeQL check the workflows.
- Dependabot: monthly, 7-day cooldown for version updates, grouped minor/patch, separate security groups.
- Report vulnerabilities privately: see [SECURITY.md](SECURITY.md).

### Repository settings checklist (repository admin, before v0.1.0)

- [ ] Ruleset `main`: pull request required, 1 approval, code-owner review, dismiss stale approvals, approval of the latest push, conversation resolution, squash merge only, linear history, signed commits, required checks (`test (floor)`, `test (stable)`, `test-386`, `lint`, `vuln`, the four `fuzz (…)` jobs, `zizmor`, `CodeQL (go)`, `CodeQL (actions)`), code-scanning results gate, block force pushes and deletion, no bypass actors.
- [ ] Ruleset `v*` tags: restrict updates and deletions, block force pushes. Restricting tag creation needs a release GitHub App as the only bypass actor (`GITHUB_TOKEN` cannot be one). Until then, `tag-guard.yml` audits all tags on push, daily from `main`, on demand, and after each release; this detects stray tags after creation but does not prevent their publication or consumption.
- [ ] Immutable releases enabled before the first release (they apply only to later releases).
- [ ] Actions: read-only default token; "Allow GitHub Actions to create and approve pull requests" on (release-please); fork pull request workflows require approval; require full-length SHA pins once a test pull request shows that `entur/gha-security`'s `$/` references pass.
- [ ] Private vulnerability reporting, Dependabot alerts and security updates, secret scanning and push protection.
- [ ] Squash merge only; delete head branches on merge. `@entur/team-ki-effekt` as admin.
- [ ] Release pull requests opened by `GITHUB_TOKEN` start their workflow runs in an approval-required state: a user with write access approves the runs before merging. A repository-scoped release GitHub App (installed by an org admin) is later hardening; it removes that step and allows restricting tag creation.
- [ ] Record the first Scorecard result from the pinned Scorecard version.

## Versioning

Releases are made by release-please from Conventional Commits. The first release is v0.1.0; v1.0.0 follows once entur/kompass has integrated against the API. A bad release is fixed forward with a `retract` directive in a new patch release; tags are never deleted or moved.

`RulesVersion` (currently `no-public/v1`) changes when any answer in a verified span changes, through a new law or a rule fix. A pure range extension keeps the identifier.

## License

[EUPL-1.2](LICENSE.txt)
