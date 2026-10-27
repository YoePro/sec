package semantic

import "fmt"

// verifyStaticPlaceIdentities checks the independent StaticID, PlaceRootID and
// storage identity relationships across all represented functions/regions.
// Repeated roots of one static keep its storage/address space; distinct static
// declarations must not claim the same storage or Place-root identity.
// Rules: rules/compiler/semantic_ir.md — §10(12), §28(6);
// rules/mlir/sec_mlir_dialect.md — v11.41.3 (MD-116 §§2,4(6),7).
func verifyStaticPlaceIdentities(module *Module) error {
	type facts struct {
		static  StaticID
		storage uint32
		space   AddressSpace
	}
	statics := map[StaticID]facts{}
	storages := map[uint32]StaticID{}
	roots := map[uint32]StaticID{}
	var walk func([]*Block) error
	walk = func(blocks []*Block) error {
		for _, block := range blocks {
			for _, op := range block.Operations {
				if op.Kind == OpPlaceStaticRoot && op.Place != nil {
					p := op.Place
					f := facts{op.Static, p.Storage, p.AddressSpace}
					if prior, ok := statics[op.Static]; ok && prior != f {
						return fmt.Errorf("static Place roots of @%d have conflicting storage identity or address space", op.Static)
					}
					if prior, ok := storages[p.Storage]; ok && prior != op.Static {
						return fmt.Errorf("distinct static Place roots share storage identity %d", p.Storage)
					}
					if prior, ok := roots[p.Root]; ok && prior != op.Static {
						return fmt.Errorf("distinct static Place roots share root identity %d", p.Root)
					}
					statics[op.Static], storages[p.Storage], roots[p.Root] = f, op.Static, op.Static
				}
				if op.Region != nil {
					if err := walk(op.Region.Blocks); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, fn := range module.Functions {
		if err := walk(fn.Blocks); err != nil {
			return err
		}
	}
	return nil
}
