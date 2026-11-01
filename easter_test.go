package calendar

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// External fixtures: plain data produced by scripts/fetch_*.go, never
// typed by hand, never fetched by tests. Provenance: testdata/SOURCES.md.
const (
	usnoFixture   = "testdata/usno/observances-2000-2099.txt"
	usnoSHA256    = "53e84ef0b59d01f5b27f3a6165b0ab905c34e160b24afc9c40ed1f6cb25a4574"
	censusFixture = "testdata/census/easter-2000-2099.txt"
	censusSHA256  = "eabcd9510c562ca152c2a2d5e531c75248c0be0eb8de4d2347b5e75c8d002d1c"
)

// easterGauss is Gauss's Easter algorithm with Lichtenberg's 1997
// correction, structurally independent of the production algorithm. It
// returns the day as a March date (32 = 1 April).
func easterGauss(year int) int {
	k := year / 100
	m := 15 + (3*k+3)/4 - (8*k+13)/25
	s := 2 - (3*k+3)/4
	a := year % 19
	d := (19*a + m) % 30
	r := d/29 + (d/28-d/29)*(a/11)
	og := 21 + d - r // Easter full moon, March date
	sz := 7 - (year+year/4+s)%7
	oe := 7 - (og-sz)%7
	return og + oe
}

// easterEpact uses the Gregorian epact and the Clavius/Lilius table of
// paschal full moons (as formulated by Knuth, TAOCP vol. 1, 1.3.2 ex. 14).
// It returns the day as a March date (32 = 1 April).
func easterEpact(year int) int {
	golden := year%19 + 1
	century := year/100 + 1
	solar := 3*century/4 - 12       // Gregorian leap days dropped since 1582
	lunar := (8*century+5)/25 - 5   // Metonic correction
	sunday := 5*year/4 - solar - 10 // March ((-sunday) mod 7) is a Sunday
	epact := (11*golden + 20 + lunar - solar) % 30
	if epact < 0 {
		epact += 30
	}
	if epact == 25 && golden > 11 || epact == 24 {
		epact++
	}
	// Clavius's table: the paschal full moon for each epact, as a March date.
	var pfm [31]int
	for e := range pfm {
		pfm[e] = 44 - e
		if pfm[e] < 21 {
			pfm[e] += 30
		}
	}
	n := pfm[epact]
	return n + 7 - (sunday+n)%7
}

func marchDay(month time.Month, day int) int {
	if month == time.April {
		return day + 31
	}
	return day
}

func TestEaster_Oracles(t *testing.T) {
	for year := 1583; year <= 9999; year++ {
		month, day := easterMonthDay(year)
		got := marchDay(month, day)
		if g := easterGauss(year); g != got {
			t.Fatalf("%d: production %d, Gauss/Lichtenberg %d (March days)", year, got, g)
		}
		if e := easterEpact(year); e != got {
			t.Fatalf("%d: production %d, epact table %d (March days)", year, got, e)
		}
	}
}

func TestEaster_Period5700000(t *testing.T) {
	for _, year := range []int{1583, 1600, 1700, 1999, 2000, 2019, 2027, 2099, 4321, 9999} {
		m1, d1 := easterMonthDay(year)
		m2, d2 := easterMonthDay(year + 5_700_000)
		if m1 != m2 || d1 != d2 {
			t.Errorf("%d: %v %d, but %d: %v %d", year, m1, d1, year+5_700_000, m2, d2)
		}
	}
}

func TestEaster_Bounds(t *testing.T) {
	for year := 1583; year <= 9999; year++ {
		month, day := easterMonthDay(year)
		md := marchDay(month, day)
		if md < 22 || md > 56 {
			t.Fatalf("%d: Easter on March day %d, outside 22 March–25 April", year, md)
		}
		if wd := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Weekday(); wd != time.Sunday {
			t.Fatalf("%d: Easter is a %v", year, wd)
		}
	}
	// The extremes inside the supported range.
	for s, want := range map[int]string{2008: "2008-03-23", 2038: "2038-04-25", 2019: "2019-04-21", 2027: "2027-03-28"} {
		if e, err := Easter(s); err != nil || e.String() != want {
			t.Errorf("Easter(%d) = %v, %v; want %s", s, e, err, want)
		}
	}
}

func TestEaster_OutOfRange(t *testing.T) {
	for _, year := range []int{MinYear - 1, MaxYear + 1, 0, -1} {
		if d, err := Easter(year); !errors.Is(err, ErrOutOfRange) || !d.IsZero() {
			t.Errorf("Easter(%d) = %v, %v; want ErrOutOfRange", year, d, err)
		}
	}
}

// TestStatute2019_ComputusNotAstronomical pins that første påskedag follows
// the ecclesiastical computus. The astronomical full moon fell on
// 2019-03-21 01:42 UTC (02:42 in Norway), which read literally against
// helligdagsfredloven § 2 e gives Sunday 24 March. Norway observed
// 21 April 2019.
func TestStatute2019_ComputusNotAstronomical(t *testing.T) {
	e, err := Easter(2019)
	if err != nil || e.String() != "2019-04-21" {
		t.Fatalf("Easter(2019) = %v, %v; want 2019-04-21", e, err)
	}
	var c Calendar
	if ok, _ := c.IsWorkday(mustDate(t, "2019-03-25")); !ok {
		t.Fatal("Monday 2019-03-25 (the astronomical reading's annen påskedag) must be a workday")
	}
	if ok, _ := c.IsWorkday(mustDate(t, "2019-04-22")); ok {
		t.Fatal("2019-04-22 (annen påskedag) must not be a workday")
	}
}

func readFixtureLines(t *testing.T, path, wantSHA string) []string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: path is a constant fixture path
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != wantSHA {
		t.Fatalf("%s: SHA-256 %s, want %s (fixtures are never edited; regenerate with the script and update testdata/SOURCES.md)", path, got, wantSHA)
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) != MaxYear-MinYear+1 {
		t.Fatalf("%s: %d lines, want %d", path, len(lines), MaxYear-MinYear+1)
	}
	return lines
}

func TestEaster_USNO(t *testing.T) {
	lines := readFixtureLines(t, usnoFixture, usnoSHA256)
	for i, line := range lines {
		f := strings.Fields(line)
		if len(f) != 5 {
			t.Fatalf("line %d: %q", i+1, line)
		}
		year, err := strconv.Atoi(f[0])
		if err != nil || year != MinYear+i {
			t.Fatalf("line %d: year %q", i+1, f[0])
		}
		for j, o := range []Observance{Langfredag, ForstePaskedag, KristiHimmelfartsdag, ForstePinsedag} {
			got, err := o.DateIn(year)
			if err != nil || got.String() != f[j+1] {
				t.Errorf("%s %d = %v, %v; USNO says %s", o, year, got, err, f[j+1])
			}
		}
	}
}

func TestEaster_Census(t *testing.T) {
	lines := readFixtureLines(t, censusFixture, censusSHA256)
	for i, want := range lines {
		year := MinYear + i
		got, err := Easter(year)
		if err != nil || got.String() != want {
			t.Errorf("Easter(%d) = %v, %v; Census says %s", year, got, err, want)
		}
	}
}
