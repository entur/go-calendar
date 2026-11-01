// Package snapshot freezes a calendar into canonical bytes that a consumer
// commits, pins by SHA-256 and reads without the live holiday rules.
//
// A snapshot lists every workday in a span, every Monday–Friday that is
// not a workday together with the reasons, and the calendar's metadata.
// Membership is the list of workdays and nothing else, so a reader needs no
// holiday rules and a library upgrade can never change a frozen calendar.
//
// # Format
//
// ASCII, LF line endings, a final newline, no comments, fixed order:
//
//	entur-calendar-snapshot 1
//	rules no-public/v1
//	basis utc-date-labels
//	daysoff none
//	start 2026-11-01
//	end 2036-01-01
//	closed 2027-05-14
//	excluded 2026-12-25 forste-juledag
//	excluded 2027-05-14 closed
//	days
//	2026-11-02
//
// The header lines appear exactly once and in this order. daysoff is
// "none" or the opted-in customary slugs in lexicographic order,
// comma-joined. The span is [start, end). closed lists one-off closures
// (sorted, unique, inside the span; weekend closures are metadata only).
// excluded lists every Monday–Friday in the span that is not a workday,
// sorted, with its reasons: the day-off observance slugs in Observance
// order, then "closed" for a declared closure. days lists every workday,
// sorted. excluded and days together are exactly the Monday–Friday dates of
// the span. The Monday–Friday base is fixed for format version 1.
//
// The generator's module version and Go version are not part of the bytes;
// record them next to the frozen file.
package snapshot

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/entur/go-calendar"
)

// Basis records how the consumer labels days.
type Basis string

// Known bases.
const (
	// UTCDateLabels: the consumer's days are UTC calendar dates, looked up
	// as if they were Norwegian dates (entur/kompass's convention).
	UTCDateLabels Basis = "utc-date-labels"

	// OsloCivilDates: the consumer's days are civil dates in Europe/Oslo.
	OsloCivilDates Basis = "oslo-civil-dates"
)

// ErrInvalidSnapshot reports bytes that are not a canonical snapshot, a
// request Generate cannot turn into one, or a query on a nil or zero Frozen.
const ErrInvalidSnapshot calendar.Error = "snapshot: invalid snapshot"

const (
	formatLine  = "entur-calendar-snapshot 1"
	closedSlug  = "closed"
	maxBytes    = 4 << 20 // 4 MiB
	maxSpanDays = 36525   // the longest possible 100 Gregorian years
	maxRulesLen = 64
)

func (b Basis) valid() bool { return b == UTCDateLabels || b == OsloCivilDates }

type excludedRow struct {
	date    calendar.Date
	reasons calendar.ObservanceSet
	closed  bool
}

// Frozen is a parsed snapshot. It owns its state: nothing returned by its
// methods or passed to Parse can change its answers. A nil or zero Frozen
// answers every query with ErrInvalidSnapshot and every metadata accessor
// with a zero value.
type Frozen struct {
	valid    bool
	rules    string
	basis    Basis
	span     calendar.Range
	daysOff  calendar.ObservanceSet // customary observances listed in the header
	closed   []calendar.Date
	excluded []excludedRow
	days     []calendar.Date
}

// Generate freezes c over r. It validates the range and the calendar's year
// bounds, rejects an unknown basis, and rejects zero or out-of-span closure
// dates; closures are sorted and deduplicated. A closure on a weekday makes
// that day a non-workday with reason "closed"; a weekend closure is kept as
// metadata only.
func Generate(c calendar.Calendar, r calendar.Range, basis Basis, closed ...calendar.Date) ([]byte, error) {
	if !basis.valid() {
		return nil, fmt.Errorf("Generate: unknown basis %q: %w", basis, ErrInvalidSnapshot)
	}
	if _, err := c.Workdays(r); err != nil { // validates r and the calendar's bounds
		return nil, fmt.Errorf("Generate: %w", err)
	}
	closedDates := slices.Clone(closed)
	for _, d := range closedDates {
		in, err := r.Contains(d)
		if err != nil {
			return nil, fmt.Errorf("Generate: closed date: %w", err)
		}
		if !in {
			return nil, fmt.Errorf("Generate: closed date %s outside [%s, %s): %w", d, r.Start(), r.End(), calendar.ErrOutOfRange)
		}
	}
	slices.SortFunc(closedDates, calendar.Date.Compare)
	closedDates = slices.Compact(closedDates)

	f := &Frozen{
		valid:   true,
		rules:   calendar.RulesVersion,
		basis:   basis,
		span:    r,
		daysOff: c.DaysOff() & calendar.Customary(),
		closed:  closedDates,
	}
	all, _ := r.Dates() // r is valid
	for d := range all {
		if !isWeekday(d) {
			continue
		}
		obs, err := c.Observances(d)
		if err != nil {
			return nil, fmt.Errorf("Generate: %w", err)
		}
		row := excludedRow{date: d, reasons: obs & c.DaysOff(), closed: containsDate(closedDates, d)}
		if row.reasons != 0 || row.closed {
			f.excluded = append(f.excluded, row)
		} else {
			f.days = append(f.days, d)
		}
	}
	return f.encode(), nil
}

