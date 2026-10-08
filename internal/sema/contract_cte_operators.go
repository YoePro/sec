package sema

import (
	"math/big"
	"strconv"

	"sec/internal/ast"
	"sec/internal/sema/constant"
)

// compileTimeOperatorExpression keeps numeric contextual evaluation ahead of
// the legacy untyped integer literal shortcut.
// Rules: rules/types/types.md — Context shaping; rules/compiler/compile_time_evaluation.md — §2(3).
func compileTimeOperatorExpression(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.InfixExpression, *ast.PrefixExpression:
		return true
	}
	return false
}

// prepareCompileTimeConstant applies the selected scalar width to each CTE
// intermediate value. Exact decimals never pass through binary floating point.
// Rules: rules/types/types.md — Binary floating-point types, Exact decimal types;
// rules/compiler/compile_time_evaluation.md — §§2(3),47(4).
func (a *Analyzer) prepareCompileTimeConstant(value DefaultConstant, context Type) (DefaultConstant, compileTimeOutcome) {
	if value.Integer != nil && (context.Kind == FloatType || context.Kind == DecimalType) {
		value.Exact = new(big.Rat).SetInt(value.Integer)
		value.Integer = nil
	}
	if value.Exact == nil {
		return value, compileTimeEvaluated
	}
	bits := value.FloatBits
	if context.Kind == FloatType {
		bits = context.FloatBits
	}
	if bits != 0 {
		var number float64
		if bits == 32 {
			v, _ := value.Exact.Float32()
			number = float64(v)
		} else {
			number, _ = value.Exact.Float64()
		}
		converted, ok := binaryFloatConstant(number, bits)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant
		}
		converted.FloatBits = bits
		return converted, compileTimeEvaluated
	}
	decimalBits := value.DecimalBits
	if decimalBits == 0 {
		decimalBits = 64
	}
	if context.Kind == DecimalType && a.compileTimeDecimalWidth(context) == 128 {
		decimalBits = 128
	}
	text, ok := constant.DecimalText(value.Exact, decimalBits)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	value.Kind = DecimalType
	value.DecimalBits = decimalBits
	// Retain literal source spelling where available; computed values use exact text.
	if value.Lexeme == "" {
		value.Lexeme = text
	}
	return value, compileTimeEvaluated
}

// compileTimeBinary evaluates primitive operator values using the same scalar
// domains as runtime semantics. Unknown/executing operands never become values.
// Rules: rules/foundations/operators.md — arithmetic, comparisons, logical
// operators and String concatenation; rules/compiler/compile_time_evaluation.md — §2(3).
func (a *Analyzer) compileTimeBinary(op string, left, right DefaultConstant) (DefaultConstant, compileTimeOutcome) {
	isText := func(v DefaultConstant) bool { return v.Kind == StringType || v.Kind == CharType || v.Kind == RuneType }
	if isText(left) && isText(right) {
		if left.NominalText || right.NominalText {
			return DefaultConstant{}, compileTimeNotConstant
		}
		if op == "+" {
			text := left.String + right.String
			return DefaultConstant{Kind: StringType, String: text, Lexeme: strconv.Quote(text)}, compileTimeEvaluated
		}
		if op == "==" || op == "!=" {
			equal := left.String == right.String
			if op == "!=" {
				equal = !equal
			}
			return DefaultConstant{Kind: BoolType, Bool: equal, Lexeme: strconv.FormatBool(equal)}, compileTimeEvaluated
		}
	}
	if left.Kind == BoolType && right.Kind == BoolType {
		var b bool
		switch op {
		case "&&":
			b = left.Bool && right.Bool
		case "||":
			b = left.Bool || right.Bool
		case "==":
			b = left.Bool == right.Bool
		case "!=":
			b = left.Bool != right.Bool
		default:
			return DefaultConstant{}, compileTimeNotConstant
		}
		return DefaultConstant{Kind: BoolType, Bool: b, Lexeme: strconv.FormatBool(b)}, compileTimeEvaluated
	}
	exact := func(v DefaultConstant) *big.Rat {
		if v.Exact != nil {
			return new(big.Rat).Set(v.Exact)
		}
		if v.Integer != nil {
			return new(big.Rat).SetInt(v.Integer)
		}
		return nil
	}
	x, y := exact(left), exact(right)
	if x == nil || y == nil {
		return DefaultConstant{}, compileTimeNotConstant
	}
	if left.FloatBits != 0 && right.DecimalBits != 0 || right.FloatBits != 0 && left.DecimalBits != 0 || left.FloatBits != 0 && right.FloatBits != 0 && left.FloatBits != right.FloatBits {
		return DefaultConstant{}, compileTimeNotConstant
	}
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		c := x.Cmp(y)
		b := op == "==" && c == 0 || op == "!=" && c != 0 || op == "<" && c < 0 || op == "<=" && c <= 0 || op == ">" && c > 0 || op == ">=" && c >= 0
		return DefaultConstant{Kind: BoolType, Bool: b, Lexeme: strconv.FormatBool(b)}, compileTimeEvaluated
	}
	bits := left.FloatBits
	if bits == 0 {
		bits = right.FloatBits
	}
	if left.FloatBits != 0 && right.FloatBits != 0 && left.FloatBits != right.FloatBits {
		return DefaultConstant{}, compileTimeNotConstant
	}
	result, ok := constant.Numeric(op, x, y, bits)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	value := DefaultConstant{Kind: DecimalType, Exact: result, FloatBits: bits, DecimalBits: max(left.DecimalBits, right.DecimalBits)}
	if bits != 0 {
		value.Kind = FloatType
	}
	return a.prepareCompileTimeConstant(value, Type{})
}

// compileTimeDecimalWidth follows resolved nominal ancestry to the canonical
// decimal carrier rather than guessing width from an alias name.
// Rules: rules/types/types.md — Named types and Exact decimal types;
// rules/mlir/packages/sec-mlir-dialect_package6.md — §36.
func (a *Analyzer) compileTimeDecimalWidth(typ Type) int {
	seen := map[string]bool{}
	for !seen[typ.Name] {
		seen[typ.Name] = true
		if typ.Name == "decimal128" {
			return 128
		}
		if typ.Underlying == "" {
			break
		}
		next, ok := a.types[typ.Underlying]
		if !ok {
			break
		}
		typ = next
	}
	return 64
}
