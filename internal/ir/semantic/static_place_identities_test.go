package semantic

import (
	"strings"
	"testing"
)

// MD-116 §4(6)/§9: repeated roots across functions preserve static storage;
// forged storage/root/address-space relationships fail the module verifier.
func TestStaticPlaceIdentityConsistency(t *testing.T) {
	for _, name := range []string{"storage", "space", "alias", "root"} {
		t.Run(name, func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/static_place_roots.sec")
			module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 15})
			if err != nil {
				t.Fatal(err)
			}
			var first, other, repeated *Operation
			for _, fn := range module.Functions {
				for _, block := range fn.Blocks {
					for i := range block.Operations {
						op := &block.Operations[i]
						if op.Kind != OpPlaceStaticRoot {
							continue
						}
						if first == nil {
							first = op
						} else if op.Static == first.Static {
							repeated = op
						} else {
							other = op
						}
					}
				}
			}
			if first == nil || other == nil || repeated == nil {
				t.Fatal("fixture lacks identity relationships")
			}
			switch name {
			case "storage":
				repeated.Place.Storage = first.Place.Storage + 10000
			case "space":
				repeated.Place.AddressSpace = AddressSpaceMMIO
			case "alias":
				other.Place.Storage = first.Place.Storage
			case "root":
				other.Place.Root = first.Place.Root
			}
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "static Place roots") {
				t.Fatalf("accepted forged %s: %v", name, err)
			}
		})
	}
}
