package layout

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func layoutTestPlan(width uint16) ResolvedScalarPlan {
	return ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", LLVMTriple: "test-unknown-linux", ABI: "gnu", Profile: "hosted", PointerWidthBits: width, Endianness: LittleEndian}
}

// testAlignments is a test-only alignment table; real targets supply none
// until MD-141 is decided.
func testAlignments() ScalarAlignments {
	return ScalarAlignments{
		{Class: IntegerStorage, WidthBits: 8}: 1, {Class: IntegerStorage, WidthBits: 16}: 2,
		{Class: IntegerStorage, WidthBits: 32}: 4, {Class: IntegerStorage, WidthBits: 64}: 8,
		{Class: IntegerStorage, WidthBits: 128}: 16, {Class: IntegerStorage, WidthBits: 256}: 32,
		{Class: BinaryFloatStorage, WidthBits: 32}: 4, {Class: BinaryFloatStorage, WidthBits: 64}: 8,
	}
}

func testAddressDomain() AddressDomain {
	return AddressDomain{Identity: "io", AddressBits: 16, UnitBits: 8, Linear: true}
}

func scalar(identity string, kind ScalarKind, width uint16, targetSized bool) LayoutRequirement {
	return LayoutRequirement{TypeIdentity: identity, Kind: LayoutScalar, Scalar: kind, WidthBits: width, TargetSized: targetSized}
}

// corrupt replaces a cached entry's recorded plan key to model a stale or
// malformed cache entry.
func (c *LayoutCache) corrupt(plan CompilationPlanLayoutKey, identity string, contract LayoutContractID, with CompilationPlanLayoutKey) {
	key := cacheKeyOf(plan, identity, contract)
	if entry, found := c.entries[key]; found {
		entry.PlanKey = with
		c.entries[key] = entry
	}
}

// P24-10: a published layout is an immutable record carrying its semantic
// identity, plan key, contract, kind, scalar provenance, size, alignment and
// completeness; callers get detached copies.
// Rules: rules/memory/layout.md — § 39; rules/mlir/packages/sec-mlir-dialect_package24.md — § 8.
func TestResolvedLayoutRecords(t *testing.T) {
	resolver, err := NewResolver(layoutTestPlan(64), testAlignments())
	if err != nil {
		t.Fatal(err)
	}
	layout := resolver.Resolve(scalar("int32", ScalarSigned, 32, false))
	if layout.ID == 0 || layout.TypeIdentity != "int32" || layout.Contract != NativeLayoutContract || layout.Kind != LayoutScalar ||
		layout.Scalar != ScalarSigned || layout.PlanKey != resolver.Key() || !layout.Complete() || layout.Provenance != "scalar:signed" ||
		layout.Stability != LayoutTargetStable || layout.ValidBitPattern != "all 32-bit patterns" {
		t.Fatalf("record = %+v", layout)
	}
	layout.SizeBytes.SetInt64(99)
	if again, _ := resolver.Layout(layout.ID); again.SizeBytes.Cmp(big.NewInt(4)) != 0 {
		t.Fatal("a caller mutated a published layout")
	}
	record := ResolvedLayout{Count: big.NewInt(3), Variants: []ResolvedVariantLayout{{Tag: big.NewInt(1), Offset: big.NewInt(0)}}, Missing: []string{"x"}}
	copied := record.clone()
	copied.Count.SetInt64(9)
	copied.Variants[0].Tag.SetInt64(9)
	copied.Missing[0] = "y"
	if record.Count.Int64() != 3 || record.Variants[0].Tag.Int64() != 1 || record.Missing[0] != "x" {
		t.Fatal("a clone shares array, variant or diagnostic state with its source")
	}
}

