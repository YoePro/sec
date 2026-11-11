package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// cteFailureKind is the compiler-semantic outcome category of a required
// compile-time evaluation that did not establish a value
// (compile_time_evaluation.md §§ 53–56). These are not Sec types.
type cteFailureKind string

const (
	// cteNotEvaluable: the executed path reads state that is not a
	// compile-time value (for example a mutable static).
	cteNotEvaluable cteFailureKind = "NotEvaluable"
	// cteEvaluationFailed: a reached assert, panic or semantic operation
	// failure (SemanticOperationFailed: overflow, division by zero, checked
	// conversion, contract violation).
	cteEvaluationFailed cteFailureKind = "EvaluationFailed"
	// cteForbiddenEffect: the executed path performs an effect semantic CTE
	// forbids (foreign call, ambient clock, mutation of static state).
	cteForbiddenEffect cteFailureKind = "ForbiddenEffect"
	// cteResourceLimit: an evaluator budget was exhausted; not a proof of
	// nontermination (§ 49).
	cteResourceLimit cteFailureKind = "ResourceLimit"
	// cteDependencyCycle: evaluation needs its own result (§ 39).
	cteDependencyCycle cteFailureKind = "DependencyCycle"
	// cteImplementationGap: legal Sec the executor does not run yet (§ 56).
	cteImplementationGap cteFailureKind = "ImplementationGap"
	// cteSuppressed: ordinary analysis already diagnosed the root; the
	// evaluation reports nothing further.
	cteSuppressed cteFailureKind = "Suppressed"
)

// cteCallSite is one frame of the executed call chain, for provenance (§ 59).
type cteCallSite struct {
	Function string
	Call     lexer.Token
}

// cteFailure describes why a required evaluation established no value.
type cteFailure struct {
	Kind      cteFailureKind
	Reason    string
	At        lexer.Token
	CallStack []cteCallSite
}

// cteResult is the outcome of one required evaluation request.
type cteResult struct {
	Value   DefaultConstant
	Failure *cteFailure
}

// Evaluator budgets (§ 48(4)). They protect the compiler process and are not
// Sec semantics; exhausting one is a ResourceLimit outcome.
const (
	cteMaxExecutionSteps = 1_000_000
	cteMaxCallDepth      = 256
)

type cteControl int

const (
	cteNormal cteControl = iota
	cteReturn
	cteBreak
	cteContinue
	cteFallthrough
)

type cteFrame struct {
	values map[sourceTokenKey]DefaultConstant
	// receiver is the struct value an instance method or getter runs on.
	receiver *DefaultConstant
}

// cteExecutor executes canonical typed Sec semantics over the analyzed AST.
// It never performs name lookup, overload resolution or type inference of
// its own (§ 51(3)): identifiers bind through Sema's definition facts, calls
// through the resolved call facts, operators through the shared
// compile-time operator semantics (compileTimeBinary) with the resolved type
// of every intermediate. Scalar values reuse DefaultConstant, the existing
// typed compile-time value representation.
type cteExecutor struct {
	a         *Analyzer
	functions map[sourceTokenKey]*ast.FunctionDeclaration
	// getters are property getter bodies keyed by their block token, the
	// identity Sema gives the getter callable.
	getters map[sourceTokenKey]*ast.BlockStatement
	frames  []*cteFrame
	calls   []cteCallSite
	active  map[*ast.FunctionDeclaration]int
	// statics are the static initializers being evaluated, in order, to
	// detect compile-time value dependency cycles (§ 39).
	statics  []string
	visiting map[sourceTokenKey]bool
	steps    int
	failure  *cteFailure
	// returns are the declared result types of the executing callables;
	// pendingReturn is the early-return value of a propagating try.
	returns       []Type
	pendingReturn *DefaultConstant
}

func (x *cteExecutor) currentFunctionReturn() Type {
	if len(x.returns) == 0 {
		return Type{}
	}
	return x.returns[len(x.returns)-1]
}

func newCTEExecutor(a *Analyzer, program *ast.Program) *cteExecutor {
	x := &cteExecutor{a: a, functions: map[sourceTokenKey]*ast.FunctionDeclaration{}, getters: map[sourceTokenKey]*ast.BlockStatement{}, active: map[*ast.FunctionDeclaration]int{}, visiting: map[sourceTokenKey]bool{}}
	var visit func(ast.Statement)
	visit = func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement != nil && statement.Name != nil {
				x.functions[sourceTokenLocation(statement.Name.Token)] = statement
			}
		case *ast.ImplStatement:
			if statement == nil {
				return
			}
			for _, member := range statement.Members {
				switch member := member.(type) {
				case *ast.FunctionDeclaration:
					visit(member)
				case *ast.PropertyDeclaration:
					if member != nil && member.Getter != nil {
						x.getters[sourceTokenLocation(member.Getter.Token)] = member.Getter
					}
				}
			}
		}
	}
	if program != nil {
		for _, statement := range program.Statements {
			visit(statement)
		}
	}
	return x
}

