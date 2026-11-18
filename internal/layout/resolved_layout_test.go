package layout

import (
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
	return ScalarAlignments{1: 1, 2: 2, 4: 4, 8: 8, 16: 16, 32: 32}
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
		layout.Stability != LayoutPlanLocal || layout.ValidBitPattern != "all 32-bit patterns" {
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
		},
		"alignment": func(_ *ResolvedScalarPlan, a *ScalarAlignments) { (*a)[8] = 4 },
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
	if _, err := LayoutKeyOf(layoutTestPlan(64), ScalarAlignments{4: 3}); err == nil {
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
		"incomplete (MD-141)": {unresolved, scalar("int64", ScalarSigned, 64, false), LayoutIncomplete, "alignment of a 8-byte scalar"},
		"invalid width":       {complete, scalar("int7", ScalarSigned, 7, false), LayoutInvalid, "not a whole number of bytes"},
		"invalid identity":    {complete, LayoutRequirement{Kind: LayoutScalar, Scalar: ScalarSigned, WidthBits: 8}, LayoutInvalid, "semantic type identity"},
		"unsupported decimal": {complete, scalar("decimal", ScalarDecimal, 0, false), LayoutUnsupported, "P24-17"},
		"unsupported struct":  {complete, LayoutRequirement{TypeIdentity: "main::Pair", Kind: LayoutStruct}, LayoutUnsupported, "P24-19"},
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
	policy.References = ResolvedReferencePolicy{LogicalEpochBits: 32, ShortWidthProof: "proof"}
	rPolicy, _ := NewResolver(policy, testAlignments())
	if _, hit := cache.Lookup(rPolicy.Key(), "int", ""); hit {
		t.Error("a changed reference policy reused a cached layout")
	}
	aligned := testAlignments()
	aligned[8] = 4
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

// P24-16: scalar layouts keep their source kind; fixed widths, byte/char,
// target-sized int/uint and plain float (plan width) and fixed floats are
// resolved, bool and rune stay incomplete until MD-141 decides their storage
// size, and every alignment comes from the plan's alignment facts.
// Rules: rules/memory/layout.md — §§ 18, 18(4a), 18(5); rules/types/types.md — §§ 9.2–9.4.
func TestScalarLayouts(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		resolver, _ := NewResolver(layoutTestPlan(width), testAlignments())
		word := uint64(width / 8)
		for _, check := range []struct {
			requirement LayoutRequirement
			size        uint64
		}{
			{scalar("int8", ScalarSigned, 8, false), 1}, {scalar("int16", ScalarSigned, 16, false), 2},
			{scalar("int32", ScalarSigned, 32, false), 4}, {scalar("int64", ScalarSigned, 64, false), 8},
			{scalar("int128", ScalarSigned, 128, false), 16}, {scalar("int256", ScalarSigned, 256, false), 32},
			{scalar("uint8", ScalarUnsigned, 8, false), 1}, {scalar("uint256", ScalarUnsigned, 256, false), 32},
			{scalar("byte", ScalarByte, 8, false), 1}, {scalar("char", ScalarChar, 8, false), 1},
			{scalar("int", ScalarSigned, 0, true), word}, {scalar("uint", ScalarUnsigned, 0, true), word},
			{scalar("float", ScalarFloat, 0, true), word}, {scalar("float32", ScalarFloat, 32, false), 4},
			{scalar("float64", ScalarFloat, 64, false), 8},
		} {
			layout := resolver.Resolve(check.requirement)
			if !layout.Complete() || layout.SizeBytes.Uint64() != check.size || layout.AlignBytes.Uint64() != testAlignments()[check.size] || layout.Scalar != check.requirement.Scalar {
				t.Errorf("%d-bit %s = %s size=%v align=%v %v", width, check.requirement.TypeIdentity, layout.Completeness, layout.SizeBytes, layout.AlignBytes, layout.Missing)
			}
		}
		for _, check := range []struct {
			requirement LayoutRequirement
			validity    string
		}{
			{scalar("bool", ScalarBool, 0, false), "false or true only"},
			{scalar("rune", ScalarRune, 0, false), "Unicode scalar values only"},
		} {
			layout := resolver.Resolve(check.requirement)
			if layout.Completeness != LayoutIncomplete || layout.SizeBytes != nil || !strings.Contains(strings.Join(layout.Missing, ""), "MD-141") || !strings.Contains(layout.ValidBitPattern, check.validity) {
				t.Errorf("%s = %+v, want incomplete pending MD-141 with its validity predicate", check.requirement.TypeIdentity, layout)
			}
		}
	}
	unresolved, _ := NewResolver(layoutTestPlan(64), nil)
	if layout := unresolved.Resolve(scalar("int32", ScalarSigned, 32, false)); layout.Complete() || layout.AlignBytes != nil {
		t.Errorf("a scalar alignment was guessed without plan facts: %+v", layout)
	}
}
