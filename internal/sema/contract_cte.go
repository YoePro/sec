package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// compileTimeOutcome classifies an expression evaluated in a
// SemanticCompileTimeRequiredContext.
type compileTimeOutcome int

const (
	// compileTimeEvaluated: the value was established at compile time.
	compileTimeEvaluated compileTimeOutcome = iota
	// compileTimeNotConstant: the expression depends on runtime state, an
	// unknown name, or an operation that cannot complete at compile time.
	compileTimeNotConstant
	// compileTimeRequiresExecution: the expression calls a function or reads a
	// property getter. Such execution is legal semantic CTE (MD-011) but the
	// compiler's semantic CTE executor is not implemented yet.
	compileTimeRequiresExecution
	// compileTimeAlreadyInvalid: the parser already reported the expression.
	compileTimeAlreadyInvalid
	// compileTimeForbiddenClock is permanently invalid semantic CTE, unlike a
	// deterministic getter whose execution merely awaits the CTE executor.
	compileTimeForbiddenClock
)

// semanticCompileTimeConstant evaluates a contract or default expression in a
// SemanticCompileTimeRequiredContext. Contract arguments and defaults are
// ordinary expressions; this is the compiler's shared constant evaluation, not
// a contract-only evaluator: literals and literal operators, the module's
// immutable compile-time-established bindings through the shared integer
// evaluator, and width-aware operators over immutable primitive bindings. Calls and
// property getters are legal semantic CTE but need the not yet implemented
// executor; they are classified separately so the diagnostic never claims the
// source is invalid. Ambient clock input is forbidden independently of
// executor availability and can never establish a compile-time value.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.1–3.17, 3.22–3.29, 9.2
//   - rules/compiler/compile_time_evaluation.md — "SemanticCompileTimeRequiredContext"
//   - rules/types/contracts.md — contract arguments; rules/types/default_values.md — explicit defaults
//   - rules/types/temporal.md — §3
//   - rules/corrections/applied/temporal-now-correction-20260928.md — §7
func (a *Analyzer) semanticCompileTimeConstant(expr ast.Expression) (DefaultConstant, compileTimeOutcome) {
	return a.semanticCompileTimeConstantVisiting(expr, map[string]bool{}, Type{})
}

// semanticCompileTimeConstantVisiting resolves immutable bindings without
// cycles and applies contextual scalar semantics before evaluating operators.
// Rules: rules/compiler/compile_time_evaluation.md — §§2(3),47;
// rules/types/contracts.md — Contract arguments; rules/types/default_values.md — Explicit defaults;
// rules/types/temporal.md — §3 (no clock input in semantic CTE).
func (a *Analyzer) semanticCompileTimeConstantVisiting(expr ast.Expression, visiting map[string]bool, context Type) (DefaultConstant, compileTimeOutcome) {
	if expr == nil {
		return DefaultConstant{}, compileTimeAlreadyInvalid
	}
	if _, invalid := expr.(*ast.InvalidExpression); invalid {
		return DefaultConstant{}, compileTimeAlreadyInvalid
	}
	if a.compileTimeClockRead(expr) {
		return DefaultConstant{}, compileTimeForbiddenClock
	}
	if constant, ok := defaultConstantFromExpression(expr); ok && !compileTimeOperatorExpression(expr) {
		if numeric, ok := expr.(interface{ Suffix() string }); ok && numeric.Suffix() == "g" && context.Kind != FloatType {
			context = a.types["float"]
		}
		return a.prepareCompileTimeConstant(constant, context)
	}
	if value, ok := a.integerConstantValueUsing(expr, a.registerWidthConstants[a.currentModule]); ok && context.Kind != FloatType && context.Kind != DecimalType {
		return DefaultConstant{Kind: IntType, Lexeme: value.String(), Integer: value}, compileTimeEvaluated
	}
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		// A projection or getter on a clock-derived value remains clock input.
		if _, outcome := a.semanticCompileTimeConstantVisiting(expr.Object, visiting, Type{}); outcome == compileTimeForbiddenClock {
			return DefaultConstant{}, outcome
		}
	case *ast.CallExpression:
		// Arguments are evaluated before invoking a getter/function; an absent
		// executor must not classify a forbidden clock operand as legal CTE.
		callee := expr.Callee
		if callee == nil && expr.Function != nil {
			callee = expr.Function
		}
		if callee != nil {
			if _, outcome := a.semanticCompileTimeConstantVisiting(callee, visiting, Type{}); outcome == compileTimeForbiddenClock {
				return DefaultConstant{}, outcome
			}
		}
		for _, argument := range expr.Arguments {
			if _, outcome := a.semanticCompileTimeConstantVisiting(argument, visiting, Type{}); outcome == compileTimeForbiddenClock {
				return DefaultConstant{}, outcome
			}
		}
	case *ast.ConversionExpression:
		if _, outcome := a.semanticCompileTimeConstantVisiting(expr.Value, visiting, Type{}); outcome == compileTimeForbiddenClock {
			return DefaultConstant{}, outcome
		}
	case *ast.Identifier:
		binding, ok := a.moduleImmutableBindings[a.currentModule][expr.Value]
		if !ok || binding.Value == nil || visiting[expr.Value] {
			return DefaultConstant{}, compileTimeNotConstant
		}
		visiting[expr.Value] = true
		defer delete(visiting, expr.Value)
		bindingContext := context
		if binding.Type != nil {
			if typ, exists := a.types[a.resolveTypeName(binding.Type.Name)]; exists {
				bindingContext = typ
			}
		}
		value, outcome := a.semanticCompileTimeConstantVisiting(binding.Value, visiting, bindingContext)
		if bindingContext.Named && (bindingContext.Kind == StringType || bindingContext.Kind == CharType || bindingContext.Kind == RuneType) {
			value.NominalText = true
		}
		return value, outcome
	case *ast.PrefixExpression:
		inner, outcome := a.semanticCompileTimeConstantVisiting(expr.Right, visiting, context)
		if outcome != compileTimeEvaluated {
			return DefaultConstant{}, outcome
		}
		if expr.Operator == "+" && (inner.Exact != nil || inner.Integer != nil) {
			return inner, compileTimeEvaluated
		}
		if expr.Operator == "-" && inner.Exact != nil {
			inner.Exact = new(big.Rat).Neg(inner.Exact)
			inner.Lexeme = ""
			return a.prepareCompileTimeConstant(inner, context)
		}
		if expr.Operator == "!" && inner.Kind == BoolType {
			inner.Bool = !inner.Bool
			return inner, compileTimeEvaluated
		}
	case *ast.InfixExpression:
		left, outcome := a.semanticCompileTimeConstantVisiting(expr.Left, visiting, context)
		if outcome != compileTimeEvaluated {
			return DefaultConstant{}, outcome
		}
		if left.Kind == BoolType && (expr.Operator == "&&" && !left.Bool || expr.Operator == "||" && left.Bool) {
			return left, compileTimeEvaluated
		}
		rightContext := context
		if left.FloatBits != 0 {
			rightContext = Type{Kind: FloatType, FloatBits: left.FloatBits}
		}
		if left.DecimalBits == 128 {
			rightContext = Type{Kind: DecimalType, Name: "decimal128"}
		}
		right, outcome := a.semanticCompileTimeConstantVisiting(expr.Right, visiting, rightContext)
		if outcome != compileTimeEvaluated {
			return DefaultConstant{}, outcome
		}
		return a.compileTimeBinary(expr.Operator, left, right)

	}
	if a.compileTimeExpressionExecutes(expr) {
		return DefaultConstant{}, compileTimeRequiresExecution
	}
	return DefaultConstant{}, compileTimeNotConstant
}

