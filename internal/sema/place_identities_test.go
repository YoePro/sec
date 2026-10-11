package sema

import (
	"math/big"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Canonical identities are allocated deterministically from semantic keys,
// zero is the absent identity of every kind, printing is stable, and clones
// are detached (P15-11).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 21-23, 29, 30
func TestPlaceIdentityAllocationAndPrinting(t *testing.T) {
	var allocator placeIdentityAllocator
	first := allocator.storageIdentity("binding:a")
	second := allocator.storageIdentity("binding:b")
	if first != 1 || second != 2 || allocator.storageIdentity("binding:a") != first || allocator.storageIdentity("") != 0 {
		t.Fatalf("storage identities = %d, %d", first, second)
	}
	if allocator.invalidationDomain("arena:x") != 1 || allocator.epochDependency("dep") != 1 || allocator.borrow("site") != 1 || allocator.lifetime("site") != 1 {
		t.Fatal("independent identity kinds must allocate independently from 1")
	}
	if space := allocator.targetAddressSpace("dsp"); space != firstTargetDefinedAddress || space.String() != "space:target#1" {
		t.Fatalf("target-defined address space = %d %s", space, space)
	}
	for _, test := range []struct{ got, want string }{
		{StorageIdentityID(0).String(), "storage:none"},
		{first.String(), "storage#1"},
		{InvalidationDomainID(2).String(), "domain#2"},
		{EpochDependencyID(0).String(), "epoch-dep:none"},
		{BorrowID(3).String(), "borrow#3"},
		{LifetimeID(4).String(), "lifetime#4"},
		{AddressSpaceOrdinary.String(), "space:ordinary"},
		{AddressSpaceMMIO.String(), "space:mmio"},
		{AddressSpaceNone.String(), "space:none"},
	} {
		if test.got != test.want {
			t.Errorf("printed %q, want %q", test.got, test.want)
		}
	}
	snapshot := allocator.clone()
	allocator.storageIdentity("binding:c")
	if len(snapshot.storage) != 2 || len(allocator.storage) != 3 {
		t.Fatal("clone shares identity tables with the allocator")
	}
	if keys := identityTableKeys(allocator.storage); len(keys) != 3 || keys[0] != "binding:a" || keys[2] != "binding:c" {
		t.Fatalf("identity order = %v", keys)
	}
}

// Root Places carry storage-domain identity, storage origin and memory space:
// distinct bindings own distinct storage, Arena-backed references share their
// Arena domain's storage, other reference roots leave the referent storage
// unknown, register storage is MMIO, and different known storage domains are
// disjoint (P15-11, P15-17).
func TestRootPlacesCarryStorageIdentity(t *testing.T) {
	analyzer := NewAnalyzer()
	token := func(line int) lexer.Token { return lexer.Token{File: "m.sec", Line: line, Column: 1} }
	arenaRef := func(domain string, line int) Symbol {
		return Symbol{Token: token(line), Local: true, Type: Type{Kind: ReferenceType, ReferenceOriginStorage: StorageOriginArena, ReferenceOriginName: domain}}
	}
	analyzer.symbols = map[string]Symbol{
		"left":     {Token: token(1), Local: true, Storage: StorageOriginAutomatic},
		"right":    {Token: token(2), Local: true, Storage: StorageOriginAutomatic},
		"first":    arenaRef("$arena-domain-a", 3),
		"second":   arenaRef("$arena-domain-a", 4),
		"other":    arenaRef("$arena-domain-b", 5),
		"borrowed": {Token: token(6), Local: true, Parameter: true, Type: Type{Kind: ReferenceType}},
		"control":  {Token: token(7), Storage: StorageOriginStatic, RegisterAccess: RegisterReadOnly},
	}
	place := func(name string) Place {
		root, ok := analyzer.rootPlace(name)
		if !ok {
			t.Fatalf("no root Place for %s", name)
		}
		return root
	}
	left, right := place("left"), place("right")
	if left.StorageIdentity == 0 || left.StorageIdentity == right.StorageIdentity || left.StorageOrigin != StorageOriginAutomatic || left.AddressSpace != AddressSpaceOrdinary {
		t.Fatalf("local storage = %v / %v", left, right)
	}
	if place("first").StorageIdentity != place("second").StorageIdentity || place("first").StorageIdentity == place("other").StorageIdentity {
		t.Fatal("Arena-backed references must share their Arena domain's storage identity and differ across domains")
	}
	if place("borrowed").StorageIdentity != 0 {
		t.Fatal("a reference root must not claim its holder binding's storage")
	}
	if place("control").AddressSpace != AddressSpaceMMIO {
		t.Fatal("register storage must be in the MMIO memory space")
	}
	sameName := Place{Root: "value", StorageIdentity: 1}
	if got := Relationship(sameName, Place{Root: "value", StorageIdentity: 2}); got != PlaceDisjoint {
		t.Fatalf("different storage identities = %s, want disjoint", got)
	}
	if got := Relationship(sameName, Place{Root: "value", StorageIdentity: 1}); got != PlaceSame {
		t.Fatalf("same storage identity = %s, want same", got)
	}
	if got := Relationship(sameName, Place{Root: "value"}); got != PlaceSame {
		t.Fatalf("unknown storage identity must not prove disjointness, got %s", got)
	}
}

// Joined reference origins keep a finite alternative set up to the documented
// implementation bound of 16 and degrade conservatively to unknown provenance
// above it; no single origin is invented and no runtime tag exists (P15-18).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — § 20 "Alternative origins"
//   - rules/compiler/semantic_ir.md — § 45(11)
func TestAlternativeOriginSetIsBounded(t *testing.T) {
	origins := func(count int) []localReferenceOrigin {
		out := make([]localReferenceOrigin, count)
		for index := range out {
			place := Place{Root: "value", RootID: PlaceRootID(index + 1)}
			out[index] = localOriginWithPlaces(localReferenceOrigin{Name: "value"}, []Place{place})
		}
		return out
	}
	bounded := mergeLocalReferenceOrigins(origins(maxReferenceOriginAlternatives), false)
	if bounded.Unknown || len(localOriginPlaces(bounded)) != maxReferenceOriginAlternatives || !bounded.Ambiguous {
		t.Fatalf("%d alternatives = unknown %t, places %d", maxReferenceOriginAlternatives, bounded.Unknown, len(localOriginPlaces(bounded)))
	}
	overflow := mergeLocalReferenceOrigins(origins(maxReferenceOriginAlternatives+1), false)
	if !overflow.Unknown || !overflow.Ambiguous || len(localOriginPlaces(overflow)) != 0 {
		t.Fatalf("%d alternatives must degrade to unknown provenance, got %+v", maxReferenceOriginAlternatives+1, overflow)
	}
	again := mergeLocalReferenceOrigins(origins(maxReferenceOriginAlternatives), false)
	left, right := localOriginPlaces(bounded), localOriginPlaces(again)
	for index := range left {
		if left[index].RootID != right[index].RootID {
			t.Fatal("bounded origin order is not deterministic")
		}
	}
}

// Dynamic index projections are identified by their evaluated index
// expression, never by text: different index expressions get different
// identities, re-resolving one expression keeps its identity, the P14 index
// facts are retained, and two dynamic indexes stay potentially overlapping.
// Projections normalize to the canonical P15 kinds (P15-13, P15-14).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 12-14
func TestDynamicIndexIdentityAndCanonicalProjections(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.expressionTypes = map[ast.Expression]Type{}
	analyzer.resolvedArrayIndexPlans = map[*ast.IndexExpression]ResolvedArrayIndexPlan{}
	intType := Type{Name: "int", Kind: IntType}
	element := intType
	analyzer.symbols = map[string]Symbol{
		"values": {Token: lexer.Token{File: "m.sec", Line: 1, Column: 5}, Local: true, Storage: StorageOriginAutomatic, Type: Type{Kind: ArrayType, Element: &element, ArrayLength: 4}},
		"i":      {Token: lexer.Token{File: "m.sec", Line: 2, Column: 5}, Local: true, Type: intType},
	}
	index := func(name string, column int) *ast.IndexExpression {
		identifier := &ast.Identifier{Token: lexer.Token{File: "m.sec", Line: 3, Column: column, Lexeme: name}, Value: name}
		analyzer.expressionTypes[identifier] = intType
		return &ast.IndexExpression{Token: identifier.Token, Left: &ast.Identifier{Token: lexer.Token{File: "m.sec", Line: 3, Column: 1}, Value: "values"}, Index: identifier}
	}
	first, second := index("i", 10), index("i", 20)
	analyzer.resolvedArrayIndexPlans[first] = ResolvedArrayIndexPlan{IndexType: intType, IndexSigned: true, ArrayLength: big.NewInt(4), ProofKind: ArrayIndexProofRange}
	left, okLeft := analyzer.resolvePlace(first)
	right, okRight := analyzer.resolvePlace(second)
	again, okAgain := analyzer.resolvePlace(first)
	if !okLeft || !okRight || !okAgain {
		t.Fatal("index Places did not resolve")
	}
	l, r, a := left.Projections[0], right.Projections[0], again.Projections[0]
	if l.Canonical() != CanonicalProjectionDynamic || l.DynamicIndexID == 0 || l.DynamicIndexID == r.DynamicIndexID || l.DynamicIndexID != a.DynamicIndexID {
		t.Fatalf("dynamic index identities = %s %s %s", l.DynamicIndexID, r.DynamicIndexID, a.DynamicIndexID)
	}
	if l.IndexType.Kind != IntType || !l.IndexSigned || l.ArrayLength == nil || l.ArrayLength.Int64() != 4 || l.IndexProof != ArrayIndexProofRange {
		t.Fatalf("P14 index facts not retained: %+v", l)
	}
	if got := Relationship(left, right); got != PlacePotentiallyOverlapping {
		t.Fatalf("two dynamic indexes = %s, want potentially-overlapping", got)
	}
	cloned := clonePlaceProjection(l)
	cloned.ArrayLength.SetInt64(9)
	if l.ArrayLength.Int64() != 4 {
		t.Fatal("cloned projection shares the exact array length")
	}
	for _, test := range []struct {
		projection PlaceProjection
		want       CanonicalProjectionKind
	}{
		{PlaceProjection{Kind: PlaceField}, CanonicalProjectionField},
		{PlaceProjection{Kind: PlaceIndex, ConstantIndex: big.NewInt(1)}, CanonicalProjectionConstant},
		{PlaceProjection{Kind: PlaceUnionPayload}, CanonicalProjectionUnionPayload},
		{PlaceProjection{Kind: PlaceDereference}, CanonicalProjectionDereference},
		{PlaceProjection{Kind: PlaceSlice}, CanonicalProjectionSlice},
		{PlaceProjection{Kind: PlaceProperty}, CanonicalProjectionProperty},
	} {
		if got := test.projection.Canonical(); got != test.want {
			t.Errorf("%s normalizes to %s, want %s", test.projection.Kind, got, test.want)
		}
	}
}
