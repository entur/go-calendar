package calendar

import (
	"errors"
	"testing"
)

func mustRange(t testing.TB, start, end string) Range {
	t.Helper()
	r, err := NewRange(mustDate(t, start), mustDate(t, end))
	if err != nil {
		t.Fatalf("NewRange(%s, %s): %v", start, end, err)
	}
	return r
}

func TestNewRange(t *testing.T) {
	a, b := mustDate(t, "2026-11-01"), mustDate(t, "2026-11-29")
	r, err := NewRange(a, b)
	if err != nil || r.Start() != a || r.End() != b || r.IsZero() {
		t.Fatalf("NewRange = %+v, %v", r, err)
	}
	if n, err := r.Len(); err != nil || n != 28 {
		t.Fatalf("Len = %d, %v", n, err)
	}
	if _, err := NewRange(b, a); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("reversed: %v", err)
	}
	if _, err := NewRange(Date{}, b); !errors.Is(err, ErrZeroDate) {
		t.Fatalf("zero start: %v", err)
	}
	if _, err := NewRange(a, Date{}); !errors.Is(err, ErrZeroDate) {
		t.Fatalf("zero end: %v", err)
	}
	empty, err := NewRange(a, a)
	if err != nil || empty.IsZero() {
		t.Fatalf("empty range: %+v, %v", empty, err)
	}
	if n, err := empty.Len(); err != nil || n != 0 {
		t.Fatalf("empty Len = %d, %v", n, err)
	}
}

func TestRange_Zero(t *testing.T) {
	var r Range
	if !r.IsZero() || !r.Start().IsZero() || !r.End().IsZero() {
		t.Fatal("zero Range must report zero endpoints")
	}
	if _, err := r.Len(); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("Len: %v", err)
	}
	if _, err := r.Contains(mustDate(t, "2026-01-01")); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("Contains: %v", err)
	}
	if seq, err := r.Dates(); !errors.Is(err, ErrInvalidRange) || seq != nil {
		t.Errorf("Dates: %v", err)
	}
}

func TestRange_Contains(t *testing.T) {
	r := mustRange(t, "2026-11-01", "2026-11-29")
	for s, want := range map[string]bool{
		"2026-10-31": false, "2026-11-01": true, "2026-11-28": true, "2026-11-29": false,
	} {
		if got, err := r.Contains(mustDate(t, s)); err != nil || got != want {
			t.Errorf("Contains(%s) = %v, %v; want %v", s, got, err, want)
		}
	}
	if _, err := r.Contains(Date{}); !errors.Is(err, ErrZeroDate) {
		t.Errorf("Contains(zero): %v", err)
	}
}

func TestRange_DatesIteratorContract(t *testing.T) {
	r := mustRange(t, "2026-12-30", "2027-01-03")
	seq, err := r.Dates()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for d := range seq {
		got = append(got, d.String())
	}
	want := []string{"2026-12-30", "2026-12-31", "2027-01-01", "2027-01-02"}
	if len(got) != len(want) {
		t.Fatalf("Dates = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Dates = %v, want %v", got, want)
		}
	}
	// Reusable: a second range-over yields the same dates.
	n := 0
	for range seq {
		n++
	}
	if n != len(want) {
		t.Fatalf("second iteration yielded %d", n)
	}
	// Stops on break.
	calls := 0
	for range seq {
		calls++
		break
	}
	if calls != 1 {
		t.Fatalf("break: %d", calls)
	}
}
