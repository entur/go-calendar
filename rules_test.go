package calendar

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A fixture is a retained golden file and the SHA-256 it was approved with.
type fixture struct {
	path, sha256 string
}

// A rulesRecord is one verified span of one rules identifier. Records are
// append-only: never edit or delete one, and never overwrite its files.
//
//   - A changed law or a bug fix that changes any answer inside a verified
//     span gets a new rules identifier (RulesVersion) and a new record. Its
//     cutoff is the civil effective date of the law, or for a bug fix the
//     earliest date whose answer is intentionally corrected; every answer
//     before the cutoff must equal the previous identifier's. Record the
//     issue in reason and use a fix!: commit for a bug fix.
//   - A pure range extension adds a record with the same identifier, a new
//     span and new fixture paths; every answer on every overlap with earlier
//     records of that identifier must be unchanged.
type rulesRecord struct {
	rules        string // rules identifier
	start, end   string // verified span [start, end)
	cutoff       string // first date allowed to differ from the previous identifier ("" for the first record)
	reason       string
	observations fixture
	workdays     [8]fixture // indexed by customary combination, see comboCalendar
}

// rulesHistory is the append-only record of verified rules. A function, not
// a variable, so the package keeps no globals.
func rulesHistory() []rulesRecord {
	const dir = "testdata/golden/"
	return []rulesRecord{{
		rules:  "no-public/v1",
		start:  "2000-01-01",
		end:    "2100-01-01",
		cutoff: "",
		reason: "initial rules (doc/adr/0001-norwegian-calendar-design.adoc)",
		observations: fixture{dir + "observances-no-public-v1.txt",
			"c1ea4a78c0262164028a2e51fc6621e0ff212f4d97a95c7c47a6c088317c960a"},
		workdays: [8]fixture{
			{dir + "workdays-no-public-v1-none.txt", "5d43c2d6d8ccd2002f52f2b92cf7aefb8dbb8578a8bfa44cf1c2f8519ed51f2f"},
			{dir + "workdays-no-public-v1-onsdag-for-skjaertorsdag.txt", "46d9c51ba5cffada128047ca64a8012169a9b0d30518c0f0a29896ea6a76f09d"},
			{dir + "workdays-no-public-v1-julaften.txt", "40e7db243cd862104858dacfb65774d1627fdb7549b1713a989ef9e255bed2c3"},
			{dir + "workdays-no-public-v1-onsdag-for-skjaertorsdag+julaften.txt", "c1cb1448bc9ebca3a370ee4b69488e9f5faaf2201480cd778b44119e93b9ac3c"},
			{dir + "workdays-no-public-v1-nyttarsaften.txt", "e65db5556872be4b7df292c578b1e0977d5448144f4272295c210a3338a04817"},
			{dir + "workdays-no-public-v1-onsdag-for-skjaertorsdag+nyttarsaften.txt", "16d1757c7412bcca760fb5b4a94eea62a8c9b15e8ee0a229f039aeb862857228"},
			{dir + "workdays-no-public-v1-julaften+nyttarsaften.txt", "42279842741fb43d562fde280df46980231622d0bec2e4c1d91b912fe42a68f3"},
			{dir + "workdays-no-public-v1-onsdag-for-skjaertorsdag+julaften+nyttarsaften.txt", "85afc6b8683594de1996d84fdaa579da210c6d9d77ce456af1c6fdf93ff98f2f"},
		},
	}}
}

// customaryOrder fixes the bit of each customary observance in a
// combination index.
func customaryOrder() [3]Observance {
	return [3]Observance{OnsdagForSkjaertorsdag, Julaften, Nyttarsaften}
}

// comboCalendar returns the calendar for combination index i (0..7): bit k
// opts in customaryOrder()[k].
func comboCalendar(t testing.TB, i int) Calendar {
	t.Helper()
	var c Calendar
	for k, o := range customaryOrder() {
		if i&(1<<k) != 0 {
			var err error
			if c, err = c.WithDaysOff(o); err != nil {
				t.Fatal(err)
			}
		}
	}
	return c
}

