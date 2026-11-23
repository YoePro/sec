package sema

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema/constant"
	"sec/internal/sema/membership"
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
// rules/mlir/packages/sec-mlir-dialect_package20.md — §§12–20,82.
func (a *Analyzer) prepareCompileTimeConstant(value DefaultConstant, context Type) (DefaultConstant, compileTimeOutcome) {
	if value.Integer != nil && (context.Kind == FloatType || context.Kind == DecimalType) {
		value.Exact = new(big.Rat).SetInt(value.Integer)
		// MD-049 § 3(4): an integer in decimal context has scale 0.
		value.Decimal = &DecimalValue{Coefficient: new(big.Int).Set(value.Integer)}
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
		// Literal source bits share the exact direct-width rounding boundary
		// with Semantic IR. Already-rounded negative zero survives rechecking.
		// Rules: package20 §§13–19,82; compiler/compile_time_evaluation.md §2(3).
		if value.FloatBits == 0 && value.Lexeme != "" {
			pattern, err := constant.FloatLiteralBits(value.Lexeme, uint16(bits))
			if err != nil {
				return DefaultConstant{}, compileTimeNotConstant
			}
			if bits == 32 {
				number = float64(math.Float32frombits(uint32(pattern)))
			} else {
				number = math.Float64frombits(pattern)
			}
		} else if number == 0 && strings.HasPrefix(value.Lexeme, "-") {
			number = math.Copysign(0, -1)
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
	if left.Nominal.Owner != "" || right.Nominal.Owner != "" {
		if left.Kind != right.Kind || left.Nominal.Type != right.Nominal.Type || left.Nominal.Owner != right.Nominal.Owner || op != "==" && op != "!=" {
			return DefaultConstant{}, compileTimeNotConstant
		}
		equal := membership.Equal(left.Nominal, right.Nominal)
		if op == "!=" {
			equal = !equal
		}
		return DefaultConstant{Kind: BoolType, Bool: equal, Lexeme: strconv.FormatBool(equal)}, compileTimeEvaluated
	}
	if left.Integer != nil && right.Integer != nil {
		if integer, ok := constant.IntegerBinary(op, left.Integer, right.Integer); ok {
			return DefaultConstant{Kind: IntType, Lexeme: integer.String(), Integer: integer}, compileTimeEvaluated
		}
	}
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
	// P20 § 82: float operands fold with IEEE-754 semantics in their binary
	// format, including infinities, NaN (unordered) and signed zero.
	if bits, float := compileTimeFloatBits(left, right); float {
		switch op {
		case "==", "!=", "<", "<=", ">", ">=":
			return compileTimeFloatComparison(op, left, right, bits)
		}
		return compileTimeFloatArithmetic(op, left, right, bits)
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
	if bits == 0 {
		if value, handled := compileTimeDecimalArithmetic(op, left, right); handled {
			return a.prepareCompileTimeConstant(value, Type{})
		}
	}
	if bits == 32 || bits == 64 {
		return compileTimeFloatArithmetic(op, left, right, bits)
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

// compileTimeDecimalOperand is the scale-bearing pair of a decimal or integer
// constant: an integer has scale 0 (MD-049 § 3(4)).
func compileTimeDecimalOperand(value DefaultConstant) (DecimalValue, bool) {
	switch {
	case value.Decimal != nil && value.Decimal.Coefficient != nil:
		return *value.Decimal, true
	case value.Integer != nil:
		return DecimalValue{Coefficient: new(big.Int).Set(value.Integer)}, true
	}
	return DecimalValue{}, false
}

// compileTimeDecimalArithmetic evaluates decimal + - * / % with the runtime
// coefficient/scale policy (constant.EvaluateDecimal, including the MD-049
// exact zero-product scale) whenever both operands carry scale-bearing facts
// and at least one is a decimal. It reports handled=false when that fact is
// missing or the operation is outside the bounded evaluation domain, so the
// caller keeps its rational evaluation and attaches no invented scale.
// Rules: rules/foundations/operators.md — §§ 27.1, 27.1(5a); rules/compiler/semantic_ir.md — § 128(1);
// rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — § 13(1)-(2).
func compileTimeDecimalArithmetic(op string, left, right DefaultConstant) (DefaultConstant, bool) {
	if left.Kind != DecimalType && right.Kind != DecimalType {
		return DefaultConstant{}, false
	}
	switch op {
	case "+", "-", "*", "/", "%":
	default:
		return DefaultConstant{}, false
	}
	a, ok := compileTimeDecimalOperand(left)
	if !ok {
		return DefaultConstant{}, false
	}
	b, ok := compileTimeDecimalOperand(right)
	if !ok {
		return DefaultConstant{}, false
	}
	bits := max(left.DecimalBits, right.DecimalBits)
	if bits == 0 {
		bits = 64
	}
	coefficient, scale, failure, evaluated := constant.EvaluateDecimal(op, a.Coefficient, int64(a.Scale), b.Coefficient, int64(b.Scale), bits)
	if !evaluated || failure != "" {
		return DefaultConstant{}, false
	}
	decimal := DecimalValue{Coefficient: coefficient, Scale: int32(scale)}
	exact := new(big.Rat).SetInt(coefficient)
	if scale > 0 {
		exact.Quo(exact, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(scale), nil)))
	} else if scale < 0 {
		exact.Mul(exact, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(-scale), nil)))
	}
	return DefaultConstant{Kind: DecimalType, Exact: exact, Decimal: &decimal, DecimalBits: bits}, true
}

// compileTimeFloatOperand is the binary value of a float operand in its
// resolved format: the shortest round-trip spelling of an already rounded
// value (which keeps the sign of zero), otherwise the exact value rounded
// once to the format.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13-20, 82.
func compileTimeFloatOperand(value DefaultConstant, bits int) (float64, bool) {
	if value.FloatBits == bits && value.Lexeme != "" {
		number, err := strconv.ParseFloat(value.Lexeme, bits)
		return number, err == nil
	}
	exact := value.Exact
	if exact == nil && value.Integer != nil {
		exact = new(big.Rat).SetInt(value.Integer)
	}
	if exact == nil {
		return 0, false
	}
	if bits == 32 {
		number, _ := exact.Float32()
		return float64(number), true
	}
	number, _ := exact.Float64()
	return number, true
}

// compileTimeFloatArithmetic folds + - * / % exactly as the runtime
// arith.addf/subf/mulf/divf/remf do: IEEE-754 in the operands' binary
// format, one round-to-nearest-even per operation (computing a binary32
// operation in binary64 and narrowing is exact for these operations), signed
// zero preserved, remainder as fmod. A non-finite result is not a
// compile-time constant (Sec CTE has no infinity or NaN value).
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13-20, 75, 82.
func compileTimeFloatArithmetic(op string, left, right DefaultConstant, bits int) (DefaultConstant, compileTimeOutcome) {
	x, xok := compileTimeFloatOperand(left, bits)
	y, yok := compileTimeFloatOperand(right, bits)
	if !xok || !yok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	var result float64
	switch op {
	case "+":
		result = x + y
	case "-":
		result = x - y
	case "*":
		result = x * y
	case "/":
		result = x / y
	case "%":
		result = math.Mod(x, y)
	default:
		return DefaultConstant{}, compileTimeNotConstant
	}
	if bits == 32 {
		result = float64(float32(result))
	}
	return compileTimeFloatValue(result, bits)
}

// compileTimeFloatValue is the compile-time constant of a binary float
// value: finite values keep their exact rational, infinities and NaN carry
// only their spelling ("+Inf", "-Inf", "NaN") and format.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 13-20, 68, 82.
func compileTimeFloatValue(value float64, bits int) (DefaultConstant, compileTimeOutcome) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return DefaultConstant{Kind: FloatType, FloatBits: bits, Lexeme: strconv.FormatFloat(value, 'g', -1, bits)}, compileTimeEvaluated
	}
	constant, ok := binaryFloatConstant(value, bits)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	constant.FloatBits = bits
	return constant, compileTimeEvaluated
}

