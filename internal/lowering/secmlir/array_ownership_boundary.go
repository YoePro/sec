package secmlir

import "sec/internal/ir/semantic"

// validateArrayOwnershipSchema keeps a represented P17 array from being
// emitted with predecessor array metadata until schema-13 ownership exists.
// This also covers a factory that returns an owned array without local cleanup
// operations, so absence of own.* operations cannot erase its semantics.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 65–67, 79;
// rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — ownership actions.
func validateArrayOwnershipSchema(module *semantic.Module) error {
	for _, function := range module.Functions {
		var walk func([]*semantic.Block) error
		walk = func(blocks []*semantic.Block) error {
			for _, block := range blocks {
				for _, op := range block.Operations {
					if semantic.ArrayConstructionNeedsOwnership(module, op) {
						return &UnsupportedLoweringError{Feature: "non-trivial fixed-array ownership requires schema 13", Function: function.ID}
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
		if err := walk(function.Blocks); err != nil {
			return err
		}
	}
	return nil
}
