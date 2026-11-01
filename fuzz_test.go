package calendar

import (
	"errors"
	"math"
	"testing"
	"time"
)

func FuzzParseDate(f *testing.F) {
	for _, s := range []string{"2026-05-17", "0001-01-01", "9999-12-31", "2024-02-29", "2100-02-29", "0000-00-00", "", "2026-5-17", "+002-01-01"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseDate(s)
		ref, refErr := time.Parse(time.DateOnly, s)
		// time.Parse accepts the same shape but also year 0000; ParseDate
		// does not.
		refOK := refErr == nil && ref.Format(time.DateOnly) == s && ref.Year() >= 1
		if (err == nil) != refOK {
			t.Fatalf("ParseDate(%q) err %v, time.Parse err %v", s, err, refErr)
		}
		if err != nil {
			if !errors.Is(err, ErrInvalidDate) || !d.IsZero() {
				t.Fatalf("ParseDate(%q) = %v, %v", s, d, err)
			}
			return
		}
		if d.String() != s {
			t.Fatalf("round trip %q → %q", s, d.String())
		}
		b, err := d.MarshalText()
		if err != nil || string(b) != s {
			t.Fatalf("MarshalText = %q, %v", b, err)
		}
	})
}

func FuzzAddDays(f *testing.F) {
	f.Add(int32(minRD), 0)
	f.Add(int32(maxRD), 1)
	f.Add(int32(minCalendarRD), -1)
	f.Add(int32(730000), math.MaxInt)
	f.Add(int32(730000), math.MinInt)
	f.Fuzz(func(t *testing.T, rd int32, n int) {
		if rd < minRD || rd > maxRD {
			return
		}
		d := Date{rd: rd}
		got, err := d.AddDays(n)
		want := int64(rd) + int64(n)
		if n > maxRD || n < -maxRD || want < minRD || want > maxRD {
			if !errors.Is(err, ErrInvalidDate) || !got.IsZero() {
				t.Fatalf("%s.AddDays(%d) = %v, %v; want ErrInvalidDate", d, n, got, err)
			}
			return
		}
		if err != nil || int64(got.rd) != want {
			t.Fatalf("%s.AddDays(%d) = %v, %v", d, n, got, err)
		}
		if back, err := got.Sub(d); err != nil || back != n {
			t.Fatalf("Sub = %d, %v; want %d", back, err, n)
		}
		y, m, day := got.Date()
		ty := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
		if ty.Year() != y || ty.Month() != m || ty.Day() != day {
			t.Fatalf("%v is not a valid civil date", got)
		}
	})
}

func FuzzNewRange(f *testing.F) {
	f.Add(int32(minCalendarRD), int32(endCalendarRD))
	f.Add(int32(endCalendarRD), int32(endCalendarRD))
	f.Add(int32(5), int32(4))
	f.Add(int32(0), int32(10))
	f.Fuzz(func(t *testing.T, a, b int32) {
		if a < 0 || a > maxRD || b < 0 || b > maxRD {
			return
		}
		start, end := Date{rd: a}, Date{rd: b}
		r, err := NewRange(start, end)
		switch {
		case a == 0 || b == 0:
			if !errors.Is(err, ErrZeroDate) || !r.IsZero() {
				t.Fatalf("NewRange with zero endpoint = %v, %v", r, err)
			}
			return
		case b < a:
			if !errors.Is(err, ErrInvalidRange) || !r.IsZero() {
				t.Fatalf("reversed NewRange = %v, %v", r, err)
			}
			return
		case err != nil:
			t.Fatalf("NewRange(%d, %d): %v", a, b, err)
		default:
		}
		n, err := r.Len()
		if err != nil || n != int(b-a) {
			t.Fatalf("Len = %d, %v", n, err)
		}
		var c Calendar
		count, err := c.CountWorkdays(r)
		inCalendar := a >= minCalendarRD && b <= endCalendarRD
		if !inCalendar {
			if !errors.Is(err, ErrOutOfRange) {
				t.Fatalf("CountWorkdays outside the calendar range: %v", err)
			}
			return
		}
		if err != nil || count < 0 || count > n {
			t.Fatalf("CountWorkdays = %d, %v (len %d)", count, err, n)
		}
	})
}
