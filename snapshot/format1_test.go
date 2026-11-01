package snapshot

import (
	"bytes"
	"errors"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/entur/go-calendar"
)

// TestFormat1_LiveClassificationChange simulates a later rules version in
// which julaften becomes a statutory day off, so the live classification
// changes. Previously published bytes must still parse, re-encode
// identically and report the same metadata; and freezing the same calendar
// under the new classification must reproduce the same bytes.
func TestFormat1_LiveClassificationChange(t *testing.T) {
	old := []byte(sample)
	before, err := Parse(old)
	if err != nil {
		t.Fatal(err)
	}

	changed := liveModel{
		known:   calendar.Statutory() | calendar.Customary(),
		daysOff: calendar.Calendar{}.DaysOff() | calendar.ObservanceSet(1)<<calendar.Julaften,
	}
	closures := []calendar.Date{date(t, "2026-12-28"), date(t, "2027-01-02")}
	regenerated, err := generate(calendar.Calendar{}, changed, span(t, "2026-12-21", "2027-01-04"), UTCDateLabels, closures)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(regenerated, old) {
		t.Fatalf("under the changed classification the same calendar freezes differently:\n%s", regenerated)
	}

	after, err := Parse(old)
	if err != nil {
		t.Fatalf("published bytes no longer parse: %v", err)
	}
	if !bytes.Equal(after.encode(), old) {
		t.Fatal("published bytes no longer re-encode identically")
	}
	if after.Rules() != before.Rules() || after.Basis() != before.Basis() || after.Span() != before.Span() ||
		after.DaysOff() != before.DaysOff() || !slices.Equal(after.Closed(), before.Closed()) {
		t.Fatal("published bytes report different metadata")
	}
	if !after.DaysOff().Has(calendar.Julaften) || after.Rules() != "no-public/v1" {
		t.Fatalf("metadata: rules %q, daysoff %q", after.Rules(), after.DaysOff())
	}
}

// TestFormat1_GenerateRejectsUnrepresentable: a live calendar that format 1
// cannot express fails instead of silently changing what the bytes mean.
func TestFormat1_GenerateRejectsUnrepresentable(t *testing.T) {
	r := span(t, "2026-12-21", "2027-01-04")
	base := liveModelOf(calendar.Calendar{})
	cases := map[string]liveModel{
		"unknown observance": {known: base.known | 1<<17, daysOff: base.daysOff},
		"unknown day off":    {known: base.known, daysOff: base.daysOff | 1<<17},
		"format-1 day off no longer off": {
			known: base.known, daysOff: base.daysOff &^ (calendar.ObservanceSet(1) << calendar.Nyttarsdag),
		},
	}
	for name, m := range cases {
		if _, err := generate(calendar.Calendar{}, m, r, UTCDateLabels, nil); !errors.Is(err, ErrInvalidSnapshot) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := base.v1Reasons(calendar.ObservanceSet(1) << calendar.Sondag); !errors.Is(err, ErrInvalidSnapshot) {
		t.Errorf("sondag as a reason: %v", err)
	}
}

// TestFormat1_RegistryMatchesPublishedSlugs: the registry's literal slugs
// and order are those format 1 was published with.
func TestFormat1_RegistryMatchesPublishedSlugs(t *testing.T) {
	want := []string{
		"sondag", "nyttarsdag", "skjaertorsdag", "langfredag", "forste-paskedag", "andre-paskedag",
		"arbeidernes-dag", "grunnlovsdag", "kristi-himmelfartsdag", "forste-pinsedag", "andre-pinsedag",
		"forste-juledag", "andre-juledag", "onsdag-for-skjaertorsdag", "julaften", "nyttarsaften",
	}
	var got, options []string
	for _, e := range formatV1() {
		got = append(got, e.slug)
		if e.obs.Slug() != e.slug {
			t.Errorf("registry slug %q names %s, whose slug is %q", e.slug, e.obs, e.obs.Slug())
		}
		if e.option {
			options = append(options, e.slug)
		}
		if e.reason == (e.slug == "sondag") {
			t.Errorf("%q: reason eligibility", e.slug)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("registry slugs changed:\n got %v\nwant %v", got, want)
	}
	if !slices.Equal(options, []string{"onsdag-for-skjaertorsdag", "julaften", "nyttarsaften"}) {
		t.Fatalf("daysoff options changed: %v", options)
	}
}

// TestFormat1_IndependentOfLiveClassification guards the source: only
// liveModelOf may consult the live calendar's classification. Parse,
// encode and Frozen must use the format-1 registry alone.
func TestFormat1_IndependentOfLiveClassification(t *testing.T) {
	forbidden := map[string]bool{
		"Statutory": true, "Customary": true, "KindHelligdag": true, "KindHoytidsdag": true, "KindCustomary": true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := goparser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			checked++
			ast.Inspect(fn, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, isPkg := sel.X.(*ast.Ident)
				live := isPkg && pkg.Name == "calendar" && forbidden[sel.Sel.Name] ||
					sel.Sel.Name == "Kind" || sel.Sel.Name == "Slug"
				if live && fn.Name.Name != "liveModelOf" {
					t.Errorf("%s: %s uses the live classification (%s); use the format-1 registry",
						fset.Position(sel.Pos()), fn.Name.Name, sel.Sel.Name)
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("no functions checked")
	}
}
