package calendar

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func set(o ...Observance) ObservanceSet {
	var s ObservanceSet
	for _, x := range o {
		s |= bit(x)
	}
	return s
}

func TestCalendar_Collisions(t *testing.T) {
	var c Calendar
	cases := map[string]ObservanceSet{
		"2008-05-01": set(ArbeidernesDag, KristiHimmelfartsdag),
		"2007-05-17": set(Grunnlovsdag, KristiHimmelfartsdag),
		"2012-05-17": set(Grunnlovsdag, KristiHimmelfartsdag),
		"2027-05-17": set(Grunnlovsdag, AndrePinsedag),
		"2032-05-17": set(Grunnlovsdag, AndrePinsedag),
		// Pentecost Sunday on 17 May: the § 2 h observance suppresses Sondag.
		"2043-05-17": set(Grunnlovsdag, ForstePinsedag),
	}
	for s, want := range cases {
		got, err := c.Observances(mustDate(t, s))
		if err != nil || got != want {
			t.Errorf("Observances(%s) = %q, %v; want %q", s, got, err, want)
		}
		if ok, _ := c.IsWorkday(mustDate(t, s)); ok {
			t.Errorf("%s must not be a workday", s)
		}
	}
}

func TestCalendar_Sondag(t *testing.T) {
	var c Calendar
	cases := map[string]ObservanceSet{
		"2026-11-01": set(Sondag),                 // ordinary Sunday
		"2026-04-05": set(ForstePaskedag),         // § 2 e on a Sunday: no Sondag
		"2026-05-24": set(ForstePinsedag),         // § 2 h
		"2022-12-25": set(ForsteJuledag),          // § 2 j on a Sunday
		"2022-01-01": set(Nyttarsdag),             // Saturday: never Sondag
		"2023-01-01": set(Nyttarsdag),             // § 2 b on a Sunday
		"2026-05-17": set(Sondag, Grunnlovsdag),   // høytidsdag alone does not suppress Sondag
		"2022-05-01": set(Sondag, ArbeidernesDag), // 1 May on a Sunday
		"2028-12-24": set(Sondag, Julaften),       // customary on a Sunday
		"2026-11-02": 0,                           // ordinary Monday
		"2026-12-24": set(Julaften),               // Thursday
		"2026-04-01": set(OnsdagForSkjaertorsdag), // E−4 (Easter 2026-04-05)
	}
	for s, want := range cases {
		if got, err := c.Observances(mustDate(t, s)); err != nil || got != want {
			t.Errorf("Observances(%s) = %q, %v; want %q", s, got, err, want)
		}
	}
	// Every Sunday carries Sondag or a § 2 b–k observance; no weekday
	// carries Sondag.
	seq, _ := NewRange(mustDate(t, "2000-01-01"), mustDate(t, "2100-01-01"))
	dates, _ := seq.Dates()
	for d := range dates {
		got, _ := c.Observances(d)
		wd, _ := d.Weekday()
		if wd == time.Sunday && got&(set(Sondag)|namedHelligdager) == 0 {
			t.Fatalf("Sunday %s carries neither Sondag nor a § 2 b–k observance", d)
		}
		if got.Has(Sondag) && (wd != time.Sunday || got&namedHelligdager != 0) {
			t.Fatalf("%s: Sondag on %v with %q", d, wd, got)
		}
	}
}

func TestCalendar_IsWorkday(t *testing.T) {
	var c Calendar
	cases := map[string]bool{
		"2026-11-02": true,  // Monday
		"2026-11-07": false, // Saturday
		"2026-11-08": false, // Sunday
		"2026-12-24": true,  // julaften is a workday by default
		"2026-12-25": false,
		"2026-12-31": true, // nyttårsaften is a workday by default
		"2027-01-01": false,
		"2026-04-01": true,  // onsdag før skjærtorsdag
		"2026-04-02": false, // skjærtorsdag
		"2026-04-03": false,
		"2026-04-06": false,
		"2026-05-14": false, // Kristi himmelfartsdag
		"2026-05-25": false, // annen pinsedag
	}
	for s, want := range cases {
		if got, err := c.IsWorkday(mustDate(t, s)); err != nil || got != want {
			t.Errorf("IsWorkday(%s) = %v, %v; want %v", s, got, err, want)
		}
	}
}