func comboName(i int) string {
	var names []string
	for k, o := range customaryOrder() {
		if i&(1<<k) != 0 {
			names = append(names, o.Slug())
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "+")
}

// observationsFixture lists every date in r that carries an observance:
// "YYYY-MM-DD slug[,slug]".
func observationsFixture(t testing.TB, r Range) []byte {
	t.Helper()
	var c Calendar
	var b bytes.Buffer
	dates, err := r.Dates()
	if err != nil {
		t.Fatal(err)
	}
	for d := range dates {
		s, err := c.Observances(d)
		if err != nil {
			t.Fatal(err)
		}
		if s != 0 {
			fmt.Fprintf(&b, "%s %s\n", d, s)
		}
	}
	return b.Bytes()
}

// workdaysFixture records per-date membership: one line per year (or part
// of a year inside r), "YYYY-MM-DD <bits>", where the date is the first day
// of the line and each bit is 1 for a workday and 0 otherwise.
func workdaysFixture(t testing.TB, c Calendar, r Range) []byte {
	t.Helper()
	return workdaysFixtureWith(t, c, r, nil)
}

// workdaysFixtureWith is workdaysFixture with the dates in off forced to
// non-workdays; tests use it to build synthetic rules records.
func workdaysFixtureWith(t testing.TB, c Calendar, r Range, off map[Date]bool) []byte {
	t.Helper()
	var b bytes.Buffer
	dates, err := r.Dates()
	if err != nil {
		t.Fatal(err)
	}
	year := 0
	for d := range dates {
		if d.Year() != year {
			if year != 0 {
				b.WriteByte('\n')
			}
			year = d.Year()
			fmt.Fprintf(&b, "%s ", d)
		}
		ok, err := c.IsWorkday(d)
		if err != nil {
			t.Fatal(err)
		}
		if ok && !off[d] {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	if year != 0 {
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// answers maps each date of a fixture's span to its answer: the slugs for
// an observations fixture ("" when none), "1"/"0" for a workdays fixture.
type answers map[Date]string

func parseObservations(t testing.TB, b []byte, r Range) answers {
	t.Helper()
	a := answers{}
	dates, _ := r.Dates()
	for d := range dates {
		a[d] = ""
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		dateText, slugs, ok := strings.Cut(sc.Text(), " ")
		d, err := ParseDate(dateText)
		if !ok || err != nil || slugs == "" {
			t.Fatalf("observations fixture: bad line %q", sc.Text())
		}
		if _, in := a[d]; !in {
			t.Fatalf("observations fixture: %s outside [%s, %s)", d, r.start, r.end)
		}
		a[d] = slugs
	}
	return a
}

func parseWorkdays(t testing.TB, b []byte, r Range) answers {
	t.Helper()
	a := answers{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 512), 1<<20)
	for sc.Scan() {
		dateText, bitsText, ok := strings.Cut(sc.Text(), " ")
		d, err := ParseDate(dateText)
		if !ok || err != nil {
			t.Fatalf("workdays fixture: bad line %q", sc.Text())
		}
		for i := range len(bitsText) {
			if bitsText[i] != '0' && bitsText[i] != '1' {
				t.Fatalf("workdays fixture: bad bit in line for %s", d)
			}
			a[d] = bitsText[i : i+1]
			if d, err = d.AddDays(1); err != nil {
				t.Fatal(err)
			}
		}
	}
	n, _ := r.Len()
	if len(a) != n {
		t.Fatalf("workdays fixture covers %d dates, span has %d", len(a), n)
	}
	return a
}

// firstDifferences returns up to max dates in [from, to) whose answers
// differ, formatted for a test failure.
func firstDifferences(old, current answers, from, to Date, limit int) []string {
	var diffs []string
	r, err := NewRange(from, to)
	if err != nil {
		return nil
	}
	dates, _ := r.Dates()
	for d := range dates {
		o, inOld := old[d]
		c, inCurrent := current[d]
		if inOld && inCurrent && o != c {
			diffs = append(diffs, fmt.Sprintf("%s: %q → %q", d, o, c))
			if len(diffs) == limit {
				break
			}
		}
	}
	return diffs
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func recordRange(t testing.TB, rec rulesRecord) Range {
	t.Helper()
	return mustRange(t, rec.start, rec.end)
}

func TestRulesHistory_Records(t *testing.T) {
	history := rulesHistory()
	if len(history) == 0 {
		t.Fatal("empty rulesHistory")
	}
	if last := history[len(history)-1]; last.rules != RulesVersion {
		t.Fatalf("RulesVersion %q, but the newest rulesHistory record is %q", RulesVersion, last.rules)
	}
	paths := map[string]bool{}
	seen := map[string]bool{}
	for i, rec := range history {
		r := recordRange(t, rec)
		if n, _ := r.Len(); n == 0 {
			t.Errorf("record %d: empty span", i)
		}
		switch {
		case i == 0 && rec.cutoff != "":
			t.Errorf("record 0 must have no cutoff")
		case i > 0 && !seen[rec.rules] && rec.cutoff == "":
			t.Errorf("record %d: a new identifier %q needs a cutoff", i, rec.rules)
		case seen[rec.rules] && rec.cutoff != "":
			t.Errorf("record %d: a range extension of %q has no cutoff", i, rec.rules)
		default:
		}
		if i > 0 && seen[rec.rules] && history[i-1].rules != rec.rules {
			t.Errorf("record %d: identifier %q reused after a newer identifier", i, rec.rules)
		}
		seen[rec.rules] = true
		for _, f := range append([]fixture{rec.observations}, rec.workdays[:]...) {
			if paths[f.path] {
				t.Errorf("fixture %s recorded twice", f.path)
			}
			paths[f.path] = true
			b, err := os.ReadFile(f.path)
			if err != nil {
				t.Errorf("record %d: %v", i, err)
				continue
			}
			if got := sha256Hex(b); got != f.sha256 {
				t.Errorf("%s: SHA-256 %s, recorded %s (retained fixtures are never changed)", f.path, got, f.sha256)
			}
		}
	}
}

// TestRulesHistory_Current regenerates every fixture of the current
// identifier from the live rules and requires byte equality. On a mismatch
// it reports the first differing dates and writes the regenerated file to
// t.ArtifactDir() for review. There is no -update flag.
func TestRulesHistory_Current(t *testing.T) {
	for _, rec := range rulesHistory() {
		if rec.rules != RulesVersion {
			continue
		}
		r := recordRange(t, rec)
		check := func(f fixture, regenerated []byte, parse func(testing.TB, []byte, Range) answers) {
			t.Helper()
			stored, err := os.ReadFile(f.path)
			if err == nil && bytes.Equal(stored, regenerated) {
				return
			}
			out := filepath.Join(t.ArtifactDir(), filepath.Base(f.path))
			if werr := os.WriteFile(out, regenerated, 0o600); werr != nil {
				t.Errorf("write artifact: %v", werr)
			}
			if err != nil {
				t.Errorf("%s: %v; regenerated file (SHA-256 %s) written to %s", f.path, err, sha256Hex(regenerated), out)
				return
			}
			diffs := firstDifferences(parse(t, stored, r), parse(t, regenerated, r), r.start, r.end, 10)
			t.Errorf("%s: the live rules no longer reproduce this %s fixture. First differing dates:\n  %s\nRegenerated file (SHA-256 %s) written to %s. A changed answer needs a new RulesVersion and a new rulesHistory record.",
				f.path, rec.rules, strings.Join(diffs, "\n  "), sha256Hex(regenerated), out)
		}
		check(rec.observations, observationsFixture(t, r), parseObservations)
		for i, f := range rec.workdays {
			if want := fmt.Sprintf("workdays-%s-%s.txt", strings.ReplaceAll(rec.rules, "/", "-"), comboName(i)); filepath.Base(f.path) != want && rec.start == "2000-01-01" {
				t.Errorf("workdays fixture %d is %s, want %s", i, f.path, want)
			}
			check(f, workdaysFixture(t, comboCalendar(t, i), r), parseWorkdays)
		}
	}
}

// A comparison is one pair of records that must agree on the overlap of
// their spans, before cutoff when cutoff is non-empty.
type comparison struct {
	older, newer rulesRecord
	cutoff       string
}

// appendOnlyComparisons plans the append-only checks: every pair of records
// of the same identifier (range extensions) must agree on their whole
// overlap, and every record of an identifier must agree with every record
// of the previous distinct identifier before the newer identifier's cutoff.
// Non-adjacent identifiers are not compared directly: a cutoff only governs
// differences from the previous identifier, and the adjacent checks chain.
func appendOnlyComparisons(history []rulesRecord) []comparison {
	var ids []string
	byID := map[string][]rulesRecord{}
	for _, rec := range history {
		if _, seen := byID[rec.rules]; !seen {
			ids = append(ids, rec.rules)
		}
		byID[rec.rules] = append(byID[rec.rules], rec)
	}
	var plan []comparison
	for k, id := range ids {
		recs := byID[id]
		for i, older := range recs {
			for _, newer := range recs[i+1:] {
				plan = append(plan, comparison{older: older, newer: newer})
			}
		}
		if k == 0 {
			continue
		}
		cutoff := recs[0].cutoff
		for _, older := range byID[ids[k-1]] {
			for _, newer := range recs {
				plan = append(plan, comparison{older: older, newer: newer, cutoff: cutoff})
			}
		}
	}
	return plan
}

// TestRulesHistory_AppendOnly runs the append-only plan over the retained
// fixtures.
func TestRulesHistory_AppendOnly(t *testing.T) {
	for _, c := range appendOnlyComparisons(rulesHistory()) {
		for _, diff := range recordDifferences(t, c) {
			t.Error(diff)
		}
	}
}

// recordDifferences compares the retained fixtures of c.older and c.newer
// on the overlap of their spans, before c.cutoff when it is set, and
// returns one message per differing fixture.
func recordDifferences(t testing.TB, c comparison) []string {
	t.Helper()
	ro, rn := recordRange(t, c.older), recordRange(t, c.newer)
	from, to := ro.start, ro.end
	if rn.start.After(from) {
		from = rn.start
	}
	if rn.end.Before(to) {
		to = rn.end
	}
	if c.cutoff != "" {
		if cut := mustDate(t, c.cutoff); cut.Before(to) {
			to = cut
		}
	}
	if !from.Before(to) {
		return nil
	}
	type pair struct {
		a, b  fixture
		parse func(testing.TB, []byte, Range) answers
	}
	pairs := []pair{{c.older.observations, c.newer.observations, parseObservations}}
	for i := range c.older.workdays {
		pairs = append(pairs, pair{c.older.workdays[i], c.newer.workdays[i], parseWorkdays})
	}
	var out []string
	for _, p := range pairs {
		ab, errA := os.ReadFile(p.a.path)
		bb, errB := os.ReadFile(p.b.path)
		if errA != nil || errB != nil {
			out = append(out, fmt.Sprintf("read fixtures: %v %v", errA, errB))
			continue
		}
		if diffs := firstDifferences(p.parse(t, ab, ro), p.parse(t, bb, rn), from, to, 10); len(diffs) > 0 {
			out = append(out, fmt.Sprintf("%s (%s) and %s (%s) differ before %s:\n  %s",
				p.a.path, c.older.rules, p.b.path, c.newer.rules, to, strings.Join(diffs, "\n  ")))
		}
	}
	return out
}

// syntheticRecord writes retained fixtures for a made-up rules record over
// [start, end): the live observations, and the live workday membership with
// the dates in off turned into non-workdays.
func syntheticRecord(t *testing.T, dir, rules, start, end, cutoff string, off ...Date) rulesRecord {
	t.Helper()
	rec := rulesRecord{rules: rules, start: start, end: end, cutoff: cutoff, reason: "synthetic"}
	r := recordRange(t, rec)
	name := fmt.Sprintf("%s-%s-%s", strings.ReplaceAll(rules, "/", "-"), start, end)
	write := func(suffix string, b []byte) fixture {
		path := filepath.Join(dir, name+"-"+suffix+".txt")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return fixture{path, sha256Hex(b)}
	}
	rec.observations = write("observations", observationsFixture(t, r))
	flips := map[Date]bool{}
	for _, d := range off {
		flips[d] = true
	}
	for i := range rec.workdays {
		rec.workdays[i] = write(comboName(i), workdaysFixtureWith(t, comboCalendar(t, i), r, flips))
	}
	return rec
}

func planDifferences(t *testing.T, history []rulesRecord) []string {
	t.Helper()
	var all []string
	for _, c := range appendOnlyComparisons(history) {
		all = append(all, recordDifferences(t, c)...)
	}
	return all
}

// TestRulesHistory_ThreeIdentifiers: v2 corrects 2026-11-02 (its cutoff);
// v3 keeps that correction and corrects 2030-11-04 (its cutoff). Both
// adjacent comparisons hold, so the history is valid; v1 and v3 are not
// compared directly. A v3 that changes a date before its own cutoff is
// rejected.
func TestRulesHistory_ThreeIdentifiers(t *testing.T) {
	dir := t.TempDir()
	first, second := mustDate(t, "2026-11-02"), mustDate(t, "2030-11-04")
	early := mustDate(t, "2028-06-01")
	v1 := syntheticRecord(t, dir, "no-public/v1", "2026-01-01", "2031-01-01", "")
	v2 := syntheticRecord(t, dir, "no-public/v2", "2026-01-01", "2031-01-01", first.String(), first)
	v3 := syntheticRecord(t, dir, "no-public/v3", "2026-01-01", "2031-01-01", second.String(), first, second)
	if diffs := planDifferences(t, []rulesRecord{v1, v2, v3}); len(diffs) != 0 {
		t.Fatalf("valid three-identifier history rejected:\n%s", strings.Join(diffs, "\n"))
	}
	for _, c := range appendOnlyComparisons([]rulesRecord{v1, v2, v3}) {
		if c.older.rules == "no-public/v1" && c.newer.rules == "no-public/v3" {
			t.Fatal("non-adjacent identifiers must not be compared directly")
		}
	}
	bad := syntheticRecord(t, dir, "no-public/v3-bad", "2026-01-01", "2031-01-01", second.String(), first, early, second)
	if diffs := planDifferences(t, []rulesRecord{v1, v2, bad}); len(diffs) == 0 {
		t.Fatal("a change before the newer identifier's cutoff was not detected")
	}
	reverted := syntheticRecord(t, dir, "no-public/v3-reverted", "2026-01-01", "2031-01-01", second.String(), second)
	if diffs := planDifferences(t, []rulesRecord{v1, v2, reverted}); len(diffs) == 0 {
		t.Fatal("silently reverting the previous identifier's correction was not detected")
	}
}

// TestRulesHistory_RangeExtension: records of the same identifier must agree
// on their whole overlap, with no cutoff.
func TestRulesHistory_RangeExtension(t *testing.T) {
	dir := t.TempDir()
	base := syntheticRecord(t, dir, "no-public/v1", "2026-01-01", "2029-01-01", "")
	ext := syntheticRecord(t, dir, "no-public/v1", "2028-01-01", "2031-01-01", "")
	if diffs := planDifferences(t, []rulesRecord{base, ext}); len(diffs) != 0 {
		t.Fatalf("valid range extension rejected:\n%s", strings.Join(diffs, "\n"))
	}
	changed := syntheticRecord(t, dir, "no-public/v1", "2028-02-01", "2031-01-01", "", mustDate(t, "2028-06-01"))
	if diffs := planDifferences(t, []rulesRecord{base, changed}); len(diffs) == 0 {
		t.Fatal("a range extension that changes an overlapping answer was not detected")
	}
	outside := syntheticRecord(t, dir, "no-public/v1", "2028-03-01", "2031-01-01", "", mustDate(t, "2030-06-03"))
	if diffs := planDifferences(t, []rulesRecord{base, outside}); len(diffs) != 0 {
		t.Fatalf("a difference outside the overlap must not count:\n%s", strings.Join(diffs, "\n"))
	}
}

// TestFirstDifferences checks the comparison that TestRulesHistory_AppendOnly
// relies on, with synthetic answers.
func TestFirstDifferences(t *testing.T) {
	a := mustDate(t, "2030-01-01")
	b, _ := a.AddDays(1)
	c, _ := a.AddDays(2)
	end, _ := a.AddDays(3)
	old := answers{a: "1", b: "0", c: "1"}
	cur := answers{a: "1", b: "0", c: "0"}
	if d := firstDifferences(old, cur, a, end, 10); len(d) != 1 || !strings.HasPrefix(d[0], "2030-01-03") {
		t.Fatalf("diffs = %v", d)
	}
	if d := firstDifferences(old, cur, a, c, 10); len(d) != 0 {
		t.Fatalf("a cutoff before the change must hide it: %v", d)
	}
}
