package sema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// ctePendingRequest is a required semantic CTE position whose value needs
// execution of a call or getter (compile_time_evaluation.md § 50): the root
// expression and the module whose scope it belongs to.
type ctePendingRequest struct {
	Root   ast.Expression
	Module string
	File   string
	// Fact is the type fact the request establishes, when it is a range
	// bound or a type default.
	Fact *cteFactRequest
}

// prepareCompileTimeEvaluation establishes the values of required positions
// that need execution before the analysis that consumes them. Type contracts
// and explicit defaults are analyzed before function bodies, but the executor
// must run bodies Sema has already analyzed (§ 51(3)). A probe analysis of
// the same program on a freshly configured analyzer therefore analyzes every
// body, records each pending request and executes it; this analysis then
// reads the established values. Programs without such a position pay nothing.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 6, 8(5)–(7), 50, 51.
func (a *Analyzer) prepareCompileTimeEvaluation(program *ast.Program) {
	a.cteResults = nil
	if a.cteProbe || !programRequestsSemanticCTE(program) {
		return
	}
	probe := a.freshConfiguredAnalyzer()
	probe.cteProbe = true
	// The probe must not change the AST this analysis reads.
	probe.legacyDefaultAST = false
	probe.Analyze(program)
	a.cteResults = probe.executePendingCompileTimeRequests(program)
}

// executePendingCompileTimeRequests types each pending root with ordinary Sema
// resolution in its module scope and executes it.
func (a *Analyzer) executePendingCompileTimeRequests(program *ast.Program) map[ast.Expression]cteResult {
	if len(a.ctePending) == 0 {
		return nil
	}
	results := map[ast.Expression]cteResult{}
	for _, request := range a.ctePending {
		if _, done := results[request.Root]; done {
			continue
		}
		previousModule, previousFile := a.currentModule, a.currentSourceFile
		a.currentModule, a.currentSourceFile = request.Module, request.File
		// A root the ordinary analysis already rejected (an unresolved or
		// ill-typed call) gets that diagnostic only, never a CTE follow-up.
		if a.errorsWithin(request.Root) {
			results[request.Root] = cteResult{Failure: &cteFailure{Kind: cteSuppressed}}
			a.currentModule, a.currentSourceFile = previousModule, previousFile
			continue
		}
		executor := newCTEExecutor(a, program)
		executor.results = results
		if request.Fact != nil {
			executor.inProgress = []string{"the " + request.Fact.Fact + " of " + request.Fact.TypeName}
		}
		if static, ok := a.staticBindingOfInitializer(request.Root); ok {
			results[request.Root] = executor.evaluateStatic(static.Name, sourceTokenLocation(static.Declaration.Name.Token), request.Root)
		} else {
			a.inferExpression(request.Root)
			results[request.Root] = executor.evaluate(request.Root)
		}
		a.currentModule, a.currentSourceFile = previousModule, previousFile
	}
	return results
}

// staticBindingOfInitializer finds the static whose initializer is root.
func (a *Analyzer) staticBindingOfInitializer(root ast.Expression) (ResolvedStatic, bool) {
	for _, static := range a.staticBindings {
		if static.Declaration != nil && static.Declaration.Value == root && static.Declaration.Name != nil {
			return static, true
		}
	}
	return ResolvedStatic{}, false
}

// deferCompileTimeExecution records a pending request in the probe.
func (a *Analyzer) deferCompileTimeExecution(expr ast.Expression) {
	request := ctePendingRequest{Root: expr, Module: a.currentModule, File: a.currentSourceFile}
	if a.cteFactContext != nil {
		fact := *a.cteFactContext
		request.Fact = &fact
	}
	a.ctePending = append(a.ctePending, request)
}