// Parse reads a canonical snapshot. It checks the format version, the
// known basis values, the header and slug grammar, valid civil dates, order,
// uniqueness, size (at most 4 MiB) and span (at most 36525 days); that
// excluded and days are disjoint and together exactly the Monday–Friday
// dates of the span; that every excluded row has a reason; that the
// "closed" reason appears exactly for the declared weekday closures; and
// that re-encoding reproduces the input bytes.
//
// Parse never evaluates holiday rules: it does not establish that the
// reasons are legally correct or that the bytes came from Generate. The
// rules line is preserved metadata and need not equal the current
// calendar.RulesVersion. Pin the approved bytes by SHA-256 in the consumer.
// Parse copies what it keeps; later changes to b do not affect the result.
func Parse(b []byte) (*Frozen, error) {
	if len(b) > maxBytes {
		return nil, fmt.Errorf("Parse: %d bytes exceeds %d: %w", len(b), maxBytes, ErrInvalidSnapshot)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return nil, fmt.Errorf("Parse: missing final newline: %w", ErrInvalidSnapshot)
	}
	for i, c := range b {
		if c != '\n' && (c < 0x20 || c > 0x7e) {
			return nil, fmt.Errorf("Parse: byte %d: not printable ASCII: %w", i, ErrInvalidSnapshot)
		}
	}
	p := parser{lines: strings.Split(string(b[:len(b)-1]), "\n")}
	f, err := p.parse()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(f.encode(), b) {
		return nil, fmt.Errorf("Parse: not in canonical form: %w", ErrInvalidSnapshot)
	}
	return f, nil
}

// Rules returns the rules identifier recorded in the snapshot.
func (f *Frozen) Rules() string {
	if !f.ok() {
		return ""
	}
	return f.rules
}

// Basis returns the date basis recorded in the snapshot.
func (f *Frozen) Basis() Basis {
	if !f.ok() {
		return ""
	}
	return f.basis
}

// Span returns the snapshot's span [start, end).
func (f *Frozen) Span() calendar.Range {
	if !f.ok() {
		return calendar.Range{}
	}
	return f.span
}

// DaysOff returns the observances that make a weekday a day off in this
// snapshot: calendar.Statutory() plus the customary observances in its
// daysoff line.
func (f *Frozen) DaysOff() calendar.ObservanceSet {
	if !f.ok() {
		return 0
	}
	return calendar.Statutory() | f.daysOff
}

// Closed returns a copy of the declared closure dates.
func (f *Frozen) Closed() []calendar.Date {
	if !f.ok() {
		return nil
	}
	return slices.Clone(f.closed)
}

// IsWorkday reports whether d is listed in the snapshot's days. A date
// outside the span is an error (calendar.ErrOutOfRange), never false.
func (f *Frozen) IsWorkday(d calendar.Date) (bool, error) {
	if !f.ok() {
		return false, fmt.Errorf("IsWorkday: nil or zero Frozen: %w", ErrInvalidSnapshot)
	}
	if d.IsZero() {
		return false, fmt.Errorf("IsWorkday: %w", calendar.ErrZeroDate)
	}
	if in, _ := f.span.Contains(d); !in {
		return false, fmt.Errorf("IsWorkday: %s outside snapshot span [%s, %s): %w", d, f.span.Start(), f.span.End(), calendar.ErrOutOfRange)
	}
	_, found := slices.BinarySearchFunc(f.days, d, calendar.Date.Compare)
	return found, nil
}

