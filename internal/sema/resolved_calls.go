package sema

import (
	"fmt"
	"sec/internal/ast"
)

// Resolved call facts preserve the selected callable's source signature and
// dispatch category after overload resolution, independently of backend support.
// Rules: rules/declarations/functions.md — §§19,20,24;
// rules/declarations/interfaces.md — §§5,6; correction10-20260823.md.
type ResolvedCallKind string

const (
	ResolvedDirectCall          ResolvedCallKind = "direct"
	ResolvedForeignCall         ResolvedCallKind = "foreign"
	ResolvedStaticMethodCall    ResolvedCallKind = "static-method"
	ResolvedInterfaceMethodCall ResolvedCallKind = "interface-method"
)

type ResolvedCall struct {
	Function Function
	Kind     ResolvedCallKind
	// InterfaceContract is the selected invocation requirement, never a concrete body.
	InterfaceContract *OpenCallableContract
}

// recordResolvedCall publishes the exact successful overload selection. An
// interface invocation retains the same open contract used by the call graph;
// it cannot masquerade as a direct or static concrete method invocation.
// Rules: rules/declarations/functions.md — §§19,20,24;
// rules/declarations/interfaces.md — §§5,6;
// rules/analysis/call_graph.md — Interfaces, Open callable contract.
func (a *Analyzer) recordResolvedCall(call *ast.CallExpression, function Function, dispatch CallDispatchKind, receiver Type) {
	fact := ResolvedCall{Function: function, Kind: resolvedCallKind(dispatch)}
	if dispatch == CallDispatchInterface {
		fact.InterfaceContract = interfaceInvocationContract(receiver, function)
	}
	a.resolvedCalls[call] = fact
}

// ResolvedCallTarget exposes the chosen requirement and a detached public
// interface invocation contract. Absence means no unique callable was selected.
// Rules: rules/declarations/functions.md — §§19,20,24;
// rules/compiler/compiler_analysis.md — immutable analysis results.
func (a *Analyzer) ResolvedCallTarget(call *ast.CallExpression) (ResolvedCall, bool) {
	if a == nil || call == nil {
		return ResolvedCall{}, false
	}
	fact, ok := a.resolvedCalls[call]
	fact.Function.Parameters = append([]FunctionParameter(nil), fact.Function.Parameters...)
	fact.InterfaceContract = cloneOpenCallableContract(fact.InterfaceContract)
	return fact, ok
}

// resolvedCallKind translates validated dispatch into a post-Sema call fact.
// Rules: rules/compiler/semantic_ir.md — resolved calls;
// rules/analysis/call_graph.md — Interfaces, dispatch categories.
func resolvedCallKind(dispatch CallDispatchKind) ResolvedCallKind {
	switch dispatch {
	case CallDispatchForeign:
		return ResolvedForeignCall
	case CallDispatchInterface:
		return ResolvedInterfaceMethodCall
	case CallDispatchStaticMethod:
		return ResolvedStaticMethodCall
	default:
		return ResolvedDirectCall
	}
}

// refreshInterfaceSignatureTypes replaces named forward-declaration placeholders
// after type definitions have resolved, before overload selection/conformance.
// Invalid placeholders must not act as wildcard matches for another overload.
// Rules: rules/declarations/interfaces.md — §§5,6;
// rules/declarations/functions.md — §§19,20 (named identity, exact matching).
func (a *Analyzer) refreshInterfaceSignatureTypes() {
	for name, iface := range a.types {
		if iface.Kind != InterfaceType {
			continue
		}
		iface.InterfaceMethods = append([]Function(nil), iface.InterfaceMethods...)
		for i, method := range iface.InterfaceMethods {
			method.Parameters = append([]FunctionParameter(nil), method.Parameters...)
			for j := range method.Parameters {
				method.Parameters[j].Type = a.refreshInterfaceSignatureType(method.Parameters[j].Type)
			}
			method.ReturnType = a.refreshInterfaceSignatureType(method.ReturnType)
			iface.InterfaceMethods[i] = method
		}
		a.types[name] = iface
	}
	// Dispatch lookup groups contain separate copies of the inherited methods.
	for name, group := range a.functions {
		for i, method := range group {
			// Only interface groups exist at this phase; ordinary methods are registered later.
			method.Parameters = append([]FunctionParameter(nil), method.Parameters...)
			for j := range method.Parameters {
				method.Parameters[j].Type = a.refreshInterfaceSignatureType(method.Parameters[j].Type)
			}
			method.ReturnType = a.refreshInterfaceSignatureType(method.ReturnType)
			group[i] = method
		}
		a.functions[name] = group
	}
}

