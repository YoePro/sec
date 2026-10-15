package sema

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Storage overlap requires canonical provenance, not equal source spellings,
// variant names, or the possibility that two unknown aliases coincide.
// Rules: rules/analysis/data_races.md — "Storage overlap", "Union payloads".
func TestStorageOverlap(t *testing.T) {
	root := Place{Root: "value", RootID: 1, RootKind: PlaceRootLocal}
	project := func(projections ...PlaceProjection) Place {
		p := root
		p.Projections = projections
		return p
	}
	other := root
	other.RootID = 2
	unnamed := root
	unnamed.Root = ""
	missing := root
	missing.RootID = 0
	ambiguous := root
	ambiguous.AmbiguousProvenance = true
	deref := root
	deref.RootKind = PlaceRootDeref
	reference := root
	reference.Type.Kind = ReferenceType
	inconsistent := root
	inconsistent.RootKind = PlaceRootStatic
	mixed := root
	mixed.AlternativeOrigins = []Place{other}
	agree := root
	agree.AlternativeOrigins = []Place{root}
	allDisjoint := other
	allDisjoint.AlternativeOrigins = []Place{other}
	unknown := root
	unknown.AlternativeOrigins = []Place{missing}
	nested := root
	nested.AlternativeOrigins = []Place{mixed}
	rangeOf := func(start, end int64) PlaceProjection {
		return PlaceProjection{Kind: PlaceSlice, SliceStart: start, SliceEnd: end, SliceStartKnown: true, SliceEndKnown: true}
	}
	dynamic := PlaceProjection{Kind: PlaceIndex, DynamicIndex: true}
	cases := []struct {
		name        string
		left, right Place
		want        StorageOverlap
	}{
		{"same", root, root, StorageOverlapping},
		{"no presentation", root, unnamed, StorageOverlapping},
		{"same spelling distinct roots", root, other, StorageDisjoint},
		{"missing root", root, missing, StorageMayOverlap},
		{"ambiguous", root, ambiguous, StorageMayOverlap},
		{"unresolved referent", root, deref, StorageMayOverlap},
		{"same unresolved referent", deref, deref, StorageMayOverlap},
		{"reference holder", reference, reference, StorageMayOverlap},
		{"inconsistent root role", root, inconsistent, StorageMayOverlap},
		{"contains", root, project(relationshipField("left")), StorageOverlapping},
		{"fields", project(relationshipField("left")), project(relationshipField("right")), StorageDisjoint},
		{"wide index same", project(relationshipIndex("18446744073709551617")), project(relationshipIndex("18446744073709551617")), StorageOverlapping},
		{"wide indexes distinct", project(relationshipIndex("18446744073709551617")), project(relationshipIndex("1")), StorageDisjoint},
		{"dynamic index", project(dynamic), project(relationshipIndex("0")), StorageMayOverlap},
		{"dynamic distinct fields", project(dynamic, relationshipField("left")), project(dynamic, relationshipField("right")), StorageDisjoint},
		{"disjoint slices", project(rangeOf(0, 2)), project(rangeOf(2, 4)), StorageDisjoint},
		{"same slice", project(rangeOf(0, 2)), project(rangeOf(0, 2)), StorageOverlapping},
		{"partially overlapping slices", project(rangeOf(0, 3)), project(rangeOf(2, 4)), StorageMayOverlap},
		{"empty slice", project(rangeOf(1, 1)), root, StorageDisjoint},
		{"unknown slice", project(PlaceProjection{Kind: PlaceSlice}), root, StorageMayOverlap},
		{"index outside slice", project(relationshipIndex("4")), project(rangeOf(0, 2)), StorageDisjoint},
		{"union variants", project(PlaceProjection{Kind: PlaceUnionPayload, Name: "A"}), project(PlaceProjection{Kind: PlaceUnionPayload, Name: "B"}), StorageMayOverlap},
		{"getter", project(PlaceProjection{Kind: PlaceProperty, Name: "Value"}), root, StorageMayOverlap},
		{"unknown projection", project(PlaceProjection{Kind: "future"}), root, StorageMayOverlap},
		{"agreeing origins", agree, root, StorageOverlapping},
		{"all disjoint origins", allDisjoint, root, StorageDisjoint},
		{"mixed origins", mixed, root, StorageMayOverlap},
		{"unknown origin", unknown, root, StorageMayOverlap},
		{"unnormalized nested origins", nested, root, StorageMayOverlap},
	}
	snapshot := NewStorageOverlapSnapshot()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, r := snapshot.Location(tc.left), snapshot.Location(tc.right)
			if got := ClassifyStorageOverlap(l, r); got != tc.want {
				t.Fatalf("overlap = %v, want %v", got, tc.want)
			}
			if got := ClassifyStorageOverlap(r, l); got != tc.want {
				t.Fatalf("reversed overlap = %v, want %v", got, tc.want)
			}
		})
	}
	location := snapshot.Location(root)
	if ClassifyStorageOverlap(location, NewStorageOverlapSnapshot().Location(root)) != StorageMayOverlap ||
		ClassifyStorageOverlap(location, StorageLocation{}) != StorageMayOverlap ||
		ClassifyStorageOverlap(StorageLocation{}, StorageLocation{}) != StorageMayOverlap {
		t.Fatal("missing or incompatible snapshot supplied a proof")
	}
	index := project(relationshipIndex("9"))
	location = snapshot.Location(index)
	index.Projections[0].ConstantIndex.SetInt64(10)
	if ClassifyStorageOverlap(location, snapshot.Location(project(relationshipIndex("9")))) != StorageOverlapping {
		t.Fatal("published location retained mutable index storage")
	}
	location = snapshot.Location(agree)
	agree.AlternativeOrigins[0].RootID = 2
	if ClassifyStorageOverlap(location, snapshot.Location(root)) != StorageOverlapping {
		t.Fatal("published location retained mutable origin storage")
	}
	// Joins are independent of the order in which provenance producers supply
	// alternatives, including when only one pairing has established overlap.
	for _, origins := range [][]Place{{root, other}, {other, root}} {
		candidate := root
		candidate.AlternativeOrigins = origins
		if ClassifyStorageOverlap(snapshot.Location(candidate), snapshot.Location(root)) != StorageMayOverlap {
			t.Fatal("alternative order changed overlap classification")
		}
	}
}

// Real Sema reference plans feed the classifier without rediscovering storage
// from syntax. All analysis depths and analyzer reuse preserve the distinction.
// Rules: rules/analysis/data_races.md — "Canonical Place and provenance".
func TestStorageOverlapResolvedPlans(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/storage_overlap_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(source), "storage_overlap_valid.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		for reuse := 0; reuse < 2; reuse++ {
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			snapshot := NewStorageOverlapSnapshot()
			locations := map[int]StorageLocation{}
			for node := range a.resolvedReferencePlans {
				plan, ok := a.ResolvedReferencePlanOf(node)
				if ok {
					locations[plan.Source.Line] = snapshot.Location(plan.Place)
				}
			}
			if len(locations) != 3 {
				t.Fatalf("reference plans: %v", locations)
			}
			if ClassifyStorageOverlap(locations[10], locations[11]) != StorageOverlapping ||
				ClassifyStorageOverlap(locations[10], locations[12]) != StorageDisjoint {
				t.Fatal("canonical reference paths lost alias/field precision", depth, reuse)
			}
		}
	}
}