// P24-11: the plan key folds in every plan fact that can change a layout and
// rejects an invalid plan or alignment fact.
// Rules: rules/memory/layout.md — § 3; rules/mlir/packages/sec-mlir-dialect_package24.md — § 9.
func TestCompilationPlanLayoutKey(t *testing.T) {
	base, err := LayoutKeyOf(layoutTestPlan(64), testAlignments())
	if err != nil {
		t.Fatal(err)
	}
	variants := map[string]func(*ResolvedScalarPlan, *ScalarAlignments){
		"width":      func(p *ResolvedScalarPlan, _ *ScalarAlignments) { p.PointerWidthBits = 32 },
		"endianness": func(p *ResolvedScalarPlan, _ *ScalarAlignments) { p.Endianness = BigEndian },
		"triple":     func(p *ResolvedScalarPlan, _ *ScalarAlignments) { p.LLVMTriple = "other" },
		"profile":    func(p *ResolvedScalarPlan, _ *ScalarAlignments) { p.Profile = "noalloc" },
		"reference": func(p *ResolvedScalarPlan, _ *ScalarAlignments) {
			p.References.LogicalEpochBits = 32
			p.References.ShortWidthProof = "proof"
			p.References.ArenaShortWidthSafety = ArenaEpochCheckedAdvance
		},
		"alignment": func(_ *ResolvedScalarPlan, a *ScalarAlignments) { (*a)[StorageRepresentation{IntegerStorage, 64}] = 4 },
		"address": func(p *ResolvedScalarPlan, _ *ScalarAlignments) {
			p.AddressDomains = []AddressDomain{testAddressDomain()}
		},
		"no table": func(_ *ResolvedScalarPlan, a *ScalarAlignments) { *a = nil },
	}
	for name, change := range variants {
		plan, alignments := layoutTestPlan(64), testAlignments()
		change(&plan, &alignments)
		key, err := LayoutKeyOf(plan, alignments)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if key == base || key.String() == base.String() {
			t.Errorf("%s does not change the layout key", name)
		}
	}
	if !strings.Contains(base.String(), LayoutRevision) {
		t.Error("the layout key omits the compiler layout revision")
	}
	if _, err := LayoutKeyOf(ResolvedScalarPlan{}, nil); err == nil {
		t.Error("an invalid plan produced a layout key")
	}
	if _, err := LayoutKeyOf(layoutTestPlan(64), ScalarAlignments{{Class: IntegerStorage, WidthBits: 32}: 3}); err == nil {
		t.Error("a non-power-of-two alignment produced a layout key")
	}
}

// P24-13: complete, incomplete, invalid and unsupported resolver states; no
// incomplete layout receives a guessed size or alignment.
// Rules: rules/memory/layout.md — § 5.
func TestLayoutResolverStates(t *testing.T) {
	complete, _ := NewResolver(layoutTestPlan(64), testAlignments())
	unresolved, _ := NewResolver(layoutTestPlan(64), nil)
	for name, check := range map[string]struct {
		resolver    *Resolver
		requirement LayoutRequirement
		want        LayoutCompleteness
		missing     string
	}{
		"complete":            {complete, scalar("int64", ScalarSigned, 64, false), LayoutComplete, ""},
		"incomplete (MD-141)": {unresolved, scalar("int64", ScalarSigned, 64, false), LayoutIncomplete, "alignment of int64 storage"},
		"invalid width":       {complete, scalar("int7", ScalarSigned, 7, false), LayoutInvalid, "not a whole number of bytes"},
		"invalid identity":    {complete, LayoutRequirement{Kind: LayoutScalar, Scalar: ScalarSigned, WidthBits: 8}, LayoutInvalid, "semantic type identity"},
		"invalid decimal":     {complete, scalar("decimal", ScalarDecimal, 32, false), LayoutInvalid, "canonical i64 or i128"},
		"incomplete struct":   {unresolved, LayoutRequirement{TypeIdentity: "main::Pair", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "x", Requirement: scalar("int32", ScalarSigned, 32, false)}}}, LayoutIncomplete, "MD-142"},
		"unsupported field":   {complete, LayoutRequirement{TypeIdentity: "main::View", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "s", Requirement: LayoutRequirement{TypeIdentity: "slice:byte", Kind: LayoutDescriptor}}}}, LayoutUnsupported, "s: layout kind"},
		"explicit contract":   {complete, LayoutRequirement{TypeIdentity: "main::Packed", Kind: LayoutStruct, Contract: "pack(1)"}, LayoutUnsupported, "MD-019"},
	} {
		layout := check.resolver.Resolve(check.requirement)
		if layout.Completeness != check.want || check.missing != "" && !strings.Contains(strings.Join(layout.Missing, ";"), check.missing) {
			t.Errorf("%s: %s %v, want %s with %q", name, layout.Completeness, layout.Missing, check.want, check.missing)
		}
		if !layout.Complete() && (layout.SizeBytes != nil && layout.AlignBytes != nil) {
			t.Errorf("%s: a non-complete layout carries both size and alignment", name)
		}
	}
}

