package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/layout"
)

// Generic source types feed the canonical layout cache with their concrete
// identity and plan. A warm snapshot equals cold resolution even when local IDs
// start elsewhere; target, substitution and declaration changes never reuse it.
// Rules: rules/memory/layout.md §§3(5),26,43(5),44(3);
// rules/mlir/packages/sec-mlir-dialect_package24.md §9.
func TestGenericLayoutCache(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_layout_cache.sec")
	if err != nil {
		t.Fatal(err)
	}
	cache := layout.NewLayoutCache()
	alignments := layout.ScalarAlignments{
		{Class: layout.IntegerStorage, WidthBits: 8}:  1,
		{Class: layout.IntegerStorage, WidthBits: 32}: 4,
		{Class: layout.IntegerStorage, WidthBits: 64}: 8,
	}
	for _, bits := range []uint16{64, 32} {
		a, errors := analyzeArraySourceWithWidth(t, string(data), bits)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		plan := layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: bits, Endianness: layout.LittleEndian}
		newResolver := func(shared *layout.LayoutCache) *layout.Resolver {
			r, err := layout.NewResolverWithCache(plan, alignments, shared)
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
		for index, parameter := range a.functions["Shapes"][0].Parameters {
			req := LayoutRequirementOf(parameter.Type)
			before := cache.Hits()
			cold := newResolver(nil)
			expected := cold.Resolve(req)
			wantSize := []int64{32, 4, 16}[index]
			if bits == 32 && index == 0 {
				wantSize = 16
			}
			if !expected.Complete() || expected.SizeBytes.Int64() != wantSize {
				t.Fatalf("native generic layout size = %v, want %d", expected.SizeBytes, wantSize)
			}
			producer := newResolver(cache)
			got := producer.Resolve(req)
			if !got.Complete() || cache.Hits() != before || !reflect.DeepEqual(expected, got) {
				t.Fatalf("cold %d/%d: %+v", bits, index, got)
			}
			warm := newResolver(cache)
			// Occupy ID 1 so cached field, carrier and element IDs must rebase.
			warm.Resolve(LayoutRequirementOf(builtinType("int64")))
			before = cache.Hits()
			got = warm.Resolve(req)
			if cache.Hits() != before+1 || got.TypeIdentity != expected.TypeIdentity || got.SizeBytes.Cmp(expected.SizeBytes) != 0 || got.AlignBytes.Cmp(expected.AlignBytes) != 0 {
				t.Fatalf("warm %d/%d: %+v", bits, index, got)
			}
			for fieldIndex, field := range got.Fields {
				child, ok := warm.Layout(field.Layout)
				want, _ := cold.Layout(expected.Fields[fieldIndex].Layout)
				if !ok || child.TypeIdentity != want.TypeIdentity {
					t.Fatal("cached field refers to foreign snapshot ID", field, child)
				}
				if child.Element != 0 {
					element, ok := warm.Layout(child.Element)
					wantElement, _ := cold.Layout(want.Element)
					if !ok || element.TypeIdentity != wantElement.TypeIdentity {
						t.Fatal("array element ID not rebased")
					}
				}
				if child.Carrier != 0 {
					carrier, ok := warm.Layout(child.Carrier)
					wantCarrier, _ := cold.Layout(want.Carrier)
					if !ok || carrier.TypeIdentity != wantCarrier.TypeIdentity {
						t.Fatal("nominal carrier ID not rebased")
					}
				}
			}
			// Published values cannot mutate a cached result.
			got.Fields[0].Offset.SetInt64(999)
			got.SizeBytes.SetInt64(999)
			fresh := newResolver(cache).Resolve(req)
			if !reflect.DeepEqual(fresh, expected) {
				t.Fatal("cache result was not detached")
			}
		}
		changed := strings.Replace(string(data), "    value: T,\n    items: T[2],\n    flag: bool,", "    flag: bool,\n    items: T[2],\n    value: T,", 1)
		b, errors := analyzeArraySourceWithWidth(t, changed, bits)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		req := LayoutRequirementOf(b.functions["Shapes"][0].Parameters[0].Type)
		before := cache.Hits()
		got := newResolver(cache).Resolve(req)
		expected := newResolver(nil).Resolve(req)
		if cache.Hits() != before || !reflect.DeepEqual(got, expected) || got.Fields[0].Name != "flag" {
			t.Fatal("changed declaration reused old layout")
		}
	}
}
