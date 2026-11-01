package calendar

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func allObservanceValues() []Observance {
	var all []Observance
	for o := Sondag; o <= lastObservance; o++ {
		all = append(all, o)
	}
	return all
}

func TestObservance_Metadata(t *testing.T) {
	slugs := map[string]Observance{}
	for _, o := range allObservanceValues() {
		slug := o.Slug()
		if slug == "" || o.String() == "" || o.Basis() == "" || o.Kind() == 0 {
			t.Errorf("%d: incomplete metadata", o)
		}
		for _, c := range slug {
			if (c < 'a' || c > 'z') && c != '-' {
				t.Errorf("%s: slug %q is not lower-case ASCII", o, slug)
			}
		}
		if prev, dup := slugs[slug]; dup {
			t.Errorf("slug %q used by %d and %d", slug, prev, o)
		}
		slugs[slug] = o
	}
	// Slugs are part of the snapshot format and must never change.
	want := []string{
		"sondag", "nyttarsdag", "skjaertorsdag", "langfredag", "forste-paskedag", "andre-paskedag",
		"arbeidernes-dag", "grunnlovsdag", "kristi-himmelfartsdag", "forste-pinsedag", "andre-pinsedag",
		"forste-juledag", "andre-juledag", "onsdag-for-skjaertorsdag", "julaften", "nyttarsaften",
	}
	var got []string
	for _, o := range allObservanceValues() {
		got = append(got, o.Slug())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("slugs changed:\n got %v\nwant %v", got, want)
	}
	for _, unknown := range []Observance{0, lastObservance + 1, 255} {
		if unknown.Slug() != "" || unknown.Kind() != 0 || unknown.Basis() != "" || unknown.String() == "" {
			t.Errorf("unknown %d: unexpected metadata", unknown)
		}
	}
	if KindHelligdag.String() != "helligdag" || KindHoytidsdag.String() != "høytidsdag" ||
		KindCustomary.String() != "customary" || Kind(9).String() != "Kind(9)" {
		t.Error("Kind.String")
	}
}

func TestObservanceSet(t *testing.T) {
	if Statutory().Len() != 13 || Customary().Len() != 3 || Statutory()&Customary() != 0 {
		t.Fatalf("Statutory %d, Customary %d", Statutory().Len(), Customary().Len())
	}
	for o := range Statutory().All() {
		if o.Kind() == KindCustomary {
			t.Errorf("%s in Statutory", o)
		}
	}
	for o := range Customary().All() {
		if o.Kind() != KindCustomary {
			t.Errorf("%s in Customary", o)
		}
	}
	s := ObservanceSet(1<<AndrePinsedag | 1<<Grunnlovsdag | 1<<31 | 1)
	if s.Len() != 2 || !s.Has(Grunnlovsdag) || s.Has(Sondag) || s.Has(0) || s.Has(40) {
		t.Fatalf("Has/Len on %b", s)
	}
	if got := slices.Collect(s.All()); !slices.Equal(got, []Observance{Grunnlovsdag, AndrePinsedag}) {
		t.Fatalf("All = %v", got)
	}
	if s.String() != "grunnlovsdag,andre-pinsedag" || ObservanceSet(0).String() != "" {
		t.Fatalf("String = %q", s.String())
	}
	n := 0
	for range Statutory().All() {
		n++
		break
	}
	if n != 1 {
		t.Fatal("All must stop on break")
	}
}

func TestObservance_DateInErrors(t *testing.T) {
	if _, err := Sondag.DateIn(2026); !errors.Is(err, ErrInvalidObservance) {
		t.Errorf("Sondag: %v", err)
	}
	if _, err := Observance(0).DateIn(2026); !errors.Is(err, ErrInvalidObservance) {
		t.Errorf("0: %v", err)
	}
	if _, err := Observance(99).DateIn(2026); !errors.Is(err, ErrInvalidObservance) {
		t.Errorf("99: %v", err)
	}
	if _, err := Julaften.DateIn(1999); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("1999: %v", err)
	}
	if _, err := Julaften.DateIn(2100); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("2100: %v", err)
	}
}

// stepTo walks from start in direction step (±1 day) and returns the n-th
// date (n >= 1) whose weekday is wd, using the time package only.
func stepTo(start time.Time, step int, wd time.Weekday, n int) time.Time {
	t := start
	for {
		t = t.AddDate(0, 0, step)
		if t.Weekday() == wd {
			n--
			if n == 0 {
				return t
			}
		}
	}
}

// TestStatuteWording implements each parenthesis of helligdagsfredloven § 2
// literally by stepping weekdays, independently of the Easter offsets, and
// compares with DateIn for every supported year.
func TestStatuteWording(t *testing.T) {
	for year := MinYear; year <= MaxYear; year++ {
		e, err := Easter(year)
		if err != nil {
			t.Fatal(err)
		}
		y, m, d := e.Date()
		easter := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		pinse := stepTo(easter, 1, time.Sunday, 7) // "sjuende søndag etter første påskedag"
		literal := map[Observance]time.Time{
			Skjaertorsdag:          stepTo(easter, -1, time.Thursday, 1), // "siste torsdag før første påskedag"
			Langfredag:             stepTo(easter, -1, time.Friday, 1),   // "siste fredag før første påskedag"
			AndrePaskedag:          stepTo(easter, 1, time.Monday, 1),    // "første mandag etter første påskedag"
			KristiHimmelfartsdag:   stepTo(easter, 1, time.Thursday, 6),  // "sjette torsdag etter første påskedag"
			ForstePinsedag:         pinse,
			AndrePinsedag:          stepTo(pinse, 1, time.Monday, 1), // "første mandag etter første pinsedag"
			OnsdagForSkjaertorsdag: stepTo(stepTo(easter, -1, time.Thursday, 1), -1, time.Wednesday, 1),
			Nyttarsdag:             time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC),
			ArbeidernesDag:         time.Date(year, time.May, 1, 0, 0, 0, 0, time.UTC),
			Grunnlovsdag:           time.Date(year, time.May, 17, 0, 0, 0, 0, time.UTC),
			Julaften:               time.Date(year, time.December, 24, 0, 0, 0, 0, time.UTC),
			ForsteJuledag:          time.Date(year, time.December, 25, 0, 0, 0, 0, time.UTC),
			AndreJuledag:           time.Date(year, time.December, 26, 0, 0, 0, 0, time.UTC),
			Nyttarsaften:           time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC),
			ForstePaskedag:         easter,
		}
		if len(literal) != int(lastObservance)-1 {
			t.Fatalf("literal rules cover %d observances", len(literal))
		}
		for o, want := range literal {
			got, err := o.DateIn(year)
			if err != nil || got.String() != want.Format(time.DateOnly) {
				t.Fatalf("%s %d: DateIn = %v, %v; statute wording gives %s", o, year, got, err, want.Format(time.DateOnly))
			}
		}
	}
}