// refreshInterfaceSignatureType retains nominal identity through nested
// signature carriers while resolving a registered forward declaration.
// Rules: rules/declarations/interfaces.md — §6;
// rules/declarations/functions.md — §19.
func (a *Analyzer) refreshInterfaceSignatureType(typ Type) Type {
	if typ.Kind == InvalidType && typ.Name != "" {
		if resolved, ok := a.types[typ.Name]; ok && resolved.Kind != InvalidType && resolved.Module == typ.Module {
			return resolved
		}
	}
	if typ.Element != nil {
		element := a.refreshInterfaceSignatureType(*typ.Element)
		typ.Element = &element
	}
	typ.TypeArgs = append([]Type(nil), typ.TypeArgs...)
	for i := range typ.TypeArgs {
		typ.TypeArgs[i] = a.refreshInterfaceSignatureType(typ.TypeArgs[i])
	}
	typ.FunctionParameterTypes = append([]Type(nil), typ.FunctionParameterTypes...)
	for i := range typ.FunctionParameterTypes {
		typ.FunctionParameterTypes[i] = a.refreshInterfaceSignatureType(typ.FunctionParameterTypes[i])
	}
	if typ.FunctionReturnType != nil {
		result := a.refreshInterfaceSignatureType(*typ.FunctionReturnType)
		typ.FunctionReturnType = &result
	}
	return typ
}

// canPassImplicitMethodReceiver checks a selected method's declared receiver
// capability, retaining inherited interface requirements on the child receiver.
// Rules: rules/declarations/interfaces.md — §§5,6;
// rules/declarations/functions.md — §§22,24.
func (a *Analyzer) canPassImplicitMethodReceiver(function Function, receiver methodReceiverInfo) bool {
	if function.ImplTarget == "" {
		return true
	}
	if receiver.Type.Kind == InvalidType {
		return false
	}
	exactReceiver := typeDisplayName(dereferenceType(receiver.Type)) == function.ImplTarget
	inheritedCoreReceiver := false
	if !exactReceiver && dereferenceType(receiver.Type).Kind == StringType && a.isTrustedCoreSourceToken(function.Token) {
		for _, underlying := range a.relatedUnderlyingTypes(dereferenceType(receiver.Type)) {
			if underlying.Name == function.ImplTarget {
				inheritedCoreReceiver = true
				break
			}
		}
	}
	inheritedInterfaceReceiver := interfaceRetainsMethod(dereferenceType(receiver.Type), function)
	if !exactReceiver && !inheritedCoreReceiver && !inheritedInterfaceReceiver {
		return false
	}
	if function.ReceiverConsuming {
		return receiver.Type.Kind != ReferenceType && receiver.Symbol != nil
	}
	if function.ReceiverMutable {
		if receiver.Type.Kind == ReferenceType {
			return receiver.Type.ReferenceMutable
		}
		if receiver.Symbol != nil {
			return a.canWriteThroughSymbol(*receiver.Symbol)
		}
		return false
	}
	return true
}

// implicitMethodReceiverError explains a rejected borrow or receiver capability.
// Rules: rules/declarations/interfaces.md — §6;
// rules/declarations/functions.md — §§22,24.
func (a *Analyzer) implicitMethodReceiverError(function Function, receiver methodReceiverInfo) string {
	displayName := visibilityBaseName(function.Name)
	if function.ReceiverConsuming && receiver.Type.Kind == ReferenceType {
		return fmt.Sprintf("consuming method %s requires an owned receiver; %s does not transfer ownership", displayName, typeDisplayName(receiver.Type))
	}
	if receiver.Symbol != nil {
		if receiver.Symbol.Addressed {
			return fmt.Sprintf("method %s requires writable receiver storage", displayName)
		}
		return fmt.Sprintf("method %s requires mutable receiver", displayName)
	}
	return ""
}

// interfaceRetainsMethod recognizes only a validated retained requirement with
// identical declaration provenance and signature, including inherited methods.
// Rules: rules/declarations/interfaces.md — §§5,6; correction10-20260823.md.
func interfaceRetainsMethod(iface Type, function Function) bool {
	if iface.Kind != InterfaceType {
		return false
	}
	for _, required := range iface.InterfaceMethods {
		if sameSourceToken(required.Token, function.Token) && sameInterfaceRequirementSignature(required, function) {
			return true
		}
	}
	return false
}

// resolvedInterfaceOverloads uses the receiver's resolved requirement set, so
// generic substitutions and inherited overloads are not replaced by templates.
// Rules: rules/declarations/interfaces.md — §§5,6;
// rules/declarations/generics.md — §22 Generic interfaces;
// rules/declarations/functions.md — §§19,20,24.
func resolvedInterfaceOverloads(receiver Type, method string) ([]Function, bool) {
	iface := dereferenceType(receiver)
	if iface.Kind != InterfaceType {
		return nil, false
	}
	var group []Function
	for _, required := range iface.InterfaceMethods {
		if required.Name == method {
			group = append(group, required)
		}
	}
	return group, len(group) > 0
}

// interfaceReceiverForCall retains the contract owner for instance and qualified
// static requirement calls. An interface requirement never names a concrete body.
// Rules: rules/declarations/interfaces.md — §§3.4,6;
// rules/declarations/functions.md — §§23,24.
func (a *Analyzer) interfaceReceiverForCall(call *ast.CallExpression, receiver Type) (Type, bool) {
	if iface := dereferenceType(receiver); iface.Kind == InterfaceType {
		return iface, true
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || !a.expressionNamesType(member.Object) {
		return Type{}, false
	}
	path, ok := typePathFromExpression(member.Object)
	if !ok {
		return Type{}, false
	}
	iface, ok := a.types[a.resolveTypeName(path)]
	return iface, ok && iface.Kind == InterfaceType
}
