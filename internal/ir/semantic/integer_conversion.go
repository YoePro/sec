package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildIntegerConversion lowers an explicit built-in integer conversion that
// Sema proved exact to one numeric.convert-exact operation; the argument is
// built in its own source type, so no implicit shaping hides a narrowing.
// A conversion that can fail needs the checked form, its ConversionError
// channel and `try`, which are not lowered yet.
//
// Rules:
//   - rules/types/types.md — "Explicit conversions"
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 44, 60–63
func (fb *functionBuilder) buildIntegerConversion(call *ast.CallExpression, conversion sema.ResolvedIntegerConversion) (builtValue, error) {
	if !conversion.Exact || !conversion.Proof.Proven() {
		return builtValue{}, fb.unsupported("checked integer conversion "+conversion.SourceType.Name+" to "+conversion.TargetType.Name+" (needs try)", call.Token)
	}
	sourceType, err := fb.owner.internType(conversion.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(conversion.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	value, err := fb.buildExpr(call.Arguments[0], sourceType)
	if err != nil {
		return builtValue{}, err
	}
	if value.typ == targetType {
		return value, nil
	}
	return fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, ConversionProof: conversionProofOf(conversion.Proof), Location: location(call.Token)}, targetType), nil
}

// conversionProofOf carries Sema's proof into Semantic IR unchanged.
func conversionProofOf(proof sema.IntegerConversionProof) *ConversionProof {
	return &ConversionProof{Basis: string(proof.Basis), PlanPointerWidthBits: proof.PlanPointerWidthBits}
}

// compilerWideningProof is the proof of the builder's own lossless widening
// into a type wider than every source domain on every plan (for-loop range
// distances into int128/int256).
var compilerWideningProof = ConversionProof{Basis: "compiler-widening"}

// validConversionProof checks a proof's basis and plan compatibility.
func validConversionProof(module *Module, proof *ConversionProof) error {
	if proof == nil {
		return fmt.Errorf("missing exactness proof")
	}
	switch proof.Basis {
	case string(sema.IntegerConversionConstant), string(sema.IntegerConversionSourceDomain),
		string(sema.IntegerConversionClosedEnumDomain), string(sema.IntegerConversionOpenEnumDomain), compilerWideningProof.Basis,
		registerFieldIntervalProof.Basis:
	default:
		return fmt.Errorf("unknown exactness proof basis %q", proof.Basis)
	}
	if proof.PlanPointerWidthBits != 0 && proof.PlanPointerWidthBits != module.PointerWidthBits {
		return fmt.Errorf("exactness proof made for a %d-bit plan, module plan is %d-bit", proof.PlanPointerWidthBits, module.PointerWidthBits)
	}
	return nil
}
