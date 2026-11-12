package sema

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// cteFailureKind is the compiler-semantic outcome category of a required
// compile-time evaluation that did not establish a value
// (compile_time_evaluation.md §§ 53–56). These are not Sec types.
type cteFailureKind string

const (
	// cteResultNotMaterializable: execution succeeded, but the complete
	// returned graph has no legal static representation (§§30,36,54).
	cteResultNotMaterializable cteFailureKind = "ResultNotMaterializable"
	// cteNotEvaluable: the executed path reads state that is not a
	// compile-time value (for example a mutable static).
	cteNotEvaluable cteFailureKind = "NotEvaluable"
	// cteEvaluationFailed: a reached assert, panic or semantic operation
	// failure (SemanticOperationFailed: overflow, division by zero, checked
	// conversion, contract violation).
	cteEvaluationFailed cteFailureKind = "EvaluationFailed"
	// cteSemanticOperationFailed: a language-defined operation failure such
	// as checked overflow, division by zero or an out-of-range conversion
	// (§ 47(4)); not a Sec error type.
	cteSemanticOperationFailed cteFailureKind = "SemanticOperationFailed"
	// cteCompilerInvariantFailure: the evaluator itself failed; this is a
	// compiler bug, never a property of the source (§ 57).
	cteCompilerInvariantFailure cteFailureKind = "CompilerInvariantFailure"
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
	// Module is the callee's origin module and Instantiation the concrete
	// generic arguments of a generic callee, for provenance (§ 59).
	Module        string
	Instantiation string
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
	// Type is the canonical Sec type of the established value: the
	// resolved type of the root expression, or a static's declared type.
	Type Type
	// Dependencies are the conservative inputs of the outcome (§ 58).
	Dependencies []CompileTimeDependency
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
	// moved marks owned bindings whose responsibility a move or discard
	// ended; cleanups is the invocation's unified cleanup stack; scopes are
	// the active lexical scopes; pending are bindings the next scope owns
	// (cte_ownership.go).
	moved    map[sourceTokenKey]bool
	aliases  map[sourceTokenKey]ast.Expression
	cleanups []cteCleanup
	scopes   []*cteScope
	pending  []sourceTokenKey
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
	// transient counts evaluator-owned scalar cells and UTF-8 bytes allocated
	// during this request, independent of target runtime allocation.
	transient int
	// dependencies collects the request's dependencies, one set per active
	// call; rootModule is the module the request belongs to.
	dependencies []map[CompileTimeDependency]bool
	rootModule   string
	// ownership tracks custom-free values and their destruction.
	ownership *cteOwnership
	// program is the analyzed program; results are the outcomes of this
	// analysis's requests, shared with facts established on demand;
	// inProgress names the type facts enclosing evaluations establish.
	program    *ast.Program
	results    map[ast.Expression]cteResult
	inProgress []string
	// patternBindings are the bindings the current arm's pattern made.
	patternBindings []sourceTokenKey
}

func (x *cteExecutor) currentFunctionReturn() Type {
	if len(x.returns) == 0 {
		return Type{}
	}
	return x.returns[len(x.returns)-1]
}