// P24-14: deterministic memoization and the in-progress containment stack;
// a by-value cycle is invalid with its concrete path, and IDs are stable
// across resolver runs.
// Rules: rules/memory/layout.md — § 25; rules/mlir/packages/sec-mlir-dialect_package24.md — § 16.
func TestLayoutResolverMemoizationAndCycles(t *testing.T) {
	build := func() *Resolver {
		resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
		return resolver
	}
	resolver := build()
	first := resolver.Resolve(scalar("uint16", ScalarUnsigned, 16, false))
	second := resolver.Resolve(scalar("uint16", ScalarUnsigned, 16, false))
	if first.ID != second.ID || len(resolver.Layouts()) != 1 {
		t.Fatalf("the same requirement resolved twice: %d %d", first.ID, second.ID)
	}
	b := LayoutRequirement{TypeIdentity: "main::B", Kind: LayoutStruct}
	a := LayoutRequirement{TypeIdentity: "main::A", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "b", Requirement: b}}}
	b.Fields = []FieldRequirement{{Name: "a", Requirement: LayoutRequirement{TypeIdentity: "main::A", Kind: LayoutStruct}}}
	a.Fields[0].Requirement = b
	cyclic := resolver.Resolve(a)
	if cyclic.Completeness != LayoutInvalid || !strings.Contains(strings.Join(cyclic.Missing, ""), "main::A -> main::B -> main::A") {
		t.Fatalf("cycle = %s %v", cyclic.Completeness, cyclic.Missing)
	}
	indirect := LayoutRequirement{TypeIdentity: "main::Node", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "next", Requirement: LayoutRequirement{TypeIdentity: "ref main::Node", Kind: LayoutDescriptor}},
	}}
	if layout := resolver.Resolve(indirect); layout.Completeness == LayoutInvalid {
		t.Fatalf("an indirect cycle through a reference was reported as by-value: %v", layout.Missing)
	}
	order := func(r *Resolver) []string {
		out := []string{}
		for _, layout := range r.Layouts() {
			out = append(out, layout.TypeIdentity)
		}
		return out
	}
	again := build()
	again.Resolve(scalar("uint16", ScalarUnsigned, 16, false))
	again.Resolve(a)
	again.Resolve(indirect)
	if strings.Join(order(resolver), ",") != strings.Join(order(again), ",") {
		t.Fatalf("layout IDs are not stable across runs: %v vs %v", order(resolver), order(again))
	}
}

