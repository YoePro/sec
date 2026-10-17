package semantic

import (
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
	if !conversion.Exact {
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
	return fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, Location: location(call.Token)}, targetType), nil
}
