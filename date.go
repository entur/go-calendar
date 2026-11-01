package calendar

import (
	"cmp"
	"fmt"
	"time"
)

// Day numbers ("rata die"): days since 0000-12-31, so 0001-01-01 is 1.
const (
	minRD  = 1       // 0001-01-01
	maxRD  = 3652059 // 9999-12-31
	unixRD = 719163  // 1970-01-01

	// rdShift converts between rata die and the day index used by
	// rdFromCivil and civilFromRD, which counts from 0000-03-01 (index 0);
	// 0000-12-31 (rata die 0) has index 305.
	rdShift = 305
)

const digits = "0123456789"

// Date is a proleptic Gregorian civil date between 0001-01-01 and
// 9999-12-31. It has no time of day and no location.
//
// Date is a comparable value: use == and map keys freely. The zero value
// is "no date": IsZero reports true, String returns "0000-00-00", and every
// method that needs a date returns ErrZeroDate.
type Date struct {
	rd int32 // days since 0000-12-31; 0 is the zero value
}

// NewDate returns the date year-month-day. It is strict: unlike time.Date it
// never normalises, so 2026-02-30 is an error, not 2026-03-02. Years must be
// 1..9999.
func NewDate(year int, month time.Month, day int) (Date, error) {
	if year < 1 || year > 9999 || month < time.January || month > time.December ||
		day < 1 || day > daysIn(year, month) {
		return Date{}, fmt.Errorf("%04d-%02d-%02d: %w", year, int(month), day, ErrInvalidDate)
	}
	return Date{rd: rdFromCivil(year, month, day)}, nil
}

// ParseDate parses a date in exactly the form YYYY-MM-DD (ISO 8601
// calendar date, four-digit year, no sign, no time).
func ParseDate(s string) (Date, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return Date{}, fmt.Errorf("parse %q: %w", s, ErrInvalidDate)
	}
	year, ok1 := atoi(s[0:4])
	month, ok2 := atoi(s[5:7])
	day, ok3 := atoi(s[8:10])
	if !ok1 || !ok2 || !ok3 {
		return Date{}, fmt.Errorf("parse %q: %w", s, ErrInvalidDate)
	}
	d, err := NewDate(year, time.Month(month), day)
	if err != nil {
		return Date{}, fmt.Errorf("parse %q: %w", s, ErrInvalidDate)
	}
	return d, nil
}

// DateAt returns the civil date of the instant t in loc. The location is
// explicit and must not be nil (ErrInvalidLocation): the same instant has
// different dates in different zones (2026-11-01T23:30:00Z is Sunday
// 1 November in UTC and Monday 2 November in Europe/Oslo). This package
// never loads a location itself; pass time.UTC or a location you loaded.
func DateAt(t time.Time, loc *time.Location) (Date, error) {
	if loc == nil {
		return Date{}, fmt.Errorf("DateAt: nil location: %w", ErrInvalidLocation)
	}
	year, month, day := t.In(loc).Date()
	return NewDate(year, month, day)
}

// Date returns the year, month and day of d. The zero Date returns
// (0, 0, 0).
func (d Date) Date() (year int, month time.Month, day int) {
	if d.rd == 0 {
		return 0, 0, 0
	}
	return civilFromRD(d.rd)
}

// Year returns the year of d, or 0 for the zero Date.
func (d Date) Year() int {
	year, _, _ := d.Date()
	return year
}

// Month returns the month of d, or 0 for the zero Date.
func (d Date) Month() time.Month {
	_, month, _ := d.Date()
	return month
}

// Day returns the day of the month of d, or 0 for the zero Date.
func (d Date) Day() int {
	_, _, day := d.Date()
	return day
}

// Weekday returns the day of the week of d.
func (d Date) Weekday() (time.Weekday, error) {
	if d.rd == 0 {
		return 0, fmt.Errorf("Weekday: %w", ErrZeroDate)
	}
	return weekdayOf(d.rd), nil
}

// AddDays returns d plus n days (n may be negative). The result must stay
// within 0001-01-01..9999-12-31; the bounds are checked before any
// narrowing, so every int, including math.MinInt and math.MaxInt, is safe.
func (d Date) AddDays(n int) (Date, error) {
	if d.rd == 0 {
		return Date{}, fmt.Errorf("AddDays: %w", ErrZeroDate)
	}
	if n > maxRD-minRD || n < minRD-maxRD {
		return Date{}, fmt.Errorf("%s %+d days: %w", d, n, ErrInvalidDate)
	}
	r := int64(d.rd) + int64(n)
	if r < minRD || r > maxRD {
		return Date{}, fmt.Errorf("%s %+d days: %w", d, n, ErrInvalidDate)
	}
	return Date{rd: int32(r)}, nil
}

