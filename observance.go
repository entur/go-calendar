package calendar

import (
	"fmt"
	"iter"
	"math/bits"
	"strconv"
	"strings"
	"time"
)

// Kind is the legal category of an Observance.
type Kind uint8

// Kinds of observance.
const (
	// KindHelligdag is a helligdag under helligdagsfredloven § 2 (all Sundays
	// and the listed holy days).
	KindHelligdag Kind = iota + 1

	// KindHoytidsdag is a høytidsdag under lov om 1. og 17. mai som
	// høgtidsdager § 1: equal to a Sunday except for church rules and
	// helligdagsfred.
	KindHoytidsdag

	// KindCustomary is a day that is commonly, but not by law, a day off. It
	// is never a day off unless a Calendar opts in with WithDaysOff.
	KindCustomary
)

// String returns the Norwegian name of the kind ("helligdag",
// "høytidsdag") or "customary".
func (k Kind) String() string {
	switch k {
	case KindHelligdag:
		return "helligdag"
	case KindHoytidsdag:
		return "høytidsdag"
	case KindCustomary:
		return "customary"
	default:
		return "Kind(" + strconv.Itoa(int(k)) + ")"
	}
}

// Observance is a named Norwegian holiday or customary day. The values
// start at 1; the zero value is not an observance.
type Observance uint8

// The observances. Their order is the order of ObservanceSet.All and of
// slugs in snapshots. E is første påskedag by the Gregorian ecclesiastical
// computus (see Easter).
const (
	// Sondag is an ordinary Sunday ("vanlige søndager", helligdagsfredloven
	// § 2 a): every Sunday that carries no § 2 b–k observance.
	Sondag Observance = iota + 1
	// Nyttarsdag is nyttårsdag, 1 January (§ 2 b).
	Nyttarsdag
	// Skjaertorsdag is skjærtorsdag, E−3 (§ 2 c).
	Skjaertorsdag
	// Langfredag is langfredag, E−2 (§ 2 d).
	Langfredag
	// ForstePaskedag is første påskedag, E (§ 2 e).
	ForstePaskedag
	// AndrePaskedag is annen påskedag, E+1 (§ 2 f).
	AndrePaskedag
	// ArbeidernesDag is 1 May (lov om 1. og 17. mai § 1).
	ArbeidernesDag
	// Grunnlovsdag is 17 May (lov om 1. og 17. mai § 1).
	Grunnlovsdag
	// KristiHimmelfartsdag is Kristi himmelfartsdag, E+39 (§ 2 g).
	KristiHimmelfartsdag
	// ForstePinsedag is første pinsedag, E+49 (§ 2 h).
	ForstePinsedag
	// AndrePinsedag is annen pinsedag, E+50 (§ 2 i).
	AndrePinsedag
	// ForsteJuledag is første juledag, 25 December (§ 2 j).
	ForsteJuledag
	// AndreJuledag is annen juledag, 26 December (§ 2 k).
	AndreJuledag
	// OnsdagForSkjaertorsdag is the Wednesday before skjærtorsdag, E−4.
	// Customary: collective agreements only.
	OnsdagForSkjaertorsdag
	// Julaften is 24 December. Customary: not a statutory day off
	// (arbeidsmiljøloven § 10-10 only gives time off from 15:00).
	Julaften
	// Nyttarsaften is 31 December. Customary: collective agreements only.
	Nyttarsaften

	lastObservance = Nyttarsaften
)

type observanceInfo struct {
	name, slug, basis string
	kind              Kind
	// Exactly one of the following describes the date: a fixed month and
	// day, an offset from første påskedag, or neither (Sondag).
	month      time.Month
	day        int
	fromEaster bool
	offset     int32
}

func (o Observance) info() (observanceInfo, bool) {
	const hfl = "helligdagsfredloven § 2 bokstav "
	const may = "lov om 1. og 17. mai som høgtidsdager § 1"
	switch o {
	case Sondag:
		return observanceInfo{name: "søndag", slug: "sondag", basis: hfl + "a", kind: KindHelligdag}, true
	case Nyttarsdag:
		return observanceInfo{name: "nyttårsdag", slug: "nyttarsdag", basis: hfl + "b", kind: KindHelligdag, month: time.January, day: 1}, true
	case Skjaertorsdag:
		return observanceInfo{name: "skjærtorsdag", slug: "skjaertorsdag", basis: hfl + "c", kind: KindHelligdag, fromEaster: true, offset: -3}, true
	case Langfredag:
		return observanceInfo{name: "langfredag", slug: "langfredag", basis: hfl + "d", kind: KindHelligdag, fromEaster: true, offset: -2}, true
	case ForstePaskedag:
		return observanceInfo{name: "første påskedag", slug: "forste-paskedag", basis: hfl + "e", kind: KindHelligdag, fromEaster: true, offset: 0}, true
	case AndrePaskedag:
		return observanceInfo{name: "annen påskedag", slug: "andre-paskedag", basis: hfl + "f", kind: KindHelligdag, fromEaster: true, offset: 1}, true
	case ArbeidernesDag:
		return observanceInfo{name: "1. mai", slug: "arbeidernes-dag", basis: may, kind: KindHoytidsdag, month: time.May, day: 1}, true
	case Grunnlovsdag:
		return observanceInfo{name: "17. mai", slug: "grunnlovsdag", basis: may, kind: KindHoytidsdag, month: time.May, day: 17}, true
	case KristiHimmelfartsdag:
		return observanceInfo{name: "Kristi himmelfartsdag", slug: "kristi-himmelfartsdag", basis: hfl + "g", kind: KindHelligdag, fromEaster: true, offset: 39}, true
	case ForstePinsedag:
		return observanceInfo{name: "første pinsedag", slug: "forste-pinsedag", basis: hfl + "h", kind: KindHelligdag, fromEaster: true, offset: 49}, true
	case AndrePinsedag:
		return observanceInfo{name: "annen pinsedag", slug: "andre-pinsedag", basis: hfl + "i", kind: KindHelligdag, fromEaster: true, offset: 50}, true
	case ForsteJuledag:
		return observanceInfo{name: "første juledag", slug: "forste-juledag", basis: hfl + "j", kind: KindHelligdag, month: time.December, day: 25}, true
	case AndreJuledag:
		return observanceInfo{name: "annen juledag", slug: "andre-juledag", basis: hfl + "k", kind: KindHelligdag, month: time.December, day: 26}, true
	case OnsdagForSkjaertorsdag:
		return observanceInfo{name: "onsdag før skjærtorsdag", slug: "onsdag-for-skjaertorsdag", basis: "ingen lovbestemt fridag (tariffavtaler)", kind: KindCustomary, fromEaster: true, offset: -4}, true
	case Julaften:
		return observanceInfo{name: "julaften", slug: "julaften", basis: "ingen lovbestemt fridag (arbeidsmiljøloven § 10-10 (1): arbeidsfri fra kl. 15)", kind: KindCustomary, month: time.December, day: 24}, true
	case Nyttarsaften:
		return observanceInfo{name: "nyttårsaften", slug: "nyttarsaften", basis: "ingen lovbestemt fridag (tariffavtaler)", kind: KindCustomary, month: time.December, day: 31}, true
	default:
		return observanceInfo{}, false
	}
}

