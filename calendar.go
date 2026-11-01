package calendar

import (
	"fmt"
	"iter"
	"time"
)

// Supported range and rules identity.
const (
	// MinYear is the first year calendar queries support.
	MinYear = 2000

	// MaxYear is the last year calendar queries support. 2100-01-01 is
	// accepted as the exclusive end of a Range.
	MaxYear = 2099

	// RulesVersion identifies the holiday rules of this module version. It
	// changes when any answer in a previously verified span changes (a new
	// law or a bug fix), never for a pure range extension. Snapshots record
	// it; callers who need answers that never change freeze a snapshot.
	RulesVersion = "no-public/v1"
)

// Rata die of the calendar bounds; TestCalendarBounds checks them.
const (
	minCalendarRD = 730120 // 2000-01-01
	endCalendarRD = 766645 // 2100-01-01, exclusive
)

// Calendar answers Norwegian working-day questions. It is a small,
// comparable value; the zero value is ready to use and treats Saturdays,
// Sundays and every Statutory observance as days off. WithDaysOff adds
// customary observances.
//
// Calendar{} follows the rules of this module version (RulesVersion): a
// module upgrade that fixes or changes a rule changes its answers. Freeze a
// snapshot (package snapshot) when answers must never change.
type Calendar struct {
	off ObservanceSet // opted-in customary observances only
}

// WithDaysOff returns a copy of c that also treats the given customary
// observances as days off. Statutory observances are always days off, so
// passing one is a no-op. An unknown value returns ErrInvalidObservance and
// leaves c unchanged.
func (c Calendar) WithDaysOff(o ...Observance) (Calendar, error) {
	off := c.off
	for _, x := range o {
		switch x.Kind() {
		case KindCustomary:
			off |= bit(x)
		case KindHelligdag, KindHoytidsdag:
			// Statutory: always off.
		default:
			return c, fmt.Errorf("WithDaysOff: %s: %w", x, ErrInvalidObservance)
		}
	}
	return Calendar{off: off}, nil
}

// DaysOff returns the observances that make a weekday a day off: Statutory
// plus the opted-in customary observances.
func (c Calendar) DaysOff() ObservanceSet { return statutorySet | c.off&customarySet }

// Observances returns every known observance on d, including customary
// observances the calendar does not treat as days off. A date can carry
// several (2027-05-17 is grunnlovsdag and annen pinsedag).
func (c Calendar) Observances(d Date) (ObservanceSet, error) {
	if err := checkDate(d); err != nil {
		return 0, err
	}
	return observancesOn(d.rd), nil
}

// IsWorkday reports whether d is Monday–Friday and carries no observance in
// c.DaysOff(). This is the conventional office-day calendar, not a legal
// "virkedag".
func (c Calendar) IsWorkday(d Date) (bool, error) {
	if err := checkDate(d); err != nil {
		return false, err
	}
	return c.isWorkday(d.rd), nil
}

// Workdays returns an iterator over the workdays in r. The range is
// validated before the iterator is returned, so the iterator cannot fail;
// it is reusable and stops as soon as the loop body breaks.
func (c Calendar) Workdays(r Range) (iter.Seq[Date], error) {
	if err := checkRange(r); err != nil {
		return nil, err
	}
	return func(yield func(Date) bool) {
		for d := range r.dates() {
			if c.isWorkday(d.rd) && !yield(d) {
				return
			}
		}
	}, nil
}

// CountWorkdays returns the number of workdays in r.
func (c Calendar) CountWorkdays(r Range) (int, error) {
	if err := checkRange(r); err != nil {
		return 0, err
	}
	n := 0
	for rd := r.start.rd; rd < r.end.rd; rd++ {
		if c.isWorkday(rd) {
			n++
		}
	}
	return n, nil
}

func (c Calendar) isWorkday(rd int32) bool {
	switch weekdayOf(rd) {
	case time.Saturday, time.Sunday:
		return false
	default:
		return observancesOn(rd)&c.DaysOff() == 0
	}
}

// observancesOn returns the observances on a date in the supported range.
// It reads the same table as Observance.DateIn, so the two cannot disagree.
func observancesOn(rd int32) ObservanceSet {
	year, month, day := civilFromRD(rd)
	fromEaster := rd - easterRD(year)
	var s ObservanceSet
	for o := Sondag + 1; o <= lastObservance; o++ {
		in, _ := o.info()
		if in.fromEaster && in.offset == fromEaster || !in.fromEaster && in.month == month && in.day == day {
			s |= 1 << o
		}
	}
	if weekdayOf(rd) == time.Sunday && s&namedHelligdager == 0 {
		s |= 1 << Sondag
	}
	return s
}

func checkDate(d Date) error {
	if d.IsZero() {
		return ErrZeroDate
	}
	if d.rd < minCalendarRD || d.rd >= endCalendarRD {
		return fmt.Errorf("%s outside %d-01-01..%d-12-31: %w", d, MinYear, MaxYear, ErrOutOfRange)
	}
	return nil
}

func checkRange(r Range) error {
	if err := r.valid(); err != nil {
		return err
	}
	if r.start.rd < minCalendarRD || r.end.rd > endCalendarRD {
		return fmt.Errorf("[%s, %s) outside [%d-01-01, %d-01-01): %w", r.start, r.end, MinYear, MaxYear+1, ErrOutOfRange)
	}
	return nil
}