// P24-15: cold versus warm cache; a different width, reference policy,
// alignment fact or generic substitution misses, and a stale entry is
// rejected fail-closed.
// Rules: rules/memory/layout.md — § 3(5); rules/mlir/packages/sec-mlir-dialect_package24.md — § 9.
func TestLayoutCacheSafety(t *testing.T) {
	cache := NewLayoutCache()
	r64, _ := NewResolver(layoutTestPlan(64), testAlignments())
	word := r64.Resolve(scalar("int", ScalarSigned, 0, true))
	cache.Store(word)
	if cached, hit := cache.Lookup(r64.Key(), "int", ""); !hit || cached.SizeBytes.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("warm lookup = %+v %t", cached, hit)
	}
	r32, _ := NewResolver(layoutTestPlan(32), testAlignments())
	if _, hit := cache.Lookup(r32.Key(), "int", ""); hit {
		t.Error("a 64-bit layout answered a 32-bit plan")
	}
	policy := layoutTestPlan(64)
	policy.References = ResolvedReferencePolicy{LogicalEpochBits: 32, ShortWidthProof: "proof", ArenaShortWidthSafety: ArenaEpochCheckedAdvance}
	rPolicy, _ := NewResolver(policy, testAlignments())
	if _, hit := cache.Lookup(rPolicy.Key(), "int", ""); hit {
		t.Error("a changed reference policy reused a cached layout")
	}
	aligned := testAlignments()
	aligned[StorageRepresentation{IntegerStorage, 64}] = 4
	rAligned, _ := NewResolver(layoutTestPlan(64), aligned)
	if _, hit := cache.Lookup(rAligned.Key(), "int", ""); hit {
		t.Error("a changed alignment fact reused a cached layout")
	}
	if _, hit := cache.Lookup(r64.Key(), "main::Box[T=int64]", ""); hit {
		t.Error("a different generic substitution reused a cached layout")
	}
	rAddress, _ := NewResolver(func() ResolvedScalarPlan {
		plan := layoutTestPlan(64)
		plan.AddressDomains = []AddressDomain{testAddressDomain()}
		return plan
	}(), testAlignments())
	if _, hit := cache.Lookup(rAddress.Key(), "int", ""); hit {
		t.Error("a changed address-domain fact reused a cached layout")
	}
	if _, hit := cache.Lookup(r64.Key(), "int", "pack(1)"); hit {
		t.Error("an explicit layout contract reused the native layout")
	}
	cache.corrupt(r64.Key(), "int", NativeLayoutContract, r32.Key())
	if _, hit := cache.Lookup(r64.Key(), "int", ""); hit || cache.Rejected != 1 {
		t.Fatalf("a stale entry was trusted (rejected=%d)", cache.Rejected)
	}
}