// String returns the Norwegian name as written in the statute, for example
// "første påskedag" or "annen pinsedag". Unknown values return
// "Observance(n)".
func (o Observance) String() string {
	if in, ok := o.info(); ok {
		return in.name
	}
	return "Observance(" + strconv.Itoa(int(o)) + ")"
}

// Slug returns a stable ASCII identifier such as "forste-paskedag". Slugs
// are used in snapshots and goldens and are never renamed. Unknown values
// return "".
func (o Observance) Slug() string {
	in, _ := o.info()
	return in.slug
}

// Kind returns the legal category of o, or 0 for an unknown value.
func (o Observance) Kind() Kind {
	in, _ := o.info()
	return in.kind
}

// Basis returns the legal basis of o, in Norwegian, for example
// "helligdagsfredloven § 2 bokstav e". Unknown values return "".
func (o Observance) Basis() string {
	in, _ := o.info()
	return in.basis
}

// DateIn returns the date of o in year. Sondag has no single date and
// returns ErrInvalidObservance, as do unknown values. Years outside
// MinYear..MaxYear return ErrOutOfRange.
func (o Observance) DateIn(year int) (Date, error) {
	in, ok := o.info()
	if !ok || o == Sondag {
		return Date{}, fmt.Errorf("DateIn: %s: %w", o, ErrInvalidObservance)
	}
	if err := checkYear(year); err != nil {
		return Date{}, err
	}
	if in.fromEaster {
		return Date{rd: easterRD(year) + in.offset}, nil
	}
	return Date{rd: rdFromCivil(year, in.month, in.day)}, nil
}

// ObservanceSet is a set of observances. It is a comparable value; the
// zero value is the empty set.
type ObservanceSet uint32

const (
	allObservances ObservanceSet = (1<<(lastObservance+1) - 1) &^ 1

	statutorySet = 1<<Sondag | 1<<Nyttarsdag | 1<<Skjaertorsdag | 1<<Langfredag |
		1<<ForstePaskedag | 1<<AndrePaskedag | 1<<ArbeidernesDag | 1<<Grunnlovsdag |
		1<<KristiHimmelfartsdag | 1<<ForstePinsedag | 1<<AndrePinsedag |
		1<<ForsteJuledag | 1<<AndreJuledag

	customarySet = 1<<OnsdagForSkjaertorsdag | 1<<Julaften | 1<<Nyttarsaften

	// namedHelligdager are the § 2 b–k observances: a Sunday carrying one
	// of them is not an ordinary Sunday.
	namedHelligdager = statutorySet &^ (1<<Sondag | 1<<ArbeidernesDag | 1<<Grunnlovsdag)
)

// Statutory returns the 13 statutory observances: Sondag, the ten other
// helligdager and the two høytidsdager. They are always days off.
func Statutory() ObservanceSet { return statutorySet }

// Customary returns the 3 customary observances: OnsdagForSkjaertorsdag,
// Julaften and Nyttarsaften. They are days off only when opted in.
func Customary() ObservanceSet { return customarySet }

func bit(o Observance) ObservanceSet {
	if o == 0 || o > lastObservance {
		return 0
	}
	return 1 << o
}

// Has reports whether o is in s.
func (s ObservanceSet) Has(o Observance) bool {
	b := bit(o)
	return b != 0 && s&b != 0
}

// All returns an iterator over the observances in s, in Observance order.
// Bits that are not observances are ignored.
func (s ObservanceSet) All() iter.Seq[Observance] {
	return func(yield func(Observance) bool) {
		for o := Sondag; o <= lastObservance; o++ {
			if s.Has(o) && !yield(o) {
				return
			}
		}
	}
}

// Len returns the number of observances in s.
func (s ObservanceSet) Len() int {
	return bits.OnesCount32(uint32(s & allObservances))
}

// String returns the slugs of s in Observance order, comma-joined, for
// example "grunnlovsdag,andre-pinsedag". The empty set returns "".
func (s ObservanceSet) String() string {
	var b strings.Builder
	for o := range s.All() {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(o.Slug())
	}
	return b.String()
}
