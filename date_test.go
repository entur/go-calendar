package calendar

import (
	"encoding"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"
	_ "time/tzdata" // hermetic Europe/Oslo and Pacific/Apia in tests only
)

var (
	_ encoding.TextAppender    = Date{}
	_ encoding.TextMarshaler   = Date{}
	_ encoding.TextUnmarshaler = (*Date)(nil)
)

func mustDate(t testing.TB, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func mustLocation(t testing.TB, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// TestDate_ExhaustiveVsTime checks every day 0001-01-01..9999-12-31 against
// the time package. It is not skipped under -short.
func TestDate_ExhaustiveVsTime(t *testing.T) {
	want := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	for rd := int32(minRD); rd <= maxRD; rd++ {
		d := Date{rd: rd}
		y, m, day := d.Date()
		if y != want.Year() || m != want.Month() || day != want.Day() {
			t.Fatalf("rd %d: Date() = %04d-%02d-%02d, time says %s", rd, y, m, day, want.Format(time.DateOnly))
		}
		wd, err := d.Weekday()
		if err != nil || wd != want.Weekday() {
			t.Fatalf("%s: Weekday() = %v, %v; want %v", want.Format(time.DateOnly), wd, err, want.Weekday())
		}
		back, err := NewDate(y, m, day)
		if err != nil || back != d {
			t.Fatalf("NewDate(%04d-%02d-%02d) = %v, %v; want rd %d", y, m, day, back.rd, err, rd)
		}
		if rd%997 == 0 || rd == maxRD { // String allocates; sample it
			if got := d.String(); got != want.Format(time.DateOnly) {
				t.Fatalf("String() = %q, want %q", got, want.Format(time.DateOnly))
			}
		}
		want = want.AddDate(0, 0, 1)
	}
	if want.Format(time.DateOnly) != "10000-01-01" {
		t.Fatalf("loop ended at %s", want.Format(time.DateOnly))
	}
}

func TestDate_Bounds(t *testing.T) {
	if rdFromCivil(1, time.January, 1) != minRD || rdFromCivil(9999, time.December, 31) != maxRD {
		t.Fatal("minRD/maxRD constants disagree with rdFromCivil")
	}
	if rdFromCivil(1970, time.January, 1) != unixRD {
		t.Fatal("unixRD disagrees with rdFromCivil")
	}
	if rdFromCivil(MinYear, time.January, 1) != minCalendarRD || rdFromCivil(MaxYear+1, time.January, 1) != endCalendarRD {
		t.Fatal("calendar bound constants disagree with rdFromCivil")
	}
}

func TestNewDate_Invalid(t *testing.T) {
	cases := []struct {
		y     int
		m     time.Month
		d     int
		valid bool
	}{
		{2024, time.February, 29, true},
		{2000, time.February, 29, true},
		{1900, time.February, 29, false},
		{2100, time.February, 29, false},
		{2026, time.February, 30, false},
		{2026, time.April, 31, false},
		{2026, time.December, 31, true},
		{2026, 0, 1, false},
		{2026, 13, 1, false},
		{2026, time.January, 0, false},
		{2026, time.January, 32, false},
		{0, time.January, 1, false},
		{1, time.January, 1, true},
		{9999, time.December, 31, true},
		{10000, time.January, 1, false},
		{-1, time.January, 1, false},
		{math.MaxInt, time.January, 1, false},
		{math.MinInt, time.January, 1, false},
	}
	for _, c := range cases {
		d, err := NewDate(c.y, c.m, c.d)
		if c.valid {
			if err != nil || d.IsZero() {
				t.Errorf("NewDate(%d, %d, %d) = %v, %v; want valid", c.y, c.m, c.d, d, err)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidDate) || !d.IsZero() {
			t.Errorf("NewDate(%d, %d, %d) = %v, %v; want zero, ErrInvalidDate", c.y, c.m, c.d, d, err)
		}
	}
}

func TestParseDate(t *testing.T) {
	valid := []string{"2026-05-17", "0001-01-01", "9999-12-31", "2024-02-29"}
	for _, s := range valid {
		d, err := ParseDate(s)
		if err != nil || d.String() != s {
			t.Errorf("ParseDate(%q) = %v, %v", s, d, err)
		}
	}
	invalid := []string{
		"", "2026-5-17", "2026-05-7", "26-05-17", "+2026-05-17", " 2026-05-17", "2026-05-17 ",
		"2026/05/17", "2026-13-01", "2026-02-29", "0000-01-01", "2026-05-17T00:00:00Z",
		"20260517", "2026-0a-17", "２０２６-05-17", "2026-05--1",
	}
	for _, s := range invalid {
		if d, err := ParseDate(s); !errors.Is(err, ErrInvalidDate) || !d.IsZero() {
			t.Errorf("ParseDate(%q) = %v, %v; want ErrInvalidDate", s, d, err)
		}
	}
}

func TestDate_ZeroValue(t *testing.T) {
	var z Date
	if !z.IsZero() || z.String() != "0000-00-00" {
		t.Fatalf("zero Date: IsZero %v, String %q", z.IsZero(), z.String())
	}
	if y, m, d := z.Date(); y != 0 || m != 0 || d != 0 || z.Year() != 0 || z.Month() != 0 || z.Day() != 0 {
		t.Fatal("zero Date must have zero components")
	}
	if _, err := z.Weekday(); !errors.Is(err, ErrZeroDate) {
		t.Errorf("Weekday: %v", err)
	}
	if _, err := z.AddDays(1); !errors.Is(err, ErrZeroDate) {
		t.Errorf("AddDays: %v", err)
	}
	if _, err := z.Sub(mustDate(t, "2026-01-01")); !errors.Is(err, ErrZeroDate) {
		t.Errorf("Sub: %v", err)
	}
	if _, err := mustDate(t, "2026-01-01").Sub(z); !errors.Is(err, ErrZeroDate) {
		t.Errorf("Sub(zero): %v", err)
	}
	if _, err := z.In(time.UTC); !errors.Is(err, ErrZeroDate) {
		t.Errorf("In: %v", err)
	}
	if _, err := z.MarshalText(); !errors.Is(err, ErrZeroDate) {
		t.Errorf("MarshalText: %v", err)
	}
	if b, err := z.AppendText([]byte("x")); !errors.Is(err, ErrZeroDate) || string(b) != "x" {
		t.Errorf("AppendText: %q, %v", b, err)
	}
	if z.Compare(mustDate(t, "0001-01-01")) != -1 || !z.Before(mustDate(t, "0001-01-01")) {
		t.Error("zero Date must sort before every date")
	}
}

func TestDate_AddDays(t *testing.T) {
	d := mustDate(t, "2026-11-28")
	got, err := d.AddDays(-27)
	if err != nil || got.String() != "2026-11-01" {
		t.Fatalf("AddDays(-27) = %v, %v", got, err)
	}
	if got, err := d.AddDays(0); err != nil || got != d {
		t.Fatalf("AddDays(0) = %v, %v", got, err)
	}
	last := mustDate(t, "9999-12-31")
	first := mustDate(t, "0001-01-01")
	for _, c := range []struct {
		d Date
		n int
	}{
		{last, 1}, {first, -1}, {d, math.MaxInt}, {d, math.MinInt}, {d, math.MaxInt32}, {d, math.MinInt32},
		{first, maxRD}, {last, -maxRD}, {d, maxRD - minRD + 1}, {d, minRD - maxRD - 1},
	} {
		if r, err := c.d.AddDays(c.n); !errors.Is(err, ErrInvalidDate) || !r.IsZero() {
			t.Errorf("%s.AddDays(%d) = %v, %v; want ErrInvalidDate", c.d, c.n, r, err)
		}
	}
	if r, err := first.AddDays(maxRD - minRD); err != nil || r != last {
		t.Errorf("first.AddDays(max span) = %v, %v", r, err)
	}
	if r, err := last.AddDays(minRD - maxRD); err != nil || r != first {
		t.Errorf("last.AddDays(-max span) = %v, %v", r, err)
	}
}

func TestDate_SubCompare(t *testing.T) {
	a, b := mustDate(t, "2026-11-01"), mustDate(t, "2026-11-28")
	if n, err := b.Sub(a); err != nil || n != 27 {
		t.Fatalf("Sub = %d, %v", n, err)
	}
	if n, _ := a.Sub(b); n != -27 {
		t.Fatalf("Sub reversed = %d", n)
	}
	if a.Compare(b) != -1 || b.Compare(a) != 1 || a.Compare(mustDate(t, "2026-11-01")) != 0 || !a.Before(b) || !b.After(a) || a.After(b) {
		t.Fatal("Compare/Before/After disagree")
	}
}

func TestDateAt(t *testing.T) {
	oslo := mustLocation(t, "Europe/Oslo")
	instant := time.Date(2026, time.November, 1, 23, 30, 0, 0, time.UTC)
	utc, err := DateAt(instant, time.UTC)
	if err != nil || utc.String() != "2026-11-01" {
		t.Fatalf("UTC: %v, %v", utc, err)
	}
	local, err := DateAt(instant, oslo)
	if err != nil || local.String() != "2026-11-02" {
		t.Fatalf("Oslo: %v, %v", local, err)
	}
	// 17 May 00:30 in Oslo (CEST) is 16 May in UTC: the UTC-label convention.
	may := time.Date(2026, time.May, 17, 0, 30, 0, 0, oslo)
	if d, _ := DateAt(may, time.UTC); d.String() != "2026-05-16" {
		t.Fatalf("UTC label of 17 May 00:30 Oslo = %v", d)
	}
	if d, _ := DateAt(may, oslo); d.String() != "2026-05-17" {
		t.Fatalf("Oslo date of 17 May 00:30 Oslo = %v", d)
	}
	// Around the DST change (last Sunday of March 2026, 02:00 → 03:00).
	dst := time.Date(2026, time.March, 29, 0, 59, 0, 0, time.UTC)
	if d, _ := DateAt(dst, oslo); d.String() != "2026-03-29" {
		t.Fatalf("DST day: %v", d)
	}
	if _, err := DateAt(instant, nil); !errors.Is(err, ErrInvalidLocation) {
		t.Fatalf("nil location: %v", err)
	}
	if _, err := DateAt(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), time.UTC); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("year 10000: %v", err)
	}
}

func TestDate_In(t *testing.T) {
	oslo := mustLocation(t, "Europe/Oslo")
	d := mustDate(t, "2026-03-29")
	got, err := d.In(oslo)
	if err != nil || got.Format(time.RFC3339) != "2026-03-29T00:00:00+01:00" {
		t.Fatalf("In(Oslo) = %v, %v", got, err)
	}
	if got, err := d.In(time.UTC); err != nil || !got.Equal(time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("In(UTC) = %v, %v", got, err)
	}
	if _, err := d.In(nil); !errors.Is(err, ErrInvalidLocation) {
		t.Fatalf("In(nil) = %v", err)
	}
	// Samoa skipped 2011-12-30 entirely; time.Date would silently return
	// 2011-12-29T00:00:00-10:00.
	apia := mustLocation(t, "Pacific/Apia")
	if got, err := mustDate(t, "2011-12-30").In(apia); !errors.Is(err, ErrNonexistentMidnight) || !got.IsZero() {
		t.Fatalf("In(Apia) on the skipped day = %v, %v; want ErrNonexistentMidnight", got, err)
	}
	if got, err := mustDate(t, "2011-12-31").In(apia); err != nil || got.Day() != 31 {
		t.Fatalf("In(Apia) on 2011-12-31 = %v, %v", got, err)
	}
}

func TestDate_Text(t *testing.T) {
	d := mustDate(t, "2027-05-17")
	b, err := d.MarshalText()
	if err != nil || string(b) != "2027-05-17" {
		t.Fatalf("MarshalText = %q, %v", b, err)
	}
	if b, err := d.AppendText([]byte("on ")); err != nil || string(b) != "on 2027-05-17" {
		t.Fatalf("AppendText = %q, %v", b, err)
	}
	var back Date
	if err := back.UnmarshalText(b); err != nil || back != d {
		t.Fatalf("UnmarshalText = %v, %v", back, err)
	}
	keep := d
	if err := keep.UnmarshalText([]byte("bad")); !errors.Is(err, ErrInvalidDate) || keep != d {
		t.Fatalf("UnmarshalText(bad) changed the date or returned %v", err)
	}
	type row struct {
		Day  Date `json:"day"`
		Skip Date `json:"skip,omitzero"`
	}
	js, err := json.Marshal(row{Day: d})
	if err != nil || string(js) != `{"day":"2027-05-17"}` {
		t.Fatalf("json.Marshal = %s, %v", js, err)
	}
	var r row
	if err := json.Unmarshal(js, &r); err != nil || r.Day != d {
		t.Fatalf("json.Unmarshal = %+v, %v", r, err)
	}
}