func newCTEExecutor(a *Analyzer, program *ast.Program) *cteExecutor {
	x := &cteExecutor{a: a, program: program, functions: map[sourceTokenKey]*ast.FunctionDeclaration{}, getters: map[sourceTokenKey]*ast.BlockStatement{}, active: map[*ast.FunctionDeclaration]int{}, visiting: map[sourceTokenKey]bool{}}
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
				case *ast.FreeDeclaration:
					if member != nil && member.Body != nil {
						x.ownershipState().freeBody[sourceTokenLocation(member.Token)] = member.Body
					}
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
func (x *cteExecutor) evaluate(root ast.Expression) (result cteResult) {
	// An evaluator fault is a compiler invariant failure, reported as such
	// and never as a property of the source (§ 57).
	defer func() {
		if recovered := recover(); recovered != nil {
			result = cteResult{Failure: &cteFailure{Kind: cteCompilerInvariantFailure, Reason: fmt.Sprintf("internal compile-time evaluator failure: %v", recovered), At: expressionToken(root), CallStack: append([]cteCallSite(nil), x.calls...)}}
		}
	}()
	x.frames = []*cteFrame{newCTEFrame(nil)}
	x.rootModule = x.a.currentModule
	if static, ok := x.a.staticBindingOfInitializer(root); ok && static.Module != "" {
		x.rootModule = static.Module
	}
	x.dependencies = []map[CompileTimeDependency]bool{{}}
	defer func() {
		result.Dependencies = sortedDependencies(x.dependencies[0])
	}()
	rootType, _ := x.a.ResolvedTypeOf(root)
	x.dependOnType(rootType)
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
	// Validate before any recursive ownership accounting, so an unsupported
	// cyclic result cannot reach a tree-only cleanup or serialization walk.
	// Rules: rules/compiler/compile_time_evaluation.md — §§30,36,57.
	prepared := finishCTEMaterialization(cteResult{Value: value, Type: rootType})
	if prepared.Failure != nil {
		prepared.Failure.At = expressionToken(root)
		return prepared
	}
	if leaked, found := x.unaccountedOwnership(value); found {
		return cteResult{Failure: &cteFailure{Kind: cteImplementationGap, Reason: "a temporary " + typeDisplayName(leaked) + " with a custom free is destroyed at a point the compile-time evaluator does not model yet", At: expressionToken(root)}}
	}
	return prepared
}

// eval evaluates one expression through the shared compile-time evaluator
// with executor-owned identifier and expression resolvers.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,24,48–49,51(3).
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
		if _, literal := expr.(*ast.StringLiteral); literal && !x.reserveStringStorage(len(value.String), 0, expressionToken(expr)) {
			return DefaultConstant{}, false
		}
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
		if alias, aliased := x.frame().aliases[declaration]; aliased {
			value, ok := x.eval(alias)
			if !ok {
				return DefaultConstant{}, compileTimeNotConstant
			}
			return value, compileTimeEvaluated
		}
		if value, bound := x.frame().values[declaration]; bound {
			x.noteMove(identifier, value)
			return value, compileTimeEvaluated
		}
		if static, isStatic := x.a.staticBindings[declaration]; isStatic {
			return x.staticValue(identifier, declaration, static)
		}
	}
	value, outcome := x.a.semanticCompileTimeConstantVisiting(identifier, map[string]bool{}, Type{})
	if outcome == compileTimeEvaluated {
		x.depend(CTEDependsOnImmutableValue, identifier.Value)
		return value, outcome
	}
	x.fail(cteNotEvaluable, identifier.Token, "%s is not a compile-time value; semantic CTE reads only locals, parameters and immutable compile-time-established values, never mutable static state", identifier.Value)
	return DefaultConstant{}, compileTimeNotConstant
}

