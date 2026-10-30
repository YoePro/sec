package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sort"
)

// validateInterfaceConformance validates explicit concrete conformance and records
// compiler-owned results for tooling.
// Rules: rules/declarations/interfaces.md — §§4–6,12;
// rules/compiler/semantic_ir.md — §109(1–4).
func (a *Analyzer) validateInterfaceConformance() {
	a.resolveInterfaceSetterContracts()
	names := make([]string, 0, len(a.types))
	for name := range a.types {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		typ := a.types[name]
		if typ.Kind == InvalidType || len(typ.Implements) == 0 {
			continue
		}
		if typ.Kind == InterfaceType {
			continue
		}
		for _, iface := range typ.Implements {
			if a.invalidInterfaceInheritance[iface.Name] {
				continue
			}
			a.validateTypeImplementsInterface(typ, iface)
		}
	}
}

// validateTypeImplementsInterface diagnoses incompatible members and publishes
// their structured identities alongside the existing canonical validity check.
// Rules: rules/declarations/interfaces.md — §§4–6,12;
// rules/compiler/semantic_ir.md — §109(1–4).
func (a *Analyzer) validateTypeImplementsInterface(typ Type, iface Type) {
	fact := InterfaceConformanceFact{TypeName: typ.Name, InterfaceName: iface.Name,
		Interface: typeDisplayName(iface), TypeDeclaration: a.typeDefinitionTokens[typ.Name],
		InterfaceDeclaration: a.typeDefinitionTokens[iface.Name],
		Clause:               a.interfaceClauseAnchor(typ, iface, a.typeDefinitionTokens[typ.Name])}
	defer func() {
		fact.State = diagnostics.ProofUnproven
		if a.typeSatisfiesInterfaceRequirements(typ, iface) {
			fact.State = diagnostics.ProofValid
		}
		if len(fact.Failures) > 0 {
			fact.State = diagnostics.ProofInvalid
		}
		// Reused analyzers may retain declaration caches; only declarations
		// recorded in the current program snapshot are tooling evidence.
		if fact.TypeDeclaration.Line > 0 {
			a.interfaceConformanceFacts = append(a.interfaceConformanceFacts, fact)
		}
	}()
	addFailure := func(member string, token lexer.Token, requirement lexer.Token, id string, help string, format string, args ...any) {
		diagnostic := a.addInterfaceConformanceError(token, requirement, id, help, format, args...)
		fact.Failures = append(fact.Failures, InterfaceConformanceFailure{Member: member, Diagnostic: diagnostic})
	}
	clause, hasClause := a.implementsClauseTokens[typ.Name+"\x00"+iface.Name]
	for _, required := range iface.InterfaceMethods {
		methods := a.functions[typ.Name+"."+required.Name]
		if len(methods) == 0 {
			anchor := required.Token
			if hasClause {
				anchor = clause
			}
			addFailure(required.Name, anchor, required.Token, diagnostics.InterfaceMemberMissing,
				"declare "+implMethodDisplay(substituteInterfaceSelf(required, typ))+" in the impl of "+typ.Name,
				"type %s implements %s but is missing method %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !hasCompatibleInterfaceMethod(typ, iface, methods, required) {
			method := closestInterfaceCandidate(methods, required)
			// An unresolved type is already diagnosed where it is written;
			// comparing against it would only report a cascade.
			if functionSignatureHasInvalidType(required) || functionSignatureHasInvalidType(method) {
				continue
			}
			anchor := method.Token
			if !validDefinitionToken(anchor) {
				anchor = required.Token
			}
			expected := substituteInterfaceSelf(required, typ)
			addFailure(required.Name, anchor, required.Token, diagnostics.InterfaceMemberIncompatible,
				"interface "+iface.Name+" requires "+interfaceRequirementDisplay(expected),
				"type %s method %s does not match interface %s: %s", typ.Name, required.Name, iface.Name, interfaceMethodMismatch(method, expected))
		}
	}

	for _, required := range iface.InterfaceProperties {
		property, ok := lookupProperty(typ, required.Name)
		if !ok {
			addFailure(required.Name, a.interfaceClauseAnchor(typ, iface, required.Token), required.Token, diagnostics.InterfaceMemberMissing, "declare the property in the impl of "+typ.Name, "type %s implements %s but is missing property %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !sameConcreteType(property.Type, required.Type) {
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s must be %s for interface %s, got %s", typ.Name, required.Name, typeDisplayName(required.Type), iface.Name, typeDisplayName(property.Type))
			continue
		}
		// rules/declarations/properties.md, section 10. Interface
		// conformance includes the static-versus-instance category.
		if property.Static != required.Static {
			requiredKind, actualKind := "instance", "static"
			if required.Static {
				requiredKind, actualKind = "static", "instance"
			}
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s must be %s for interface %s, got %s property", typ.Name, required.Name, requiredKind, iface.Name, actualKind)
			continue
		}
		if required.RequiresGet && !property.HasGetter {
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s must provide get for interface %s", typ.Name, required.Name, iface.Name)
		}
		if required.RequiresSet && !property.HasSetter {
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s must provide set for interface %s", typ.Name, required.Name, iface.Name)
		} else if required.RequiresSet && property.Fallible != required.SetterFallible {
			requiredKind := "infallible"
			actualKind := "fallible"
			if required.SetterFallible {
				requiredKind, actualKind = "fallible", "infallible"
			}
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s must provide %s setter for interface %s, got %s setter", typ.Name, required.Name, requiredKind, iface.Name, actualKind)
		} else if contract := a.interfaceSetterErrors[iface.Name+"."+required.Name]; required.RequiresSet && required.SetterFallible && !setterErrorContractSatisfied(property, contract) {
			addFailure(required.Name, interfaceMemberAnchor(property.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s property %s setter error %s does not satisfy %s required by interface %s",
				typ.Name, required.Name, typeDisplayName(*property.Error), typeDisplayName(*contract), iface.Name)
		}
	}

	for _, required := range iface.InterfaceEvents {
		event, ok := lookupEvent(typ, required.Name)
		if !ok {
			addFailure(required.Name, a.interfaceClauseAnchor(typ, iface, required.Token), required.Token, diagnostics.InterfaceMemberMissing, "declare the event in the impl of "+typ.Name, "type %s implements %s but is missing event %s", typ.Name, iface.Name, required.Name)
			continue
		}
		if !sameConcreteType(event.Payload, required.Payload) {
			addFailure(required.Name, interfaceMemberAnchor(event.Token, required.Token), required.Token, diagnostics.InterfaceMemberIncompatible, "", "type %s event %s payload must be %s for interface %s, got %s", typ.Name, required.Name, typeDisplayName(required.Payload), iface.Name, typeDisplayName(event.Payload))
		}
	}
}

// hasCompatibleInterfaceMethod checks the substituted signature and receiver contract.
// Rules: rules/declarations/interfaces.md — §6 "Conformance requirements".
func hasCompatibleInterfaceMethod(typ Type, iface Type, methods []Function, required Function) bool {
	required = substituteInterfaceSelf(required, typ)
	for _, method := range methods {
		if compatibleInterfaceMethodSignature(method, required) &&
			method.Static == required.Static &&
			(required.ReceiverMutable || !method.ReceiverMutable) &&
			sameConcreteType(method.ReturnType, required.ReturnType) {
			return true
		}
	}
	return false
}
