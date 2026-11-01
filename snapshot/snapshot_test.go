package snapshot

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/entur/go-calendar"
)

func date(t testing.TB, s string) calendar.Date {
	t.Helper()
	d, err := calendar.ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func span(t testing.TB, start, end string) calendar.Range {
	t.Helper()
	r, err := calendar.NewRange(date(t, start), date(t, end))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const sample = `entur-calendar-snapshot 1
rules no-public/v1
basis utc-date-labels
daysoff julaften
start 2026-12-21
end 2027-01-04
closed 2026-12-28
closed 2027-01-02
excluded 2026-12-24 julaften
excluded 2026-12-25 forste-juledag
excluded 2026-12-28 closed
excluded 2027-01-01 nyttarsdag
days
2026-12-21
2026-12-22
2026-12-23
2026-12-29
2026-12-30
2026-12-31
`

func sampleCalendar(t testing.TB) calendar.Calendar {
	t.Helper()
	c, err := calendar.Calendar{}.WithDaysOff(calendar.Julaften)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGenerate_Exact(t *testing.T) {
	// Closures out of order and duplicated: Generate sorts and dedups.
	got, err := Generate(sampleCalendar(t), span(t, "2026-12-21", "2027-01-04"), UTCDateLabels,
		date(t, "2027-01-02"), date(t, "2026-12-28"), date(t, "2026-12-28"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != sample {
		t.Fatalf("Generate:\n%s\nwant:\n%s", got, sample)
	}
}

func TestParse_Sample(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if f.Rules() != "no-public/v1" || f.Basis() != UTCDateLabels || f.Span() != span(t, "2026-12-21", "2027-01-04") {
		t.Fatalf("metadata: %q %q %v", f.Rules(), f.Basis(), f.Span())
	}
	if f.DaysOff() != calendar.Statutory()|sampleCalendar(t).DaysOff() {
		t.Fatalf("DaysOff = %q", f.DaysOff())
	}
	if got := f.Closed(); len(got) != 2 || got[0] != date(t, "2026-12-28") || got[1] != date(t, "2027-01-02") {
		t.Fatalf("Closed = %v", got)
	}
	for s, want := range map[string]bool{
		"2026-12-21": true, "2026-12-24": false, "2026-12-26": false, "2026-12-28": false, "2026-12-31": true, "2027-01-03": false,
	} {
		if got, err := f.IsWorkday(date(t, s)); err != nil || got != want {
			t.Errorf("IsWorkday(%s) = %v, %v; want %v", s, got, err, want)
		}
	}
	if n, err := f.CountWorkdays(f.Span()); err != nil || n != 6 {
		t.Fatalf("CountWorkdays = %d, %v", n, err)
	}
	seq, err := f.Workdays(span(t, "2026-12-23", "2026-12-30"))
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Collect(seq); len(got) != 2 || got[0] != date(t, "2026-12-23") || got[1] != date(t, "2026-12-29") {
		t.Fatalf("Workdays = %v", got)
	}
}

func TestFrozen_OutsideSpan(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"2026-12-20", "2027-01-04"} {
		if _, err := f.IsWorkday(date(t, s)); !errors.Is(err, calendar.ErrOutOfRange) {
			t.Errorf("IsWorkday(%s): %v", s, err)
		}
	}
	if _, err := f.IsWorkday(calendar.Date{}); !errors.Is(err, calendar.ErrZeroDate) {
		t.Errorf("IsWorkday(zero): %v", err)
	}
	if _, err := f.CountWorkdays(span(t, "2026-12-20", "2026-12-25")); !errors.Is(err, calendar.ErrOutOfRange) {
		t.Errorf("CountWorkdays before span: %v", err)
	}
	if seq, err := f.Workdays(span(t, "2026-12-30", "2027-01-05")); !errors.Is(err, calendar.ErrOutOfRange) || seq != nil {
		t.Errorf("Workdays after span: %v", err)
	}
	if _, err := f.CountWorkdays(calendar.Range{}); !errors.Is(err, calendar.ErrInvalidRange) {
		t.Errorf("CountWorkdays(zero Range): %v", err)
	}
}

func TestFrozen_NilAndZero(t *testing.T) {
	d := date(t, "2026-12-21")
	r := span(t, "2026-12-21", "2026-12-22")
	for name, f := range map[string]*Frozen{"nil": nil, "zero": {}} {
		if _, err := f.IsWorkday(d); !errors.Is(err, ErrInvalidSnapshot) {
			t.Errorf("%s IsWorkday: %v", name, err)
		}
		if seq, err := f.Workdays(r); !errors.Is(err, ErrInvalidSnapshot) || seq != nil {
			t.Errorf("%s Workdays: %v", name, err)
		}
		if _, err := f.CountWorkdays(r); !errors.Is(err, ErrInvalidSnapshot) {
			t.Errorf("%s CountWorkdays: %v", name, err)
		}
		if f.Rules() != "" || f.Basis() != "" || !f.Span().IsZero() || f.DaysOff() != 0 || f.Closed() != nil {
			t.Errorf("%s metadata must be zero", name)
		}
	}
}

// TestFrozen_Aliasing: Parse copies what it keeps and Closed returns a copy.
func TestFrozen_Aliasing(t *testing.T) {
	b := []byte(sample)
	f, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	// Turn the workday 2026-12-29 into 2026-12-27 in the caller's buffer.
	i := bytes.Index(b, []byte("2026-12-29\n"))
	b[i+9] = '7'
	if ok, err := f.IsWorkday(date(t, "2026-12-29")); err != nil || !ok {
		t.Fatalf("mutating the input changed the answer: %v, %v", ok, err)
	}
	closed := f.Closed()
	closed[0] = date(t, "2026-12-22")
	if got := f.Closed(); got[0] != date(t, "2026-12-28") {
		t.Fatalf("mutating Closed() changed the snapshot: %v", got)
	}
	if ok, _ := f.IsWorkday(date(t, "2026-12-22")); !ok {
		t.Fatal("mutating Closed() changed membership")
	}
}

// TestParse_PreviousRulesIdentifier: the rules line is metadata; a snapshot
// frozen under an earlier identifier still parses after an upgrade.
func TestParse_PreviousRulesIdentifier(t *testing.T) {
	old := strings.Replace(sample, "rules no-public/v1", "rules no-public/v0", 1)
	f, err := Parse([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if f.Rules() != "no-public/v0" {
		t.Fatalf("Rules = %q", f.Rules())
	}
}

// TestParse_FormatLimitsNotCalendarBounds: Parse checks the format's limits,
// not the current calendar's supported years.
func TestParse_FormatLimitsNotCalendarBounds(t *testing.T) {
	text := "entur-calendar-snapshot 1\nrules no-public/v1\nbasis oslo-civil-dates\ndaysoff none\n" +
		"start 1999-12-27\nend 2000-01-01\ndays\n1999-12-27\n1999-12-28\n1999-12-29\n1999-12-30\n1999-12-31\n"
	f, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := f.CountWorkdays(f.Span()); n != 5 {
		t.Fatalf("CountWorkdays = %d", n)
	}
}

func TestParse_Rejects(t *testing.T) {
	replace := func(old, repl string) string {
		if !strings.Contains(sample, old) {
			t.Fatalf("sample lacks %q", old)
		}
		return strings.Replace(sample, old, repl, 1)
	}
	cases := map[string]string{
		"empty":                      "",
		"CRLF":                       strings.ReplaceAll(sample, "\n", "\r\n"),
		"no final newline":           strings.TrimSuffix(sample, "\n"),
		"blank line":                 sample + "\n",
		"format version":             replace("entur-calendar-snapshot 1", "entur-calendar-snapshot 2"),
		"rules uppercase":            replace("rules no-public/v1", "rules NO-public/v1"),
		"rules missing":              replace("rules no-public/v1\n", ""),
		"rules too long":             replace("rules no-public/v1", "rules "+strings.Repeat("a", 65)),
		"unknown basis":              replace("basis utc-date-labels", "basis utc"),
		"header order":               replace("rules no-public/v1\nbasis utc-date-labels", "basis utc-date-labels\nrules no-public/v1"),
		"daysoff statutory":          replace("daysoff julaften", "daysoff forste-juledag"),
		"daysoff unsorted":           replace("daysoff julaften", "daysoff nyttarsaften,julaften"),
		"daysoff duplicate":          replace("daysoff julaften", "daysoff julaften,julaften"),
		"daysoff empty":              replace("daysoff julaften", "daysoff "),
		"daysoff missing customary":  replace("daysoff julaften", "daysoff none"),
		"bad start":                  replace("start 2026-12-21", "start 2026-12-1"),
		"end before start":           replace("end 2027-01-04", "end 2026-12-20"),
		"span too long":              replace("start 2026-12-21", "start 1926-12-20"),
		"closed outside span":        replace("closed 2027-01-02", "closed 2027-01-04"),
		"closed unsorted":            replace("closed 2026-12-28\nclosed 2027-01-02", "closed 2027-01-02\nclosed 2026-12-28"),
		"closed without reason":      replace("excluded 2026-12-28 closed\n", "") + "",
		"closed reason undeclared":   replace("closed 2026-12-28\n", ""),
		"closed reason first":        replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 closed,forste-juledag"),
		"unknown reason":             replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 christmas"),
		"sondag reason":              replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 sondag"),
		"reasons out of order":       replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 forste-juledag,nyttarsdag"),
		"reason duplicate":           replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 forste-juledag,forste-juledag"),
		"no reasons":                 replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25"),
		"empty reasons":              replace("excluded 2026-12-25 forste-juledag", "excluded 2026-12-25 "),
		"excluded weekend":           replace("excluded 2026-12-28 closed", "excluded 2026-12-26 andre-juledag\nexcluded 2026-12-28 closed"),
		"excluded unsorted":          replace("excluded 2026-12-24 julaften\nexcluded 2026-12-25 forste-juledag", "excluded 2026-12-25 forste-juledag\nexcluded 2026-12-24 julaften"),
		"missing days line":          replace("days\n", ""),
		"days unsorted":              replace("2026-12-22\n2026-12-23", "2026-12-23\n2026-12-22"),
		"days duplicate":             replace("2026-12-22\n", "2026-12-22\n2026-12-22\n"),
		"days weekend":               replace("2026-12-23\n", "2026-12-23\n2026-12-27\n"),
		"days overlaps excluded":     replace("2026-12-23\n", "2026-12-23\n2026-12-24\n"),
		"weekday missing":            replace("2026-12-30\n", ""),
		"day outside span":           sample + "2027-01-04\n",
		"trailing space":             replace("2026-12-21\n", "2026-12-21 \n"),
		"tab":                        replace("days\n", "days\t\n"),
		"non-ASCII":                  replace("julaften\nexcluded", "julaften\nexcluded "),
		"lowercase header key twice": replace("start 2026-12-21\n", "start 2026-12-21\nstart 2026-12-21\n"),
		"unknown line":               replace("days\n", "note x\ndays\n"),
	}
	for name, text := range cases {
		if f, err := Parse([]byte(text)); !errors.Is(err, ErrInvalidSnapshot) || f != nil {
			t.Errorf("%s: Parse = %v, %v; want ErrInvalidSnapshot", name, f, err)
		}
	}
	big := make([]byte, maxBytes+1)
	if _, err := Parse(big); !errors.Is(err, ErrInvalidSnapshot) {
		t.Errorf("oversized: %v", err)
	}
}

func TestGenerate_Errors(t *testing.T) {
	var c calendar.Calendar
	r := span(t, "2026-12-21", "2027-01-04")
	if _, err := Generate(c, r, Basis("utc")); !errors.Is(err, ErrInvalidSnapshot) {
		t.Errorf("unknown basis: %v", err)
	}
	if _, err := Generate(c, calendar.Range{}, UTCDateLabels); !errors.Is(err, calendar.ErrInvalidRange) {
		t.Errorf("zero range: %v", err)
	}
	if _, err := Generate(c, span(t, "1999-12-01", "2000-01-05"), UTCDateLabels); !errors.Is(err, calendar.ErrOutOfRange) {
		t.Errorf("before MinYear: %v", err)
	}
	if _, err := Generate(c, r, UTCDateLabels, calendar.Date{}); !errors.Is(err, calendar.ErrZeroDate) {
		t.Errorf("zero closure: %v", err)
	}
	if _, err := Generate(c, r, UTCDateLabels, date(t, "2027-01-04")); !errors.Is(err, calendar.ErrOutOfRange) {
		t.Errorf("closure at the exclusive end: %v", err)
	}
}

// TestGenerateParse_AllCombinations freezes the full supported range for
// every customary combination, with closures, and checks that the frozen
// answers equal the live calendar's on every date.
func TestGenerateParse_AllCombinations(t *testing.T) {
	full := span(t, "2000-01-01", "2100-01-01")
	closures := []calendar.Date{date(t, "2027-05-14"), date(t, "2027-05-15"), date(t, "2099-12-31")}
	customary := []calendar.Observance{calendar.OnsdagForSkjaertorsdag, calendar.Julaften, calendar.Nyttarsaften}
	for mask := range 8 {
		var opts []calendar.Observance
		for k, o := range customary {
			if mask&(1<<k) != 0 {
				opts = append(opts, o)
			}
		}
		c, err := calendar.Calendar{}.WithDaysOff(opts...)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Generate(c, full, OsloCivilDates, closures...)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) > maxBytes {
			t.Fatalf("full-range snapshot is %d bytes", len(b))
		}
		f, err := Parse(b)
		if err != nil {
			t.Fatalf("combination %d: %v", mask, err)
		}
		if f.DaysOff() != c.DaysOff() {
			t.Fatalf("DaysOff %q, want %q", f.DaysOff(), c.DaysOff())
		}
		all, _ := full.Dates()
		wantCount := 0
		for d := range all {
			want, _ := c.IsWorkday(d)
			if slices.Contains(closures, d) {
				want = false
			}
			if want {
				wantCount++
			}
			got, err := f.IsWorkday(d)
			if err != nil || got != want {
				t.Fatalf("combination %d, %s: frozen %v (%v), live %v", mask, d, got, err, want)
			}
		}
		if n, err := f.CountWorkdays(full); err != nil || n != wantCount {
			t.Fatalf("combination %d: CountWorkdays %d, %v; want %d", mask, n, err, wantCount)
		}
	}
}

func TestGenerate_KompassSpan(t *testing.T) {
	b, err := Generate(calendar.Calendar{}, span(t, "2026-11-01", "2036-01-01"), UTCDateLabels)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Generate(calendar.Calendar{}, span(t, "2026-11-01", "2036-01-01"), UTCDateLabels)
	if !bytes.Equal(b, again) {
		t.Fatal("Generate is not deterministic")
	}
	f, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	// W(2026-12-31) = 4–31 Dec 2026: 19 workdays.
	n, err := f.CountWorkdays(span(t, "2026-12-04", "2027-01-01"))
	if err != nil || n != 19 {
		t.Fatalf("W(2026-12-31) = %d, %v", n, err)
	}
	if !strings.HasPrefix(string(b), "entur-calendar-snapshot 1\nrules no-public/v1\nbasis utc-date-labels\ndaysoff none\nstart 2026-11-01\nend 2036-01-01\nexcluded 2026-12-25 forste-juledag\n") {
		t.Fatalf("unexpected head:\n%s", b[:200])
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(sample))
	f.Add([]byte(strings.Replace(sample, "daysoff julaften", "daysoff none", 1)))
	f.Add([]byte("entur-calendar-snapshot 1\nrules x\nbasis utc-date-labels\ndaysoff none\nstart 2026-12-26\nend 2026-12-28\ndays\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		fr, err := Parse(b)
		if err != nil {
			if !errors.Is(err, ErrInvalidSnapshot) || fr != nil {
				t.Fatalf("Parse error %v with result %v", err, fr)
			}
			return
		}
		if !bytes.Equal(fr.encode(), b) {
			t.Fatal("accepted input does not re-encode byte-identically")
		}
		n, err := fr.CountWorkdays(fr.Span())
		if err != nil || n != len(fr.days) {
			t.Fatalf("CountWorkdays(span) = %d, %v; %d day lines", n, err, len(fr.days))
		}
		again, err := Parse(fr.encode())
		if err != nil || !bytes.Equal(again.encode(), b) {
			t.Fatalf("re-parse: %v", err)
		}
	})
}

func ExampleGenerate() {
	r, _ := calendar.NewRange(mustParse("2026-12-21"), mustParse("2026-12-28"))
	b, err := Generate(calendar.Calendar{}, r, UTCDateLabels)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(b))
	// Output:
	// entur-calendar-snapshot 1
	// rules no-public/v1
	// basis utc-date-labels
	// daysoff none
	// start 2026-12-21
	// end 2026-12-28
	// excluded 2026-12-25 forste-juledag
	// days
	// 2026-12-21
	// 2026-12-22
	// 2026-12-23
	// 2026-12-24
}

func ExampleParse() {
	f, err := Parse([]byte(sample))
	if err != nil {
		panic(err)
	}
	n, _ := f.CountWorkdays(f.Span())
	ok, _ := f.IsWorkday(mustParse("2026-12-24"))
	fmt.Println(f.Rules(), f.Basis(), n, ok)
	// Output: no-public/v1 utc-date-labels 6 false
}

func mustParse(s string) calendar.Date {
	d, err := calendar.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}