// establishedCompileTimeValue returns the probe's outcome for a root.
func (a *Analyzer) establishedCompileTimeValue(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	result, ok := a.cteResults[expr]
	if !ok {
		return DefaultConstant{}, compileTimeNotConstant, false
	}
	if result.Failure != nil {
		return DefaultConstant{}, compileTimeExecutionFailed, true
	}
	return result.Value, compileTimeEvaluated, true
}

// reportCompileTimeExecutionFailure diagnoses a required evaluation that
// executed but established no value, naming the failure category, the root
// expression, the reason and the executed call chain (§ 72).
func (a *Analyzer) reportCompileTimeExecutionFailure(expr ast.Expression, position string) bool {
	result, ok := a.cteResults[expr]
	if !ok || result.Failure == nil {
		return false
	}
	failure := result.Failure
	if failure.Kind == cteSuppressed {
		return true
	}
	id, help := diagnostics.SemanticCompileTimeExecutionUnavailable, "Use a literal or an immutable compile-time binding, or restructure the evaluated code into forms the compile-time evaluator executes."
	category := "is not executed by the compile-time evaluator yet"
	switch failure.Kind {
	case cteResultNotMaterializable:
		id, category = diagnostics.CompileTimeResultNotMaterializable, "was evaluated, but its result is not statically materializable"
		help = "The compile-time computation succeeded. Its complete stored value graph must have a defined static representation; transient addresses, backing storage and cyclic evaluator objects cannot escape into runtime storage."
	case cteEvaluationFailed, cteNotEvaluable:
		id, category = diagnostics.CompileTimeEvaluationFailed, "failed during compile-time evaluation"
		help = "The evaluation ran ordinary Sec semantics and reached a failure; fix the evaluated code or its arguments so the required value can be established."
		if failure.Kind == cteNotEvaluable {
			category = "is not compile-time evaluable"
			help = "Semantic CTE may read locals, parameters and immutable compile-time values only."
		}
	case cteSemanticOperationFailed:
		id, category = diagnostics.CompileTimeEvaluationFailed, "failed a semantic operation during compile-time evaluation"
		help = "The evaluated code reached a language-defined operation failure (SemanticOperationFailed), such as checked overflow, division by zero or an out-of-range conversion; fix the evaluated code or its arguments."
	case cteCompilerInvariantFailure:
		id, category = diagnostics.CompileTimeCompilerInvariantFailure, "stopped on an internal compiler failure during compile-time evaluation"
		help = "This is a compiler bug, not an error in the source. Please report it with this source."
	case cteForbiddenEffect:
		id, category = diagnostics.CompileTimeForbiddenEffect, "performs an effect forbidden during compile-time evaluation"
		help = "Compile-time evaluation runs ordinary Sec code without I/O, clocks, foreign calls or static mutation; move that effect to runtime code or keep it on a path this evaluation does not execute."
	case cteResourceLimit:
		id, category = diagnostics.CompileTimeResourceLimit, "reached a compile-time evaluation resource limit"
		help = "This is a compiler budget, not proof that the code never terminates; reduce the work done at compile time."
	case cteDependencyCycle:
		id, category = diagnostics.CompileTimeDependencyCycle, "depends on its own compile-time value"
		help = "Break the cycle so the value does not require itself."
	}
	chain := ""
	if names := compactCallChain(failure.CallStack, a.currentModule); len(names) > 0 {
		chain = " (in " + strings.Join(names, " -> ") + ")"
	}
	err := Error{ID: id, Severity: diagnostics.SeverityError, Help: help,
		Message: position + " " + expr.String() + " " + category + ": " + failure.Reason + chain}
	token := expressionToken(expr)
	err.File, err.Line, err.Column = token.File, token.Line, token.Column
	err.EndLine, err.EndColumn = token.EndPosition()
	if failure.At.Line > 0 && failure.At != token {
		err.PreviousFile, err.PreviousLine, err.PreviousColumn = failure.At.File, failure.At.Line, failure.At.Column
		err.RelatedLabel = "evaluation stopped here"
	}
	a.appendError(err)
	return true
}

