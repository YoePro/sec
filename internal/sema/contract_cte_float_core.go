package sema

import (
	"math"

	"sec/internal/ast"
)

// floatCoreCallConstant folds a total float core method on compile-time
// operands of one binary format, with the runtime float.* semantics: Abs keeps
// NaN and clears the sign; Min/Max propagate NaN and order -0 below +0;
// classification uses the format's smallest normal; Floor, Ceiling, Truncate
// and Round (ties to even) keep signed zero, infinities and NaN. Clamp yields
// a Result and stays executed code.
// Rules: rules/library/core-library.md — § 9.3;
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 29–36, 82.
func (a *Analyzer) floatCoreCallConstant(call *ast.CallExpression, visiting map[string]bool, identifier compileTimeIdentifierResolver, expression compileTimeExpressionResolver) (DefaultConstant, compileTimeOutcome, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return DefaultConstant{}, compileTimeNotConstant, false
	}
	site := member.Property.Token
	known, ok := a.CompilerKnownMemberAt(site.File, site.Line, site.Column)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant, false
	}
	if _, total := floatCoreMethodKinds[known.ID]; !total {
		return DefaultConstant{}, compileTimeNotConstant, false
	}
	receiver, outcome := a.semanticCompileTimeConstantResolving(member.Object, visiting, Type{}, identifier, expression)
	if outcome != compileTimeEvaluated {
		return DefaultConstant{}, outcome, true
	}
	bits := receiver.FloatBits
	if bits != 32 && bits != 64 || receiver.DecimalBits != 0 {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	x, ok := compileTimeFloatOperand(receiver, bits)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	operands := []float64{x}
	for _, argument := range call.Arguments {
		value, outcome := a.semanticCompileTimeConstantResolving(argument, visiting, Type{Kind: FloatType, FloatBits: bits}, identifier, expression)
		if outcome != compileTimeEvaluated {
			return DefaultConstant{}, outcome, true
		}
		if value.FloatBits != 0 && value.FloatBits != bits {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		number, ok := compileTimeFloatOperand(value, bits)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		operands = append(operands, number)
	}
	if len(operands) != floatCoreMethods[known.Name]+1 {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	switch kind := floatCoreMethodKinds[known.ID]; kind {
	case "abs":
		return compileTimeFloatOrOutcome(math.Abs(x), bits)
	case "min":
		return compileTimeFloatOrOutcome(math.Min(x, operands[1]), bits)
	case "max":
		return compileTimeFloatOrOutcome(math.Max(x, operands[1]), bits)
	case "floor":
		return compileTimeFloatOrOutcome(math.Floor(x), bits)
	case "ceiling":
		return compileTimeFloatOrOutcome(math.Ceil(x), bits)
	case "truncate":
		return compileTimeFloatOrOutcome(math.Trunc(x), bits)
	case "nearest-even":
		return compileTimeFloatOrOutcome(math.RoundToEven(x), bits)
	default:
		truth := floatClassification(kind, x, bits)
		return DefaultConstant{Kind: BoolType, Bool: truth, Lexeme: map[bool]string{true: "true", false: "false"}[truth]}, compileTimeEvaluated, true
	}
}

// floatCoreMethodKinds names the total float core methods CTE folds.
var floatCoreMethodKinds = map[string]string{
	"CKM-FLOAT-ABS": "abs", "CKM-FLOAT-MIN": "min", "CKM-FLOAT-MAX": "max",
	"CKM-FLOAT-ISFINITE": "finite", "CKM-FLOAT-ISINFINITE": "infinite", "CKM-FLOAT-ISNAN": "nan",
	"CKM-FLOAT-ISNORMAL": "normal", "CKM-FLOAT-ISSUBNORMAL": "subnormal",
	"CKM-FLOAT-FLOOR": "floor", "CKM-FLOAT-CEILING": "ceiling", "CKM-FLOAT-TRUNCATE": "truncate",
	"CKM-FLOAT-ROUND": "nearest-even",
}

// compileTimeFloatOrOutcome wraps one folded float result as a handled value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 20, 82.
func compileTimeFloatOrOutcome(value float64, bits int) (DefaultConstant, compileTimeOutcome, bool) {
	result, outcome := compileTimeFloatValue(value, bits)
	return result, outcome, true
}

// floatClassification is the IEEE class test in binary32 or binary64.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 31–32.
func floatClassification(kind string, x float64, bits int) bool {
	smallestNormal := 0x1p-1022
	if bits == 32 {
		smallestNormal = 0x1p-126
	}
	finite := !math.IsInf(x, 0) && !math.IsNaN(x)
	switch kind {
	case "finite":
		return finite
	case "infinite":
		return math.IsInf(x, 0)
	case "nan":
		return math.IsNaN(x)
	case "normal":
		return finite && math.Abs(x) >= smallestNormal
	case "subnormal":
		return finite && x != 0 && math.Abs(x) < smallestNormal
	}
	return false
}