// P24-16 and MD-141: scalar layouts keep their source kind; fixed widths,
// byte/char, target-sized int/uint and plain float (plan width), fixed
// floats, bool (1 byte) and rune (4 bytes) resolve, and every alignment comes
// from the plan's typed fact for the storage representation.
// Rules: rules/memory/layout.md — §§ 18, 18(4a), 18(5); rules/types/types.md — §§ 9.2–9.4;
// MD-141 correction §§ 2–3.
func TestScalarLayouts(t *testing.T) {
	integer := func(width uint16) StorageRepresentation { return StorageRepresentation{IntegerStorage, width} }
	binary := func(width uint16) StorageRepresentation { return StorageRepresentation{BinaryFloatStorage, width} }
	for _, width := range []uint16{32, 64} {
		resolver, _ := NewResolver(layoutTestPlan(width), testAlignments())
		word := uint64(width / 8)
		for _, check := range []struct {
			requirement    LayoutRequirement
			size           uint64
			representation StorageRepresentation
		}{
			{scalar("int8", ScalarSigned, 8, false), 1, integer(8)}, {scalar("int16", ScalarSigned, 16, false), 2, integer(16)},
			{scalar("int32", ScalarSigned, 32, false), 4, integer(32)}, {scalar("int64", ScalarSigned, 64, false), 8, integer(64)},
			{scalar("int128", ScalarSigned, 128, false), 16, integer(128)}, {scalar("int256", ScalarSigned, 256, false), 32, integer(256)},
			{scalar("uint8", ScalarUnsigned, 8, false), 1, integer(8)}, {scalar("uint256", ScalarUnsigned, 256, false), 32, integer(256)},
			{scalar("byte", ScalarByte, 8, false), 1, integer(8)}, {scalar("char", ScalarChar, 8, false), 1, integer(8)},
			{scalar("bool", ScalarBool, 0, false), 1, integer(8)}, {scalar("rune", ScalarRune, 0, false), 4, integer(32)},
			{scalar("int", ScalarSigned, 0, true), word, integer(width)}, {scalar("uint", ScalarUnsigned, 0, true), word, integer(width)},
			{scalar("float", ScalarFloat, 0, true), word, binary(width)}, {scalar("float32", ScalarFloat, 32, false), 4, binary(32)},
			{scalar("float64", ScalarFloat, 64, false), 8, binary(64)},
		} {
			layout := resolver.Resolve(check.requirement)
			if !layout.Complete() || layout.SizeBytes.Uint64() != check.size || layout.AlignBytes.Uint64() != testAlignments()[check.representation] || layout.Scalar != check.requirement.Scalar {
				t.Errorf("%d-bit %s = %s size=%v align=%v %v", width, check.requirement.TypeIdentity, layout.Completeness, layout.SizeBytes, layout.AlignBytes, layout.Missing)
			}
		}
	}
	resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
	if layout := resolver.Resolve(scalar("bool", ScalarBool, 0, false)); !strings.Contains(layout.ValidBitPattern, "0x00 (false) or 0x01 (true) only") {
		t.Errorf("bool validity = %q", layout.ValidBitPattern)
	}
	if layout := resolver.Resolve(scalar("rune", ScalarRune, 0, false)); !strings.Contains(layout.ValidBitPattern, "without surrogates") {
		t.Errorf("rune validity = %q", layout.ValidBitPattern)
	}
	unresolved, _ := NewResolver(layoutTestPlan(64), nil)
	if layout := unresolved.Resolve(scalar("int32", ScalarSigned, 32, false)); layout.Complete() || layout.AlignBytes != nil || !strings.Contains(strings.Join(layout.Missing, ""), "int32") {
		t.Errorf("a scalar alignment was guessed without plan facts: %+v", layout)
	}
}

// MD-141 § 3(9), § 10(3): integer and float storage of equal size are
// distinct facts. A synthetic plan gives float32 alignment 2 and int32
// alignment 4; each scalar uses its own fact and the plan keys differ.
func TestScalarAlignmentIsKeyedByRepresentation(t *testing.T) {
	split := testAlignments()
	split[StorageRepresentation{BinaryFloatStorage, 32}] = 2
	resolver, err := NewResolver(layoutTestPlan(64), split)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolver.Resolve(scalar("float32", ScalarFloat, 32, false)).AlignBytes; got.Uint64() != 2 {
		t.Fatalf("float32 alignment = %v, want 2", got)
	}
	if got := resolver.Resolve(scalar("int32", ScalarSigned, 32, false)).AlignBytes; got.Uint64() != 4 {
		t.Fatalf("int32 alignment = %v, want 4", got)
	}
	plain, _ := NewResolver(layoutTestPlan(64), testAlignments())
	if plain.Key() == resolver.Key() {
		t.Fatal("plans with different float32 alignment share a layout key")
	}
	if _, err := LayoutKeyOf(layoutTestPlan(64), ScalarAlignments{{IntegerStorage, 24}: 4}); err == nil {
		t.Fatal("an unknown storage representation was accepted")
	}
}

