package calendar

// Error is the type of this package's sentinel errors. The sentinels are
// constants, so no importer can reassign them (an exported
// `var ErrX = errors.New(...)` can be), and errors.Is works on wrapped
// errors because Error is comparable.
type Error string

// Error returns the error message.
func (e Error) Error() string { return string(e) }

// Sentinel errors. Functions return them wrapped with context; test with
// errors.Is.
const (
	// ErrZeroDate reports that a zero Date ("no date") was used where a
	// date is required.
	ErrZeroDate Error = "calendar: zero date"

	// ErrInvalidDate reports a malformed or non-existent date, a date
	// outside 0001-01-01..9999-12-31, or date arithmetic leaving that range.
	ErrInvalidDate Error = "calendar: invalid date"

	// ErrOutOfRange reports a date or year outside the range that calendar
	// queries support (MinYear..MaxYear; 2100-01-01 is allowed as an
	// exclusive range end).
	ErrOutOfRange Error = "calendar: outside supported range"

	// ErrInvalidRange reports a reversed range or a zero Range.
	ErrInvalidRange Error = "calendar: invalid range"

	// ErrInvalidObservance reports an unknown Observance value, or an
	// Observance that is not valid for the operation (for example DateIn on
	// Sondag).
	ErrInvalidObservance Error = "calendar: invalid observance"

	// ErrInvalidLocation reports a nil *time.Location.
	ErrInvalidLocation Error = "calendar: invalid location"

	// ErrNonexistentMidnight reports that a date has no 00:00:00 in a
	// location, for example 2011-12-30 in Pacific/Apia, which skipped that
	// whole day.
	ErrNonexistentMidnight Error = "calendar: midnight does not exist in location"
)