// Workdays returns an iterator over the snapshot's workdays in r, which
// must lie inside the span. The iterator cannot fail, is reusable and
// stops when the loop body breaks.
func (f *Frozen) Workdays(r calendar.Range) (iter.Seq[calendar.Date], error) {
	days, err := f.slice(r)
	if err != nil {
		return nil, fmt.Errorf("Workdays: %w", err)
	}
	return slices.Values(days), nil
}

// CountWorkdays returns the number of the snapshot's workdays in r, which
// must lie inside the span.
func (f *Frozen) CountWorkdays(r calendar.Range) (int, error) {
	days, err := f.slice(r)
	if err != nil {
		return 0, fmt.Errorf("CountWorkdays: %w", err)
	}
	return len(days), nil
}

func (f *Frozen) ok() bool { return f != nil && f.valid }

// slice returns the workdays in r; the caller must not modify it.
func (f *Frozen) slice(r calendar.Range) ([]calendar.Date, error) {
	if !f.ok() {
		return nil, fmt.Errorf("nil or zero Frozen: %w", ErrInvalidSnapshot)
	}
	if r.IsZero() {
		return nil, fmt.Errorf("zero Range: %w", calendar.ErrInvalidRange)
	}
	if r.Start().Before(f.span.Start()) || r.End().After(f.span.End()) {
		return nil, fmt.Errorf("[%s, %s) outside snapshot span [%s, %s): %w", r.Start(), r.End(), f.span.Start(), f.span.End(), calendar.ErrOutOfRange)
	}
	lo, _ := slices.BinarySearchFunc(f.days, r.Start(), calendar.Date.Compare)
	hi, _ := slices.BinarySearchFunc(f.days, r.End(), calendar.Date.Compare)
	return f.days[lo:hi:hi], nil
}

// encode writes f in canonical form.
func (f *Frozen) encode() []byte {
	var b bytes.Buffer
	b.Grow(64 + 11*len(f.days) + 40*len(f.excluded) + 18*len(f.closed))
	b.WriteString(formatLine + "\n")
	b.WriteString("rules " + f.rules + "\n")
	b.WriteString("basis " + string(f.basis) + "\n")
	b.WriteString("daysoff " + daysOffString(f.daysOff) + "\n")
	b.WriteString("start " + f.span.Start().String() + "\n")
	b.WriteString("end " + f.span.End().String() + "\n")
	for _, d := range f.closed {
		b.WriteString("closed " + d.String() + "\n")
	}
	for _, row := range f.excluded {
		b.WriteString("excluded " + row.date.String() + " " + reasonsString(row) + "\n")
	}
	b.WriteString("days\n")
	for _, d := range f.days {
		b.WriteString(d.String() + "\n")
	}
	return b.Bytes()
}

func daysOffString(s calendar.ObservanceSet) string {
	var slugs []string
	for o := range (s & calendar.Customary()).All() {
		slugs = append(slugs, o.Slug())
	}
	if len(slugs) == 0 {
		return "none"
	}
	slices.Sort(slugs)
	return strings.Join(slugs, ",")
}

func reasonsString(row excludedRow) string {
	s := row.reasons.String()
	if row.closed {
		if s != "" {
			s += ","
		}
		s += closedSlug
	}
	return s
}

func isWeekday(d calendar.Date) bool {
	wd, err := d.Weekday()
	return err == nil && wd != time.Saturday && wd != time.Sunday
}

func containsDate(sorted []calendar.Date, d calendar.Date) bool {
	_, found := slices.BinarySearchFunc(sorted, d, calendar.Date.Compare)
	return found
}

// observanceBySlug resolves a slug to an observance. Slugs are never
// renamed, so every slug ever published resolves.
func observanceBySlug(slug string) (calendar.Observance, bool) {
	for o := range (calendar.Statutory() | calendar.Customary()).All() {
		if o.Slug() == slug {
			return o, true
		}
	}
	return 0, false
}

