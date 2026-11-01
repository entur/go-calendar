# go-calendar

Norwegian public holidays and conventional working days for Go. **Norway only.**

- Statutory holidays from the law: the helligdager of helligdagsfredloven § 2 (every Sunday, nyttårsdag, skjærtorsdag, langfredag, første and annen påskedag, Kristi himmelfartsdag, første and annen pinsedag, første and annen juledag) and the høytidsdager 1 and 17 May (lov om 1. og 17. mai som høgtidsdager § 1).
- Customary days (onsdag før skjærtorsdag, julaften, nyttårsaften) only when you opt in.
- Easter by the Gregorian ecclesiastical computus, the way Norway observes it.
- A civil `Date` type with no time zone. Lookups never take a `time.Time`.
- Snapshots: canonical, hash-pinnable files that freeze a calendar, so a library upgrade can never change a published result.
- Standard library only, no package state, no dependencies (tests included).

## Install

```sh
go get github.com/entur/go-calendar
```

Requires Go 1.26 or later.

## API tour

```go
import "github.com/entur/go-calendar"

var c calendar.Calendar // zero value: weekends and statutory holidays are days off

d, _ := calendar.ParseDate("2027-05-17")
ok, _ := c.IsWorkday(d)    // false
set, _ := c.Observances(d) // 17. mai and annen pinsedag: a date can carry several
for o := range set.All() {
	fmt.Println(o, o.Kind(), o.Basis())
	// 17. mai høytidsdag lov om 1. og 17. mai som høgtidsdager § 1
	// annen pinsedag helligdag helligdagsfredloven § 2 bokstav i
}

// Opt in to customary days off.
c2, _ := calendar.Calendar{}.WithDaysOff(calendar.Julaften, calendar.Nyttarsaften)

// Ranges are half-open, [start, end). A 28-day window ending on (and including) D:
start, _ := D.AddDays(-27)
end, _ := D.AddDays(1)
window, _ := calendar.NewRange(start, end)
n, _ := c.CountWorkdays(window)
days, _ := c.Workdays(window) // iter.Seq[calendar.Date], validated before it is returned
for d := range days {
	// ...
}

e, _ := calendar.Easter(2019) // 2019-04-21, not the astronomical 2019-03-24
```

What the package does not model: half days, work hours, fellesferie, bridge days ("inneklemte dager"), substitute days, and the legal word "virkedag", which means different things in ferieloven § 5 and domstolloven § 149. `IsWorkday` is the conventional office calendar: Monday–Friday without a day-off observance.

Every invalid input returns an error; nothing is turned into a silent zero or empty result. Errors wrap constant sentinels (`ErrZeroDate`, `ErrInvalidDate`, `ErrOutOfRange`, `ErrInvalidRange`, `ErrInvalidObservance`, `ErrInvalidLocation`, `ErrNonexistentMidnight`); test them with `errors.Is`.

Calendar queries support **2000–2099**; `2100-01-01` is allowed as an exclusive range end. Other years return `ErrOutOfRange`. The rules project current law onto those years: this is not a historical-law service.

## Dates and time zones

`Date` is a civil date. Converting an instant needs an explicit location:

```go
calendar.DateAt(t, time.UTC) // the UTC calendar date of t
calendar.DateAt(t, oslo)     // oslo loaded by you with time.LoadLocation("Europe/Oslo")
```

The package never reads the clock, never picks the process-local zone itself, never calls `time.LoadLocation` and never loads tzdata. `Date.In(loc)` returns midnight on the date and reports `ErrNonexistentMidnight` rather than silently moving to another day when a zone skips midnight.

**UTC date labels.** A consumer that counts per UTC day may deliberately look those UTC date labels up as if they were Norwegian dates. That is a convention, not an exact legal day boundary: activity between 00:00 and 02:00 Norwegian summer time on 17 May falls on the UTC date 16 May, which is a working day. Snapshots record which convention a consumer uses in their `basis` line (`utc-date-labels` or `oslo-civil-dates`).

## Freezing a calendar

`calendar.Calendar{}` follows the rules of the module version you build with (`calendar.RulesVersion`). A rule fix or a new law changes its answers after an upgrade. When answers must never change, for example in a published statistic, freeze a snapshot:

```sh
go run github.com/entur/go-calendar/cmd/calendar-snapshot@vX.Y.Z \
  -start 2026-11-01 -end 2036-01-01 -basis utc-date-labels > calendar-v1.txt
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

Then, in the consuming program:

1. Commit and embed the exact file (`//go:embed`).
2. Record the calendar version, span, basis, the file's SHA-256, this module's version, the Go version and the command line next to it.
3. Pin the approved SHA-256 in a test and verify the embedded bytes against it at start-up, rejecting a mismatch or a window outside the span.
4. Read membership with `snapshot.Parse` and `(*Frozen).CountWorkdays`, or with a few lines of your own code, never from the live rules.
5. Never regenerate or overwrite a released file. A changed set of dates is a new version of your calendar.

`-verify-sha256 HEX` makes the command write nothing and fail unless the output has that digest. An empty or malformed value is an error, never a skipped check. `snapshot.Parse` checks structure and canonical form. It does not evaluate holiday rules, and it accepts snapshots made under earlier rules identifiers.

## Stability and versioning

- Semantic versions, released from Conventional Commits. The first release is v0.1.0. v1.0.0 follows once a first consumer has integrated against the API; until then, minor releases may change the API.
- `RulesVersion` (currently `no-public/v1`) changes whenever any answer in a verified span changes, through a new law or a rule fix. A pure range extension keeps the identifier.
- The snapshot format (`entur-calendar-snapshot 1`) and its observance slugs never change. A different format would be a new format version, and readers keep accepting format 1.
- A bad release is fixed forward. The fix release adds a `retract` directive for the bad version, and tags are never deleted or moved.

## Verifying a release

This is a source-only module: there are no binaries or other release assets.

- **Module content.** The Go checksum database (sum.golang.org) records every version's content, and the go command verifies it on download (`go mod download`, `go mod verify`). Each release is checked against the tagged source when it is published.
- **Releases** are immutable GitHub releases with a platform attestation: `gh release verify vX.Y.Z --repo entur/go-calendar`.
- **Commits** on `main` are verified (signed). Tags are lightweight release tags and are not signed.

Report vulnerabilities privately: see [SECURITY.md](SECURITY.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The design and its reasons are in [doc/adr/0001-norwegian-calendar-design.adoc](doc/adr/0001-norwegian-calendar-design.adoc).

## License

[EUPL-1.2](LICENSE.txt)
