package calendar

import (
	"fmt"
	"iter"
)

// Range is a half-open span of dates [Start, End). Construct it with
// NewRange. Equal endpoints make a valid, empty range. The zero Range is
// invalid: Len, Contains, Dates and every Calendar range query return
// ErrInvalidRange for it.
//
// Half-open ranges add up: the count over [a, c) equals the count over
// [a, b) plus [b, c). A 28-day window ending on D (inclusive) is
// NewRange(D−27, D+1).
type Range struct {
	start, end Date
}

// NewRange returns the range [start, end). Both dates must be non-zero
// (ErrZeroDate) and start must not be after end; a reversed range is
// ErrInvalidRange, never a negative count.
func NewRange(start, end Date) (Range, error) {
	if start.IsZero() || end.IsZero() {
		return Range{}, fmt.Errorf("NewRange: %w", ErrZeroDate)
	}
	if end.Before(start) {
		return Range{}, fmt.Errorf("NewRange [%s, %s): end before start: %w", start, end, ErrInvalidRange)
	}
	return Range{start: start, end: end}, nil
}

// Start returns the first date of r (inclusive). The zero Range returns the
// zero Date.
func (r Range) Start() Date { return r.start }

// End returns the end of r (exclusive). The zero Range returns the zero
// Date.
func (r Range) End() Date { return r.end }

// IsZero reports whether r is the zero Range (not constructed by NewRange).
func (r Range) IsZero() bool { return r.start.IsZero() }

// Len returns the number of dates in r.
func (r Range) Len() (int, error) {
	if err := r.valid(); err != nil {
		return 0, err
	}
	return int(r.end.rd - r.start.rd), nil
}

// Contains reports whether d is in r. A zero d is ErrZeroDate.
func (r Range) Contains(d Date) (bool, error) {
	if err := r.valid(); err != nil {
		return false, err
	}
	if d.IsZero() {
		return false, fmt.Errorf("Contains: %w", ErrZeroDate)
	}
	return r.start.rd <= d.rd && d.rd < r.end.rd, nil
}

// Dates returns an iterator over every date in r, in ascending order. The
// range is validated before the iterator is returned, so the iterator
// cannot fail. It is reusable and stops as soon as the loop body breaks.
func (r Range) Dates() (iter.Seq[Date], error) {
	if err := r.valid(); err != nil {
		return nil, err
	}
	return r.dates(), nil
}

func (r Range) valid() error {
	if r.IsZero() {
		return fmt.Errorf("zero Range: %w", ErrInvalidRange)
	}
	return nil
}

// dates iterates over a validated range.
func (r Range) dates() iter.Seq[Date] {
	return func(yield func(Date) bool) {
		for rd := r.start.rd; rd < r.end.rd; rd++ {
			if !yield(Date{rd: rd}) {
				return
			}
		}
	}
}
