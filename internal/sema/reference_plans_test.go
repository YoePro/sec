package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
)

const referencePlanSource = `module main

type Pair struct {
    left: int,
    right: int,
}

let shared: int := 7

fn Read(value: ref Pair) int {
    return value.left
}

fn Write(value: ref mut Pair) void {
    value.left = 3
}

fn Same(a: ref int, b: ref int) bool {
    return a == b
}

fn Pass(value: ref Pair) int {
    return Read(value)
}

fn Echo(value: ref int) ref int {
    return value
}

fn Use() int {
    let mut pair := Pair { left: 1, right: 2 }
    let first := ref pair.left
    discard first
    let total := Read(pair)
    Write(pair)
    let fixed := ref shared
    discard fixed
    return total
}

fn Arena() Result[void, AllocationError] {
    let mut mem: Arena := Arena {}
    let values := try mem.Alloc[byte](4u)
    let element := ref values[0]
    discard element
    return Ok()
}
`

func referencePlanAt(t *testing.T, analyzer *Analyzer, line int) ResolvedReferencePlan {
	t.Helper()
	for node, plan := range analyzer.resolvedReferencePlans {
		if plan.Source.Line == line {
			queried, ok := analyzer.ResolvedReferencePlanOf(node)
			if !ok {
				t.Fatalf("plan at line %d is not queryable", line)
			}
			return queried
		}
	}
	t.Fatalf("no reference plan at line %d", line)
	return ResolvedReferencePlan{}
}

func referenceUseOfKind(analyzer *Analyzer, kind ResolvedReferenceUseKind, line int) (ResolvedReferenceUse, bool) {
	for node, use := range analyzer.resolvedReferenceUses {
		if use.Kind == kind && use.Source.Line == line {
			return analyzer.ResolvedReferenceUseOf(node)
		}
	}
	return ResolvedReferenceUse{}, false
}

// Every source reference creation, implicit call-site borrow and reborrow has
// a read-only plan with complete reference facts: kind and copy class
// (shared CopyTrivial, mutable MoveOnly), origin Places and storage
// identities, borrow/lifetime identities, address space, provenance and an
// independently classified validity (P15-20..P15-25).
func TestReferencePlansPublishCompleteFacts(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, referencePlanSource)
	assertSemaErrors(t, errors, nil)

	local := referencePlanAt(t, analyzer, 32)
	if local.Facts.Kind != ReferenceSharedDirect || local.CopyClass != CopyTrivial || local.Reborrow || local.Implicit {
		t.Fatalf("local ref plan = %+v", local)
	}
	if local.Facts.Validity != ReferenceValidityProven || local.Facts.ProofProvenance != "lexical-borrow" || local.Facts.Provenance != ReferenceProvenanceLocal {
		t.Fatalf("local ref validity = %s/%s/%s", local.Facts.Validity, local.Facts.ProofProvenance, local.Facts.Provenance)
	}
	if local.Facts.Borrow == 0 || local.Facts.Lifetime == 0 || local.Facts.AddressSpace != AddressSpaceOrdinary || len(local.Facts.StorageIdentities) != 1 || len(local.Facts.OriginPlaces) != 1 || local.Place.String() != "pair.left" {
		t.Fatalf("local ref facts = %+v", local.Facts)
	}
	if local.Facts.AddressStability != AddressStabilityStable {
		t.Fatalf("local address stability = %s", local.Facts.AddressStability)
	}

	read := referencePlanAt(t, analyzer, 34)
	write := referencePlanAt(t, analyzer, 35)
	if !read.Implicit || read.Facts.Kind != ReferenceSharedDirect || !write.Implicit || write.Facts.Kind != ReferenceMutableDirect || write.CopyClass != CopyMoveOnly || !write.Facts.MutableAuthority {
		t.Fatalf("implicit call-site borrows = %+v / %+v", read, write)
	}
	if read.Facts.Borrow == write.Facts.Borrow {
		t.Fatal("distinct borrow creations share one BorrowID")
	}

	static := referencePlanAt(t, analyzer, 36)
	if static.Facts.Provenance != ReferenceProvenanceStatic || static.Facts.ProofProvenance != "static-storage" {
		t.Fatalf("static ref = %+v", static.Facts)
	}

	arena := referencePlanAt(t, analyzer, 44)
	if !arena.Reborrow || arena.Facts.Validity != ReferenceValidityDynamicEpoch || arena.Facts.Provenance != ReferenceProvenanceArena || arena.Facts.InvalidationDomain == 0 || arena.Facts.EpochDependency == 0 || !arena.Facts.Checkable() {
		t.Fatalf("arena element ref = %+v", arena.Facts)
	}
	if _, ok := referenceUseOfKind(analyzer, ReferenceUseReborrow, 44); !ok {
		t.Fatal("arena element borrow is not recorded as a reborrow use")
	}
}