// layout.md § 7(7): the maximum byte extent is 2^(W-1) - 1, and an array
// whose unpadded extent already exceeds it is invalid before placement.
func TestMaximumExtent(t *testing.T) {
	if got := MaximumExtent(layoutTestPlan(32)); got.Cmp(big.NewInt(1<<31-1)) != 0 {
		t.Fatalf("32-bit maximum = %s", got)
	}
	if got := MaximumExtent(layoutTestPlan(64)); got.Cmp(new(big.Int).SetUint64(1<<63-1)) != 0 {
		t.Fatalf("64-bit maximum = %s", got)
	}
	resolver, _ := NewResolver(layoutTestPlan(32), testAlignments())
	element := scalar("int64", ScalarSigned, 64, false)
	huge := resolver.Resolve(LayoutRequirement{TypeIdentity: "array:fixed:268435456:int64", Kind: LayoutArray, Element: &element, Count: big.NewInt(1 << 28)})
	if huge.Completeness != LayoutInvalid || !strings.Contains(strings.Join(huge.Missing, ""), "2147483647") {
		t.Fatalf("an over-extent array = %s %v", huge.Completeness, huge.Missing)
	}
	fits := resolver.Resolve(LayoutRequirement{TypeIdentity: "array:fixed:4:int64", Kind: LayoutArray, Element: &element, Count: big.NewInt(4)})
	if !fits.Complete() || fits.Count.Int64() != 4 || fits.Element == 0 || fits.SizeBytes.Int64() != 32 || fits.AlignBytes.Int64() != 8 {
		t.Fatalf("a fitting array = %+v", fits)
	}
}

// fieldOffsets lists the resolved field offsets of a layout.
func fieldOffsets(layout ResolvedLayout) []int64 {
	offsets := make([]int64, len(layout.Fields))
	for index, field := range layout.Fields {
		offsets[index] = -1
		if field.Offset != nil {
			offsets[index] = field.Offset.Int64()
		}
	}
	return offsets
}

// P24-19 / MD-142 § 6 / layout.md § 24(6): an empty struct has size 0 and
// alignment 1; a nonempty struct places its fields in declaration order with
// interior and tail padding, and zero-sized fields occupy no bytes.
func TestStructLayouts(t *testing.T) {
	resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
	unit := LayoutRequirement{TypeIdentity: "main::Unit", Kind: LayoutStruct}
	empty := resolver.Resolve(unit)
	if !empty.Complete() || empty.SizeBytes.Sign() != 0 || empty.AlignBytes.Int64() != 1 {
		t.Fatalf("empty struct = %+v", empty)
	}
	for _, check := range []struct {
		name        string
		fields      []FieldRequirement
		offsets     []int64
		size, align int64
	}{
		// byte at 0, 7 bytes interior padding, int64 at 8, bool at 16, 7 bytes tail padding.
		{"main::Pair", []FieldRequirement{
			{Name: "b", Requirement: scalar("byte", ScalarByte, 8, false)},
			{Name: "n", Requirement: scalar("int64", ScalarSigned, 64, false)},
			{Name: "flag", Requirement: scalar("bool", ScalarBool, 0, false)},
		}, []int64{0, 8, 16}, 24, 8},
		// Declaration order is kept: int16 after the rune needs no padding.
		{"main::Glyph", []FieldRequirement{
			{Name: "r", Requirement: scalar("rune", ScalarRune, 0, false)},
			{Name: "w", Requirement: scalar("int16", ScalarSigned, 16, false)},
		}, []int64{0, 4}, 8, 4},
		// A zero-sized field takes no bytes and shares the next offset.
		{"main::Gap", []FieldRequirement{
			{Name: "a", Requirement: scalar("int8", ScalarSigned, 8, false)},
			{Name: "u", Requirement: unit},
			{Name: "b", Requirement: scalar("int16", ScalarSigned, 16, false)},
		}, []int64{0, 1, 2}, 4, 2},
		// Only zero-sized fields: size 0, alignment 1, separate identities.
		{"main::Units", []FieldRequirement{{Name: "u", Requirement: unit}, {Name: "v", Requirement: unit}}, []int64{0, 0}, 0, 1},
	} {
		layout := resolver.Resolve(LayoutRequirement{TypeIdentity: check.name, Kind: LayoutStruct, Fields: check.fields})
		offsets := fieldOffsets(layout)
		if !layout.Complete() || layout.SizeBytes.Int64() != check.size || layout.AlignBytes.Int64() != check.align || fmt.Sprint(offsets) != fmt.Sprint(check.offsets) {
			t.Errorf("%s = %s size=%v align=%v offsets=%v %v", check.name, layout.Completeness, layout.SizeBytes, layout.AlignBytes, offsets, layout.Missing)
			continue
		}
		for index, field := range check.fields {
			if layout.Fields[index].Name != field.Name || layout.Fields[index].Index != index || layout.Fields[index].Layout == 0 {
				t.Errorf("%s field %d = %+v", check.name, index, layout.Fields[index])
			}
		}
	}
	outer := resolver.Resolve(LayoutRequirement{TypeIdentity: "main::Outer", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "tag", Requirement: scalar("byte", ScalarByte, 8, false)},
		{Name: "glyph", Requirement: LayoutRequirement{TypeIdentity: "main::Glyph", Kind: LayoutStruct}},
	}})
	if !outer.Complete() || fmt.Sprint(fieldOffsets(outer)) != "[0 4]" || outer.SizeBytes.Int64() != 12 || outer.AlignBytes.Int64() != 4 {
		t.Errorf("nested struct = %s offsets=%v size=%v align=%v", outer.Completeness, fieldOffsets(outer), outer.SizeBytes, outer.AlignBytes)
	}
}