// expression handles the forms the shared literal/operator evaluator does
// not execute: calls, conversions and checked arithmetic.
func (x *cteExecutor) expression(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	if reason, at, forbidden := x.forbiddenOperation(expr); forbidden {
		x.fail(cteForbiddenEffect, at, "%s, which semantic CTE never performs", reason)
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	x.dependOnLayout(expr)
	if value, ok, handled := x.variantExpression(expr); handled {
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
	}
	switch expr := expr.(type) {
	case *ast.PrefixExpression:
		if expr.Operator != "<-" || expr.Right == nil {
			break
		}
		// An explicit move transfers the Place's value; the source's
		// responsibility ends where Sema accepted the transfer.
		value, ok := x.eval(expr.Right)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, true
		}
		return value, compileTimeEvaluated, true
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
			x.noteMove(expr, value)
			// A field read of a temporary ends the temporary with its full
			// expression; the field itself is detached when moved out.
			if !isPlaceExpression(expr.Object) {
				x.destroyValue(object, true, expressionToken(expr))
				if x.failure != nil {
					return DefaultConstant{}, compileTimeNotConstant, true
				}
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

// binary executes a typed binary operation, reserving transient text storage
// before concatenation and preserving checked arithmetic semantics.
// Rules: rules/compiler/compile_time_evaluation.md — §§20,24,33,47–49;
// rules/foundations/operators.md — String concatenation.
func (x *cteExecutor) binary(operator string, left, right DefaultConstant, typed ast.Expression, at lexer.Token) (DefaultConstant, bool) {
	if (operator == "/" || operator == "%") && right.Integer != nil && right.Integer.Sign() == 0 {
		x.fail(cteSemanticOperationFailed, at, "division by zero")
		return DefaultConstant{}, false
	}
	if operator == "+" && (left.Kind == StringType || left.Kind == CharType || left.Kind == RuneType) &&
		(right.Kind == StringType || right.Kind == CharType || right.Kind == RuneType) && !left.NominalText && !right.NominalText {
		if !x.reserveStringStorage(len(left.String), len(right.String), at) {
			return DefaultConstant{}, false
		}
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
	x.dependOnType(typ)
	if value.Integer == nil {
		return value, true
	}
	low, high, integer := scalarIntegerDomain(typ)
	if integer && (value.Integer.Cmp(low) < 0 || value.Integer.Cmp(high) > 0) {
		x.fail(cteSemanticOperationFailed, at, "%s: %s does not fit %s", what, value.Integer.String(), typeDisplayName(typ))
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
	x.dependOnType(target)
	if hasContracts(target) {
		// Outside try only a conversion Sema proved valid remains; a
		// failure here is a semantic operation failure.
		call, isCall := expr.(*ast.CallExpression)
		if !isCall || len(call.Arguments) != 1 {
			x.fail(cteImplementationGap, expressionToken(expr), "checked conversion into constrained type %s is not executed by the compile-time evaluator yet", typeDisplayName(target))
			return DefaultConstant{}, false
		}
		converted, failure, ok := x.checkConversion(call, target, value)
		if !ok {
			return DefaultConstant{}, false
		}
		if failure != nil {
			x.fail(cteSemanticOperationFailed, expressionToken(expr), "conversion %s failed with ConversionError.%s", expr.String(), renderCompileTimeValue(*failure))
			return DefaultConstant{}, false
		}
		return converted, true
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
	case len(function.GenericParameters) > 0 && function.GenericSubstitution == nil:
		x.fail(cteImplementationGap, call.Token, "generic call %s has no concrete instantiation to execute", function.Name)
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
	// A returned reference must keep aliasing its Place in the caller; the
	// executor fails closed rather than return a copy that would silently
	// lose writes through it (compile_time_evaluation.md §§ 28, 56).
	if function.ReturnType.Kind == ReferenceType {
		x.fail(cteImplementationGap, call.Token, "%s returns a reference, which the compile-time evaluator does not execute yet", function.Name)
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
	frame := newCTEFrame(nil)
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
	// A reference parameter binds the argument Place's value; a `ref mut`
	// parameter's final value is written back to that Place when the call
	// returns. Borrowing rules exclude every other access to the Place
	// during the call, so this is exact (compile_time_evaluation.md § 28).
	type cteWriteBack struct {
		parameter sourceTokenKey
		place     ast.Expression
	}
	writeBacks := []cteWriteBack{}
	for index, argument := range call.Arguments {
		parameter := function.Parameters[index]
		if parameter.Variadic {
			x.fail(cteImplementationGap, expressionToken(argument), "variadic parameter %s is not executed by the compile-time evaluator yet", parameter.Name)
			return DefaultConstant{}, false
		}
		reference := parameter.Ref || parameter.MutableRef || parameter.Type.Kind == ReferenceType
		mutable := parameter.MutableRef || parameter.Type.Kind == ReferenceType && parameter.Type.ReferenceMutable
		if reference {
			place := argument
			if reference, ok := argument.(*ast.RefExpression); ok {
				place = reference.Value
			}
			if !isPlaceExpression(place) {
				// Sema rejects a borrow of a temporary.
				x.fail(cteImplementationGap, expressionToken(argument), "reference argument %s does not name a local Place the compile-time evaluator can bind", argument.String())
				return DefaultConstant{}, false
			}
			if mutable {
				writeBacks = append(writeBacks, cteWriteBack{parameter: sourceTokenLocation(parameter.Token), place: place})
			}
			argument = place
		}
		value, ok := x.eval(argument)
		if !ok {
			return DefaultConstant{}, false
		}
		frame.values[sourceTokenLocation(parameter.Token)] = value
	}
	// A call without a receiver depends only on its arguments and immutable
	// compile-time values, so its successful result is reused within this
	// analysis; failures are never cached (§§ 49(4), 60).
	memoKey := ""
	ownedArgument := false
	for _, value := range frame.values {
		ownedArgument = ownedArgument || len(ownedIdentities(value, nil)) > 0
	}
	if frame.receiver == nil && len(writeBacks) == 0 && !ownedArgument {
		memoKey = x.memoKey(function, call.Arguments, frame)
		if value, hit := x.a.cteMemo[memoKey]; hit && memoKey != "" {
			x.mergeDependencies(x.a.cteMemoDependencies[memoKey])
			return value, true
		}
	}
	x.frames = append(x.frames, frame)
	x.calls = append(x.calls, cteCallSite{Function: function.Name, Call: call.Token, Module: function.Module, Instantiation: genericInstantiationText(function)})
	returnType := function.ReturnType
	if function.GenericSubstitution != nil {
		returnType = substituteGenericType(returnType, function.GenericSubstitution)
	}
	x.returns = append(x.returns, returnType)
	x.pushDependencies()
	x.depend(CTEDependsOnDeclaration, string(callableID(function)))
	if instantiation := genericInstantiationText(function); instantiation != "" {
		x.depend(CTEDependsOnGenericInstance, function.Module+"."+function.Name+"["+instantiation+"]")
	}
	x.dependOnModule(function.Module)
	x.active[declaration]++
	for _, parameter := range function.Parameters {
		if !parameter.Ref && !parameter.MutableRef && parameter.Type.Kind != ReferenceType {
			x.registerBinding(sourceTokenLocation(parameter.Token), substituteGenericType(parameter.Type, function.GenericSubstitution))
		}
	}
	control, value := x.invocation(declaration.Body)
	x.active[declaration]--
	callDependencies := x.popDependencies()
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
		} else if !x.writeBack(receiverExpr, *frame.receiver, call.Token) {
			return DefaultConstant{}, false
		}
	}
	for _, back := range writeBacks {
		if !x.writeBack(back.place, frame.values[back.parameter], call.Token) {
			return DefaultConstant{}, false
		}
	}
	// A receiver that was a temporary ends with the call's full
	// expression (destruction.md § 8).
	if frame.receiver != nil && receiverExpr != nil && !isPlaceExpression(receiverExpr) {
		x.destroyValue(*frame.receiver, true, call.Token)
	}
	if x.failure != nil {
		return DefaultConstant{}, false
	}
	if control != cteReturn {
		if function.ReturnType.Kind == VoidType || function.ReturnType.Kind == InvalidType && function.ReturnType.Name == "" {
			return DefaultConstant{Kind: VoidType}, true
		}
		x.fail(cteImplementationGap, call.Token, "%s finished without returning a value", function.Name)
		return DefaultConstant{}, false
	}
	value, ok = x.inDomain(value, returnType, call.Token, "returned value out of range")
	// A result holding a custom-free value has an identity of its own and is
	// never reused.
	if ok && memoKey != "" && len(ownedIdentities(value, nil)) == 0 {
		if x.a.cteMemo == nil {
			x.a.cteMemo = map[string]DefaultConstant{}
		}
		x.a.cteMemo[memoKey] = value
		if x.a.cteMemoDependencies == nil {
			x.a.cteMemoDependencies = map[string][]CompileTimeDependency{}
		}
		x.a.cteMemoDependencies[memoKey] = callDependencies
	}
	return value, ok
}

// block executes a nested block as its own lexical scope: its owned
// bindings are destroyed when it ends, its defers stay registered.
func (x *cteExecutor) block(block *ast.BlockStatement) (cteControl, DefaultConstant) {
	if block == nil {
		return cteNormal, DefaultConstant{}
	}
	x.enterScope()
	control, value := x.blockStatements(block)
	x.leaveScope()
	return control, value
}

// blockStatements executes statements in order until a control transfer.
func (x *cteExecutor) blockStatements(block *ast.BlockStatement) (cteControl, DefaultConstant) {
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
	if reason, at, forbidden := forbiddenStatement(statement); forbidden {
		x.fail(cteForbiddenEffect, at, "%s, which semantic CTE never performs", reason)
		return cteNormal, DefaultConstant{}
	}
	switch statement := statement.(type) {
	case *ast.CommentStatement:
		return cteNormal, DefaultConstant{}
	case *ast.ExpressionStatement:
		x.eval(statement.Expression)
		return cteNormal, DefaultConstant{}
	case *ast.DiscardStatement:
		// discard ends the value's ownership now (destruction.md § 12).
		if statement.Value != nil {
			if value, ok := x.eval(statement.Value); ok {
				if identifier, named := statement.Value.(*ast.Identifier); named {
					if declaration, found := x.declarationOf(identifier.Token); found {
						x.frame().moved[declaration] = true
					}
				}
				x.destroyValue(value, true, statement.Token)
			}
		}
		return cteNormal, DefaultConstant{}
	case *ast.DeferStatement:
		x.registerDefer(statement.Body)
		return cteNormal, DefaultConstant{}
	case *ast.LetStatement:
		if statement.Static || statement.Value == nil || statement.Name == nil {
			x.fail(cteImplementationGap, statement.Token, "this let form is not executed by the compile-time evaluator yet")
			return cteNormal, DefaultConstant{}
		}
		// `let r := ref x` / `ref mut x` binds a reference to a local Place:
		// reads and writes through r reach the Place itself (§ 28(3)).
		if reference, isRef := statement.Value.(*ast.RefExpression); isRef && reference.Value != nil && isPlaceExpression(reference.Value) {
			binding := sourceTokenLocation(statement.Name.Token)
			if x.frame().aliases == nil {
				x.frame().aliases = map[sourceTokenKey]ast.Expression{}
			}
			x.frame().aliases[binding] = reference.Value
			x.declare(binding)
			return cteNormal, DefaultConstant{}
		}
		// Copying a reference binding aliases the same Place; any other
		// reference-typed value has no evaluator representation yet.
		if valueType, typed := x.a.ResolvedTypeOf(statement.Value); typed && valueType.Kind == ReferenceType {
			if identifier, named := statement.Value.(*ast.Identifier); named {
				key, found := x.declarationOf(identifier.Token)
				if alias, aliased := x.frame().aliases[key]; found && aliased {
					binding := sourceTokenLocation(statement.Name.Token)
					x.frame().aliases[binding] = alias
					x.declare(binding)
					return cteNormal, DefaultConstant{}
				}
			}
			// A call reports why its reference result has no representation.
			if call, isCall := statement.Value.(*ast.CallExpression); isCall {
				x.eval(call)
			}
			x.fail(cteImplementationGap, statement.Token, "this reference-valued binding is not executed by the compile-time evaluator yet")
			return cteNormal, DefaultConstant{}
		}
		value, ok := x.eval(statement.Value)
		if ok {
			binding := sourceTokenLocation(statement.Name.Token)
			x.frame().values[binding] = value
			x.declare(binding)
			typ, _ := x.a.ResolvedTypeOf(statement.Value)
			x.registerBinding(binding, typ)
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
	return x.storeValue(target, value, at, true)
}

// writeBack updates a Place through which a call mutated its value (a
// `ref mut` argument or a mutating receiver). Parts of the old value that
// the new value no longer holds were replaced inside the call and are
// destroyed; parts it still holds stay alive.
func (x *cteExecutor) writeBack(target ast.Expression, value DefaultConstant, at lexer.Token) bool {
	if old, ok := x.placeValue(target); ok {
		x.destroyReplaced(old, value, at)
		if x.failure != nil {
			return false
		}
	}
	return x.storeValue(target, value, at, false)
}

func (x *cteExecutor) storeValue(target ast.Expression, value DefaultConstant, at lexer.Token, replace bool) bool {
	root, path := memberPath(target)
	identifier, ok := root.(*ast.Identifier)
	if !ok {
		x.fail(cteImplementationGap, at, "assignment to %s is not executed by the compile-time evaluator yet", target.String())
		return false
	}
	return x.storePlace(identifier, path, value, at, replace, target)
}

// storePlace writes through a binding or a field path of it. A local
// reference binding writes through to the Place it refers to.
func (x *cteExecutor) storePlace(identifier *ast.Identifier, path []string, value DefaultConstant, at lexer.Token, replace bool, target ast.Expression) bool {
	if declaration, found := x.declarationOf(identifier.Token); found {
		if alias, aliased := x.frame().aliases[declaration]; aliased {
			aliasRoot, aliasPath := memberPath(alias)
			if root, ok := aliasRoot.(*ast.Identifier); ok {
				return x.storePlace(root, append(append([]string(nil), aliasPath...), path...), value, at, replace, target)
			}
		}
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
	frame := x.frame()
	if !replace {
		// The caller already destroyed what the call replaced.
	} else if len(path) == 0 && frame.registered(declaration) {
		// Replacing an available owned value destroys the old value and keeps
		// the registration; reinitializing a moved binding registers anew
		// (destruction.md §§ 6.1(4)–(5), 13).
		if frame.moved[declaration] {
			frame.values[declaration] = value
			typ, _ := x.a.ResolvedTypeOf(identifier)
			x.registerBinding(declaration, typ)
			return true
		}
		x.destroyValue(current, true, at)
		if x.failure != nil {
			return false
		}
	} else if len(path) > 0 {
		if old, ok := fieldAtPath(current, path); ok {
			x.destroyValue(old, false, at)
			if x.failure != nil {
				return false
			}
		}
	}
	updated, ok := x.withField(current, path, value, at)
	if !ok {
		return false
	}
	frame.values[declaration] = updated
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
			x.fail(cteSemanticOperationFailed, statement.StepToken, "for-loop step must be a positive integer")
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
			x.frame().pending = []sourceTokenKey{sourceTokenLocation(binding.Token)}
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
	x.depend(CTEDependsOnImmutableValue, static.Module+"."+static.Name)
	x.dependOnModule(static.Module)
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
	x.frames = []*cteFrame{newCTEFrame(nil)}
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
	if property.Type.Kind == ReferenceType {
		x.fail(cteImplementationGap, expressionToken(expr), "getter %s returns a reference, which the compile-time evaluator does not execute yet", expr.String())
		return DefaultConstant{}, false
	}
	getterAt := property.GetterCallable.Token
	x.depend(CTEDependsOnDeclaration, fmt.Sprintf("getter %s.%s at %s:%d:%d", typeDisplayName(access.OwnerType), access.Name, getterAt.File, getterAt.Line, getterAt.Column))
	x.dependOnType(access.OwnerType)
	if body == nil {
		x.fail(cteImplementationGap, expressionToken(expr), "the getter of %s is not available to this compile-time evaluation", expr.String())
		return DefaultConstant{}, false
	}
	if len(x.frames) >= cteMaxCallDepth {
		x.fail(cteResourceLimit, expressionToken(expr), "the CallDepth budget of %d nested calls was reached", cteMaxCallDepth)
		return DefaultConstant{}, false
	}
	name := typeDisplayName(access.OwnerType) + "." + access.Name
	x.frames = append(x.frames, newCTEFrame(receiver))
	x.calls = append(x.calls, cteCallSite{Function: name, Call: expressionToken(expr)})
	control, value := x.invocation(body)
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

// memoKey identifies a receiver-free call by callable identity and the
// rendered argument values in parameter order.
func (x *cteExecutor) memoKey(function Function, arguments []ast.Expression, frame *cteFrame) string {
	parts := []string{string(callableID(function))}
	for _, parameter := range function.Parameters {
		value, ok := frame.values[sourceTokenLocation(parameter.Token)]
		if !ok {
			return ""
		}
		parts = append(parts, string(value.Kind)+":"+renderCompileTimeValue(value))
	}
	return strings.Join(parts, "\x00")
}

// genericInstantiationText spells a generic callee's concrete arguments in
// declaration order, for example `T = int`.
func genericInstantiationText(function Function) string {
	if function.GenericSubstitution == nil {
		return ""
	}
	names := make([]string, 0, len(function.GenericSubstitution))
	for name := range function.GenericSubstitution {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := []string{}
	for _, name := range names {
		parts = append(parts, name+" = "+typeDisplayName(function.GenericSubstitution[name]))
	}
	return strings.Join(parts, ", ")
}