func (x *cteExecutor) fail(kind cteFailureKind, at lexer.Token, format string, args ...any) {
	if x.failure != nil {
		return
	}
	x.failure = &cteFailure{Kind: kind, Reason: fmt.Sprintf(format, args...), At: at, CallStack: append([]cteCallSite(nil), x.calls...)}
}

func (x *cteExecutor) step(at lexer.Token) bool {
	x.steps++
	if x.steps > cteMaxExecutionSteps {
		x.fail(cteResourceLimit, at, "the ExecutionSteps budget of %d steps was reached", cteMaxExecutionSteps)
		return false
	}
	return x.failure == nil
}

// evaluateStatic executes a static initializer, recording its declaration
// as being established so a dependency back to it is a cycle.
func (x *cteExecutor) evaluateStatic(name string, declaration sourceTokenKey, root ast.Expression) cteResult {
	x.visiting[declaration] = true
	x.statics = append(x.statics, name)
	return x.evaluate(root)
}

// evaluate executes a root expression in a fresh top-level frame.
func (x *cteExecutor) evaluate(root ast.Expression) cteResult {
	x.frames = []*cteFrame{{values: map[sourceTokenKey]DefaultConstant{}}}
	value, ok := x.eval(root)
	if x.failure != nil {
		return cteResult{Failure: x.failure}
	}
	if x.pendingReturn != nil {
		return cteResult{Failure: &cteFailure{Kind: cteEvaluationFailed, Reason: "a return or try propagation left the evaluated expression", At: expressionToken(root)}}
	}
	if !ok {
		return cteResult{Failure: &cteFailure{Kind: cteNotEvaluable, Reason: "the expression does not establish a compile-time value", At: expressionToken(root)}}
	}
	return cteResult{Value: value}
}

// eval evaluates one expression through the shared compile-time evaluator
// with executor-owned identifier and expression resolvers.
func (x *cteExecutor) eval(expr ast.Expression) (DefaultConstant, bool) {
	if expr == nil || !x.step(expressionToken(expr)) {
		return DefaultConstant{}, false
	}
	value, outcome := x.a.semanticCompileTimeConstantResolving(expr, map[string]bool{}, Type{}, x.identifier, x.expression)
	if x.failure != nil || x.pendingReturn != nil {
		return DefaultConstant{}, false
	}
	switch outcome {
	case compileTimeEvaluated:
		return value, true
	case compileTimeForbiddenClock:
		x.fail(cteForbiddenEffect, expressionToken(expr), "%s reads the current wall-clock time", expr.String())
	default:
		x.fail(cteImplementationGap, expressionToken(expr), "%s is not executed by the compile-time evaluator yet", expr.String())
	}
	return DefaultConstant{}, false
}

// declarationOf returns the declaration token Sema bound to a use.
func (x *cteExecutor) declarationOf(use lexer.Token) (sourceTokenKey, bool) {
	definitions := x.a.DefinitionsAt(use.File, use.Line, use.Column)
	if len(definitions) != 1 {
		return sourceTokenKey{}, false
	}
	return sourceTokenLocation(definitions[0]), true
}

func (x *cteExecutor) frame() *cteFrame { return x.frames[len(x.frames)-1] }

// identifier reads a local, parameter or loop binding of the current frame,
// or an immutable module value established by semantic CTE (§ 21(3)).
func (x *cteExecutor) identifier(identifier *ast.Identifier) (DefaultConstant, compileTimeOutcome) {
	if value, ok, handled := x.receiverPlace(identifier); handled {
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant
		}
		return value, compileTimeEvaluated
	}
	if declaration, ok := x.declarationOf(identifier.Token); ok {
		if value, bound := x.frame().values[declaration]; bound {
			return value, compileTimeEvaluated
		}
		if static, isStatic := x.a.staticBindings[declaration]; isStatic {
			return x.staticValue(identifier, declaration, static)
		}
	}
	value, outcome := x.a.semanticCompileTimeConstantVisiting(identifier, map[string]bool{}, Type{})
	if outcome == compileTimeEvaluated {
		return value, outcome
	}
	x.fail(cteNotEvaluable, identifier.Token, "%s is not a compile-time value; semantic CTE reads only locals, parameters and immutable compile-time-established values, never mutable static state", identifier.Value)
	return DefaultConstant{}, compileTimeNotConstant
}

