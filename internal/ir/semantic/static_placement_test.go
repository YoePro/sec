package semantic

import (
	"strings"
	"testing"
)

// Static roots without any physical placement representation must stay in
// ordinary memory. Consistent forged MMIO/target facts are not sufficient:
// storage duration never grants the separate hardware/address authority.
// Rules: rules/declarations/static.md — §22;
// rules/compiler/semantic_ir.md — §28; rules/memory/storage.md — §16.
func TestStaticPlaceRootsCannotAcquirePhysicalSpace(t *testing.T) {
	for _, space := range []AddressSpace{AddressSpaceMMIO, "target:device"} {
		program, a := sliceBoundaryInput(t, "semantic_ir/static_place_roots.sec")
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 15})
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					op := &block.Operations[i]
					if op.Kind != OpPlaceStaticRoot {
						continue
					}
					count++
					if op.Place.AddressSpace != AddressSpaceOrdinary {
						t.Fatalf("source static selected a physical space: %+v", op.Place)
					}
					op.Place.AddressSpace = space
				}
			}
		}
		if count == 0 {
			t.Fatal("missing static roots")
		}
		if err := verifyStaticPlaceIdentities(module); err == nil || !strings.Contains(err.Error(), "ordinary address space") {
			t.Fatalf("accepted physical space %s: %v", space, err)
		}
		if err := Verify(module); err == nil {
			t.Fatalf("module accepted forged space %s", space)
		}
	}
}