type parser struct {
	lines []string
	n     int // index of the next line
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("Parse: line %d: %s: %w", p.n, fmt.Sprintf(format, args...), ErrInvalidSnapshot)
}

// header consumes a "key value" line and returns the value.
func (p *parser) header(key string) (string, error) {
	if p.n >= len(p.lines) {
		return "", p.errorf("missing %q line", key)
	}
	line := p.lines[p.n]
	p.n++
	value, ok := strings.CutPrefix(line, key+" ")
	if !ok || value == "" {
		return "", p.errorf("want %q line", key)
	}
	return value, nil
}

func (p *parser) date(s string) (calendar.Date, error) {
	d, err := calendar.ParseDate(s)
	if err != nil {
		return calendar.Date{}, p.errorf("invalid date %q", s)
	}
	return d, nil
}

func (p *parser) parse() (*Frozen, error) {
	f := &Frozen{valid: true}
	if len(p.lines) == 0 || p.lines[0] != formatLine {
		return nil, fmt.Errorf("Parse: line 1: want %q: %w", formatLine, ErrInvalidSnapshot)
	}
	p.n = 1
	var err error
	if f.rules, err = p.header("rules"); err != nil {
		return nil, err
	}
	if !validRules(f.rules) {
		return nil, p.errorf("invalid rules identifier")
	}
	basis, err := p.header("basis")
	if err != nil {
		return nil, err
	}
	if f.basis = Basis(basis); !f.basis.valid() {
		return nil, p.errorf("unknown basis %q", basis)
	}
	daysOff, err := p.header("daysoff")
	if err != nil {
		return nil, err
	}
	if f.daysOff, err = p.parseDaysOff(daysOff); err != nil {
		return nil, err
	}
	if err := p.parseSpan(f); err != nil {
		return nil, err
	}
	if err := p.parseRows(f); err != nil {
		return nil, err
	}
	if err := checkPartition(f); err != nil {
		return nil, err
	}
	return f, nil
}

func validRules(s string) bool {
	if len(s) > maxRulesLen || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '/' && c != '.' {
			return false
		}
	}
	return true
}

func (p *parser) parseDaysOff(s string) (calendar.ObservanceSet, error) {
	if s == "none" {
		return 0, nil
	}
	var set calendar.ObservanceSet
	prev := ""
	for slug := range strings.SplitSeq(s, ",") {
		o, ok := observanceBySlug(slug)
		if !ok || o.Kind() != calendar.KindCustomary {
			return 0, p.errorf("daysoff: %q is not a customary observance", slug)
		}
		if slug <= prev {
			return 0, p.errorf("daysoff: slugs must be sorted and unique")
		}
		prev = slug
		set |= calendar.ObservanceSet(1) << o
	}
	return set, nil
}

func (p *parser) parseSpan(f *Frozen) error {
	startText, err := p.header("start")
	if err != nil {
		return err
	}
	start, err := p.date(startText)
	if err != nil {
		return err
	}
	endText, err := p.header("end")
	if err != nil {
		return err
	}
	end, err := p.date(endText)
	if err != nil {
		return err
	}
	if f.span, err = calendar.NewRange(start, end); err != nil {
		return p.errorf("span: end before start")
	}
	if n, _ := f.span.Len(); n > maxSpanDays {
		return p.errorf("span of %d days exceeds %d", n, maxSpanDays)
	}
	return nil
}

func (p *parser) parseRows(f *Frozen) error {
	for p.n < len(p.lines) {
		value, ok := strings.CutPrefix(p.lines[p.n], "closed ")
		if !ok {
			break
		}
		p.n++
		d, err := p.date(value)
		if err != nil {
			return err
		}
		if err := p.inSpanAfter(f, d, f.closed); err != nil {
			return err
		}
		f.closed = append(f.closed, d)
	}
	for p.n < len(p.lines) {
		value, ok := strings.CutPrefix(p.lines[p.n], "excluded ")
		if !ok {
			break
		}
		p.n++
		dateText, reasons, ok := strings.Cut(value, " ")
		if !ok {
			return p.errorf("excluded: missing reasons")
		}
		d, err := p.date(dateText)
		if err != nil {
			return err
		}
		row, err := p.parseReasons(f, d, reasons)
		if err != nil {
			return err
		}
		if n := len(f.excluded); n > 0 && !f.excluded[n-1].date.Before(d) {
			return p.errorf("excluded: dates must be sorted and unique")
		}
		if err := p.weekdayInSpanAfter(f, d, nil); err != nil {
			return err
		}
		f.excluded = append(f.excluded, row)
	}
	if p.n >= len(p.lines) || p.lines[p.n] != "days" {
		p.n++
		return p.errorf(`want "days" line`)
	}
	p.n++
	for p.n < len(p.lines) {
		p.n++
		d, err := p.date(p.lines[p.n-1])
		if err != nil {
			return err
		}
		if err := p.weekdayInSpanAfter(f, d, f.days); err != nil {
			return err
		}
		f.days = append(f.days, d)
	}
	return nil
}

