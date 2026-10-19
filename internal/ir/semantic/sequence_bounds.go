package semantic

// validSequenceBoundsFailureBlock accepts either the ordinary terminating bounds
// endpoint or typed IndexError.OutOfBounds propagation/handler flow. Rules:
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 48-54;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§53,81–83.
func validSequenceBoundsFailureBlock(module *Module, block *Block, values map[ValueID]Value, operation string) bool {
	if block == nil || len(block.Operations) == 0 {
		return false
	}
	if len(block.Operations) == 1 {
		failure := block.Operations[0]
		return failure.Kind == OpBoundsFailure && failure.ArrayOperation == operation
	}
	constant := block.Operations[0]
	if constant.Kind != OpEnumConstant || len(constant.Results) != 1 {
		return false
	}
	definition, ok := enumDefinition(module, constant.Results[0].Type)
	if !ok || definition.SymbolID != "core::IndexError" || int(constant.EnumCase) >= len(definition.Cases) || definition.Cases[constant.EnumCase].Name != "OutOfBounds" {
		return false
	}
	errorValue := constant.Results[0].ID
	construct := block.Operations[1]
	if construct.Kind != OpResultErr {
		// A locally handled fallible bounds edge may execute the selected handler
		// directly after materializing the sole concrete IndexError variant. The
		// shared try-handler verifier validates its dispatch/merge metadata.
		for _, operation := range block.Operations[1:] {
			if operation.Kind == OpBoundsFailure {
				return false
			}
		}
		return block.Operations[len(block.Operations)-1].IsTerminator()
	}
	if len(construct.Operands) != 1 || construct.Operands[0] != errorValue || len(construct.Results) != 1 {
		return false
	}
	result, ok := module.Types.Lookup(construct.Results[0].Type)
	if !ok || result.Kind != TypeResult || result.Error != values[errorValue].Type {
		return false
	}
	last := block.Operations[len(block.Operations)-1]
	return len(block.Operations) == 3 && last.Kind == OpReturn && len(last.Operands) == 1 && last.Operands[0] == construct.Results[0].ID
}