// programRequestsSemanticCTE reports whether any type contract or explicit
// default contains a call or member read (a property getter), the positions
// whose required value may need execution. It inspects syntax only.
func programRequestsSemanticCTE(program *ast.Program) bool {
	if program == nil {
		return false
	}
	found := false
	var visitStatement func(ast.Statement)
	visitStatement = func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.TypeDeclStatement:
			if statement != nil && (containsExecution(statement.Contract) || containsExecution(statement.Default)) {
				found = true
			}
		case *ast.ImplStatement:
			if statement == nil {
				return
			}
			for _, member := range statement.Members {
				if declaration, ok := member.(*ast.TypeDeclStatement); ok {
					visitStatement(declaration)
				}
			}
		}
	}
	for _, statement := range program.Statements {
		visitStatement(statement)
		if found {
			return true
		}
	}
	// A contract or default that reads a module static whose initializer
	// executes also needs the probe.
	executing := map[string]bool{}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.LetStatement:
			if statement != nil && statement.Name != nil && containsExecution(statement.Value) {
				executing[statement.Name.Value] = true
			}
		case *ast.LetGroupStatement:
			if statement == nil {
				continue
			}
			for _, let := range statement.Lets {
				if let != nil && let.Name != nil && containsExecution(let.Value) {
					executing[let.Name.Value] = true
				}
			}
		}
	}
	if len(executing) == 0 {
		return false
	}
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.TypeDeclStatement); ok && declaration != nil &&
			(namesAnyIdentifier(declaration.Contract, executing) || namesAnyIdentifier(declaration.Default, executing)) {
			return true
		}
	}
	return false
}

var identifierType = reflect.TypeOf(&ast.Identifier{})

// namesAnyIdentifier walks a subtree for an identifier in names.
func namesAnyIdentifier(node any, names map[string]bool) bool {
	if node == nil {
		return false
	}
	var walk func(reflect.Value, int) bool
	walk = func(value reflect.Value, depth int) bool {
		if depth > 64 || !value.IsValid() {
			return false
		}
		switch value.Kind() {
		case reflect.Interface, reflect.Pointer:
			if value.IsNil() {
				return false
			}
			if value.Type() == identifierType {
				return names[value.Interface().(*ast.Identifier).Value]
			}
			return walk(value.Elem(), depth+1)
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() && walk(value.Field(index), depth+1) {
					return true
				}
			}
		case reflect.Slice:
			for index := 0; index < value.Len(); index++ {
				if walk(value.Index(index), depth+1) {
					return true
				}
			}
		}
		return false
	}
	return walk(reflect.ValueOf(node), 0)
}

var (
	callExpressionType   = reflect.TypeOf(&ast.CallExpression{})
	memberExpressionType = reflect.TypeOf(&ast.MemberExpression{})
)

// containsExecution walks an AST subtree for calls and member reads.
func containsExecution(node any) bool {
	if node == nil {
		return false
	}
	return reflectContainsExecution(reflect.ValueOf(node), 0)
}

func reflectContainsExecution(value reflect.Value, depth int) bool {
	if depth > 64 || !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return false
		}
		return reflectContainsExecution(value.Elem(), depth+1)
	case reflect.Pointer:
		if value.IsNil() {
			return false
		}
		if value.Type() == callExpressionType || value.Type() == memberExpressionType {
			return true
		}
		return reflectContainsExecution(value.Elem(), depth+1)
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			if value.Type().Field(index).IsExported() && reflectContainsExecution(value.Field(index), depth+1) {
				return true
			}
		}
	case reflect.Slice:
		for index := 0; index < value.Len(); index++ {
			if reflectContainsExecution(value.Index(index), depth+1) {
				return true
			}
		}
	}
	return false
}

