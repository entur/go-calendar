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
		if ok {
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

// TestRulesHistory_AppendOnly compares the retained fixtures of every pair
// of records on their overlap: the same identifier must agree everywhere; a
// newer identifier must agree before its cutoff.
func TestRulesHistory_AppendOnly(t *testing.T) {
	history := rulesHistory()
	for i, older := range history {
		for _, newer := range history[i+1:] {
			if older.rules == newer.rules {
				compareRecords(t, older, newer, "")
				continue
			}
			compareRecords(t, older, newer, firstCutoff(history, newer.rules))
		}
	}
}

func firstCutoff(history []rulesRecord, rules string) string {
	for _, rec := range history {
		if rec.rules == rules {
			return rec.cutoff
		}
	}
	return ""
}

// compareRecords checks that two records agree on the overlap of their
// spans, up to cutoff when cutoff is non-empty.
func compareRecords(t *testing.T, older, newer rulesRecord, cutoff string) {
	t.Helper()
	ro, rn := recordRange(t, older), recordRange(t, newer)
	from, to := ro.start, ro.end
	if rn.start.After(from) {
		from = rn.start
	}
	if rn.end.Before(to) {
		to = rn.end
	}
	if cutoff != "" {
		if c := mustDate(t, cutoff); c.Before(to) {
			to = c
		}
	}
	if !from.Before(to) {
		return
	}
	pairs := []struct {
		a, b  fixture
		parse func(testing.TB, []byte, Range) answers
	}{{older.observations, newer.observations, parseObservations}}
	for i := range older.workdays {
		pairs = append(pairs, struct {
			a, b  fixture
			parse func(testing.TB, []byte, Range) answers
		}{older.workdays[i], newer.workdays[i], parseWorkdays})
	}
	for _, p := range pairs {
		ab, errA := os.ReadFile(p.a.path)
		bb, errB := os.ReadFile(p.b.path)
		if errA != nil || errB != nil {
			t.Errorf("read fixtures: %v %v", errA, errB)
			continue
		}
		if diffs := firstDifferences(p.parse(t, ab, ro), p.parse(t, bb, rn), from, to, 10); len(diffs) > 0 {
			t.Errorf("%s (%s) and %s (%s) differ before %s:\n  %s", p.a.path, older.rules, p.b.path, newer.rules, to, strings.Join(diffs, "\n  "))
		}
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
