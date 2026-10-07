package sema

import (
	"encoding/json"
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
	contract := &OpenCallableContract{Operation: required.Name, ReturnType: graphTypeIdentity(required.ReturnType), Receiver: graphTypeIdentity(iface),
		ReceiverMutable: required.ReceiverMutable, ReceiverConsuming: required.ReceiverConsuming,
		ABI: "Sec", Unsafe: required.Unsafe, Provenance: "validated-interface-declaration"}
	for _, parameter := range required.Parameters {
		contract.Parameters = append(contract.Parameters, CallableParameterContract{TypeIdentity: graphTypeIdentity(parameter.Type),
			Ref: parameter.Ref, MutableRef: parameter.MutableRef, Consuming: parameter.Consuming, Variadic: parameter.Variadic})
	}
	contract.setIdentity()
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