// TestCalendar_NoSubstituteDays: a holiday on a weekend does not move.
func TestCalendar_NoSubstituteDays(t *testing.T) {
	var c Calendar
	// 2026-05-17 is a Sunday, 2021-12-25/26 a weekend, 2022-01-01 a Saturday.
	for _, s := range []string{"2026-05-18", "2021-12-27", "2021-12-28", "2022-01-03"} {
		if ok, err := c.IsWorkday(mustDate(t, s)); err != nil || !ok {
			t.Errorf("%s must be a workday (no substitute day): %v, %v", s, ok, err)
		}
	}
}

func TestCalendar_WithDaysOff(t *testing.T) {
	var c Calendar
	if c.DaysOff() != Statutory() {
		t.Fatal("zero Calendar must have exactly Statutory() off")
	}
	c2, err := c.WithDaysOff(Julaften, Nyttarsaften, Grunnlovsdag)
	if err != nil {
		t.Fatal(err)
	}
	if c2.DaysOff() != Statutory()|set(Julaften, Nyttarsaften) {
		t.Fatalf("DaysOff = %q", c2.DaysOff())
	}
	if ok, _ := c2.IsWorkday(mustDate(t, "2026-12-24")); ok {
		t.Error("julaften must be off when opted in")
	}
	if ok, _ := c.IsWorkday(mustDate(t, "2026-12-24")); !ok {
		t.Error("WithDaysOff must not modify the receiver")
	}
	same, _ := c.WithDaysOff(Nyttarsaften, Julaften)
	if same != c2 {
		t.Error("Calendar must be a comparable value independent of option order")
	}
	if noop, err := c.WithDaysOff(Sondag, Nyttarsdag, Grunnlovsdag); err != nil || noop != c {
		t.Errorf("statutory opt-in must be a no-op: %v, %v", noop, err)
	}
	for _, bad := range []Observance{0, lastObservance + 1, 200} {
		got, err := c2.WithDaysOff(Julaften, bad)
		if !errors.Is(err, ErrInvalidObservance) || got != c2 {
			t.Errorf("WithDaysOff(%d) = %v, %v; want unchanged, ErrInvalidObservance", bad, got, err)
		}
	}
}

func TestCalendar_RangeLimits(t *testing.T) {
	var c Calendar
	for _, s := range []string{"1999-12-31", "2100-01-01", "0001-01-01", "9999-12-31"} {
		if _, err := c.IsWorkday(mustDate(t, s)); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("IsWorkday(%s): %v", s, err)
		}
		if _, err := c.Observances(mustDate(t, s)); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Observances(%s): %v", s, err)
		}
	}
	if _, err := c.IsWorkday(Date{}); !errors.Is(err, ErrZeroDate) {
		t.Errorf("IsWorkday(zero): %v", err)
	}
	full := mustRange(t, "2000-01-01", "2100-01-01")
	if n, err := c.CountWorkdays(full); err != nil || n < 25000 {
		t.Fatalf("full range: %d, %v", n, err)
	}
	if n, err := c.CountWorkdays(mustRange(t, "2100-01-01", "2100-01-01")); err != nil || n != 0 {
		t.Fatalf("empty range at the exclusive end: %d, %v", n, err)
	}
	for _, r := range []Range{mustRange(t, "1999-12-31", "2000-01-02"), mustRange(t, "2099-12-31", "2100-01-02")} {
		if _, err := c.CountWorkdays(r); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("CountWorkdays(%v): %v", r, err)
		}
		if seq, err := c.Workdays(r); !errors.Is(err, ErrOutOfRange) || seq != nil {
			t.Errorf("Workdays(%v): %v", r, err)
		}
	}
	if _, err := c.CountWorkdays(Range{}); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("CountWorkdays(zero Range): %v", err)
	}
	if seq, err := c.Workdays(Range{}); !errors.Is(err, ErrInvalidRange) || seq != nil {
		t.Errorf("Workdays(zero Range): %v", err)
	}
}