// Reference-mediated uses are planned for reads, writes, equality, passing an
// existing reference and returning one (P15-24).
func TestReferenceUsePlansCoverAccessKinds(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, referencePlanSource)
	assertSemaErrors(t, errors, nil)
	for _, test := range []struct {
		kind ResolvedReferenceUseKind
		line int
	}{{ReferenceUseRead, 11}, {ReferenceUseWrite, 15}, {ReferenceUseEquality, 19}, {ReferenceUsePass, 23}, {ReferenceUseReturn, 27}} {
		use, ok := referenceUseOfKind(analyzer, test.kind, test.line)
		if !ok {
			t.Errorf("no %s use at line %d", test.kind, test.line)
			continue
		}
		if !use.Facts.Checkable() {
			t.Errorf("%s use at line %d is not checkable: %+v", test.kind, test.line, use.Facts)
		}
	}
	if use, _ := referenceUseOfKind(analyzer, ReferenceUseRead, 11); !use.CopyTrivialRead {
		t.Error("an int field read through ref is a copy-trivial read")
	}
	if use, _ := referenceUseOfKind(analyzer, ReferenceUseWrite, 15); !use.TrivialReplacement {
		t.Error("an int field write through ref mut is a trivial replacement")
	}
}

// Queries are read-only: repeating them returns equal facts without adding
// borrows, plans or identities, results are defensive copies, and
// ResolvedPlaceOf exposes analyzed Places only (P15-22, P15-29).
func TestReferencePlanQueriesAreReadOnly(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, referencePlanSource)
	assertSemaErrors(t, errors, nil)
	before := len(analyzer.resolvedReferencePlans) + len(analyzer.resolvedReferenceUses) + len(analyzer.placeIdentities.borrows)
	borrowsBefore := len(analyzer.borrows)
	for node := range analyzer.resolvedReferencePlans {
		first, _ := analyzer.ResolvedReferencePlanOf(node)
		second, _ := analyzer.ResolvedReferencePlanOf(node)
		if !reflect.DeepEqual(first, second) {
			t.Fatal("repeated plan query changed the result")
		}
		if len(first.Facts.OriginPlaces) > 0 {
			first.Facts.OriginPlaces[0].Root = "mutated"
			first.Facts.StorageIdentities = append(first.Facts.StorageIdentities, 99)
		}
		third, _ := analyzer.ResolvedReferencePlanOf(node)
		if len(third.Facts.OriginPlaces) > 0 && third.Facts.OriginPlaces[0].Root == "mutated" {
			t.Fatal("plan query result shares origin storage with the analyzer")
		}
		if _, ok := analyzer.ResolvedPlaceOf(node); ok {
			continue
		}
	}
	if after := len(analyzer.resolvedReferencePlans) + len(analyzer.resolvedReferenceUses) + len(analyzer.placeIdentities.borrows); after != before || len(analyzer.borrows) != borrowsBefore {
		t.Fatal("queries mutated analyzer state")
	}
	if _, ok := analyzer.ResolvedPlaceOf(nil); ok {
		t.Fatal("a nil expression has no Place")
	}
}

// The reference policy defaults to a 64-bit logical epoch on 32- and 64-bit
// plans; a shorter width needs a compile-time proof, and the width never
// enters source reference type identity (P15-26, P15-30).
func TestReferencePolicyEpochWidth(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian})
		if bits := analyzer.ReferencePolicy().EpochBits(); bits != 64 {
			t.Fatalf("%d-bit plan epoch width = %d, want 64", width, bits)
		}
	}
	if err := (layout.ResolvedReferencePolicy{LogicalEpochBits: 32}).Validate(); err == nil || !strings.Contains(err.Error(), "compile-time proof") {
		t.Fatalf("unproven short width accepted: %v", err)
	}
	if err := (layout.ResolvedReferencePolicy{LogicalEpochBits: 32, ShortWidthProof: "retire-on-exhaustion"}).Validate(); err != nil {
		t.Fatalf("proven short width rejected: %v", err)
	}
	if err := (layout.ResolvedReferencePolicy{LogicalEpochBits: 24, ShortWidthProof: "proof"}).Validate(); err == nil {
		t.Fatal("non-byte-multiple width accepted")
	}
	shared := Type{Kind: ReferenceType, Name: "ref int"}
	if strings.Contains(referenceTypeName(Type{Name: "int", Kind: IntType}, false), "64") || shared.Name != "ref int" {
		t.Fatal("epoch width leaked into the source reference type")
	}
}

// Unknown or ambiguous provenance is neither proven nor dynamically
// checkable, so such a reference is not lowerable as a safe reference (P15-25).
func TestUncheckableReferenceFacts(t *testing.T) {
	analyzer := NewAnalyzer()
	facts := analyzer.referenceFactsForPlace(Place{Root: "holder", AmbiguousProvenance: true}, false, lexer.Token{File: "m.sec", Line: 1, Column: 1})
	if facts.Checkable() || facts.Validity != "" || facts.Provenance != ReferenceProvenanceUnknown || facts.AddressStability != AddressStabilityUnknown {
		t.Fatalf("ambiguous provenance facts = %+v", facts)
	}
}

// Reference derivation only preserves or narrows authority: a mutable
// reference is never derived from a shared reference or an immutable Place,
// and returning a local-origin reference stays invalid (P15-27, P15-28).
func TestReferenceDerivationNeverWidensAuthority(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Pair struct {
    left: int,
    right: int,
}

fn Widen(value: ref Pair) void {
    let field := ref mut value.left
    discard field
}

fn Leak() ref int {
    let local := 1
    return ref local
}
`)
	messages := []string{}
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "cannot create mutable reference to immutable place value.left") || !strings.Contains(joined, "cannot return reference to local variable local") {
		t.Fatalf("errors = %v", messages)
	}
}
