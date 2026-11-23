package sema

import (
	"math"
	"math/big"

	"sec/internal/ast"
)

// numericConversionConstant folds a typed primitive conversion whose recorded
// P20 plan Sema already proved exact; it never reclassifies the conversion.
// Unproven or known-failing conversions stay with the checked-conversion
// diagnostics, and an unresolved platform width is never guessed.
// Float results keep IEEE categories and the sign of zero; float-to-bool is
// the `une 0` predicate, so NaN converts to true.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20,42–55,59–60;
// rules/types/types.md — §25(3–5,9–11).
func (a *Analyzer) numericConversionConstant(call *ast.CallExpression, visiting map[string]bool, identifier compileTimeIdentifierResolver, expression compileTimeExpressionResolver) (DefaultConstant, compileTimeOutcome, bool) {
	plan, ok := a.resolvedNumericConversionPlans[call]
	if !ok || !plan.ProvenExact || len(call.Arguments) != 1 {
		return DefaultConstant{}, compileTimeNotConstant, false
	}
	if plan.SourceType.Kind == FloatType && plan.SourceBits != 32 && plan.SourceBits != 64 ||
		plan.TargetType.Kind == FloatType && plan.TargetBits != 32 && plan.TargetBits != 64 {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	source := plan.SourceType
	if source.Kind == FloatType {
		source.FloatBits = plan.SourceBits
	}
	value, outcome := a.semanticCompileTimeConstantResolving(call.Arguments[0], visiting, source, identifier, expression)
	if outcome != compileTimeEvaluated {
		return DefaultConstant{}, outcome, true
	}
	if source.Kind == FloatType {
		if value.FloatBits != 0 && value.FloatBits != plan.SourceBits {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		number, ok := compileTimeFloatOperand(value, plan.SourceBits)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		result, outcome := floatConversionConstant(number, plan)
		return result, outcome, true
	}
	if value.Integer == nil || plan.TargetType.Kind != FloatType {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	exact := new(big.Rat).SetInt(value.Integer)
	if !ratExactInFloat(exact, plan.TargetBits) {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	number, _ := exact.Float64()
	result, outcome := compileTimeFloatValue(number, plan.TargetBits)
	return result, outcome, true
}

// floatConversionConstant converts one binary-float value to the plan target.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§20,44–55.
func floatConversionConstant(number float64, plan ResolvedNumericConversionPlan) (DefaultConstant, compileTimeOutcome) {
	switch plan.TargetType.Kind {
	case FloatType:
		converted := number
		if plan.TargetBits == 32 {
			converted = float64(float32(number))
		}
		if converted != number && !math.IsNaN(number) {
			return DefaultConstant{}, compileTimeNotConstant
		}
		return compileTimeFloatValue(converted, plan.TargetBits)
	case BoolType:
		truth := number != 0
		lexeme := "false"
		if truth {
			lexeme = "true"
		}
		return DefaultConstant{Kind: BoolType, Bool: truth, Lexeme: lexeme}, compileTimeEvaluated
	case IntType, UintType:
		if math.IsInf(number, 0) || math.IsNaN(number) {
			return DefaultConstant{}, compileTimeNotConstant
		}
		exact := new(big.Rat).SetFloat64(number)
		if exact == nil || finiteNumericConversionFailure(plan.TargetType, exact) != "" {
			return DefaultConstant{}, compileTimeNotConstant
		}
		integer := new(big.Int).Set(exact.Num())
		return DefaultConstant{Kind: IntType, Lexeme: integer.String(), Integer: integer}, compileTimeEvaluated
	}
	return DefaultConstant{}, compileTimeNotConstant
}
