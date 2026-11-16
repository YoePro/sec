package sema

import (
	"math/big"

	"sec/internal/ast"
)

// numericFloatConstant preserves finite values separately from IEEE categories;
// no rational, integer sentinel or host conversion can represent NaN/infinity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§47–55,59.
type numericFloatConstant struct {
	value   *big.Rat
	special string
}

// numericFloatConstantOf reads immutable, resolved primitive floating facts.
// Mutable/volatile bindings never inherit a constant proof from an initializer.
// Rules: rules/types/types.md — §25(7); rules/mlir/packages/sec-mlir-dialect_package20.md — §§47–55,59.
func (a *Analyzer) numericFloatConstantOf(expr ast.Expression, typ Type) (numericFloatConstant, bool) {
	if typ.Kind != FloatType {
		return numericFloatConstant{}, false
	}
	switch node := expr.(type) {
	case *ast.CallExpression:
		// A literal-shaped constructor already has its target format; this is
		// nearest-even literal shaping, not a checked typed conversion.
		if len(node.Arguments) == 1 && a.isExplicitConversionExpression(node) && isUntypedNumericExpression(node.Arguments[0]) {
			value, known := a.floatingConstantValue(node.Arguments[0], typ)
			return numericFloatConstant{value: value}, known
		}
		if plan, ok := a.resolvedNumericConversionPlans[node]; ok && plan.ProvenExact && plan.TargetType.Kind == FloatType {
			if plan.Proof.FloatCategory != "" {
				return numericFloatConstant{special: plan.Proof.FloatCategory}, true
			}
			if plan.Proof.Constant != nil {
				return numericFloatConstant{value: new(big.Rat).Set(plan.Proof.Constant)}, true
			}
			if plan.Proof.SourceMin != nil && plan.Proof.SourceMin.Cmp(plan.Proof.SourceMax) == 0 {
				return numericFloatConstant{value: new(big.Rat).SetInt(plan.Proof.SourceMin)}, true
			}
		}
	case *ast.Identifier:
		if symbol, ok := a.symbols.lookup(node.Value); ok && !symbol.Mutable && !symbol.Volatile && symbol.numericFloatSpecial != "" {
			return numericFloatConstant{special: symbol.numericFloatSpecial}, true
		}
	case *ast.PrefixExpression:
		if node.Operator == "+" || node.Operator == "-" {
			if value, known := a.numericFloatConstantOf(node.Right, typ); known {
				if node.Operator == "-" {
					if value.value != nil {
						value.value.Neg(value.value)
					}
					switch value.special {
					case "infinity":
						value.special = "negative-infinity"
					case "negative-infinity":
						value.special = "infinity"
					}
				}
				return value, true
			}
		}
	case *ast.MemberExpression:
		if owner, ok := a.StaticMemberOwnerOf(node); ok && floatCoreReceiver(owner) && node.Property != nil {
			switch node.Property.Value {
			case "NaN":
				return numericFloatConstant{special: "nan"}, true
			case "Infinity":
				return numericFloatConstant{special: "infinity"}, true
			case "NegativeInfinity":
				return numericFloatConstant{special: "negative-infinity"}, true
			}
		}
	}
	value, known := a.floatingConstantValue(expr, typ)
	return numericFloatConstant{value: value}, known
}

// finiteNumericConversionFailure uses exact mathematical bounds, then exact
// representability, in that order. Integer upper bounds are exclusive powers
// of two: converting a float never compares against a rounded integer maximum.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§47–54;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§12–15.
func finiteNumericConversionFailure(target Type, value *big.Rat) ConversionFailureKind {
	if target.Kind == BoolType {
		return ""
	}
	if target.Kind == FloatType {
		precision, exponent, ok := floatFormat(target.FloatBits)
		if !ok {
			return ""
		}
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), exponent), new(big.Int).Lsh(big.NewInt(1), exponent-precision))
		if new(big.Rat).Abs(value).Cmp(new(big.Rat).SetInt(maximum)) > 0 {
			return ConversionFailureOutOfRange
		}
		if !ratExactInFloat(value, target.FloatBits) {
			return ConversionFailurePrecisionLoss
		}
		return ""
	}
	if minimum, maximum, ok := scalarIntegerDomain(target); ok {
		upper := new(big.Int).Add(maximum, big.NewInt(1))
		if value.Cmp(new(big.Rat).SetInt(minimum)) < 0 || value.Cmp(new(big.Rat).SetInt(upper)) >= 0 {
			return ConversionFailureOutOfRange
		}
		if !value.IsInt() {
			return ConversionFailurePrecisionLoss
		}
	}
	return ""
}

