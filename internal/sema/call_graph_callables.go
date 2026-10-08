package sema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// CallableParameterContract preserves resolved type identity and invocation
// ownership modes without retaining mutable type bodies or parameter labels.
// Rules: rules/analysis/call_graph.md — "Callable contract".
type CallableParameterContract struct {
	TypeIdentity                         string
	Ref, MutableRef, Consuming, Variadic bool
}

// OpenCallableContract is the validated invocation boundary for targets not
// completely enumerated in this graph. Empty optional guarantees mean unknown,
// never an inferred promise from the known implementation subset. Signature
// facts come from ordinary Sema; stack/import guarantees have their own owners.
// Rules: rules/analysis/call_graph.md — "Callable contract", "Open callable
// contract", "Conservative unknown facts"; rules/declarations/interfaces.md.
type OpenCallableContract struct {
	Operation                          string
	ID                                 CallableContractID
	Parameters                         []CallableParameterContract
	ReturnType                         string
	Receiver                           string
	ReceiverMutable, ReceiverConsuming bool
	Capability                         CallableCapability
	ABI                                string
	Unsafe                             bool
	Provenance                         string
}

// functionTypeGraphContract constructs the public invocation contract after
// the existing function-type/argument validator succeeds. Function types carry
// no optional effect, retention, stack or reentry guarantees in this producer.
// Rules: rules/analysis/call_graph.md — "Unknown concrete target", "Callable contract";
// rules/corrections/applied/types-callable-model-correction-20260816.md — "Required correction".
func functionTypeGraphContract(typ Type) *OpenCallableContract {
	if typ.Kind != FunctionType || typ.FunctionReturnType == nil {
		return nil
	}
	contract := &OpenCallableContract{ReturnType: graphTypeIdentity(*typ.FunctionReturnType),
		Capability: normalizedCallableCapability(typ.FunctionCapability), ABI: "Sec", Provenance: "validated-function-type"}
	for _, typ := range typ.FunctionParameterTypes {
		contract.Parameters = append(contract.Parameters, CallableParameterContract{TypeIdentity: graphTypeIdentity(typ),
			Ref: typ.Kind == ReferenceType, MutableRef: typ.ReferenceMutable})
	}
	if typ.FunctionVariadic && len(contract.Parameters) > 0 {
		contract.Parameters[len(contract.Parameters)-1].Variadic = true
	}
	contract.setIdentity()
	return contract
}

// setIdentity keys a contract by public invocation facts, not implementation
// bodies, optional inferred effects or current navigation coordinates.
// Rules: rules/analysis/call_graph.md — "Open callable contract", "Callable node identity".
func (contract *OpenCallableContract) setIdentity() {
	contract.ID = ""
	key, _ := json.Marshal(contract)
	contract.ID = CallableContractID("callable-contract|" + graphIdentityDigest(key))
}

// cloneOpenCallableContract detaches public signature facts from snapshots.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results.
func cloneOpenCallableContract(contract *OpenCallableContract) *OpenCallableContract {
	if contract == nil {
		return nil
	}
	result := *contract
	result.Parameters = append([]CallableParameterContract(nil), contract.Parameters...)
	return &result
}

// openFunctionValueTargets retains known body alternatives even when closure
// environments, loop flow or unknown writes prevent a closed identity proof.
// The validated type contract covers every omitted or future alternative;
// this projection never claims closure-environment/retention proof.
// Rules: rules/analysis/call_graph.md — "Function values", "Open callable contract";
// rules/analysis/closure_analysis.md — "Soundness of target sets".
func (a *Analyzer) openFunctionValueTargets(expression ast.Expression, contract *OpenCallableContract) CallableTargetSet {
	targets := CallableTargetSet{HasOpenContract: true, OpenContract: contract.ID, Contract: contract}
	seen := map[CallableBodyID]bool{}
	add := func(identity ResolvedCallableIdentity) {
		for _, body := range identity.Targets.KnownTargets {
			if _, exists := a.callGraph.bodyNodes[body]; exists && !seen[body] {
				seen[body] = true
				targets.KnownTargets = append(targets.KnownTargets, body)
			}
		}
	}
	if identity, ok := a.resolvedCallableIdentities[expression]; ok {
		add(identity)
	}
	if identifier, ok := expression.(*ast.Identifier); ok {
		if symbol, exists := a.symbols[identifier.Value]; exists {
			if symbol.HasCallableIdentity {
				add(symbol.CallableIdentity)
			}
			for _, write := range a.nestedCallableWrites[sourceTokenLocation(symbol.Token)] {
				if write.Known {
					add(write.Identity)
				}
			}
		}
	}
	sort.Slice(targets.KnownTargets, func(i, j int) bool { return targets.KnownTargets[i] < targets.KnownTargets[j] })
	return targets
}

