package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/entur/go-calendar"
	"github.com/entur/go-calendar/snapshot"
)

func runCmd(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun_MatchesGenerate(t *testing.T) {
	code, out, stderr := runCmd("-start", "2026-12-21", "-end", "2027-01-04", "-basis", "utc-date-labels",
		"-daysoff", "julaften", "-closed", "2027-01-02,2026-12-28")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	c, _ := calendar.Calendar{}.WithDaysOff(calendar.Julaften)
	s, _ := calendar.ParseDate("2026-12-21")
	e, _ := calendar.ParseDate("2027-01-04")
	r, _ := calendar.NewRange(s, e)
	c1, _ := calendar.ParseDate("2026-12-28")
	c2, _ := calendar.ParseDate("2027-01-02")
	want, err := snapshot.Generate(c, r, snapshot.UTCDateLabels, c1, c2)
	if err != nil {
		t.Fatal(err)
	}
	if out != string(want) {
		t.Fatalf("output differs from snapshot.Generate:\n%s", out)
	}
}

func TestRun_Verify(t *testing.T) {
	args := []string{"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc-date-labels"}
	code, out, stderr := runCmd(args...) // omitted: no verification
	if code != 0 || out == "" {
		t.Fatalf("without -verify-sha256: exit %d, %s", code, stderr)
	}
	sum := sha256.Sum256([]byte(out))
	digest := hex.EncodeToString(sum[:])

	code, verified, stderr := runCmd(append(args, "-verify-sha256", digest)...)
	if code != 0 || verified != out {
		t.Fatalf("matching digest: exit %d, %s", code, stderr)
	}
	code, mismatched, stderr := runCmd(append(args, "-verify-sha256", strings.Repeat("0", 64))...)
	if code != 1 || mismatched != "" || !strings.Contains(stderr, "SHA-256 mismatch") {
		t.Fatalf("mismatch: exit %d, stdout %q, stderr %q", code, mismatched, stderr)
	}
}

// TestRun_VerifyInvalid: a present but empty or malformed digest (for
// example an unset shell variable) is a usage error with nothing written,
// never a skipped check.
func TestRun_VerifyInvalid(t *testing.T) {
	args := []string{"-start", "2026-11-02", "-end", "2026-11-03", "-basis", "utc-date-labels"}
	_, out, _ := runCmd(args...)
	sum := sha256.Sum256([]byte(out))
	digest := hex.EncodeToString(sum[:])
	for name, value := range map[string]string{
		"explicitly empty": "",
		"63 characters":    digest[:63],
		"65 characters":    digest + "0",
		"uppercase":        strings.ToUpper(digest),
		"non-hex":          strings.Repeat("g", 64),
		"space padded":     " " + digest[1:],
	} {
		for _, form := range [][]string{{"-verify-sha256", value}, {"-verify-sha256=" + value}} {
			code, stdout, stderr := runCmd(append(slices.Clone(args), form...)...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "-verify-sha256") {
				t.Errorf("%s (%q): exit %d, stdout %d bytes, stderr %q", name, form, code, len(stdout), stderr)
			}
		}
	}
}

func TestRun_UsageErrors(t *testing.T) {
	for name, args := range map[string][]string{
		"missing basis":     {"-start", "2026-11-01", "-end", "2026-12-01"},
		"missing start":     {"-end", "2026-12-01", "-basis", "utc-date-labels"},
		"bad basis":         {"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc"},
		"bad date":          {"-start", "2026-11-1", "-end", "2026-12-01", "-basis", "utc-date-labels"},
		"reversed":          {"-start", "2026-12-01", "-end", "2026-11-01", "-basis", "utc-date-labels"},
		"out of range":      {"-start", "1999-12-01", "-end", "2000-01-05", "-basis", "utc-date-labels"},
		"statutory daysoff": {"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc-date-labels", "-daysoff", "grunnlovsdag"},
		"closed outside":    {"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc-date-labels", "-closed", "2026-12-01"},
		"bad closed":        {"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc-date-labels", "-closed", "x"},
		"extra argument":    {"-start", "2026-11-01", "-end", "2026-12-01", "-basis", "utc-date-labels", "file.txt"},
		"unknown flag":      {"-output", "file.txt"},
	} {
		code, out, stderr := runCmd(args...)
		if code != 2 || out != "" || stderr == "" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, out, stderr)
		}
	}
	if code, out, _ := runCmd("-h"); code != 0 || out != "" {
		t.Errorf("-h: exit %d, stdout %q", code, out)
	}
}
