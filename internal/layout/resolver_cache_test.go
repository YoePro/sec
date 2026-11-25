package layout

import (
	"math/big"
	"reflect"
	"sync"
	"testing"
)

// Complete generic graphs reuse only exact plan/type/requirement keys.
// Each mismatch recomputes identically to a cold resolver; corrupted graphs
// publish no partial nodes, and incomplete results never poison later queries.
// Rules: rules/memory/layout.md §§3(5),26(2–5),43(5),44(3);
// rules/mlir/packages/sec-mlir-dialect_package24.md §9.
func TestResolverCacheIsolation(t *testing.T) {
	requirement := LayoutRequirement{TypeIdentity: "main::Box[int]", Kind: LayoutStruct, Fields: []FieldRequirement{
		{Name: "value", Requirement: scalar("int", ScalarSigned, 0, true)},
		{Name: "flag", Requirement: scalar("bool", ScalarBool, 0, false)},
	}}
	cache := NewLayoutCache()
	resolve := func(plan ResolvedScalarPlan, align ScalarAlignments, req LayoutRequirement, shared *LayoutCache) (*Resolver, ResolvedLayout) {
		r, err := NewResolverWithCache(plan, align, shared)
		if err != nil {
			t.Fatal(err)
		}
		return r, r.Resolve(req)
	}
	base := layoutTestPlan(64)
	producer, expected := resolve(base, testAlignments(), requirement, cache)
	for _, change := range []string{"width", "endian", "target", "abi", "profile", "reference", "address", "alignment", "substitution", "fields", "contract"} {
		t.Run(change, func(t *testing.T) {
			plan, align, req := base, testAlignments(), requirement
			req.Fields = append([]FieldRequirement(nil), req.Fields...)
			switch change {
			case "width":
				plan.PointerWidthBits = 32
			case "endian":
				plan.Endianness = BigEndian
			case "target":
				plan.TargetArch = "another"
				plan.LLVMTriple = "another-unknown-linux"
			case "abi":
				plan.ABI = "other"
			case "profile":
				plan.Profile = "freestanding"
			case "reference":
				plan.References = ResolvedReferencePolicy{LogicalEpochBits: 32, ShortWidthProof: "proof", ArenaShortWidthSafety: ArenaEpochCheckedAdvance}
			case "address":
				plan.AddressDomains = []AddressDomain{testAddressDomain()}
			case "alignment":
				align[StorageRepresentation{IntegerStorage, 64}] = 4
			case "substitution":
				req.TypeIdentity = "main::Box[byte]"
				req.Fields[0].Requirement = scalar("byte", ScalarByte, 8, false)
			case "fields":
				req.Fields[0], req.Fields[1] = req.Fields[1], req.Fields[0]
			case "contract":
				req.Contract = "pack(1)"
			}
			before := cache.Hits()
			_, got := resolve(plan, align, req, cache)
			_, cold := resolve(plan, align, req, nil)
			if cache.Hits() != before || !reflect.DeepEqual(got, cold) {
				t.Fatalf("incompatible cache reused: %+v", got)
			}
		})
	}
	for _, corrupt := range []string{"root", "child", "plan", "size"} {
		t.Run("corrupt-"+corrupt, func(t *testing.T) {
			local := NewLayoutCache()
			r, _ := resolve(base, testAlignments(), requirement, local)
			key, _ := r.graphKey(requirement)
			graph := local.graphs[key]
			root := graph.layouts[graph.root]
			switch corrupt {
			case "root":
				graph.root = 999999
			case "child":
				root.Fields[0].Layout = 999999
			case "plan":
				root.PlanKey.PointerWidthBits = 32
			case "size":
				root.SizeBytes = big.NewInt(999)
			}
			graph.layouts[root.ID] = root
			local.graphs[key] = graph
			r, got := resolve(base, testAlignments(), requirement, local)
			if local.Hits() != 0 || local.Rejected != 1 || !reflect.DeepEqual(got, expected) || len(r.Layouts()) != len(producer.Layouts()) {
				t.Fatalf("malformed graph trusted or partially installed: %+v", got)
			}
		})
	}
	incomplete := NewLayoutCache()
	_, got := resolve(base, nil, requirement, incomplete)
	if got.Complete() || len(incomplete.graphs) != 0 {
		t.Fatal("incomplete graph cached")
	}
	_, got = resolve(base, testAlignments(), requirement, incomplete)
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("missing alignment poisoned cache")
	}
}

// Independent compiler snapshots may share an immutable graph cache without
// racing or aliasing each other's published layouts.
// Rules: rules/memory/layout.md §§26(5),39,43(5).
func TestResolverCacheConcurrentSnapshots(t *testing.T) {
	cache := NewLayoutCache()
	req := scalar("int", ScalarSigned, 0, true)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := NewResolverWithCache(layoutTestPlan(64), testAlignments(), cache)
			if err != nil {
				t.Error(err)
				return
			}
			for n := 0; n < 20; n++ {
				got := r.Resolve(req)
				if !got.Complete() || got.SizeBytes.Int64() != 8 {
					t.Error("wrong shared layout")
					return
				}
				got.SizeBytes.SetInt64(999)
			}
		}()
	}
	wg.Wait()
}