// recordInterfaceGraphCall records an open interface invocation with every
// currently validated explicit concrete implementation as a known may-target.
// Ordinary Sema has no closed-build proof, so future implementations remain
// covered by the declared interface boundary rather than a fabricated body.
// Rules: rules/analysis/call_graph.md — "Interfaces", "Open callable contract";
// rules/declarations/interfaces.md — §§4–6 explicit conformance.
func (a *Analyzer) recordInterfaceGraphCall(receiver Type, required Function, source lexer.Token, execution CallExecutionRelation) {
	iface := dereferenceType(receiver)
	contract := interfaceInvocationContract(receiver, required)
	targets := CallableTargetSet{HasOpenContract: true, OpenContract: contract.ID, Contract: contract}
	types := []Type{}
	for _, typ := range a.types {
		if typ.Kind != InterfaceType && typ.Kind != GenericType && len(typ.GenericParameters) == 0 && a.hasValidExplicitInterfaceConformance(typ, iface) {
			types = append(types, typ)
		}
	}
	sort.Slice(types, func(i, j int) bool { return graphTypeIdentity(types[i]) < graphTypeIdentity(types[j]) })
	seen := map[CallableBodyID]bool{}
	methodName := required.Name
	if index := strings.LastIndex(methodName, "."); index >= 0 {
		methodName = methodName[index+1:]
	}
	for _, typ := range types {
		expected := substituteInterfaceSelf(required, typ)
		for _, method := range a.functions[typ.Name+"."+methodName] {
			if method.Module != typ.Module || !compatibleInterfaceMethodSignature(method, expected) || method.Static != expected.Static ||
				(!expected.ReceiverMutable && method.ReceiverMutable) || !sameConcreteType(method.ReturnType, expected.ReturnType) {
				continue
			}
			a.callGraph.addCallable(method)
			body := callableBodyID(method)
			if !seen[body] {
				seen[body] = true
				targets.KnownTargets = append(targets.KnownTargets, body)
			}
		}
	}
	sort.Slice(targets.KnownTargets, func(i, j int) bool { return targets.KnownTargets[i] < targets.KnownTargets[j] })
	a.callGraph.addTargetSetCall(a.currentCallable, targets, source, CallDispatchInterface, execution)
}

// validGraphCallableContract checks the public invocation payload against its
// semantic identity and concrete scope before workspace/cache publication.
// Integrity cannot turn a malformed or mismatched signature into a contract.
// Rules: rules/analysis/call_graph.md — "Unknown callable contract", "One graph per `CompilationPlan`";
// rules/compiler/incremental_compilation.md — cache compatibility.
func validGraphCallableContract(contract *OpenCallableContract, scope CallGraphScope) bool {
	if contract == nil || contract.ReturnType == "" || contract.ABI != "Sec" {
		return false
	}
	switch contract.Provenance {
	case "validated-function-type":
		if contract.Receiver != "" || contract.Operation != "" || (contract.Capability != CallableShared && contract.Capability != CallableMutable && contract.Capability != CallableConsuming) {
			return false
		}
	case "validated-interface-declaration":
		if contract.Receiver == "" || contract.Operation == "" {
			return false
		}
	default:
		return false
	}
	for i, parameter := range contract.Parameters {
		if parameter.TypeIdentity == "" || (parameter.MutableRef && !parameter.Ref) || (parameter.Variadic && i != len(contract.Parameters)-1) {
			return false
		}
	}
	copy := cloneOpenCallableContract(contract)
	copy.setIdentity()
	expected := copy.ID
	if scope != (CallGraphScope{}) {
		expected = CallableContractID(scopedGraphIdentity(scope, "contract", string(expected)))
	}
	return expected == contract.ID
}

