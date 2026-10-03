package sema

import (
	"math/big"
	"testing"
)

func relationshipField(name string) PlaceProjection {
	return PlaceProjection{Kind: PlaceField, Name: name}
}

func relationshipIndex(value string) PlaceProjection {
	index, _ := new(big.Int).SetString(value, 10)
	return PlaceProjection{Kind: PlaceIndex, ConstantIndex: index}
}

func relationshipPlace(root string, projections ...PlaceProjection) Place {
	return Place{Root: root, Projections: projections}
}

// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §15–§20, §138 "Required Sema place tests"
func TestPlaceRelationshipClassifiesCanonicalPaths(t *testing.T) {
	dynamic := PlaceProjection{Kind: PlaceIndex, DynamicIndex: true}
	wide := "18446744073709551617"
	cases := []struct {
		name        string
		left, right Place
		want        PlaceRelationship
	}{
		{"same field", relationshipPlace("pair", relationshipField("Left")), relationshipPlace("pair", relationshipField("Left")), PlaceSame},
		{"distinct fields", relationshipPlace("pair", relationshipField("Left")), relationshipPlace("pair", relationshipField("Right")), PlaceDisjoint},
		{"root contains field", relationshipPlace("pair"), relationshipPlace("pair", relationshipField("Left")), PlaceContains},
		{"field contained by root", relationshipPlace("pair", relationshipField("Left")), relationshipPlace("pair"), PlaceContainedBy},
		{"different roots", relationshipPlace("left"), relationshipPlace("right"), PlaceDisjoint},
		{"equal constant index", relationshipPlace("values", relationshipIndex("2")), relationshipPlace("values", relationshipIndex("2")), PlaceSame},
		{"distinct constant indexes", relationshipPlace("values", relationshipIndex("1")), relationshipPlace("values", relationshipIndex("2")), PlaceDisjoint},
		{"wide constant indexes stay exact", relationshipPlace("values", relationshipIndex(wide)), relationshipPlace("values", relationshipIndex("1")), PlaceDisjoint},
		{"dynamic indexes potentially overlap", relationshipPlace("values", dynamic), relationshipPlace("values", dynamic), PlacePotentiallyOverlapping},
		{"dynamic and constant index", relationshipPlace("values", dynamic), relationshipPlace("values", relationshipIndex("0")), PlacePotentiallyOverlapping},
		{"distinct fields under dynamic index", relationshipPlace("values", dynamic, relationshipField("X")), relationshipPlace("values", dynamic, relationshipField("Y")), PlaceDisjoint},
		{"array contains dynamic element", relationshipPlace("values"), relationshipPlace("values", dynamic), PlaceContains},
		{"different union variants", relationshipPlace("shape", PlaceProjection{Kind: PlaceUnionPayload, Name: "Circle"}), relationshipPlace("shape", PlaceProjection{Kind: PlaceUnionPayload, Name: "Square"}), PlaceDisjoint},
		{"dereference path", relationshipPlace("holder", PlaceProjection{Kind: PlaceDereference}), relationshipPlace("holder", PlaceProjection{Kind: PlaceDereference}, relationshipField("Left")), PlaceContains},
		{"property receiver", relationshipPlace("box", PlaceProjection{Kind: PlaceProperty, Name: "Size"}), relationshipPlace("box", PlaceProjection{Kind: PlaceProperty, Name: "Size"}), PlacePotentiallyOverlapping},
		{"statically disjoint slices", relationshipPlace("values", PlaceProjection{Kind: PlaceSlice, SliceStartKnown: true, SliceEndKnown: true, SliceEnd: 2}), relationshipPlace("values", PlaceProjection{Kind: PlaceSlice, SliceStartKnown: true, SliceStart: 2, SliceEndKnown: true, SliceEnd: 4}), PlaceDisjoint},
		{"unresolved root", relationshipPlace(""), relationshipPlace("values"), PlaceUnknown},
	}
	for _, test := range cases {
		if got := Relationship(test.left, test.right); got != test.want {
			t.Errorf("%s: Relationship(%s, %s) = %s, want %s", test.name, test.left, test.right, got, test.want)
		}
	}
}

func TestPlaceRelationshipJoinsAlternativeOrigins(t *testing.T) {
	left := relationshipPlace("pair", relationshipField("Left"))
	right := relationshipPlace("pair", relationshipField("Right"))
	other := relationshipPlace("other")

	disjoint := relationshipPlace("pair", relationshipField("Left"))
	disjoint.AlternativeOrigins = []Place{relationshipPlace("third")}
	if got := Relationship(disjoint, right); got != PlaceDisjoint {
		t.Fatalf("all-disjoint alternatives = %s, want disjoint", got)
	}

	mixed := relationshipPlace("pair", relationshipField("Left"))
	mixed.AlternativeOrigins = []Place{other}
	if got := Relationship(mixed, left); got != PlacePotentiallyOverlapping {
		t.Fatalf("mixed alternatives = %s, want potentially-overlapping", got)
	}

	agreeing := relationshipPlace("pair", relationshipField("Left"))
	agreeing.AlternativeOrigins = []Place{relationshipPlace("pair", relationshipField("Left"))}
	if got := Relationship(agreeing, left); got != PlaceSame {
		t.Fatalf("agreeing alternatives = %s, want same", got)
	}
}

func TestPlaceRelationshipDegradesAmbiguousProvenanceToUnknown(t *testing.T) {
	ambiguous := relationshipPlace("holder", PlaceProjection{Kind: PlaceDereference})
	ambiguous.AmbiguousProvenance = true
	if got := Relationship(ambiguous, relationshipPlace("elsewhere")); got != PlaceUnknown {
		t.Fatalf("ambiguous provenance = %s, want unknown", got)
	}
	if !PlaceUnknown.MayOverlap() || !PlaceUnknown.EnclosesOrMayEnclose() {
		t.Fatal("unknown relationship must be treated conservatively")
	}
}

func TestLegacyPlacesOverlapDelegatesToRelationship(t *testing.T) {
	pairs := [][2]Place{
		{relationshipPlace("pair", relationshipField("Left")), relationshipPlace("pair", relationshipField("Right"))},
		{relationshipPlace("pair"), relationshipPlace("pair", relationshipField("Left"))},
		{relationshipPlace("values", relationshipIndex("1")), relationshipPlace("values", relationshipIndex("1"))},
		{relationshipPlace("values", PlaceProjection{Kind: PlaceIndex, DynamicIndex: true}), relationshipPlace("values", relationshipIndex("3"))},
		{relationshipPlace("left"), relationshipPlace("right")},
	}
	for _, pair := range pairs {
		want := Relationship(pair[0], pair[1]) != PlaceDisjoint
		if got := PlacesOverlap(pair[0], pair[1]); got != want {
			t.Errorf("PlacesOverlap(%s, %s) = %v, want %v", pair[0], pair[1], got, want)
		}
	}
	if PlacesOverlap(relationshipPlace(""), relationshipPlace("values")) {
		t.Fatal("a Place without a resolved root names no tracked storage")
	}
}