// inSpanAfter checks that d lies in the span and is after the last element
// of prev (sorted, unique).
func (p *parser) inSpanAfter(f *Frozen, d calendar.Date, prev []calendar.Date) error {
	if in, _ := f.span.Contains(d); !in {
		return p.errorf("%s outside the span", d)
	}
	if n := len(prev); n > 0 && !prev[n-1].Before(d) {
		return p.errorf("dates must be sorted and unique")
	}
	return nil
}

// weekdayInSpanAfter is inSpanAfter for rows that must be Monday–Friday.
func (p *parser) weekdayInSpanAfter(f *Frozen, d calendar.Date, prev []calendar.Date) error {
	if err := p.inSpanAfter(f, d, prev); err != nil {
		return err
	}
	if !isWeekday(d) {
		return p.errorf("%s is not Monday–Friday", d)
	}
	return nil
}

func (p *parser) parseReasons(f *Frozen, d calendar.Date, s string) (excludedRow, error) {
	row := excludedRow{date: d}
	last := calendar.Observance(0)
	tokens := strings.Split(s, ",")
	for i, slug := range tokens {
		if slug == closedSlug {
			if i != len(tokens)-1 {
				return row, p.errorf(`"closed" must be the last reason`)
			}
			row.closed = true
			continue
		}
		o, ok := observanceBySlug(slug)
		if !ok || o == calendar.Sondag {
			return row, p.errorf("unknown reason %q", slug)
		}
		if o.Kind() == calendar.KindCustomary && f.daysOff&(calendar.ObservanceSet(1)<<o) == 0 {
			return row, p.errorf("%q is not in daysoff", slug)
		}
		if o <= last {
			return row, p.errorf("reasons must be in observance order and unique")
		}
		last = o
		row.reasons |= calendar.ObservanceSet(1) << o
	}
	if row.closed != containsDate(f.closed, d) {
		return row, p.errorf(`"closed" reason for %s does not match the closed lines`, d)
	}
	return row, nil
}

// checkPartition verifies that excluded and days are disjoint and together
// exactly the Monday–Friday dates of the span, and that every weekday
// closure is excluded with the "closed" reason.
func checkPartition(f *Frozen) error {
	ex, dy := 0, 0
	all, _ := f.span.Dates()
	for d := range all {
		if !isWeekday(d) {
			continue
		}
		switch {
		case ex < len(f.excluded) && f.excluded[ex].date == d:
			ex++
			if dy < len(f.days) && f.days[dy] == d {
				return fmt.Errorf("Parse: %s is both excluded and a workday: %w", d, ErrInvalidSnapshot)
			}
		case dy < len(f.days) && f.days[dy] == d:
			dy++
		default:
			return fmt.Errorf("Parse: weekday %s is neither excluded nor a workday: %w", d, ErrInvalidSnapshot)
		}
	}
	if ex != len(f.excluded) || dy != len(f.days) {
		return fmt.Errorf("Parse: rows outside the weekday partition: %w", ErrInvalidSnapshot)
	}
	for _, d := range f.closed {
		if !isWeekday(d) {
			continue
		}
		i, found := slices.BinarySearchFunc(f.excluded, d, func(row excludedRow, d calendar.Date) int {
			return row.date.Compare(d)
		})
		if !found || !f.excluded[i].closed {
			return fmt.Errorf("Parse: weekday closure %s is not excluded as closed: %w", d, ErrInvalidSnapshot)
		}
	}
	return nil
}
