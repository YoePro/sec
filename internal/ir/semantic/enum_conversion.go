package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildEnumConversion consumes Sema's exact-conversion proof and fallibility.
// Potentially failing enum-to-integer conversions cannot become unconditional
// enum.to-integer, even if inconsistent facts also supply a proven bound. The
// checked path extracts the unchanged carrier before numeric failure routing.
// Historical package scopes stop before operand evaluation or IR allocation.
// Rules: rules/declarations/enums.md — §12.1;
// rules/types/types.md — "Exact integer conversion proof";
// rules/compiler/semantic_ir.md — §124.
func (fb *functionBuilder) buildEnumConversion(call *ast.CallExpression, conversion sema.ResolvedEnumConversion, resultType TypeID) (builtValue, error) {
	if len(call.Arguments) != 1 {
		return builtValue{}, fb.unsupported("enum conversion arity", call.Token)
	}
	if conversion.Kind == sema.ResolvedEnumToInteger {
		if conversion.Fallible {
			if fb.owner.maxPackage < 20 {
				return builtValue{}, fb.unsupported("fallible enum to integer conversion "+conversion.EnumType.Name+" to "+conversion.IntegerType.Name+" (needs checked failure control flow)", call.Token)
			}
			return fb.buildCheckedEnumConversion(call, conversion)
		}
		if !conversion.Proof.Proven() {
			return builtValue{}, fb.unsupported("enum to integer conversion "+conversion.EnumType.Name+" to "+conversion.IntegerType.Name+" that is not proven exact (needs a checked conversion)", call.Token)
		}
	}
	operandType, err := fb.owner.internType(conversion.IntegerType)
	if conversion.Kind == sema.ResolvedEnumToInteger {
		operandType, err = fb.owner.internType(conversion.EnumType)
	}
	if err != nil {
		return builtValue{}, err
	}
	operand, err := fb.buildExpr(call.Arguments[0], operandType)
	if err != nil {
		return builtValue{}, err
	}
	kind := OpEnumFromInteger
	var proof *ConversionProof
	if conversion.Kind == sema.ResolvedEnumToInteger {
		kind = OpEnumToInteger
		proof = conversionProofOf(conversion.Proof)
	}
	return fb.result(Operation{Kind: kind, Operands: []ValueID{operand.id}, ConversionProof: proof, Location: location(call.Token)}, resultType), nil
}
