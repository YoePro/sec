package secmlir

import "sec/internal/ir/semantic"

// rejectUnverifiedVolatilePlans keeps effectful physical reads/writes from
// becoming ordinary load/store or ownership operations. No output is published
// until a verified region/access plan and semantics-preserving lowering exist.
// Rules: rules/platform/volatile.md §§9–12; rules/memory/copy_move.md §19;
// rules/compiler/semantic_ir.md §§47,85.
func rejectUnverifiedVolatilePlans(module *semantic.Module) error {
	var inspect func([]*semantic.Block, semantic.FunctionID) error
	inspect = func(blocks []*semantic.Block, function semantic.FunctionID) error {
		for _, block := range blocks {
			for _, op := range block.Operations {
				if op.Kind == semantic.OpVolatileRead || op.Kind == semantic.OpVolatileWrite {
					return &UnsupportedLoweringError{Feature: string(op.Kind) + " needs a verified physical volatile access plan", Function: function}
				}
				if op.Region != nil {
					if err := inspect(op.Region.Blocks, function); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, function := range module.Functions {
		if err := inspect(function.Blocks, function.ID); err != nil {
			return err
		}
	}
	return nil
}
