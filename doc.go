// Package calendar models Norwegian public holidays and conventional working
// days on civil dates. It covers Norway only.
//
// # What it models
//
// Statutory observances (always days off, see Statutory):
//
//   - helligdager under lov om helligdager og helligdagsfred
//     (helligdagsfredloven) § 2: every Sunday (Sondag, § 2 a, on Sundays that
//     carry no § 2 b–k observance), nyttårsdag, skjærtorsdag, langfredag,
//     første and annen påskedag, Kristi himmelfartsdag, første and annen
//     pinsedag, første and annen juledag;
//   - høytidsdager under lov om 1. og 17. mai som høgtidsdager § 1: 1 May and
//     17 May.
//
// Customary observances (days off only when a Calendar opts in with
// WithDaysOff, see Customary): onsdag før skjærtorsdag, julaften and
// nyttårsaften. No law makes them full days off; collective agreements and
// employers differ.
//
// A date can carry several observances: 2027-05-17 is both grunnlovsdag and
// annen pinsedag, so the API returns an ObservanceSet. There are no
// substitute days: a holiday on a weekend does not move to a weekday.
//
// Not modelled: half days, work hours, fellesferie, bridge days
// (inneklemte dager), or the legal word "virkedag", which means different
// things in different statutes. IsWorkday is the conventional office-day
// calendar: Monday–Friday without a day-off observance.
//
// Første påskedag follows the Gregorian ecclesiastical computus (see Easter),
// not the astronomical reading of the statute's wording.
//
// # Supported years
//
// Calendar queries support MinYear..MaxYear (2000–2099); 2100-01-01 is
// accepted as an exclusive range end. Other years return ErrOutOfRange. The
// rules project current law onto those years; the package is not a
// historical-law service. Date itself covers 0001-01-01..9999-12-31.
//
// # Dates and time zones
//
// Date is a civil date without a time of day or a location. Norwegian
// observances are properties of Norwegian civil dates, so lookups take a
// Date, never a time.Time. Converting an instant to a date needs an explicit
// location (DateAt); the package never reads the clock, never chooses the
// process-local zone itself, never calls time.LoadLocation and never loads
// tzdata. Passing time.Local lets the standard library consult the system
// time zone; reproducible callers pass time.UTC or a location they loaded.
//
// A consumer that counts per UTC day may deliberately look those UTC date
// labels up as if they were Norwegian dates. Activity between 00:00 and
// 02:00 Norwegian summer time on 17 May 2024 then counts on the UTC date
// 16 May 2024, a working day (in 2016 and 2027, 16 May is itself a
// holiday). That is the consumer's documented convention, not an exact
// legal day boundary; snapshots record it in their basis line.
//
// # Errors
//
// Invalid input is never turned into a silent zero or empty result. Errors
// wrap the constant sentinels of type Error (ErrZeroDate, ErrInvalidDate,
// ErrOutOfRange, ErrInvalidRange, ErrInvalidObservance, ErrInvalidLocation,
// ErrNonexistentMidnight); test them with errors.Is.
//
// # Stability
//
// The zero Calendar follows the rules of this module version, identified by
// RulesVersion. A changed law or a rule fix changes RulesVersion and the
// answers of a live Calendar after an upgrade. A consumer whose answers must
// never change (a released statistic, for example) freezes a snapshot with
// package snapshot, pins its SHA-256, and reads membership from those bytes
// without the live rules.
//
// The package has no package-level variables, no init functions, no caches
// and no goroutines. All types are immutable values and safe for concurrent
// use.
package calendar