// TestCalendar_28DayWindows: every 28-day window holds 20 weekdays; the
// default calendar subtracts statutory weekday holidays.
func TestCalendar_28DayWindows(t *testing.T) {
	var c Calendar
	cases := map[string]int{
		"2026-11-28": 20, // 2–27 Nov: no holidays
		"2026-12-31": 19, // 4–31 Dec: 25 Dec (Fri) off, 26 Dec is a Saturday; 24 and 31 Dec are workdays
		"2027-01-01": 18, // 5 Dec–1 Jan: 25 Dec and 1 Jan (both Fridays) off
		"2027-04-04": 17, // 8 Mar–4 Apr: skjærtorsdag 25 Mar, langfredag 26 Mar, annen påskedag 29 Mar
		"2027-05-31": 18, // 4–31 May: Kristi himmelfartsdag 6 May; 17 May is also annen pinsedag (one date)
	}
	for end, want := range cases {
		d := mustDate(t, end)
		start, _ := d.AddDays(-27)
		stop, _ := d.AddDays(1)
		r, _ := NewRange(start, stop)
		got, err := c.CountWorkdays(r)
		if err != nil || got != want {
			t.Errorf("W(%s) = %d, %v; want %d", end, got, err, want)
		}
	}
}

func TestCalendar_WorkdaysIteratorContract(t *testing.T) {
	var c Calendar
	r := mustRange(t, "2026-12-21", "2027-01-04")
	seq, err := c.Workdays(r)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for d := range seq {
		got = append(got, d.String())
	}
	want := []string{"2026-12-21", "2026-12-22", "2026-12-23", "2026-12-24", "2026-12-28", "2026-12-29", "2026-12-30", "2026-12-31"}
	if !slices.Equal(got, want) {
		t.Fatalf("Workdays = %v, want %v", got, want)
	}
	again := 0
	for range seq {
		again++
	}
	if again != len(want) {
		t.Fatalf("not reusable: %d", again)
	}
	calls := 0
	for range seq {
		calls++
		if calls == 2 {
			break
		}
	}
	if calls != 2 {
		t.Fatalf("break: %d", calls)
	}
	// No partial output before an error: an invalid range fails before any
	// iterator exists.
	bad := mustRange(t, "2099-12-30", "2100-01-05")
	if seq, err := c.Workdays(bad); err == nil || seq != nil {
		t.Fatal("invalid range must return an error and no iterator")
	}
}

func TestCalendar_CountWorkdaysProperties(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed keeps the property test reproducible
	cals := []Calendar{{}, {off: Customary()}, {off: set(Julaften)}}
	for range 500 {
		a := minCalendarRD + rng.Int32N(endCalendarRD-minCalendarRD+1)
		b := minCalendarRD + rng.Int32N(endCalendarRD-minCalendarRD+1)
		cc := minCalendarRD + rng.Int32N(endCalendarRD-minCalendarRD+1)
		x := []int32{a, b, cc}
		slices.Sort(x)
		r1 := Range{Date{x[0]}, Date{x[1]}}
		r2 := Range{Date{x[1]}, Date{x[2]}}
		r12 := Range{Date{x[0]}, Date{x[2]}}
		for _, c := range cals {
			n1, err1 := c.CountWorkdays(r1)
			n2, err2 := c.CountWorkdays(r2)
			n12, err12 := c.CountWorkdays(r12)
			if err1 != nil || err2 != nil || err12 != nil {
				t.Fatal(err1, err2, err12)
			}
			if n1+n2 != n12 {
				t.Fatalf("partition: [%s,%s)=%d + [%s,%s)=%d != %d", r1.start, r1.end, n1, r2.start, r2.end, n2, n12)
			}
			seq, _ := c.Workdays(r12)
			count := 0
			prev := Date{}
			for d := range seq {
				in, _ := r12.Contains(d)
				if !in || !d.After(prev) {
					t.Fatalf("Workdays yielded %s out of order or range", d)
				}
				prev = d
				count++
			}
			if count != n12 {
				t.Fatalf("iterator %d != count %d", count, n12)
			}
		}
	}
}

func TestZeroAllocs(t *testing.T) {
	c, _ := Calendar{}.WithDaysOff(Julaften)
	d := mustDate(t, "2027-05-17")
	allocs := testing.AllocsPerRun(1000, func() {
		if _, err := c.IsWorkday(d); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Observances(d); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("IsWorkday/Observances allocate %v times per call", allocs)
	}
}

func BenchmarkIsWorkday(b *testing.B) {
	var c Calendar
	d := Date{rd: minCalendarRD + 9000}
	for b.Loop() {
		_, _ = c.IsWorkday(d)
	}
}

func BenchmarkCountWorkdays28(b *testing.B) {
	var c Calendar
	r := Range{Date{minCalendarRD + 9000}, Date{minCalendarRD + 9028}}
	for b.Loop() {
		_, _ = c.CountWorkdays(r)
	}
}