// compactCallChain renders the executed call chain, collapsing repeated
// consecutive frames of recursion as `Name ×N` and eliding the middle of a
// long chain.
func compactCallChain(stack []cteCallSite, module string) []string {
	label := func(site cteCallSite) string {
		name := site.Function
		if site.Module != "" && module != "" && site.Module != module {
			name = site.Module + "." + name
		}
		if site.Instantiation != "" {
			name += "[" + site.Instantiation + "]"
		}
		return name
	}
	names := []string{}
	for index := 0; index < len(stack); {
		end := index
		for end < len(stack) && label(stack[end]) == label(stack[index]) {
			end++
		}
		if count := end - index; count > 1 {
			names = append(names, fmt.Sprintf("%s ×%d", label(stack[index]), count))
		} else {
			names = append(names, label(stack[index]))
		}
		index = end
	}
	if len(names) > 8 {
		names = append(append(append([]string(nil), names[:4]...), "…"), names[len(names)-3:]...)
	}
	return names
}

// errorsWithin reports a recorded error located inside the root expression's
// first source line from its start column on.
func (a *Analyzer) errorsWithin(root ast.Expression) bool {
	start := expressionToken(root)
	for _, err := range a.errors {
		if err.File == start.File && err.Line == start.Line && err.Column >= start.Column {
			return true
		}
	}
	return false
}

// deferredStaticInitializer is a static initializer that executes Sec code,
// established after the analysis has analyzed every body.
type deferredStaticInitializer struct {
	name, module, file string
	value              ast.Expression
}

// executeDeferredStaticInitializers runs the static initializers deferred by
// this analysis (static.md § 15; compile_time_evaluation.md § 9) before
// static materialization reads them, reporting failures fail-closed.
func (a *Analyzer) executeDeferredStaticInitializers(program *ast.Program) {
	for _, deferred := range a.deferredStatics {
		if _, done := a.cteResults[deferred.value]; done || a.errorsWithin(deferred.value) {
			continue
		}
		static, ok := a.staticBindingOfInitializer(deferred.value)
		if !ok {
			continue
		}
		previousModule, previousFile := a.currentModule, a.currentSourceFile
		a.currentModule, a.currentSourceFile = deferred.module, deferred.file
		result := newCTEExecutor(a, program).evaluateStatic(static.Name, sourceTokenLocation(static.Declaration.Name.Token), deferred.value)
		if result.Failure == nil && static.Type.Kind != "" {
			result.Type = static.Type
		}
		a.currentModule, a.currentSourceFile = previousModule, previousFile
		if a.cteResults == nil {
			a.cteResults = map[ast.Expression]cteResult{}
		}
		a.cteResults[deferred.value] = result
		if result.Failure != nil {
			a.reportCompileTimeExecutionFailure(deferred.value, "static initializer for "+deferred.name)
		}
	}
}

// CompileTimeValueFact is the outcome of one required semantic CTE request,
// for tooling (compile_time_evaluation.md § 73): the root expression's
// position, its established value rendered as Sec source, or the failure
// category and reason with the executed call chain.
type CompileTimeValueFact struct {
	Root  string
	Value string
	// Type is the display name of the value's canonical Sec type.
	Type string
	// Context names the required position's evaluator class and construct.
	Context   string
	Failure   string
	Reason    string
	CallChain []string
}

// CompileTimeValueAt returns the CTE outcome of the required root expression
// that starts at a source position. It reads the outcomes this analysis
// established; tooling never evaluates on its own (§ 73(2)).
func (a *Analyzer) CompileTimeValueAt(file string, line, column int) (CompileTimeValueFact, bool) {
	if a == nil {
		return CompileTimeValueFact{}, false
	}
	for root, result := range a.cteResults {
		token := expressionStartToken(root)
		if token.File != file || token.Line != line || token.Column != column {
			continue
		}
		text := root.String()
		if _, infix := root.(*ast.InfixExpression); infix && strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
			text = text[1 : len(text)-1]
		}
		fact := CompileTimeValueFact{Root: text}
		if context, classified := a.compileTimeContexts[root]; classified {
			fact.Context = context.Construct + " (" + string(context.Class) + ")"
		}
		if result.Failure != nil {
			if result.Failure.Kind == cteSuppressed {
				return CompileTimeValueFact{}, false
			}
			fact.Failure, fact.Reason, fact.CallChain = string(result.Failure.Kind), result.Failure.Reason, compactCallChain(result.Failure.CallStack, "")
			return fact, true
		}
		fact.Value = renderCompileTimeValue(result.Value)
		if result.Type.Kind != "" {
			fact.Type = typeDisplayName(result.Type)
		}
		return fact, true
	}
	return CompileTimeValueFact{}, false
}