// primitiveNumericConstantFailure checks a known primitive value at the
// selected widths. With no plan it reports a guaranteed failure only when all
// possible format/target pairs fail; otherwise the portable runtime check stays.
// Literal shaping remains distinct from conversion of an already typed value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§44–59;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2;
// rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — §§4–5.
func (a *Analyzer) primitiveNumericConstantFailure(target, source Type, expr ast.Expression) (ConversionFailureKind, bool) {
	if !numericConversionPlanType(target) || !numericConversionPlanType(source) ||
		(source.Kind != FloatType && target.Kind != FloatType) || source.Kind == BoolType || isUntypedNumericExpression(expr) {
		return "", false
	}
	// A planless platform float binding has no single rounded finite value.
	if a.ScalarPlanWidthBits() == 0 && source.Name == "float" {
		if value, known := a.numericFloatConstantOf(expr, source); !known || value.special == "" {
			return "", false
		}
	}
	var first ConversionFailureKind
	widths := []uint16{32, 64}
	if a.ScalarPlanWidthBits() != 0 {
		widths = []uint16{a.ScalarPlanWidthBits()}
	}
	for _, width := range widths {
		targetAt, sourceAt := portableDomainType(target, width), portableDomainType(source, width)
		var failure ConversionFailureKind
		if sourceAt.Kind == FloatType {
			value, known := a.numericFloatConstantOf(expr, sourceAt)
			if !known {
				return "", false
			}
			if value.special != "" {
				if targetAt.Kind != FloatType && targetAt.Kind != BoolType {
					failure = ConversionFailureNonFinite
				}
			} else {
				failure = finiteNumericConversionFailure(targetAt, value.value)
			}
		} else {
			value, known := a.integerConstantValue(expr)
			if !known || !a.numericConstantExpressionImmutable(expr) {
				return "", false
			}
			failure = finiteNumericConversionFailure(targetAt, new(big.Rat).SetInt(value))
		}
		if failure == "" {
			return "", true
		}
		if first == "" {
			first = failure
		} else if first != failure {
			return "", false
		}
	}
	return first, true
}

// numericConstantExpressionImmutable prevents a mutable integer's current value
// becoming a compile-time failure diagnostic. Runtime path intervals may still
// establish a safe conversion independently.
// Rules: rules/types/types.md — §25(7); rules/foundations/names_scopes_visibility.md — Lexical scopes.
func (a *Analyzer) numericConstantExpressionImmutable(expr ast.Expression) bool {
	switch node := expr.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols.lookup(node.Value)
		return ok && !symbol.Mutable && !symbol.Volatile
	case *ast.PrefixExpression:
		return a.numericConstantExpressionImmutable(node.Right)
	case *ast.InfixExpression:
		return a.numericConstantExpressionImmutable(node.Left) && a.numericConstantExpressionImmutable(node.Right)
	case *ast.IntegerLiteral:
		return true
	}
	return false
}

// validatePrimitiveNumericConversion rejects a known invalid typed conversion
// even under try, instead of manufacturing a guaranteed runtime error path.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §59;
// rules/types/types.md — §25(7); rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md — §2.
func (a *Analyzer) validatePrimitiveNumericConversion(target, source Type, expr ast.Expression) bool {
	if failure, known := a.primitiveNumericConstantFailure(target, source, expr); known && failure != "" {
		a.addErrorAtToken(expressionToken(expr), "constant conversion from %s to %s fails with ConversionError.%s", typeDisplayName(source), typeDisplayName(target), failure)
		return false
	}
	return true
}
