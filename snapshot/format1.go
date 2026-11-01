package snapshot

import (
	"fmt"
	"math/bits"
	"slices"
	"strings"

	"github.com/entur/go-calendar"
)

// v1Entry is one published slug of snapshot format 1.
type v1Entry struct {
	slug string
	// obs is the observance the slug names. Parse and encode use it only to
	// report Frozen.DaysOff; it never consults the live classification.
	obs calendar.Observance
	// option: the slug may appear in the daysoff line (a day off only when
	// listed). Entries that are not options are always days off in format 1.
	option bool
	// reason: the slug may appear as an excluded reason (a weekday
	// observance). Sondag never can.
	reason bool
}

// formatV1 is the format-1 registry: every published slug, whether it is a
// daysoff option, and, by position, the order of reasons in excluded rows.
// It is the format's contract and is written out literally, never derived
// from the live calendar's classification (calendar.Statutory,
// calendar.Customary, Observance.Kind), so a rules change cannot change how
// existing snapshot bytes parse or re-encode. It is only ever appended to;
// a different meaning for an existing slug needs a new format version.
func formatV1() [16]v1Entry {
	return [16]v1Entry{
		{slug: "sondag", obs: calendar.Sondag},
		{slug: "nyttarsdag", obs: calendar.Nyttarsdag, reason: true},
		{slug: "skjaertorsdag", obs: calendar.Skjaertorsdag, reason: true},
		{slug: "langfredag", obs: calendar.Langfredag, reason: true},
		{slug: "forste-paskedag", obs: calendar.ForstePaskedag, reason: true},
		{slug: "andre-paskedag", obs: calendar.AndrePaskedag, reason: true},
		{slug: "arbeidernes-dag", obs: calendar.ArbeidernesDag, reason: true},
		{slug: "grunnlovsdag", obs: calendar.Grunnlovsdag, reason: true},
		{slug: "kristi-himmelfartsdag", obs: calendar.KristiHimmelfartsdag, reason: true},
		{slug: "forste-pinsedag", obs: calendar.ForstePinsedag, reason: true},
		{slug: "andre-pinsedag", obs: calendar.AndrePinsedag, reason: true},
		{slug: "forste-juledag", obs: calendar.ForsteJuledag, reason: true},
		{slug: "andre-juledag", obs: calendar.AndreJuledag, reason: true},
		{slug: "onsdag-for-skjaertorsdag", obs: calendar.OnsdagForSkjaertorsdag, option: true, reason: true},
		{slug: "julaften", obs: calendar.Julaften, option: true, reason: true},
		{slug: "nyttarsaften", obs: calendar.Nyttarsaften, option: true, reason: true},
	}
}

// v1Set is a set of format-1 registry entries: bit i is formatV1()[i].
type v1Set uint32

func v1Bit(i int) v1Set { return 1 << i }

func (s v1Set) has(i int) bool { return s&v1Bit(i) != 0 }

// v1BySlug returns the registry index of a published slug.
func v1BySlug(slug string) (int, bool) {
	for i, e := range formatV1() {
		if e.slug == slug {
			return i, true
		}
	}
	return 0, false
}

// v1ByObservance returns the registry index that names o.
func v1ByObservance(o calendar.Observance) (int, bool) {
	for i, e := range formatV1() {
		if e.obs == o {
			return i, true
		}
	}
	return 0, false
}

// daysOffLine encodes the options in s for the daysoff line: "none", or the
// slugs in lexicographic order, comma-joined.
func (s v1Set) daysOffLine() string {
	var slugs []string
	for i, e := range formatV1() {
		if e.option && s.has(i) {
			slugs = append(slugs, e.slug)
		}
	}
	if len(slugs) == 0 {
		return "none"
	}
	slices.Sort(slugs)
	return strings.Join(slugs, ",")
}

// reasonsLine encodes the entries in s in registry order, comma-joined.
func (s v1Set) reasonsLine() string {
	var slugs []string
	for i, e := range formatV1() {
		if s.has(i) {
			slugs = append(slugs, e.slug)
		}
	}
	return strings.Join(slugs, ",")
}

// v1DaysOff returns the observances that are days off under format 1 with
// the given daysoff options: every entry that is not an option, plus the
// listed options.
func v1DaysOff(options v1Set) calendar.ObservanceSet {
	var set calendar.ObservanceSet
	for i, e := range formatV1() {
		if !e.option || options.has(i) {
			set |= calendar.ObservanceSet(1) << e.obs
		}
	}
	return set
}

// liveModel is the live calendar's classification as Generate sees it. It is
// the only place the snapshot package consults it; tests construct other
// models to simulate a changed classification.
type liveModel struct {
	known   calendar.ObservanceSet // every observance the live rules define
	daysOff calendar.ObservanceSet // the observances the calendar treats as days off
}

func liveModelOf(c calendar.Calendar) liveModel {
	return liveModel{known: calendar.Statutory() | calendar.Customary(), daysOff: c.DaysOff()}
}

// v1Options maps the live model onto format 1 and returns the daysoff
// options. It fails when format 1 cannot represent the live calendar: an
// observance the registry does not know, or a format-1 day off that the live
// calendar does not treat as one. Either forces a deliberate registry or
// format decision instead of silently changing what the bytes mean.
func (m liveModel) v1Options() (v1Set, error) {
	// Walk every set bit, not ObservanceSet.All, which only yields the
	// observances this package version knows: the point is to catch new ones.
	for rest := uint32(m.known | m.daysOff); rest != 0; rest &= rest - 1 {
		o := calendar.Observance(bits.TrailingZeros32(rest)) //nolint:gosec // G115: a bit index of a uint32 is 0..31
		if _, ok := v1ByObservance(o); !ok {
			return 0, fmt.Errorf("observance %d is not in snapshot format 1: %w", o, ErrInvalidSnapshot)
		}
	}
	var options v1Set
	for i, e := range formatV1() {
		off := m.daysOff.Has(e.obs)
		switch {
		case e.option && off:
			options |= v1Bit(i)
		case !e.option && !off:
			return 0, fmt.Errorf("snapshot format 1 always treats %q as a day off, the calendar does not: %w", e.slug, ErrInvalidSnapshot)
		default:
		}
	}
	return options, nil
}

// v1Reasons maps the day-off observances on a weekday to format-1 reasons.
func (m liveModel) v1Reasons(observed calendar.ObservanceSet) (v1Set, error) {
	var reasons v1Set
	for o := range (observed & m.daysOff).All() {
		i, ok := v1ByObservance(o)
		if !ok || !formatV1()[i].reason {
			return 0, fmt.Errorf("observance %s cannot be an excluded reason in snapshot format 1: %w", o, ErrInvalidSnapshot)
		}
		reasons |= v1Bit(i)
	}
	return reasons, nil
}