// renderCompileTimeValue spells an evaluator value as Sec source text.
func renderCompileTimeValue(value DefaultConstant) string {
	if aggregate := value.Aggregate; aggregate != nil {
		if aggregate.Variant != "" {
			if aggregate.Payload == nil {
				return aggregate.Variant
			}
			return aggregate.Variant + "(" + renderCompileTimeValue(*aggregate.Payload) + ")"
		}
		fields := make([]string, 0, len(aggregate.Names))
		for _, name := range aggregate.Names {
			fields = append(fields, name+": "+renderCompileTimeValue(aggregate.Fields[name]))
		}
		return typeDisplayName(aggregate.Type) + " { " + strings.Join(fields, ", ") + " }"
	}
	switch {
	case value.Nominal.Owner != "":
		return value.Nominal.Owner + "." + value.Nominal.Member
	case value.Integer != nil:
		return value.Integer.String()
	case value.Kind == BoolType:
		return strconv.FormatBool(value.Bool)
	case value.Kind == StringType:
		return strconv.Quote(value.String)
	case value.Lexeme != "":
		return value.Lexeme
	case value.Exact != nil:
		return value.Exact.RatString()
	}
	return string(value.Kind)
}

// OptionalCompileTimeValue lets an optimizer try early evaluation of an
// ordinary expression with the same executor (compile_time_evaluation.md
// § 7(5)–(7)). A failed optional evaluation only means the fold is not used:
// it never produces a source diagnostic and never changes validity. An
// evaluator fault is still a compiler invariant failure and is returned as
// an error rather than hidden (§ 7(6)).
func (a *Analyzer) OptionalCompileTimeValue(program *ast.Program, expr ast.Expression) (DefaultConstant, bool, error) {
	if a == nil || expr == nil {
		return DefaultConstant{}, false, nil
	}
	errorsBefore := len(a.errors)
	result := newCTEExecutor(a, program).evaluate(expr)
	a.errors = a.errors[:errorsBefore]
	if result.Failure != nil {
		if result.Failure.Kind == cteCompilerInvariantFailure {
			return DefaultConstant{}, false, fmt.Errorf("%s", result.Failure.Reason)
		}
		return DefaultConstant{}, false, nil
	}
	return result.Value, true, nil
}

// CompileTimeValue is the typed outcome of one required semantic CTE request
// (compile_time_evaluation.md § 11): the canonical Sec type, its
// module-qualified TypeIdentity, independent of display spelling, import
// aliases and evaluator representation, and the SemanticValue the executor
// established.
type CompileTimeValue struct {
	Type         Type
	TypeIdentity string
	Value        DefaultConstant
}

// CompileTimeValueOf returns the typed compile-time value this analysis
// established for a required root expression, or false when the root was
// not evaluated or its evaluation failed.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 11, 31(3), 73.
func (a *Analyzer) CompileTimeValueOf(expr ast.Expression) (CompileTimeValue, bool) {
	if a == nil || expr == nil {
		return CompileTimeValue{}, false
	}
	result, ok := a.cteResults[expr]
	if !ok || result.Failure != nil || result.Type.Kind == "" {
		return CompileTimeValue{}, false
	}
	return CompileTimeValue{Type: result.Type, TypeIdentity: canonicalTypeIdentity(result.Type), Value: result.Value}, true
}