// compileTimeExpressionExecutes reports whether expr contains a call or a
// member read that is not an enum-member reference, i.e. code whose
// compile-time result requires executing a function or a getter. A member of
// an enum type names a value (or an unknown member), never a getter.
func (a *Analyzer) compileTimeExpressionExecutes(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		return true
	case *ast.MemberExpression:
		if owner, ok := expr.Object.(*ast.Identifier); ok {
			if typ, exists := a.types[owner.Value]; exists && typ.Kind == EnumType {
				return false
			}
		}
		return true
	case *ast.PrefixExpression:
		return a.compileTimeExpressionExecutes(expr.Right)
	case *ast.InfixExpression:
		return a.compileTimeExpressionExecutes(expr.Left) || a.compileTimeExpressionExecutes(expr.Right)
	case *ast.ConversionExpression:
		return a.compileTimeExpressionExecutes(expr.Value)
	}
	return false
}

// semanticCompileTimeInteger evaluates an integer-valued required position.
func (a *Analyzer) semanticCompileTimeInteger(expr ast.Expression) (*big.Int, compileTimeOutcome) {
	constant, outcome := a.semanticCompileTimeConstant(expr)
	if outcome != compileTimeEvaluated {
		return nil, outcome
	}
	if constant.Integer == nil {
		return nil, compileTimeNotConstant
	}
	return new(big.Int).Set(constant.Integer), compileTimeEvaluated
}

// semanticCompileTimeExact evaluates an exact numeric required position, used
// by decimal and floating-point range bounds.
func (a *Analyzer) semanticCompileTimeExact(expr ast.Expression) (*big.Rat, string, compileTimeOutcome) {
	if exact, lexeme, ok := exactNumericConstant(expr); ok {
		return exact, lexeme, compileTimeEvaluated
	}
	constant, outcome := a.semanticCompileTimeConstant(expr)
	if outcome != compileTimeEvaluated {
		return nil, "", outcome
	}
	switch {
	case constant.Exact != nil:
		return new(big.Rat).Set(constant.Exact), constant.Lexeme, compileTimeEvaluated
	case constant.Integer != nil:
		return new(big.Rat).SetInt(constant.Integer), constant.Lexeme, compileTimeEvaluated
	}
	return nil, "", compileTimeNotConstant
}

// reportCompileTimeRequirement diagnoses a required position that could not
// be established: S1101 for a call or getter awaiting the semantic CTE
// executor, otherwise the owning invalid-argument diagnostic. Forbidden clock
// input retains a permanent semantic error and deterministic-value guidance.
// Rules: rules/compiler/compile_time_evaluation.md — §8(5);
// rules/types/temporal.md — §3;
// rules/corrections/applied/temporal-now-correction-20260928.md — §7.
func (a *Analyzer) reportCompileTimeRequirement(expr ast.Expression, outcome compileTimeOutcome, position string, invalidID string, help string) {
	switch outcome {
	case compileTimeAlreadyInvalid, compileTimeEvaluated:
		return
	case compileTimeRequiresExecution:
		a.addErrorAtTokenWithMetadata(expressionToken(expr), diagnostics.SemanticCompileTimeExecutionUnavailable,
			"Use a literal or an immutable compile-time binding until compile-time execution of calls and property getters is implemented.",
			"%s %s requires compile-time execution of a call or property getter, which the compiler does not implement yet", position, expr.String())
	case compileTimeForbiddenClock:
		a.addErrorAtTokenWithMetadata(expressionToken(expr), invalidID,
			"Use deterministic compile-time values; read the clock in runtime code.",
			"current UTC wall-clock access is forbidden in compile-time-required %s %s", position, expr.String())
	default:
		a.addErrorAtTokenWithMetadata(expressionToken(expr), invalidID, help, "%s %s cannot be evaluated at compile time", position, expr.String())
	}
}
