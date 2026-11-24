package layout

import (
	"math/big"
	"strings"
	"testing"
)

// atomicLayoutPlan is a synthetic target whose int64 has natural alignment 4
// (as on some 32-bit ABIs) but whose atomic int64 storage needs 8 (layout.md
// § 47.6(2)); the facts are test fixtures, not a registered target.
func atomicLayoutPlan(atomic ScalarAlignments) (ResolvedScalarPlan, ScalarAlignments) {
	plan := layoutTestPlan(32)
	plan.AtomicAlignments = atomic
	scalars := testAlignments()
	scalars[StorageRepresentation{Class: IntegerStorage, WidthBits: 64}] = 4
	return plan, scalars
}

func atomicOf(contained LayoutRequirement) LayoutRequirement {
	return LayoutRequirement{TypeIdentity: "core:struct:Atomic[" + contained.TypeIdentity + "]", Kind: LayoutAtomic, Carrier: &contained}
}

// TestAtomicStorageLayout resolves Atomic[T] through the canonical service:
// the target atomic minimum raises the natural alignment (T39), an atomic
// field and an array of atomics honour it in offset, aggregate alignment and
// stride (T40, T43), the access extent is published apart from the object
// layout (T46), named layers and integer enums keep their identity and
// validity over the integer carrier (T04, T05), and a minimum below the
// natural alignment never lowers it.
// Rules: rules/memory/layout.md — §§ 8(8), 47.3(4), 47.6; rules/concurrency/atomics.md — § 37A(7).
func TestAtomicStorageLayout(t *testing.T) {
	plan, scalars := atomicLayoutPlan(ScalarAlignments{{Class: IntegerStorage, WidthBits: 64}: 8, {Class: IntegerStorage, WidthBits: 8}: 1, {Class: IntegerStorage, WidthBits: 32}: 2})
	r, err := NewResolver(plan, scalars)
	if err != nil {
		t.Fatal(err)
	}
	u64 := scalar("core:uint:uint64", ScalarUnsigned, 64, false)
	if natural := r.Resolve(u64); natural.AlignBytes.Int64() != 4 {
		t.Fatal("fixture natural alignment", natural.AlignBytes)
	}
	atomic := r.Resolve(atomicOf(u64))
	if !atomic.Complete() || atomic.Kind != LayoutAtomic || atomic.SizeBytes.Int64() != 8 || atomic.AlignBytes.Int64() != 8 ||
		atomic.Atomic != (ResolvedAtomicAccess{Representation: StorageRepresentation{Class: IntegerStorage, WidthBits: 64}, SizeBytes: 8, AlignBytes: 8}) {
		t.Fatalf("Atomic[uint64]: %+v", atomic)
	}
	holder := r.Resolve(LayoutRequirement{TypeIdentity: "app:struct:Holder", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "flag", Requirement: scalar("core:bool:bool", ScalarBool, 0, false)},
		{Name: "count", Requirement: atomicOf(u64)},
	}})
	if !holder.Complete() || holder.Fields[1].Offset.Int64() != 8 || holder.AlignBytes.Int64() != 8 || holder.SizeBytes.Int64() != 16 {
		t.Fatalf("atomic field placement: %+v", holder)
	}
	plain := r.Resolve(LayoutRequirement{TypeIdentity: "app:struct:Plain", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "flag", Requirement: scalar("core:bool:bool", ScalarBool, 0, false)},
		{Name: "count", Requirement: u64},
	}})
	if plain.Fields[1].Offset.Int64() != 4 {
		t.Fatalf("ordinary uint64 field must keep its natural alignment: %+v", plain)
	}
	element := atomicOf(scalar("core:uint:uint32", ScalarUnsigned, 32, false))
	element.TypeIdentity = "core:struct:Atomic[core:uint:uint32]"
	array := r.Resolve(LayoutRequirement{TypeIdentity: "app:array:Atomic32x3", Kind: LayoutArray, Element: &element, Count: big.NewInt(3)})
	if !array.Complete() || array.SizeBytes.Int64() != 12 || array.AlignBytes.Int64() != 4 {
		t.Fatalf("a 32-bit atomic minimum of 2 never lowers natural alignment 4: %+v", array)
	}
	big64 := atomicOf(u64)
	big64.TypeIdentity = "app:array-element"
	wide := r.Resolve(LayoutRequirement{TypeIdentity: "app:array:Atomic64x3", Kind: LayoutArray, Element: &big64, Count: big.NewInt(3)})
	if !wide.Complete() || wide.SizeBytes.Int64() != 24 || wide.AlignBytes.Int64() != 8 {
		t.Fatalf("array stride honours the atomic minimum: %+v", wide)
	}
	named := LayoutRequirement{TypeIdentity: "app:named:RequestCount", Kind: LayoutNamed, Carrier: &u64, Validity: "app:named:RequestCount contracts: RangeContract"}
	outer := LayoutRequirement{TypeIdentity: "app:named:Outer", Kind: LayoutNamed, Carrier: &named}
	counted := r.Resolve(atomicOf(outer))
	if !counted.Complete() || counted.Atomic.Representation.WidthBits != 64 || counted.AlignBytes.Int64() != 8 || !strings.Contains(counted.ValidBitPattern, "RangeContract") {
		t.Fatalf("named layers keep validity over the carrier: %+v", counted)
	}
	byteCarrier := scalar("core:uint:uint8", ScalarUnsigned, 8, false)
	state := LayoutRequirement{TypeIdentity: "app:enum:State", Kind: LayoutEnum, Carrier: &byteCarrier, EnumMembers: []EnumMemberRequirement{{Name: "Idle", Value: big.NewInt(0)}, {Name: "Busy", Value: big.NewInt(1)}}}
	enum := r.Resolve(atomicOf(state))
	if !enum.Complete() || enum.Atomic.SizeBytes != 1 || !strings.Contains(enum.ValidBitPattern, "closed enum values") {
		t.Fatalf("integer enum keeps its closed value domain: %+v", enum)
	}
}

