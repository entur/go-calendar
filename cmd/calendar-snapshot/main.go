// Command calendar-snapshot writes a canonical calendar snapshot to stdout.
//
//	calendar-snapshot -start 2026-11-01 -end 2036-01-01 -basis utc-date-labels \
//	    [-daysoff julaften,nyttarsaften] [-closed 2027-05-14] [-verify-sha256 HEX]
//
// It reads no files, fetches nothing and writes only to stdout (diagnostics
// to stderr). With -verify-sha256 it writes the snapshot only if its
// SHA-256 matches, and otherwise exits with status 1, so a pipeline never
// receives unapproved bytes. When the flag is present its value must be
// exactly 64 lowercase hexadecimal characters; an empty or malformed value
// (for example from an unset shell variable) is a usage error, never a
// skipped check. Usage errors exit with status 2.
//
// Record the command line, this module's version and the Go version next to
// the frozen file; they are not part of the snapshot bytes. Run a released
// version reproducibly with
//
//	go run github.com/entur/go-calendar/cmd/calendar-snapshot@vX.Y.Z ...
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/entur/go-calendar"
	"github.com/entur/go-calendar/snapshot"
)

// Exit statuses.
const (
	exitOK       = 0
	exitMismatch = 1 // digest mismatch or write error
	exitUsage    = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command and returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("calendar-snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	start := fs.String("start", "", "first date of the span, YYYY-MM-DD (required)")
	end := fs.String("end", "", "exclusive end of the span, YYYY-MM-DD (required)")
	basis := fs.String("basis", "", "date basis: utc-date-labels or oslo-civil-dates (required)")
	daysOff := fs.String("daysoff", "none", "customary days off, comma-separated: onsdag-for-skjaertorsdag, julaften, nyttarsaften; or none")
	closed := fs.String("closed", "", "one-off closure dates inside the span, comma-separated YYYY-MM-DD")
	verify := fs.String("verify-sha256", "", "expected SHA-256 of the output, 64 lowercase hex characters; on mismatch nothing is written and the exit status is 1")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		return usage(stderr, fmt.Errorf("unexpected arguments: %q", fs.Args()))
	}
	if *start == "" || *end == "" || *basis == "" {
		return usage(stderr, errors.New("-start, -end and -basis are required"))
	}
	// Presence, not the value, turns verification on: an explicitly empty
	// digest must fail, not silently skip the check.
	verifyRequested := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "verify-sha256" {
			verifyRequested = true
		}
	})
	if verifyRequested && !isSHA256Hex(*verify) {
		return usage(stderr, fmt.Errorf("-verify-sha256: want 64 lowercase hex characters, got %d characters", len(*verify)))
	}

	r, err := parseRange(*start, *end)
	if err != nil {
		return usage(stderr, err)
	}
	c, err := parseDaysOff(*daysOff)
	if err != nil {
		return usage(stderr, err)
	}
	closedDates, err := parseDates(*closed)
	if err != nil {
		return usage(stderr, err)
	}
	out, err := snapshot.Generate(c, r, snapshot.Basis(*basis), closedDates...)
	if err != nil {
		return usage(stderr, err)
	}

	if verifyRequested {
		sum := sha256.Sum256(out)
		got := hex.EncodeToString(sum[:])
		if got != *verify {
			_, _ = fmt.Fprintf(stderr, "calendar-snapshot: SHA-256 mismatch: got %s, want %s; nothing written\n", got, *verify)
			return exitMismatch
		}
	}
	if _, err := stdout.Write(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "calendar-snapshot: %v\n", err)
		return exitMismatch
	}
	return exitOK
}

// isSHA256Hex reports whether s is exactly 64 lowercase hex characters.
func isSHA256Hex(s string) bool {
	if len(s) != 2*sha256.Size {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func usage(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "calendar-snapshot: %v\n", err)
	return exitUsage
}

func parseRange(start, end string) (calendar.Range, error) {
	s, err := calendar.ParseDate(start)
	if err != nil {
		return calendar.Range{}, fmt.Errorf("-start: %w", err)
	}
	e, err := calendar.ParseDate(end)
	if err != nil {
		return calendar.Range{}, fmt.Errorf("-end: %w", err)
	}
	return calendar.NewRange(s, e)
}

func parseDaysOff(s string) (calendar.Calendar, error) {
	var c calendar.Calendar
	if s == "none" || s == "" {
		return c, nil
	}
	for slug := range strings.SplitSeq(s, ",") {
		found := false
		for o := range calendar.Customary().All() {
			if o.Slug() == slug {
				var err error
				if c, err = c.WithDaysOff(o); err != nil {
					return c, err
				}
				found = true
			}
		}
		if !found {
			return c, fmt.Errorf("-daysoff: %q is not a customary observance", slug)
		}
	}
	return c, nil
}

func parseDates(s string) ([]calendar.Date, error) {
	if s == "" {
		return nil, nil
	}
	var dates []calendar.Date
	for text := range strings.SplitSeq(s, ",") {
		d, err := calendar.ParseDate(text)
		if err != nil {
			return nil, fmt.Errorf("-closed: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, nil
}