// MD-142 § 7(1-3): T[N] uses the element alignment, stride
// AlignUp(Size, Align) and size N × stride; T[0] and zero-sized elements
// have size zero while the logical count is kept.
func TestArrayLayouts(t *testing.T) {
	resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
	padded := LayoutRequirement{TypeIdentity: "main::Padded", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "n", Requirement: scalar("int32", ScalarSigned, 32, false)},
		{Name: "b", Requirement: scalar("byte", ScalarByte, 8, false)},
	}}
	unit := LayoutRequirement{TypeIdentity: "main::Unit", Kind: LayoutStruct}
	for _, check := range []struct {
		name        string
		element     LayoutRequirement
		count       int64
		size, align int64
	}{
		{"array:fixed:0:Padded", padded, 0, 0, 4},
		{"array:fixed:1:Padded", padded, 1, 8, 4},
		{"array:fixed:3:Padded", padded, 3, 24, 4},
		{"array:fixed:3:Unit", unit, 3, 0, 1},
		{"array:fixed:5:bool", scalar("bool", ScalarBool, 0, false), 5, 5, 1},
	} {
		element := check.element
		layout := resolver.Resolve(LayoutRequirement{TypeIdentity: check.name, Kind: LayoutArray, Element: &element, Count: big.NewInt(check.count)})
		if !layout.Complete() || layout.SizeBytes.Int64() != check.size || layout.AlignBytes.Int64() != check.align || layout.Count.Int64() != check.count {
			t.Errorf("%s = %s size=%v align=%v count=%v %v", check.name, layout.Completeness, layout.SizeBytes, layout.AlignBytes, layout.Count, layout.Missing)
		}
	}
	inner := scalar("int16", ScalarSigned, 16, false)
	row := LayoutRequirement{TypeIdentity: "array:fixed:3:int16", Kind: LayoutArray, Element: &inner, Count: big.NewInt(3)}
	grid := resolver.Resolve(LayoutRequirement{TypeIdentity: "array:fixed:2:array:fixed:3:int16", Kind: LayoutArray, Element: &row, Count: big.NewInt(2)})
	if !grid.Complete() || grid.SizeBytes.Int64() != 12 || grid.AlignBytes.Int64() != 2 {
		t.Errorf("embedded array = %s size=%v align=%v", grid.Completeness, grid.SizeBytes, grid.AlignBytes)
	}
}

