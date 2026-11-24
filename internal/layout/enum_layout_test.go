package layout

import (
	"math/big"
	"testing"
)

// Enum layouts and cached graphs retain their carrier and aliases, reject
// malformed/out-of-range values and propagate absent target alignment facts.
// Rules: rules/memory/layout.md — §§14,26,31; rules/declarations/enums.md — §14.1.
func TestIntegerEnumLayout(t *testing.T) {
	carrier := scalar("int8", ScalarSigned, 8, false)
	req := LayoutRequirement{TypeIdentity: "main::Status", Kind: LayoutEnum, Carrier: &carrier, EnumMembers: []EnumMemberRequirement{{Name: "Low", Value: big.NewInt(-128)}, {Name: "High", Value: big.NewInt(127)}, {Name: "Alias", Value: big.NewInt(127)}}}
	cache := NewLayoutCache()
	for _, width := range []uint16{32, 64} {
		for attempt := 0; attempt < 2; attempt++ {
			r, err := NewResolverWithCache(layoutTestPlan(width), testAlignments(), cache)
			if err != nil {
				t.Fatal(err)
			}
			resolved := r.Resolve(req)
			if !resolved.Complete() || resolved.Carrier == 0 || resolved.SizeBytes.Int64() != 1 || resolved.AlignBytes.Int64() != 1 || len(resolved.Variants) != 3 || resolved.ValidBitPattern != "closed enum values {-128,127}" {
				t.Fatalf("resolved %+v", resolved)
			}
			resolved.Variants[0].Tag.SetInt64(0)
			if again, _ := r.Layout(resolved.ID); again.Variants[0].Tag.Int64() != -128 {
				t.Fatal("mutable enum snapshot")
			}
		}
	}
	for _, value := range []*big.Int{nil, big.NewInt(-129), big.NewInt(128)} {
		bad := req
		bad.EnumMembers = []EnumMemberRequirement{{Name: "Bad", Value: value}}
		r, _ := NewResolver(layoutTestPlan(64), testAlignments())
		if resolved := r.Resolve(bad); resolved.Completeness != LayoutInvalid || resolved.SizeBytes != nil {
			t.Fatalf("invalid enum %+v", resolved)
		}
	}
	if cache.Hits() == 0 {
		t.Fatal("enum graph did not reuse a warm cache")
	}
	r, _ := NewResolver(layoutTestPlan(64), nil)
	if resolved := r.Resolve(req); resolved.Completeness != LayoutIncomplete || resolved.SizeBytes != nil {
		t.Fatalf("missing alignment %+v", resolved)
	}
}