// Sub returns the number of days from e to d (d − e).
func (d Date) Sub(e Date) (int, error) {
	if d.rd == 0 || e.rd == 0 {
		return 0, fmt.Errorf("Sub: %w", ErrZeroDate)
	}
	return int(d.rd) - int(e.rd), nil
}

// Compare returns -1, 0 or +1 as d is before, equal to or after e. The zero
// Date sorts before every date. It fits slices.SortFunc(ds, Date.Compare).
func (d Date) Compare(e Date) int { return cmp.Compare(d.rd, e.rd) }

// Before reports whether d is before e.
func (d Date) Before(e Date) bool { return d.rd < e.rd }

// After reports whether d is after e.
func (d Date) After(e Date) bool { return d.rd > e.rd }

// IsZero reports whether d is the zero Date. Because Date has an IsZero
// method, a struct field tagged `json:",omitzero"` omits it (Go 1.24+).
func (d Date) IsZero() bool { return d.rd == 0 }

// In returns midnight (00:00:00) on d in loc. Unlike time.Date it never
// normalises: if loc has no midnight on d (a clock change at 00:00, or a
// skipped day such as 2011-12-30 in Pacific/Apia) it returns
// ErrNonexistentMidnight. If midnight occurs twice, either instant may be
// returned. A nil loc is ErrInvalidLocation.
func (d Date) In(loc *time.Location) (time.Time, error) {
	if d.rd == 0 {
		return time.Time{}, fmt.Errorf("In: %w", ErrZeroDate)
	}
	if loc == nil {
		return time.Time{}, fmt.Errorf("In: nil location: %w", ErrInvalidLocation)
	}
	year, month, day := civilFromRD(d.rd)
	t := time.Date(year, month, day, 0, 0, 0, 0, loc)
	gy, gm, gd := t.Date()
	hh, mm, ss := t.Clock()
	if gy != year || gm != month || gd != day || hh != 0 || mm != 0 || ss != 0 || t.Nanosecond() != 0 {
		return time.Time{}, fmt.Errorf("In: %s in %s: %w", d, loc, ErrNonexistentMidnight)
	}
	return t, nil
}

// String returns d as YYYY-MM-DD, or "0000-00-00" for the zero Date.
func (d Date) String() string {
	if d.rd == 0 {
		return "0000-00-00"
	}
	return string(d.appendISO(make([]byte, 0, 10)))
}

// AppendText appends d as YYYY-MM-DD to b. It implements
// encoding.TextAppender (Go 1.24+). The zero Date is an error.
func (d Date) AppendText(b []byte) ([]byte, error) {
	if d.rd == 0 {
		return b, fmt.Errorf("AppendText: %w", ErrZeroDate)
	}
	return d.appendISO(b), nil
}

// MarshalText implements encoding.TextMarshaler. Through it, Date encodes
// as a JSON string with encoding/json. The zero Date is an error.
func (d Date) MarshalText() ([]byte, error) {
	return d.AppendText(make([]byte, 0, 10))
}

// UnmarshalText implements encoding.TextUnmarshaler using ParseDate. On
// error, *d is unchanged.
func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Date) appendISO(b []byte) []byte {
	year, month, day := civilFromRD(d.rd)
	m := int(month)
	return append(b,
		digits[year/1000], digits[year/100%10], digits[year/10%10], digits[year%10], '-',
		digits[m/10], digits[m%10], '-',
		digits[day/10], digits[day%10])
}

// atoi parses a fixed-width run of ASCII digits.
func atoi(s string) (int, bool) {
	n := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func daysIn(year int, month time.Month) int {
	switch month {
	case time.February:
		if isLeap(year) {
			return 29
		}
		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	default:
		return 31
	}
}

// weekdayOf returns the weekday of a valid rata die. 0001-01-01 (rd 1) was
// a Monday in the proleptic Gregorian calendar.
func weekdayOf(rd int32) time.Weekday {
	return time.Weekday(rd % 7)
}

// rdFromCivil converts a valid date to rata die. It follows Howard
// Hinnant's days_from_civil, counted from 0000-03-01 so that the leap day
// is the last day of the computational year.
func rdFromCivil(year int, month time.Month, day int) int32 {
	y := year
	m := int(month)
	if m <= 2 {
		y--
	}
	era := y / 400 // y >= 0 for years 1..9999
	yoe := y - era*400
	mp := (m + 9) % 12 // March = 0
	doy := (153*mp+2)/5 + day - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return int32(era*146097 + doe - rdShift) //nolint:gosec // G115: callers pass validated dates, so the result is minRD..maxRD
}

// civilFromRD converts a valid rata die (minRD..maxRD) to a date. It is the
// inverse of rdFromCivil (Hinnant's civil_from_days).
func civilFromRD(rd int32) (year int, month time.Month, day int) {
	z := rd + rdShift
	era := z / 146097
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return int(y), time.Month(m), int(d)
}