// P24-17 / MD-142 § 7(4-6): decimal and decimal128 place coefficient then
// scale with the ordinary field algorithm, never as a packed 12/20-byte
// record; a plan with a different i128 alignment changes the result.
func TestDecimalCarriers(t *testing.T) {
	resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
	for _, check := range []struct {
		width       uint16
		coefficient string
		offsets     string
		size, align int64
	}{{64, "int64", "[0 8]", 16, 8}, {128, "int128", "[0 16]", 32, 16}} {
		layout := resolver.Resolve(scalar(fmt.Sprintf("decimal%d", check.width), ScalarDecimal, check.width, false))
		if !layout.Complete() || layout.Scalar != ScalarDecimal || len(layout.Fields) != 2 ||
			layout.Fields[0].Name != "coefficient" || layout.Fields[1].Name != "scale-storage" ||
			fmt.Sprint(fieldOffsets(layout)) != check.offsets || layout.SizeBytes.Int64() != check.size || layout.AlignBytes.Int64() != check.align {
			t.Fatalf("decimal%d = %s offsets=%v size=%v align=%v %v", check.width, layout.Completeness, fieldOffsets(layout), layout.SizeBytes, layout.AlignBytes, layout.Missing)
		}
		coefficient, _ := resolver.Layout(layout.Fields[0].Layout)
		scaleStorage, _ := resolver.Layout(layout.Fields[1].Layout)
		if coefficient.TypeIdentity != check.coefficient || scaleStorage.TypeIdentity != "int32" || !coefficient.Complete() || !scaleStorage.Complete() {
			t.Errorf("decimal%d carrier fields = %s/%s", check.width, coefficient.TypeIdentity, scaleStorage.TypeIdentity)
		}
	}
	narrow := testAlignments()
	narrow[StorageRepresentation{IntegerStorage, 128}] = 8
	other, _ := NewResolver(layoutTestPlan(64), narrow)
	layout := other.Resolve(scalar("decimal128", ScalarDecimal, 128, false))
	if !layout.Complete() || fmt.Sprint(fieldOffsets(layout)) != "[0 16]" || layout.SizeBytes.Int64() != 24 || layout.AlignBytes.Int64() != 8 {
		t.Fatalf("decimal128 with 8-byte i128 alignment = %s offsets=%v size=%v align=%v", layout.Completeness, fieldOffsets(layout), layout.SizeBytes, layout.AlignBytes)
	}
}

// P24-18: a named or constrained type reuses its carrier's layout while
// keeping its own nominal identity and combining validity predicates.
func TestNamedLayouts(t *testing.T) {
	resolver, _ := NewResolver(layoutTestPlan(64), testAlignments())
	carrier := scalar("int32", ScalarSigned, 32, false)
	named := resolver.Resolve(LayoutRequirement{TypeIdentity: "main::Percent", Kind: LayoutNamed, Carrier: &carrier, Validity: "main::Percent contract: 0 <= value <= 100"})
	base, _ := resolver.Layout(named.Carrier)
	if !named.Complete() || named.TypeIdentity != "main::Percent" || base.TypeIdentity != "int32" || named.SizeBytes.Int64() != 4 ||
		named.Scalar != ScalarSigned || named.ValidBitPattern != "all 32-bit patterns; main::Percent contract: 0 <= value <= 100" || named.Provenance != "named over int32" {
		t.Fatalf("named = %+v", named)
	}
	if named.ID == base.ID {
		t.Fatal("a named type was merged into its carrier's identity")
	}
	flag := scalar("bool", ScalarBool, 0, false)
	unresolved, _ := NewResolver(layoutTestPlan(64), nil)
	if layout := unresolved.Resolve(LayoutRequirement{TypeIdentity: "main::Flag", Kind: LayoutNamed, Carrier: &flag}); layout.Completeness != LayoutIncomplete {
		t.Errorf("a named type over an incomplete carrier = %s", layout.Completeness)
	}
	if layout := resolver.Resolve(LayoutRequirement{TypeIdentity: "main::Orphan", Kind: LayoutNamed}); layout.Completeness != LayoutInvalid {
		t.Errorf("a named type without a carrier = %s", layout.Completeness)
	}
}