// interfaceInvocationContract keys the exact selected requirement by resolved
// interface identity, parameter identities, ownership modes and result contract.
// Resolved call facts and graph sites consume this single contract constructor.
// Rules: rules/declarations/interfaces.md — §§5,6;
// rules/declarations/functions.md — §§19,20,24;
// rules/analysis/call_graph.md — Interfaces, Open callable contract.
func interfaceInvocationContract(receiver Type, required Function) *OpenCallableContract {
	iface := dereferenceType(receiver)
	contract := &OpenCallableContract{Operation: required.Name, ReturnType: graphTypeIdentity(required.ReturnType), Receiver: graphTypeIdentity(iface),
		ReceiverMutable: required.ReceiverMutable, ReceiverConsuming: required.ReceiverConsuming,
		ABI: "Sec", Unsafe: required.Unsafe, Provenance: "validated-interface-declaration"}
	for _, parameter := range required.Parameters {
		contract.Parameters = append(contract.Parameters, CallableParameterContract{TypeIdentity: graphTypeIdentity(parameter.Type),
			Ref: parameter.Ref || parameter.Type.Kind == ReferenceType, MutableRef: parameter.MutableRef || (parameter.Type.Kind == ReferenceType && parameter.Type.ReferenceMutable), Consuming: parameter.Consuming, Variadic: parameter.Variadic})
	}
	contract.setIdentity()
	return contract
}

// sameStackExecution identifies the currently represented relations that keep
// cleanup and ordinary invocation in the caller's execution context.
// Rules: rules/analysis/call_graph.md — "Same-stack execution".
func sameStackExecution(execution CallExecutionRelation) bool {
	return execution == CallExecutionSynchronous || execution == CallExecutionDeferred
}

// recordDeferCallable gives every validated defer body a distinct synthetic
// node, including bodies without calls. Only reachable registration contributes
// a cleanup edge; body checking keeps the original flow reachability. Identity
// is source-local, scoped by its enclosing callable, rather than AST addresses
// or traversal counters. Repeated loop registrations share this may-graph node.
// Rules: rules/analysis/call_graph.md — "`defer` bodies", "Callable node identity",
// "Dispatch kinds", "Same-stack execution";
// rules/control-flow/defer.md — §§2–3, 12–13, 29.
func (a *Analyzer) recordDeferCallable(stmt *ast.DeferStatement) CallableID {
	if a.summaryPass || a.currentCallable == "" || a.callGraph == nil {
		return a.currentCallable
	}
	caller := a.currentCallable
	body := CallableBodyID("callable-body|" + a.callableIdentityKey("defer", stmt.Token))
	id := CallableID(body)
	if _, exists := a.callGraph.nodes[id]; !exists {
		a.callGraph.nodes[id] = CallableNode{
			ID: id, Kind: CallableBodyDefer, Name: "defer", Module: a.currentModule,
			Declaration: stmt.Token,
		}
		a.callGraph.nodeOrder = append(a.callGraph.nodeOrder, id)
		a.callGraph.bodyNodes[body] = id
	}
	if a.callGraphPathReachable {
		a.callGraph.addTargetSetCall(caller, exactCallableTargetSet(body), stmt.Token, CallDispatchGenerated, CallExecutionDeferred)
	}
	return id
}

// testCallableID encodes structured semantic identity without conflating path
// components, display labels, source positions, or generated linker symbols.
// The graph scope supplies CompilationPlan compatibility independently.
// Rules: rules/tooling/testing.md — §§6.1, 6.4, 33.1;
// rules/analysis/call_graph.md — "Callable node identity", "Test roots".
func testCallableID(identity TestIdentity) CallableID {
	encoded, _ := json.Marshal(identity) // This string/slice-only type cannot fail.
	return CallableID("test-body|" + string(encoded))
}