// compileTimeFloatBits selects the binary format of a float operation: at
// least one operand is a resolved float, both resolved operands agree and no
// decimal participates.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 9, 82.
func compileTimeFloatBits(left, right DefaultConstant) (int, bool) {
	bits := left.FloatBits
	if bits == 0 {
		bits = right.FloatBits
	}
	if bits != 32 && bits != 64 || left.FloatBits != 0 && right.FloatBits != 0 && left.FloatBits != right.FloatBits ||
		left.DecimalBits != 0 || right.DecimalBits != 0 {
		return 0, false
	}
	return bits, true
}

// compileTimeFloatComparison compares in IEEE-754 order: NaN is unordered
// (only != holds) and -0 equals +0.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 77, 82.
func compileTimeFloatComparison(op string, left, right DefaultConstant, bits int) (DefaultConstant, compileTimeOutcome) {
	x, xok := compileTimeFloatOperand(left, bits)
	y, yok := compileTimeFloatOperand(right, bits)
	if !xok || !yok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	var result bool
	switch op {
	case "==":
		result = x == y
	case "!=":
		result = x != y
	case "<":
		result = x < y
	case "<=":
		result = x <= y
	case ">":
		result = x > y
	case ">=":
		result = x >= y
	}
	return DefaultConstant{Kind: BoolType, Bool: result, Lexeme: strconv.FormatBool(result)}, compileTimeEvaluated
}

// floatPropertyConstant evaluates the compiler-known float type properties
// T.Min, T.Max, T.Epsilon, T.Infinity, T.NegativeInfinity and T.NaN of
// float, float32 and float64 in semantic CTE with their P20 definitions
// (Min is the most negative finite value; NaN is the canonical quiet NaN).
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 28, 68;
// rules/library/core-library.md — § 9.1.
func (a *Analyzer) floatPropertyConstant(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return DefaultConstant{}, 0, false
	}
	owner, ok := member.Object.(*ast.Identifier)
	if !ok || owner.Value != "float" && owner.Value != "float32" && owner.Value != "float64" {
		return DefaultConstant{}, 0, false
	}
	if _, shadowed := a.symbols.lookup(owner.Value); shadowed {
		return DefaultConstant{}, 0, false
	}
	bits := a.types[owner.Value].FloatBits
	if bits != 32 && bits != 64 {
		return DefaultConstant{}, 0, false
	}
	maximum := math.MaxFloat64
	epsilon := math.Ldexp(1, -52)
	if bits == 32 {
		maximum, epsilon = math.MaxFloat32, math.Ldexp(1, -23)
	}
	var value float64
	switch member.Property.Value {
	case "Max":
		value = maximum
	case "Min":
		value = -maximum
	case "Epsilon":
		value = epsilon
	case "Infinity":
		value = math.Inf(1)
	case "NegativeInfinity":
		value = math.Inf(-1)
	case "NaN":
		value = math.NaN()
	default:
		return DefaultConstant{}, 0, false
	}
	result, outcome := compileTimeFloatValue(value, bits)
	return result, outcome, true
}