// TestAtomicStorageLayoutFailsClosed keeps a missing target atomic fact an
// incomplete layout (T44, never zero or the natural alignment), rejects
// non-integer carriers, keeps a raw-pointer carrier unsupported and an
// incomplete carrier incomplete, and rejects malformed plan facts.
// Rules: rules/memory/layout.md — §§ 47.6(1), 47.6(6); rules/concurrency/atomics.md — §§ 8-9, 37A(7).
func TestAtomicStorageLayoutFailsClosed(t *testing.T) {
	plan, scalars := atomicLayoutPlan(nil)
	r, err := NewResolver(plan, scalars)
	if err != nil {
		t.Fatal(err)
	}
	missing := r.Resolve(atomicOf(scalar("core:uint:uint64", ScalarUnsigned, 64, false)))
	if missing.Completeness != LayoutIncomplete || missing.SizeBytes != nil || missing.AlignBytes != nil || missing.Atomic != (ResolvedAtomicAccess{}) ||
		len(missing.Missing) != 1 || !strings.Contains(missing.Missing[0], "atomic storage alignment of int64") {
		t.Fatalf("missing atomic fact: %+v", missing)
	}
	holder := r.Resolve(LayoutRequirement{TypeIdentity: "app:struct:Holder", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "count", Requirement: atomicOf(scalar("core:int:int32", ScalarSigned, 32, false))}}})
	if holder.Completeness != LayoutIncomplete {
		t.Fatalf("an aggregate with incomplete atomic storage stays incomplete: %+v", holder)
	}
	withFacts, _ := atomicLayoutPlan(ScalarAlignments{{Class: IntegerStorage, WidthBits: 64}: 8})
	r, _ = NewResolver(withFacts, scalars)
	for name, contained := range map[string]LayoutRequirement{
		"float":   scalar("core:float:float64", ScalarFloat, 64, false),
		"rune":    scalar("core:rune:rune", ScalarRune, 0, false),
		"struct":  {TypeIdentity: "app:struct:Pair", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "a", Requirement: scalar("core:int:int32", ScalarSigned, 32, false)}}},
		"decimal": {TypeIdentity: "core:decimal:decimal", Kind: LayoutScalar, Scalar: ScalarDecimal, WidthBits: 64},
	} {
		if got := r.Resolve(atomicOf(contained)); got.Completeness != LayoutInvalid {
			t.Fatalf("%s carrier accepted: %+v", name, got)
		}
	}
	if got := r.Resolve(atomicOf(LayoutRequirement{TypeIdentity: "core:rawptr:RawPtr[app:A]", Kind: LayoutDescriptor})); got.Completeness != LayoutUnsupported {
		t.Fatalf("raw-pointer carrier: %+v", got)
	}
	if got := r.Resolve(LayoutRequirement{TypeIdentity: "core:struct:Atomic[?]", Kind: LayoutAtomic}); got.Completeness != LayoutInvalid {
		t.Fatalf("Atomic without contained type: %+v", got)
	}
	unresolved, _ := NewResolver(withFacts, nil)
	if got := unresolved.Resolve(atomicOf(scalar("core:uint:uint64", ScalarUnsigned, 64, false))); got.Completeness != LayoutIncomplete || strings.Contains(got.Missing[0], "atomic storage") {
		t.Fatalf("an unresolved carrier keeps its own missing fact: %+v", got)
	}
	for name, facts := range map[string]ScalarAlignments{
		"float fact":  {{Class: BinaryFloatStorage, WidthBits: 64}: 8},
		"zero":        {{Class: IntegerStorage, WidthBits: 64}: 0},
		"not a power": {{Class: IntegerStorage, WidthBits: 64}: 12},
		"odd width":   {{Class: IntegerStorage, WidthBits: 24}: 4},
	} {
		bad, _ := atomicLayoutPlan(facts)
		if bad.Validate() == nil {
			t.Fatalf("%s: plan accepted", name)
		}
	}
}

// TestAtomicStorageLayoutKey binds layouts and fingerprints to the atomic
// facts: different minima give different layout keys and layout
// fingerprints, so no layout or proof is reused across them (§ 47.2(4)).
// Rules: rules/memory/layout.md — §§ 47.2(4), 47.6; rules/concurrency/atomics.md — § 37A(14).
func TestAtomicStorageLayoutKey(t *testing.T) {
	a, scalars := atomicLayoutPlan(ScalarAlignments{{Class: IntegerStorage, WidthBits: 64}: 8})
	b, _ := atomicLayoutPlan(ScalarAlignments{{Class: IntegerStorage, WidthBits: 64}: 16})
	none, _ := atomicLayoutPlan(nil)
	ka, _ := LayoutKeyOf(a, scalars)
	kb, _ := LayoutKeyOf(b, scalars)
	kn, _ := LayoutKeyOf(none, scalars)
	if ka == kb || ka == kn || ka.String() == kb.String() {
		t.Fatal("atomic facts missing from the layout key", ka, kb, kn)
	}
	if a.Fingerprints().Layout == b.Fingerprints().Layout || a.Fingerprints().Layout == none.Fingerprints().Layout {
		t.Fatal("atomic facts missing from the layout fingerprint")
	}
}