// recordTestCallable retains a validated test body's internal executable node
// without introducing a Function, function value, or source-visible name.
// Root membership is selected separately by CallGraphForTestPlan.
// Rules: rules/tooling/testing.md — §§5.7, 9.1, 33.1;
// rules/analysis/call_graph.md — "Callable node", "Test roots".
func (a *Analyzer) recordTestCallable(declaration *ast.TestDeclaration) CallableID {
	metadata, valid := a.resolvedTestMetadata[declaration]
	if !valid || a.summaryPass || a.callGraph == nil {
		return ""
	}
	id := testCallableID(metadata.Identity)
	if _, exists := a.callGraph.nodes[id]; !exists {
		a.callGraph.nodes[id] = CallableNode{
			ID: id, Kind: CallableBodyTest, Name: metadata.Name,
			Module: metadata.Identity.Module, Declaration: declaration.Token,
		}
		a.callGraph.nodeOrder = append(a.callGraph.nodeOrder, id)
		a.callGraph.bodyNodes[CallableBodyID(id)] = id
	}
	return id
}

// CallGraphForTestPlan creates a detached canonical graph view whose entry
// roots are exactly the explicit selected tests in the supplied plan scope.
// It removes production entry roots without deleting shared callable facts;
// ordinary reachability determines which helpers, cleanup and worker entries
// execute. Empty selection means no roots, never implicit "select all".
// Invalid/unknown/duplicate selections and failed Sema snapshots are rejected
// transactionally. This API owns graph selection, not source discovery, harness
// generation, linking or runtime execution in the compiler driver.
// Rules: rules/analysis/call_graph.md — "Test roots", "Root reachability classes",
// "One graph per `CompilationPlan`"; rules/tooling/testing.md — §§5.7, 26, 27, 34.4.
func (a *Analyzer) CallGraphForTestPlan(scope CallGraphScope, selected []TestIdentity) (*CallGraph, error) {
	if a == nil || a.callGraph == nil || len(a.errors) != 0 {
		return nil, fmt.Errorf("test graph requires successful semantic analysis")
	}
	if scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" {
		return nil, fmt.Errorf("test graph requires complete module, compilation-plan and compiler-model scope")
	}
	ids := make([]CallableID, 0, len(selected))
	seen := map[CallableID]bool{}
	for _, identity := range selected {
		id := testCallableID(identity)
		node, exists := a.callGraph.nodes[id]
		if !exists || node.Kind != CallableBodyTest {
			return nil, fmt.Errorf("selected test identity is not a validated test: %s", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate selected test identity: %s", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	graph := a.callGraph.clone()
	graph.roots = map[CallRootID]CallRoot{}
	graph.rootOrder = nil
	for _, id := range ids {
		node := graph.nodes[id]
		rootID := graph.addRoot(CallRootTestEntry, id, node.Declaration)
		root := graph.roots[rootID]
		delete(graph.roots, rootID)
		encodedScope, _ := json.Marshal(scope)
		root.ID = CallRootID(string(rootID) + "|" + string(encodedScope))
		root.Scope = scope
		graph.roots[root.ID] = root
		graph.rootOrder[len(graph.rootOrder)-1] = root.ID
	}
	return graph.ForCompilationPlan(scope)
}

// validTestRootScope prevents publishing or restoring selected test roots under
// another plan/model, and rejects test entries targeting ordinary functions.
// Rules: rules/analysis/call_graph.md — "Test roots", "One graph per `CompilationPlan`";
// rules/compiler/incremental_compilation.md — §§31–32.
func validTestRootScope(graph *CallGraph, scope CallGraphScope) bool {
	for _, root := range graph.roots {
		if root.Kind == CallRootTestEntry {
			node, exists := graph.nodes[root.Node]
			if !exists || node.Kind != CallableBodyTest || root.Scope != scope {
				return false
			}
		}
	}
	return true
}
