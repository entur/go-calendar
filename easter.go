package calendar

import (
	"fmt"
	"time"
)

// Easter returns første påskedag (Easter Sunday) in year, by the Gregorian
// ecclesiastical computus that Norway observes.
//
// Helligdagsfredloven § 2 e describes første påskedag as the first Sunday
// after the first full moon on or after 21 March. Read with the
// astronomical full moon that wording would give 24 March 2019; Norway
// observed 21 April 2019, the computus date. This function implements the
// computus. Years outside MinYear..MaxYear return ErrOutOfRange.
func Easter(year int) (Date, error) {
	if err := checkYear(year); err != nil {
		return Date{}, err
	}
	return Date{rd: easterRD(year)}, nil
}

// easterRD returns første påskedag of a valid year as rata die.
func easterRD(year int) int32 {
	month, day := easterMonthDay(year)
	return rdFromCivil(year, month, day)
}

// easterMonthDay is the anonymous Gregorian algorithm (Meeus/Jones/
// Butcher), valid for every Gregorian year from 1583. Tests check it
// against two independent algorithms for 1583–9999 and two external tables
// for 2000–2099.
func easterMonthDay(year int) (time.Month, int) {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	n := h + l - 7*m + 114
	return time.Month(n / 31), n%31 + 1
}

// checkYear reports ErrOutOfRange for years outside MinYear..MaxYear.
func checkYear(year int) error {
	if year < MinYear || year > MaxYear {
		return fmt.Errorf("year %d outside %d–%d: %w", year, MinYear, MaxYear, ErrOutOfRange)
	}
	return nil
}
