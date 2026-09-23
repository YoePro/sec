package sema

import (
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Canonical closure facts retain explicit copy/move transfer and reference
// dependencies without reconstructing either property from source spelling.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/declarations/lambda-functions.md — §§15–20 "Capture forms"
func TestResolvedLambdaCaptureFacts(t *testing.T) {
	source := `module main

@noCopy
type Resource struct { value: int, }

fn Test(ref view: int) void {
	let number := 1
	let resource := Resource { value: 2 }
	let copied := capture(number, view) fn() int { return number }
	let moved := capture(<-resource) fn() int { return resource.value }
	discard copied
	discard moved
}
`
	p := parser.New(lexer.NewWithFile(source, "closure-facts.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	copyLambda := function.Body.Statements[2].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	moveLambda := function.Body.Statements[3].(*ast.LetStatement).Value.(*ast.LambdaExpression)

	copied, ok := analyzer.ResolvedLambdaCapturesOf(copyLambda)
	if !ok || len(copied) != 2 {
		t.Fatalf("copy capture facts = %#v, found=%v", copied, ok)
	}
	if copied[0].Name != "number" || copied[0].SourcePlace.String() != "number" || copied[0].Mode != CaptureCopy || copied[0].Transfer != CaptureTransferCopy || copied[0].SourceBecomesUnavailable {
		t.Fatalf("number capture = %#v", copied[0])
	}
	if copied[1].Name != "view" || copied[1].Mode != CaptureCopy || copied[1].Transfer != CaptureTransferCopy || len(copied[1].Dependencies.ReferentPlaces) != 1 || copied[1].Dependencies.ReferentPlaces[0].String() != "view" {
		t.Fatalf("reference-value capture = %#v", copied[1])
	}

	moved, ok := analyzer.ResolvedLambdaCapturesOf(moveLambda)
	if !ok || len(moved) != 1 || moved[0].Name != "resource" || moved[0].Mode != CaptureMove || moved[0].Transfer != CaptureTransferMove || !moved[0].SourceBecomesUnavailable {
		t.Fatalf("move capture = %#v, found=%v", moved, ok)
	}

	// Query results must not expose analyzer-owned slices.
	copied[1].Dependencies.ReferentPlaces[0].Root = "changed"
	again, _ := analyzer.ResolvedLambdaCapturesOf(copyLambda)
	if again[1].Dependencies.ReferentPlaces[0].Root != "view" {
		t.Fatalf("capture query exposed mutable provenance: %#v", again[1])
	}
	if _, ok := analyzer.ResolvedLambdaCapturesOf(&ast.LambdaExpression{}); ok {
		t.Fatal("unknown lambda unexpectedly acquired capture facts")
	}
}

// Aggregate value capture retains field-sensitive referent dependencies rather
// than collapsing them into an unknown dependency on the whole aggregate.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Closure environment as dependency carrier"
//   - rules/memory/lifetime_analysis.md — §12 "References carried by aggregates"
func TestLambdaCapturePreservesNestedReferentDependencies(t *testing.T) {
	source := `module main
type Holder struct { view: ref int, }
fn Test(ref input: int) void {
	let holder := Holder { view: input }
	let callback := capture(holder) fn() void {}
	discard callback
}
`
	p := parser.New(lexer.NewWithFile(source, "nested-capture-dependencies.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	lambda := function.Body.Statements[1].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	facts, ok := analyzer.ResolvedLambdaCapturesOf(lambda)
	if !ok || len(facts) != 1 {
		t.Fatalf("capture facts = %#v, found=%v", facts, ok)
	}
	referents := facts[0].Dependencies.Referents
	if len(referents) != 1 || referents[0].CarrierPath != ".view" || referents[0].Unknown || len(referents[0].ReferentPlaces) != 1 || referents[0].ReferentPlaces[0].String() != "input" {
		t.Fatalf("nested referent dependencies = %#v", referents)
	}

	referents[0].ReferentPlaces[0].Root = "changed"
	again, _ := analyzer.ResolvedLambdaCapturesOf(lambda)
	if again[0].Dependencies.Referents[0].ReferentPlaces[0].Root != "input" {
		t.Fatalf("nested capture dependency exposed mutable Place storage: %#v", again[0])
	}
	creation, ok := analyzer.ResolvedClosureCreationOf(lambda)
	if !ok || creation.Captures[0].Dependencies.Referents[0].CarrierPath != ".view" {
		t.Fatalf("creation summary lost nested dependency: %#v, found=%v", creation, ok)
	}
}

// Capturing a locally bound function value preserves its exact callable body
// and any nested closure environment instead of retaining only its fn shape.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Captured function values"
//   - rules/analysis/closure_analysis.md — "Closure environment as dependency carrier"
func TestLambdaCapturePreservesCallableDependencies(t *testing.T) {
	source := `module main
fn Increment(value: int) int { return value + 1 }
fn Test() void {
	let operation: fn(int) int := Increment
	let wrapper := capture(operation) fn(value: int) int { return operation(value) }
	let factor := 2
	let inner := capture(factor) fn(value: int) int { return value * factor }
	let outer := capture(inner) fn(value: int) int { return inner(value) }
	let mut rebound: fn(int) int := Increment
	rebound = Alternate
	let reboundWrapper := capture(rebound) fn(value: int) int { return rebound(value) }
	discard wrapper
	discard outer
	discard reboundWrapper
}
fn WrapUnknown(operation: fn(int) int) void {
	let wrapper := capture(operation) fn(value: int) int { return operation(value) }
	discard wrapper
}
fn Alternate(value: int) int { return value - 1 }
`
	p := parser.New(lexer.NewWithFile(source, "callable-capture-dependencies.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	wrapper := function.Body.Statements[1].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	inner := function.Body.Statements[3].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	outer := function.Body.Statements[4].(*ast.LetStatement).Value.(*ast.LambdaExpression)

	wrapperCaptures, ok := analyzer.ResolvedLambdaCapturesOf(wrapper)
	if !ok || len(wrapperCaptures) != 1 {
		t.Fatalf("wrapper captures = %#v, found=%v", wrapperCaptures, ok)
	}
	namedDependency := wrapperCaptures[0].Dependencies
	if !namedDependency.CallableDependenciesKnown || namedDependency.HasCallableEnvironment || len(namedDependency.CallableTargets.KnownTargets) != 1 || !namedDependency.CallableTargets.IsClosed {
		t.Fatalf("named callable dependency = %#v", namedDependency)
	}

	innerIdentity, ok := analyzer.ResolvedCallableIdentityOf(inner)
	if !ok || !innerIdentity.HasEnvironment {
		t.Fatalf("inner identity = %#v, found=%v", innerIdentity, ok)
	}
	outerCaptures, ok := analyzer.ResolvedLambdaCapturesOf(outer)
	if !ok || len(outerCaptures) != 1 {
		t.Fatalf("outer captures = %#v, found=%v", outerCaptures, ok)
	}
	nestedDependency := outerCaptures[0].Dependencies
	if !nestedDependency.CallableDependenciesKnown || !nestedDependency.HasCallableEnvironment || nestedDependency.CallableEnvironment != innerIdentity.Environment || !exactTargetSetMatches(nestedDependency.CallableTargets, innerIdentity.Body) {
		t.Fatalf("nested callable dependency = %#v, inner=%#v", nestedDependency, innerIdentity)
	}

	// Published facts must not expose target-set backing storage.
	nestedDependency.CallableTargets.KnownTargets[0] = "changed"
	again, _ := analyzer.ResolvedLambdaCapturesOf(outer)
	if !exactTargetSetMatches(again[0].Dependencies.CallableTargets, innerIdentity.Body) {
		t.Fatalf("callable capture dependency exposed target storage: %#v", again[0])
	}

	reboundAssignment := function.Body.Statements[6].(*ast.AssignmentStatement)
	reboundIdentity, ok := analyzer.ResolvedCallableIdentityOf(reboundAssignment.Value)
	if !ok {
		t.Fatalf("rebound value has no callable identity")
	}
	reboundWrapper := function.Body.Statements[7].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	reboundCaptures, ok := analyzer.ResolvedLambdaCapturesOf(reboundWrapper)
	if !ok || len(reboundCaptures) != 1 || !exactTargetSetMatches(reboundCaptures[0].Dependencies.CallableTargets, reboundIdentity.Body) {
		t.Fatalf("rebound callable dependency = %#v, identity=%#v, found=%v", reboundCaptures, reboundIdentity, ok)
	}
	symbols := analyzer.Symbols()
	reboundSymbol, ok := symbols["rebound"]
	if !ok || !reboundSymbol.HasCallableIdentity || !exactTargetSetMatches(reboundSymbol.CallableIdentity.Targets, reboundIdentity.Body) {
		t.Fatalf("rebound symbol identity = %#v, found=%v", reboundSymbol, ok)
	}
	reboundSymbol.CallableIdentity.Targets.KnownTargets[0] = "changed"
	if immutableSymbol := analyzer.Symbols()["rebound"]; !exactTargetSetMatches(immutableSymbol.CallableIdentity.Targets, reboundIdentity.Body) {
		t.Fatalf("symbol snapshot exposed callable target storage: %#v", immutableSymbol)
	}

	unknownFunction := result.Program.Statements[3].(*ast.FunctionDeclaration)
	unknownWrapper := unknownFunction.Body.Statements[0].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	unknownCaptures, ok := analyzer.ResolvedLambdaCapturesOf(unknownWrapper)
	if !ok || len(unknownCaptures) != 1 {
		t.Fatalf("unknown callable captures = %#v, found=%v", unknownCaptures, ok)
	}
	unknownDependency := unknownCaptures[0].Dependencies
	if unknownDependency.CallableDependenciesKnown || unknownDependency.HasCallableEnvironment || len(unknownDependency.CallableTargets.KnownTargets) != 0 {
		t.Fatalf("unresolved callable parameter acquired a false dependency proof: %#v", unknownDependency)
	}

	graph := analyzer.CallGraph()
	wrapperNodes := graph.NodesForDeclaration(wrapper.Token)
	if len(wrapperNodes) != 1 {
		t.Fatalf("wrapper callable nodes = %#v", wrapperNodes)
	}
	wrapperCalls := graph.Outgoing(wrapperNodes[0].ID)
	if len(wrapperCalls) != 1 || wrapperCalls[0].Dispatch != CallDispatchFunctionValue || wrapperCalls[0].Execution != CallExecutionSynchronous || !exactTargetSetMatches(wrapperCalls[0].TargetSet, namedDependency.CallableTargets.KnownTargets[0]) || len(wrapperCalls[0].Targets) != 1 {
		t.Fatalf("wrapper function-value calls = %#v", wrapperCalls)
	}
	outerNodes := graph.NodesForDeclaration(outer.Token)
	innerNodes := graph.NodesForDeclaration(inner.Token)
	if len(outerNodes) != 1 || len(innerNodes) != 1 {
		t.Fatalf("nested callable nodes: outer=%#v inner=%#v", outerNodes, innerNodes)
	}
	outerCalls := graph.Outgoing(outerNodes[0].ID)
	if len(outerCalls) != 1 || outerCalls[0].Dispatch != CallDispatchClosure || len(outerCalls[0].Targets) != 1 || outerCalls[0].Targets[0] != innerNodes[0].ID || !exactTargetSetMatches(outerCalls[0].TargetSet, innerIdentity.Body) {
		t.Fatalf("outer closure calls = %#v", outerCalls)
	}
	unknownNodes := graph.NodesForDeclaration(unknownWrapper.Token)
	if len(unknownNodes) != 1 || len(graph.Outgoing(unknownNodes[0].ID)) != 0 {
		t.Fatalf("unknown callable acquired an unsound graph edge: nodes=%#v outgoing=%#v", unknownNodes, graph.Outgoing(unknownNodes[0].ID))
	}
}

// Invalid ownership requests remain diagnostics and never become semantic
// capture facts consumed by later compiler stages.
func TestInvalidLambdaCaptureDoesNotPublishFact(t *testing.T) {
	source := `module main
@noCopy
type Resource struct { value: int, }
fn Test() void {
	let resource := Resource { value: 1 }
	let invalid := capture(resource) fn() int { return resource.value }
	discard invalid
}
`
	p := parser.New(lexer.NewWithFile(source, "invalid-closure-facts.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) == 0 {
		t.Fatal("invalid copy capture was accepted")
	}
	function := result.Program.Statements[2].(*ast.FunctionDeclaration)
	lambda := function.Body.Statements[1].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	if facts, ok := analyzer.ResolvedLambdaCapturesOf(lambda); ok || len(facts) != 0 {
		t.Fatalf("invalid capture facts = %#v, found=%v", facts, ok)
	}
	if identity, ok := analyzer.ResolvedCallableIdentityOf(lambda); ok {
		t.Fatalf("invalid capture callable identity = %#v", identity)
	}
	if summary, ok := analyzer.ResolvedClosureCreationOf(lambda); ok {
		t.Fatalf("invalid capture closure summary = %#v", summary)
	}
}

// Callable identity facts keep body, value origin, creation site, and abstract
// environment separate while retaining exact named-function targets.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Core semantic entities"
//   - rules/analysis/closure_analysis.md — "Abstract closure identity"
func TestResolvedCallableIdentitiesAreExactAndStable(t *testing.T) {
	source := `module main
fn Transform(value: int) int { return value }
fn Transform(value: bool) bool { return value }
fn Unique(value: int) int { return value }
fn Test() void {
	let contextual: fn(int) int := Transform
	let direct := Unique
	let plain := fn(value: int) int { return value }
	let factor := 2
	let closure := capture(factor) fn(value: int) int { return value * factor }
	discard Unique(1)
	discard contextual
	discard direct
	discard plain
	discard closure
}
`
	p := parser.New(lexer.NewWithFile(source, "callable-identities.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}

	function := result.Program.Statements[4].(*ast.FunctionDeclaration)
	contextualExpr := function.Body.Statements[0].(*ast.LetStatement).Value
	directExpr := function.Body.Statements[1].(*ast.LetStatement).Value
	plainLambda := function.Body.Statements[2].(*ast.LetStatement).Value.(*ast.LambdaExpression)
	closureLambda := function.Body.Statements[4].(*ast.LetStatement).Value.(*ast.LambdaExpression)

	contextual, ok := analyzer.ResolvedCallableIdentityOf(contextualExpr)
	if !ok || contextual.Kind != CallableBodyNamedFunction || contextual.NamedTarget == "" || contextual.Body == "" || contextual.Value == "" || contextual.HasEnvironment || !exactTargetSetMatches(contextual.Targets, contextual.Body) {
		t.Fatalf("contextual named identity = %#v, found=%v", contextual, ok)
	}
	direct, ok := analyzer.ResolvedCallableIdentityOf(directExpr)
	if !ok || direct.Kind != CallableBodyNamedFunction || direct.NamedTarget == "" || direct.Body == "" || direct.Value == "" || direct.HasEnvironment || !exactTargetSetMatches(direct.Targets, direct.Body) {
		t.Fatalf("direct named identity = %#v, found=%v", direct, ok)
	}
	if contextual.NamedTarget == direct.NamedTarget || contextual.Value == direct.Value {
		t.Fatalf("distinct named values/targets were conflated: contextual=%#v direct=%#v", contextual, direct)
	}

	plain, ok := analyzer.ResolvedCallableIdentityOf(plainLambda)
	if !ok || plain.Kind != CallableBodyNonCapturingLambda || plain.Body == "" || plain.Value == "" || plain.CreationSite == "" || plain.HasEnvironment || plain.Environment != "" || plain.AbstractsRuntimeInstances || !exactTargetSetMatches(plain.Targets, plain.Body) {
		t.Fatalf("plain lambda identity = %#v, found=%v", plain, ok)
	}
	closure, ok := analyzer.ResolvedCallableIdentityOf(closureLambda)
	if !ok || closure.Kind != CallableBodyCapturingLambda || closure.Body == "" || closure.Value == "" || closure.CreationSite == "" || !closure.HasEnvironment || closure.Environment == "" || !closure.AbstractsRuntimeInstances || !exactTargetSetMatches(closure.Targets, closure.Body) {
		t.Fatalf("capturing lambda identity = %#v, found=%v", closure, ok)
	}
	if plain.Body == closure.Body || plain.CreationSite == closure.CreationSite || string(closure.Body) == string(closure.Environment) {
		t.Fatalf("callable entities were conflated: plain=%#v closure=%#v", plain, closure)
	}
	plainCreation, ok := analyzer.ResolvedClosureCreationOf(plainLambda)
	if !ok || plainCreation.Body != plain.Body || plainCreation.Value != plain.Value || plainCreation.CreationSite != plain.CreationSite || plainCreation.HasEnvironment || len(plainCreation.Captures) != 0 || !plainCreation.DependenciesKnown || !exactTargetSetMatches(plainCreation.Targets, plain.Body) {
		t.Fatalf("plain closure creation = %#v, found=%v", plainCreation, ok)
	}
	closureCreation, ok := analyzer.ResolvedClosureCreationOf(closureLambda)
	if !ok || closureCreation.Body != closure.Body || closureCreation.Value != closure.Value || closureCreation.CreationSite != closure.CreationSite || closureCreation.Environment != closure.Environment || !closureCreation.HasEnvironment || !closureCreation.AbstractsRuntimeInstances || len(closureCreation.Captures) != 1 || closureCreation.Captures[0].Name != "factor" || closureCreation.Captures[0].Transfer != CaptureTransferCopy || !closureCreation.DependenciesKnown || !exactTargetSetMatches(closureCreation.Targets, closure.Body) {
		t.Fatalf("capturing closure creation = %#v, found=%v", closureCreation, ok)
	}

	second := NewAnalyzer()
	if errors := second.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("second sema: %v", errors)
	}
	again, ok := second.ResolvedCallableIdentityOf(closureLambda)
	if !ok || !reflect.DeepEqual(again, closure) {
		t.Fatalf("identity is not deterministic: first=%#v second=%#v", closure, again)
	}

	closure.Targets.KnownTargets[0] = "changed"
	immutable, _ := analyzer.ResolvedCallableIdentityOf(closureLambda)
	if !exactTargetSetMatches(immutable.Targets, immutable.Body) {
		t.Fatalf("callable identity query exposed target storage: %#v", immutable)
	}
	closureCreation.Targets.KnownTargets[0] = "changed"
	closureCreation.Captures[0].SourcePlace.Root = "changed"
	immutableCreation, _ := analyzer.ResolvedClosureCreationOf(closureLambda)
	if !exactTargetSetMatches(immutableCreation.Targets, immutableCreation.Body) || immutableCreation.Captures[0].SourcePlace.Root != "factor" {
		t.Fatalf("closure creation query exposed mutable storage: %#v", immutableCreation)
	}

	graph := analyzer.CallGraph()
	testID := callGraphNodeIDByName(t, graph, "Test")
	sites := graph.Outgoing(testID)
	if len(sites) != 1 || len(sites[0].Targets) != 1 || !exactTargetSetMatches(sites[0].TargetSet, CallableBodyID("callable-body|"+string(sites[0].Targets[0]))) {
		t.Fatalf("direct call target set = %#v", sites)
	}
	sites[0].TargetSet.KnownTargets[0] = "changed"
	againSites := analyzer.CallGraph().Outgoing(testID)
	if len(againSites) != 1 || !exactTargetSetMatches(againSites[0].TargetSet, CallableBodyID("callable-body|"+string(againSites[0].Targets[0]))) {
		t.Fatalf("call graph exposed target-set storage: %#v", againSites)
	}
}

func exactTargetSetMatches(targets CallableTargetSet, body CallableBodyID) bool {
	return targets.IsClosed && !targets.HasOpenContract && targets.OpenContract == "" && len(targets.KnownTargets) == 1 && targets.KnownTargets[0] == body
}
