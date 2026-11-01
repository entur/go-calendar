# Test data sources

Tests never use the network. The external tables below were produced once by the committed scripts, are never typed or edited by hand, and are pinned by SHA-256 in `easter_test.go`. They are lists of computed dates: factual data. Neither source states licence terms, and none is asserted here.

## US Naval Observatory: `usno/observances-2000-2099.txt`

- Source: US Naval Observatory, Astronomical Applications Department, "Selected Christian Observances" API, `https://aa.usno.navy.mil/api/observances/christian?year=YEAR` for YEAR 2000–2099 (documented at https://aa.usno.navy.mil/data/api; service page https://aa.usno.navy.mil/data/christian_observances).
- API version: `4.0.1` (reported by every response).
- Retrieved: 2026-11-01T18:19:04Z, with `go run scripts/fetch_usno.go > testdata/usno/observances-2000-2099.txt`.
- Format: one line per year, `YEAR GOOD-FRIDAY EASTER ASCENSION WHIT-SUNDAY`, dates as `YYYY-MM-DD`.
- SHA-256: `53e84ef0b59d01f5b27f3a6165b0ab905c34e160b24afc9c40ed1f6cb25a4574`.
- Used by: `TestEaster_USNO` (langfredag, første påskedag, Kristi himmelfartsdag and første pinsedag for every year).

## US Census Bureau: `census/easter-2000-2099.txt`

- Source: US Census Bureau, "Easter Dates from 1600 to 2099", https://www.census.gov/data/software/x13as/genhol/easter-dates.html. The page says its dates were taken from a public listing previously published by M. Bednarek and checked against other sources; it is independent of the USNO computation.
- Retrieved: 2026-11-01T18:17:55Z, with `go run scripts/fetch_census.go > testdata/census/easter-2000-2099.txt`. SHA-256 of the HTML parsed: `86324c72d5aa276a59a08c79ddd4afa1a2ac1340a3da0043fb6e64ef44e89fee`.
- Format: one Easter Sunday per line, `YYYY-MM-DD`, years 2000–2099 in order.
- SHA-256: `eabcd9510c562ca152c2a2d5e531c75248c0be0eb8de4d2347b5e75c8d002d1c`.
- Used by: `TestEaster_Census`.

The USNO and Census Easter dates agreed for all 100 years when retrieved.

## Retained rules fixtures: `golden/`

Generated from the rules at the time each rules identifier was approved, through the `TestRulesHistory_Current` artifact mechanism (see CONTRIBUTING.md), reviewed, and pinned by `rulesHistory` in `rules_test.go`. They are never regenerated in place.

- `observances-no-public-v1.txt`: every date 2000-01-01..2099-12-31 that carries an observance, `YYYY-MM-DD slug[,slug]`.
- `workdays-no-public-v1-<combination>.txt`: per-date workday membership for each of the 8 combinations of customary opt-ins; one line per year, `YYYY-MM-DD` (the first date of the line) followed by one character per day, `1` for a workday and `0` otherwise.

Their independent evidence is the USNO and Census tables above, two independent Easter algorithms (`easter_test.go`), and the literal statute-wording test (`observance_test.go`).