// expression handles the forms the shared literal/operator evaluator does
// not execute: calls, conversions and checked arithmetic.
func (x *cteExecutor) expression(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	if value, ok, handled := x.variantExpression(expr); handled {
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	}
	switch expr := expr.(type) {
	case *ast.TryExpression:
		value, ok := x.tryExpression(expr)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.MatchExpression:
		control, value, ok := x.matchValue(expr, false)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		if control == cteReturn {
			x.propagate(value)
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.CallExpression:
		value, ok := x.call(expr)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.InfixExpression:
		value, ok := x.infix(expr)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.StructLiteral:
		value, ok := x.structLiteral(expr)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.MemberExpression:
		// Enum and nominal members are values the shared evaluator owns; a
		// property read executes its getter, any other member reads a field.
		if value, ok := x.a.nominalMemberConstant(expr, Type{}); ok {
			return value, compileTimeEvaluated, true
		}
		if owner, ok := expr.Object.(*ast.Identifier); ok {
			if typ, exists := x.a.types[owner.Value]; exists && typ.Kind == EnumType {
				return DefaultConstant{}, compileTimeNotConstant, false
			}
		}
		if _, property := x.a.resolvedPropertyAccesses[expr]; !property && expr.Property != nil {
			object, ok := x.eval(expr.Object)
			if !ok {
				return DefaultConstant{}, compileTimeNotConstant, true
			}
			value, ok := x.fieldOf(object, expr.Property.Value, expressionToken(expr))
			if !ok {
				return DefaultConstant{}, compileTimeNotConstant, true
			}
			return value, compileTimeEvaluated, true
		}
		value, ok := x.getter(expr)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	case *ast.ConversionExpression:
		value, ok := x.eval(expr.Value)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		if expr.Rounding != nil {
			x.fail(cteImplementationGap, expr.Token, "rounded conversion %s is not executed by the compile-time evaluator yet", expr.String())
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		value, ok = x.convert(expr, value)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	}
	return DefaultConstant{}, compileTimeNotConstant, false
}

// infix evaluates operands left to right with short-circuiting and applies
// the shared operator semantics; a result outside the resolved integer type
// and a zero divisor are semantic operation failures (§ 47(4)).
func (x *cteExecutor) infix(expr *ast.InfixExpression) (DefaultConstant, bool) {
	left, ok := x.eval(expr.Left)
	if !ok {
		return DefaultConstant{}, false
	}
	if left.Kind == BoolType && (expr.Operator == "&&" && !left.Bool || expr.Operator == "||" && left.Bool) {
		return left, true
	}
	right, ok := x.eval(expr.Right)
	if !ok {
		return DefaultConstant{}, false
	}
	return x.binary(expr.Operator, left, right, expr, expr.Token)
}

func (x *cteExecutor) binary(operator string, left, right DefaultConstant, typed ast.Expression, at lexer.Token) (DefaultConstant, bool) {
	if (operator == "/" || operator == "%") && right.Integer != nil && right.Integer.Sign() == 0 {
		x.fail(cteEvaluationFailed, at, "division by zero")
		return DefaultConstant{}, false
	}
	value, outcome := x.a.compileTimeBinary(operator, left, right)
	if outcome != compileTimeEvaluated {
		x.fail(cteImplementationGap, at, "operator %s on these operands is not executed by the compile-time evaluator yet", operator)
		return DefaultConstant{}, false
	}
	if typ, known := x.a.ResolvedTypeOf(typed); known {
		return x.inDomain(value, typ, at, "arithmetic overflow")
	}
	return value, true
}

// inDomain applies the checked-integer result rule of the resolved type.
func (x *cteExecutor) inDomain(value DefaultConstant, typ Type, at lexer.Token, what string) (DefaultConstant, bool) {
	if value.Integer == nil {
		return value, true
	}
	low, high, integer := scalarIntegerDomain(typ)
	if integer && (value.Integer.Cmp(low) < 0 || value.Integer.Cmp(high) > 0) {
		x.fail(cteEvaluationFailed, at, "%s: %s does not fit %s", what, value.Integer.String(), typeDisplayName(typ))
		return DefaultConstant{}, false
	}
	return value, true
}

// convert applies an explicit scalar conversion with its checked range.
func (x *cteExecutor) convert(expr ast.Expression, value DefaultConstant) (DefaultConstant, bool) {
	target, known := x.a.ResolvedTypeOf(expr)
	if !known {
		x.fail(cteImplementationGap, expressionToken(expr), "conversion %s has no resolved target type", expr.String())
		return DefaultConstant{}, false
	}
	if hasContracts(target) {
		x.fail(cteImplementationGap, expressionToken(expr), "checked conversion into constrained type %s is not executed by the compile-time evaluator yet", typeDisplayName(target))
		return DefaultConstant{}, false
	}
	if _, _, integer := scalarIntegerDomain(target); integer && value.Integer != nil {
		return x.inDomain(value, target, expressionToken(expr), "conversion out of range")
	}
	if value.Integer != nil && (target.Kind == FloatType || target.Kind == DecimalType) {
		prepared, outcome := x.a.prepareCompileTimeConstant(value, target)
		if outcome == compileTimeEvaluated {
			return prepared, true
		}
	}
	if value.Kind == target.Kind && value.Integer == nil {
		return value, true
	}
	x.fail(cteImplementationGap, expressionToken(expr), "conversion %s is not executed by the compile-time evaluator yet", expr.String())
	return DefaultConstant{}, false
}

// call executes a resolved direct or static call to a Sec function whose
// body is available, binding arguments to parameter declarations.
func (x *cteExecutor) call(call *ast.CallExpression) (DefaultConstant, bool) {
	resolved, ok := x.a.resolvedCalls[call]
	if !ok {
		if callee, named := call.Callee.(*ast.Identifier); named && len(call.Arguments) == 1 {
			if _, isType := x.a.types[callee.Value]; isType {
				value, ok := x.eval(call.Arguments[0])
				if !ok {
					return DefaultConstant{}, false
				}
				return x.convert(call, value)
			}
		}
		x.fail(cteImplementationGap, call.Token, "call %s has no resolved target that the compile-time evaluator can execute", call.String())
		return DefaultConstant{}, false
	}
	function := resolved.Function
	switch {
	case function.Extern:
		x.fail(cteForbiddenEffect, call.Token, "%s is a foreign call, which semantic CTE never executes", function.Name)
		return DefaultConstant{}, false
	case resolved.Kind != ResolvedDirectCall && resolved.Kind != ResolvedStaticMethodCall:
		x.fail(cteImplementationGap, call.Token, "%s dispatch of %s is not executed by the compile-time evaluator yet", resolved.Kind, function.Name)
		return DefaultConstant{}, false
	case function.GenericSubstitution != nil || len(function.GenericParameters) > 0:
		x.fail(cteImplementationGap, call.Token, "generic call %s is not executed by the compile-time evaluator yet", function.Name)
		return DefaultConstant{}, false
	}
	declaration := x.functions[sourceTokenLocation(function.Token)]
	if declaration == nil || declaration.Body == nil {
		if function.CompilerKnownID != "" {
			x.fail(cteImplementationGap, call.Token, "compiler-known operation %s is not executed by the compile-time evaluator yet", function.Name)
		} else {
			x.fail(cteImplementationGap, call.Token, "the body of %s is not available to this compile-time evaluation", function.Name)
		}
		return DefaultConstant{}, false
	}
	if len(x.frames) >= cteMaxCallDepth {
		x.fail(cteResourceLimit, call.Token, "the CallDepth budget of %d nested calls was reached", cteMaxCallDepth)
		return DefaultConstant{}, false
	}
	if len(call.Arguments) != len(function.Parameters) {
		x.fail(cteImplementationGap, call.Token, "call %s with default, variadic or spread arguments is not executed by the compile-time evaluator yet", function.Name)
		return DefaultConstant{}, false
	}
	frame := &cteFrame{values: map[sourceTokenKey]DefaultConstant{}}
	var receiverExpr ast.Expression
	if function.ImplTarget != "" && !function.Static {
		if member, ok := call.Callee.(*ast.MemberExpression); ok {
			receiverExpr = member.Object
			value, ok := x.eval(member.Object)
			if !ok {
				return DefaultConstant{}, false
			}
			frame.receiver = &value
		} else if current := x.frame().receiver; current != nil {
			value := *current
			frame.receiver = &value
		} else {
			x.fail(cteImplementationGap, call.Token, "method %s has no receiver value in this compile-time evaluation", function.Name)
			return DefaultConstant{}, false
		}
	}
	for index, argument := range call.Arguments {
		parameter := function.Parameters[index]
		if parameter.Ref || parameter.MutableRef || parameter.Variadic {
			x.fail(cteImplementationGap, expressionToken(argument), "reference or variadic parameter %s is not executed by the compile-time evaluator yet", parameter.Name)
			return DefaultConstant{}, false
		}
		value, ok := x.eval(argument)
		if !ok {
			return DefaultConstant{}, false
		}
		frame.values[sourceTokenLocation(parameter.Token)] = value
	}
	x.frames = append(x.frames, frame)
	x.calls = append(x.calls, cteCallSite{Function: function.Name, Call: call.Token})
	x.returns = append(x.returns, function.ReturnType)
	x.active[declaration]++
	control, value := x.block(declaration.Body)
	x.active[declaration]--
	x.returns = x.returns[:len(x.returns)-1]
	x.calls = x.calls[:len(x.calls)-1]
	x.frames = x.frames[:len(x.frames)-1]
	if x.failure != nil {
		return DefaultConstant{}, false
	}
	// A method that mutates its receiver writes the updated value back to
	// the receiver Place of the call (implicit `self` or a local path).
	if frame.receiver != nil && function.ReceiverMutable {
		if receiverExpr == nil {
			*x.frame().receiver = *frame.receiver
		} else if !x.store(receiverExpr, *frame.receiver, call.Token) {
			return DefaultConstant{}, false
		}
	}
	if control != cteReturn {
		if function.ReturnType.Kind == VoidType || function.ReturnType.Kind == InvalidType && function.ReturnType.Name == "" {
			return DefaultConstant{Kind: VoidType}, true
		}
		x.fail(cteImplementationGap, call.Token, "%s finished without returning a value", function.Name)
		return DefaultConstant{}, false
	}
	return x.inDomain(value, function.ReturnType, call.Token, "returned value out of range")
}

// block executes statements in order until a control transfer.
func (x *cteExecutor) block(block *ast.BlockStatement) (cteControl, DefaultConstant) {
	if block == nil {
		return cteNormal, DefaultConstant{}
	}
	for _, statement := range block.Statements {
		control, value := x.statement(statement)
		if x.failure != nil {
			return cteNormal, DefaultConstant{}
		}
		if x.pendingReturn != nil {
			value := *x.pendingReturn
			x.pendingReturn = nil
			return cteReturn, value
		}
		if control != cteNormal {
			return control, value
		}
	}
	return cteNormal, DefaultConstant{}
}

func (x *cteExecutor) statement(statement ast.Statement) (cteControl, DefaultConstant) {
	if isNilStatement(statement) {
		return cteNormal, DefaultConstant{}
	}
	if !x.step(statementToken(statement)) {
		return cteNormal, DefaultConstant{}
	}
	switch statement := statement.(type) {
	case *ast.CommentStatement:
		return cteNormal, DefaultConstant{}
	case *ast.ExpressionStatement:
		x.eval(statement.Expression)
		return cteNormal, DefaultConstant{}
	case *ast.DiscardStatement:
		if statement.Value != nil {
			x.eval(statement.Value)
		}
		return cteNormal, DefaultConstant{}
	case *ast.LetStatement:
		if statement.Static || statement.Value == nil || statement.Name == nil {
			x.fail(cteImplementationGap, statement.Token, "this let form is not executed by the compile-time evaluator yet")
			return cteNormal, DefaultConstant{}
		}
		value, ok := x.eval(statement.Value)
		if ok {
			x.frame().values[sourceTokenLocation(statement.Name.Token)] = value
		}
		return cteNormal, DefaultConstant{}
	case *ast.AssignmentStatement:
		x.assign(statement)
		return cteNormal, DefaultConstant{}
	case *ast.ReturnStatement:
		if statement.Value == nil {
			return cteReturn, DefaultConstant{Kind: VoidType}
		}
		value, ok := x.eval(statement.Value)
		if !ok {
			return cteNormal, DefaultConstant{}
		}
		return cteReturn, value
	case *ast.IfStatement:
		if binding := statement.OptionBinding; binding != nil {
			subject, ok := x.eval(binding.Subject)
			if !ok {
				return cteNormal, DefaultConstant{}
			}
			if subject.Aggregate == nil || binding.Binding == nil {
				x.fail(cteImplementationGap, statement.Token, "this if-binding is not executed by the compile-time evaluator yet")
				return cteNormal, DefaultConstant{}
			}
			if subject.Aggregate.Variant == "Some" && subject.Aggregate.Payload != nil {
				x.frame().values[sourceTokenLocation(binding.Binding.Token)] = *subject.Aggregate.Payload
				return x.block(statement.Consequence)
			}
			return x.block(statement.Alternative)
		}
		condition, ok := x.condition(statement.Condition)
		if !ok {
			return cteNormal, DefaultConstant{}
		}
		if condition {
			return x.block(statement.Consequence)
		}
		return x.block(statement.Alternative)
	case *ast.WhileStatement:
		for {
			condition, ok := x.condition(statement.Condition)
			if !ok || !condition {
				return cteNormal, DefaultConstant{}
			}
			control, value := x.block(statement.Body)
			if x.failure != nil {
				return cteNormal, DefaultConstant{}
			}
			switch control {
			case cteReturn:
				return control, value
			case cteBreak:
				return cteNormal, DefaultConstant{}
			}
		}
	case *ast.ForStatement:
		return x.forRange(statement)
	case *ast.SwitchStatement:
		return x.switchStatement(statement)
	case *ast.MatchStatement:
		if statement.Match == nil {
			return cteNormal, DefaultConstant{}
		}
		control, value, _ := x.matchValue(statement.Match, true)
		return control, value
	case *ast.FallthroughStatement:
		return cteFallthrough, DefaultConstant{}
	case *ast.BreakStatement:
		return cteBreak, DefaultConstant{}
	case *ast.ContinueStatement:
		return cteContinue, DefaultConstant{}
	case *ast.AssertStatement:
		condition, ok := x.condition(statement.Condition)
		if ok && !condition {
			message := ""
			if statement.Message != nil {
				message = ": " + statement.Message.Value
			}
			x.fail(cteEvaluationFailed, statement.Token, "assertion %s failed%s", statement.Condition.String(), message)
		}
		return cteNormal, DefaultConstant{}
	case *ast.PanicStatement:
		message := ""
		if statement.Message != nil {
			message = ": " + statement.Message.Value
		}
		x.fail(cteEvaluationFailed, statement.Token, "panic reached%s", message)
		return cteNormal, DefaultConstant{}
	}
	x.fail(cteImplementationGap, statementToken(statement), "%s is not executed by the compile-time evaluator yet", strings.TrimPrefix(fmt.Sprintf("%T", statement), "*ast."))
	return cteNormal, DefaultConstant{}
}

func (x *cteExecutor) condition(expr ast.Expression) (bool, bool) {
	value, ok := x.eval(expr)
	if !ok {
		return false, false
	}
	if value.Kind != BoolType {
		x.fail(cteImplementationGap, expressionToken(expr), "condition %s did not evaluate to bool", expr.String())
		return false, false
	}
	return value.Bool, true
}

// assign updates a local binding of the current frame; any other target is
// mutation of state outside the evaluation and therefore forbidden or not
// executed yet (§§ 20(2), 21(1)).
func (x *cteExecutor) assign(statement *ast.AssignmentStatement) {
	value, ok := x.eval(statement.Value)
	if !ok {
		return
	}
	if statement.Operator != "=" {
		current, ok := x.eval(statement.Target)
		if !ok {
			return
		}
		operator := strings.TrimSuffix(statement.Operator, "=")
		value, ok = x.binary(operator, current, value, statement.Target, statement.Token)
		if !ok {
			return
		}
	}
	x.store(statement.Target, value, statement.Token)
}

// store writes a value to a local binding, a field path of a local struct
// value or the current receiver; any other Place is state outside this
// evaluation (§§ 20(2), 21(1)).
func (x *cteExecutor) store(target ast.Expression, value DefaultConstant, at lexer.Token) bool {
	root, path := memberPath(target)
	identifier, ok := root.(*ast.Identifier)
	if !ok {
		x.fail(cteImplementationGap, at, "assignment to %s is not executed by the compile-time evaluator yet", target.String())
		return false
	}
	if receiver := x.frame().receiver; receiver != nil {
		place, resolved := x.a.ResolvedPlaceOf(identifier)
		if identifier.Value == "self" || resolved && place.RootKind == PlaceRootReceiver {
			full := path
			if identifier.Value != "self" {
				prefix := []string{}
				for _, projection := range place.Projections {
					prefix = append(prefix, projection.Name)
				}
				if len(place.Projections) == 0 {
					prefix = []string{identifier.Value}
				}
				full = append(prefix, path...)
			}
			updated, ok := x.withField(*receiver, full, value, at)
			if !ok {
				return false
			}
			*receiver = updated
			return true
		}
	}
	declaration, found := x.declarationOf(identifier.Token)
	current, local := x.frame().values[declaration]
	if !found || !local {
		x.fail(cteForbiddenEffect, identifier.Token, "assignment to %s mutates state outside this evaluation; semantic CTE never mutates static state", target.String())
		return false
	}
	updated, ok := x.withField(current, path, value, at)
	if !ok {
		return false
	}
	x.frame().values[declaration] = updated
	return true
}

// forRange executes `for name in start..end` (optionally `step`) over
// integers, binding the loop name per iteration.
func (x *cteExecutor) forRange(statement *ast.ForStatement) (cteControl, DefaultConstant) {
	rangeExpr, isRange := statement.Iterable.(*ast.RangeExpression)
	if !isRange || len(statement.Bindings) != 1 || rangeExpr.Start == nil || rangeExpr.End == nil {
		x.fail(cteImplementationGap, statement.Token, "this for loop form is not executed by the compile-time evaluator yet")
		return cteNormal, DefaultConstant{}
	}
	start, ok := x.eval(rangeExpr.Start)
	if !ok {
		return cteNormal, DefaultConstant{}
	}
	end, ok := x.eval(rangeExpr.End)
	if !ok {
		return cteNormal, DefaultConstant{}
	}
	step := big.NewInt(1)
	if statement.Step != nil {
		value, ok := x.eval(statement.Step)
		if !ok {
			return cteNormal, DefaultConstant{}
		}
		if value.Integer == nil || value.Integer.Sign() <= 0 {
			x.fail(cteEvaluationFailed, statement.StepToken, "for-loop step must be a positive integer")
			return cteNormal, DefaultConstant{}
		}
		step = value.Integer
	}
	if start.Integer == nil || end.Integer == nil {
		x.fail(cteImplementationGap, statement.Token, "non-integer ranges are not executed by the compile-time evaluator yet")
		return cteNormal, DefaultConstant{}
	}
	binding := statement.Bindings[0]
	for current := new(big.Int).Set(start.Integer); ; current = new(big.Int).Add(current, step) {
		compare := current.Cmp(end.Integer)
		if compare > 0 || rangeExpr.Exclusive && compare == 0 {
			return cteNormal, DefaultConstant{}
		}
		if !binding.Discard {
			x.frame().values[sourceTokenLocation(binding.Token)] = DefaultConstant{Kind: IntType, Lexeme: current.String(), Integer: new(big.Int).Set(current)}
		}
		control, value := x.block(statement.Body)
		if x.failure != nil {
			return cteNormal, DefaultConstant{}
		}
		switch control {
		case cteReturn:
			return control, value
		case cteBreak:
			return cteNormal, DefaultConstant{}
		}
	}
}

// switchStatement evaluates the subject once, tests value cases in source
// order and runs the first matching body; there is no implicit fallthrough,
// explicit `fallthrough` enters the next case body, and break/continue keep
// their loop meaning (flowcontrol_switch.md §§ 15, 16, 19).
func (x *cteExecutor) switchStatement(statement *ast.SwitchStatement) (cteControl, DefaultConstant) {
	subject, ok := x.eval(statement.Subject)
	if !ok {
		return cteNormal, DefaultConstant{}
	}
	cases := append([]*ast.SwitchCase(nil), statement.Cases...)
	if statement.Default != nil {
		cases = append(cases, statement.Default)
	}
	selected := -1
	for index, switchCase := range cases {
		if switchCase == nil || switchCase.Default {
			continue
		}
		for _, item := range switchCase.Items {
			valueCase, ok := item.(*ast.SwitchValueCase)
			if !ok {
				x.fail(cteImplementationGap, switchCase.Token, "this switch case form is not executed by the compile-time evaluator yet")
				return cteNormal, DefaultConstant{}
			}
			value, ok := x.eval(valueCase.Value)
			if !ok {
				return cteNormal, DefaultConstant{}
			}
			equal, outcome := x.a.compileTimeBinary("==", subject, value)
			if outcome != compileTimeEvaluated || equal.Kind != BoolType {
				x.fail(cteImplementationGap, valueCase.Token, "comparison of the switch subject with %s is not executed by the compile-time evaluator yet", valueCase.Value.String())
				return cteNormal, DefaultConstant{}
			}
			if equal.Bool {
				selected = index
				break
			}
		}
		if selected >= 0 {
			break
		}
	}
	if selected < 0 {
		for index, switchCase := range cases {
			if switchCase != nil && switchCase.Default {
				selected = index
			}
		}
	}
	for index := selected; index >= 0 && index < len(cases); index++ {
		control, value := x.block(cases[index].Body)
		if x.failure != nil || control != cteFallthrough {
			return control, value
		}
	}
	return cteNormal, DefaultConstant{}
}

// staticValue reads a static binding during semantic CTE: an immutable static
// contributes its compile-time initial value, evaluated through its own
// initializer when it is not established yet; a mutable static's runtime
// contents are never CTE state (§ 21(1)–(2)); needing a value that is being
// established is a compile-time value dependency cycle (§ 39).
func (x *cteExecutor) staticValue(identifier *ast.Identifier, declaration sourceTokenKey, static ResolvedStatic) (DefaultConstant, compileTimeOutcome) {
	if static.Mutable {
		x.fail(cteNotEvaluable, identifier.Token, "%s is a mutable static; semantic CTE never reads mutable static state", identifier.Value)
		return DefaultConstant{}, compileTimeNotConstant
	}
	if static.Declaration == nil || static.Declaration.Value == nil {
		x.fail(cteNotEvaluable, identifier.Token, "%s has no compile-time initializer", identifier.Value)
		return DefaultConstant{}, compileTimeNotConstant
	}
	if x.visiting[declaration] {
		chain := append(append([]string(nil), x.statics...), identifier.Value)
		x.fail(cteDependencyCycle, identifier.Token, "the value of %s depends on itself: %s", identifier.Value, strings.Join(chain, " -> "))
		return DefaultConstant{}, compileTimeNotConstant
	}
	if value, outcome, established := x.a.establishedCompileTimeValue(static.Declaration.Value); established && outcome == compileTimeEvaluated {
		return value, outcome
	}
	x.visiting[declaration] = true
	x.statics = append(x.statics, identifier.Value)
	frames := x.frames
	x.frames = []*cteFrame{{values: map[sourceTokenKey]DefaultConstant{}}}
	value, ok := x.eval(static.Declaration.Value)
	x.frames = frames
	x.statics = x.statics[:len(x.statics)-1]
	delete(x.visiting, declaration)
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	value, ok = x.inDomain(value, static.Type, identifier.Token, "static initial value out of range")
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant
	}
	return value, compileTimeEvaluated
}

// getter executes a static property getter Sema resolved for a member read.
// An instance getter needs a receiver value, which the evaluator does not
// represent yet.
func (x *cteExecutor) getter(expr *ast.MemberExpression) (DefaultConstant, bool) {
	access, ok := x.a.resolvedPropertyAccesses[expr]
	if !ok || access.Kind != PropertyRead {
		x.fail(cteImplementationGap, expressionToken(expr), "member read %s is not executed by the compile-time evaluator yet", expr.String())
		return DefaultConstant{}, false
	}
	var receiver *DefaultConstant
	if !access.Static {
		object, ok := x.eval(expr.Object)
		if !ok {
			return DefaultConstant{}, false
		}
		receiver = &object
	}
	property, found := lookupProperty(access.OwnerType, access.Name)
	if !found || property.GetterCallable == nil {
		x.fail(cteImplementationGap, expressionToken(expr), "property %s has no getter body available to this compile-time evaluation", expr.String())
		return DefaultConstant{}, false
	}
	body := x.getters[sourceTokenLocation(property.GetterCallable.Token)]
	if body == nil {
		x.fail(cteImplementationGap, expressionToken(expr), "the getter of %s is not available to this compile-time evaluation", expr.String())
		return DefaultConstant{}, false
	}
	if len(x.frames) >= cteMaxCallDepth {
		x.fail(cteResourceLimit, expressionToken(expr), "the CallDepth budget of %d nested calls was reached", cteMaxCallDepth)
		return DefaultConstant{}, false
	}
	name := typeDisplayName(access.OwnerType) + "." + access.Name
	x.frames = append(x.frames, &cteFrame{values: map[sourceTokenKey]DefaultConstant{}, receiver: receiver})
	x.calls = append(x.calls, cteCallSite{Function: name, Call: expressionToken(expr)})
	control, value := x.block(body)
	x.calls = x.calls[:len(x.calls)-1]
	x.frames = x.frames[:len(x.frames)-1]
	if x.failure != nil {
		return DefaultConstant{}, false
	}
	if control != cteReturn {
		x.fail(cteImplementationGap, expressionToken(expr), "getter %s finished without returning a value", name)
		return DefaultConstant{}, false
	}
	return x.inDomain(value, property.Type, expressionToken(expr), "getter result out of range")
}
