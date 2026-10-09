package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// interfaceSelfTypeName is the contextual `Self` of interface requirements.
const interfaceSelfTypeName = "Self"

// bindInterfaceSelfType makes `Self` resolve to a placeholder generic type
// while an interface declaration's requirements are resolved, and returns the
// function that restores the previous binding.
//
// Rules:
//   - rules/declarations/interfaces.md — 3.4 "Static member" (`Result[Self, ParseError]`)
func (a *Analyzer) bindInterfaceSelfType() func() {
	if a.genericTypes == nil {
		a.genericTypes = map[string]Type{}
	}
	previous, had := a.genericTypes[interfaceSelfTypeName]
	a.genericTypes[interfaceSelfTypeName] = Type{Name: interfaceSelfTypeName, Kind: GenericType}
	return func() {
		if had {
			a.genericTypes[interfaceSelfTypeName] = previous
		} else {
			delete(a.genericTypes, interfaceSelfTypeName)
		}
	}
}

// substituteInterfaceSelf replaces `Self` in a requirement's parameters and
// result with the conforming type.
func substituteInterfaceSelf(required Function, conforming Type) Function {
	substitution := map[string]Type{interfaceSelfTypeName: conforming}
	out := required
	out.Parameters = make([]FunctionParameter, len(required.Parameters))
	for index, parameter := range required.Parameters {
		parameter.Type = substituteGenericType(parameter.Type, substitution)
		out.Parameters[index] = parameter
	}
	out.ReturnType = substituteGenericType(required.ReturnType, substitution)
	return out
}

// closestInterfaceCandidate picks the overload to explain: one with the same
// static/instance membership and parameter count if any, otherwise the first.
func closestInterfaceCandidate(methods []Function, required Function) Function {
	for _, method := range methods {
		if method.Static == required.Static && len(explicitInterfaceComparableParameters(method.Parameters)) == len(explicitInterfaceComparableParameters(required.Parameters)) {
			return method
		}
	}
	for _, method := range methods {
		if method.Static == required.Static {
			return method
		}
	}
	return methods[0]
}

// interfaceMethodMismatch names the first § 6 conformance difference between
// an implementation and a requirement whose `Self` is already substituted.
//
// Rules:
//   - rules/declarations/interfaces.md — 6 "Conformance requirements", 12 "Diagnostics"
func interfaceMethodMismatch(method Function, required Function) string {
	membership := func(static bool) string {
		if static {
			return "a static"
		}
		return "an instance"
	}
	if method.Static != required.Static {
		return fmt.Sprintf("the interface requires %s method, but it is declared as %s method", membership(required.Static), membership(method.Static))
	}
	if method.ReceiverMutable && !required.ReceiverMutable {
		return "the method requires a mutable receiver, but the interface promises a shared receiver"
	}
	methodParameters := explicitInterfaceComparableParameters(method.Parameters)
	requiredParameters := explicitInterfaceComparableParameters(required.Parameters)
	if len(methodParameters) != len(requiredParameters) {
		return fmt.Sprintf("the interface requires %d parameter(s), but the method declares %d", len(requiredParameters), len(methodParameters))
	}
	for index := range requiredParameters {
		got, want := methodParameters[index], requiredParameters[index]
		if parameterModeDisplay(got) != parameterModeDisplay(want) {
			return fmt.Sprintf("parameter %d %s is %s, but the interface requires %s", index+1, got.Name, parameterModeDisplay(got), parameterModeDisplay(want))
		}
		if !sameConcreteType(got.Type, want.Type) {
			return fmt.Sprintf("parameter %d %s has type %s, but the interface requires %s", index+1, got.Name, typeDisplayName(got.Type), typeDisplayName(want.Type))
		}
	}
	if !sameConcreteType(method.ReturnType, required.ReturnType) {
		return fmt.Sprintf("the result type is %s, but the interface requires %s", typeDisplayName(method.ReturnType), typeDisplayName(required.ReturnType))
	}
	return "the callable contract differs from the requirement"
}

func parameterModeDisplay(parameter FunctionParameter) string {
	switch {
	case parameter.Consuming:
		return "consuming (->)"
	case parameter.MutableRef || (parameter.Type.Kind == ReferenceType && parameter.Type.ReferenceMutable):
		return "a ref mut borrow"
	case parameter.Ref || parameter.Type.Kind == ReferenceType:
		return "a ref borrow"
	}
	return "by value"
}

// interfaceRequirementDisplay renders a requirement as Sec source.
func interfaceRequirementDisplay(required Function) string {
	var text strings.Builder
	if required.Static {
		text.WriteString("static ")
	} else if required.ReceiverMutable {
		text.WriteString("mut ")
	}
	text.WriteString("fn ")
	text.WriteString(required.Name)
	text.WriteString("(")
	for index, parameter := range explicitInterfaceComparableParameters(required.Parameters) {
		if index > 0 {
			text.WriteString(", ")
		}
		if parameter.Consuming {
			text.WriteString("->")
		}
		text.WriteString(parameter.Name)
		text.WriteString(": ")
		if parameter.Type.Kind != ReferenceType {
			if parameter.MutableRef {
				text.WriteString("ref mut ")
			} else if parameter.Ref {
				text.WriteString("ref ")
			}
		}
		text.WriteString(typeDisplayName(parameter.Type))
	}
	text.WriteString(") ")
	text.WriteString(typeDisplayName(required.ReturnType))
	return text.String()
}

// interfaceClauseAnchor places a missing-member diagnostic on the type's
// `implements` reference to the interface, falling back to the requirement.
func (a *Analyzer) interfaceClauseAnchor(typ Type, iface Type, fallback lexer.Token) lexer.Token {
	if clause, ok := a.implementsClauseTokens[typ.Name+"\x00"+iface.Name]; ok {
		return clause
	}
	return fallback
}

// interfaceMemberAnchor places an incompatibility on the implementing member.
func interfaceMemberAnchor(member lexer.Token, fallback lexer.Token) lexer.Token {
	if validDefinitionToken(member) {
		return member
	}
	return fallback
}

// addInterfaceConformanceError reports a conformance failure at the
// implementation with the interface requirement as related location.
//
// Rules:
//   - rules/declarations/interfaces.md — 12 "Diagnostics"
//   - rules/tooling/diagnostics.md — related locations
func (a *Analyzer) addInterfaceConformanceError(token lexer.Token, requirement lexer.Token, id string, help string, format string, args ...any) {
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID:             id,
		Severity:       diagnostics.SeverityError,
		Help:           help,
		Message:        fmt.Sprintf(format, args...),
		File:           token.File,
		Line:           token.Line,
		Column:         token.Column,
		EndLine:        endLine,
		EndColumn:      endColumn,
		PreviousFile:   requirement.File,
		PreviousLine:   requirement.Line,
		PreviousColumn: requirement.Column,
		RelatedLabel:   "interface requirement",
	})
}

// functionSignatureHasInvalidType reports a parameter or result type that did
// not resolve.
func functionSignatureHasInvalidType(function Function) bool {
	if function.ReturnType.Kind == InvalidType {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == InvalidType {
			return true
		}
	}
	return false
}

// resolveImplementedInterfaces resolves explicit interface contracts and
// anchors compiler-owned requirements at the current implements clause without
// mutating a cached specialization or another declaration's diagnostic owner.
// Rules: rules/declarations/interfaces.md — §§4, 9.1, 12;
// rules/compiler/compiler_known_members.md — Iteration protocol.
func (a *Analyzer) resolveImplementedInterfaces(refs []*ast.TypeReference, targetName string) []Type {
	if len(refs) == 0 {
		return nil
	}

	implemented := []Type{}
	seen := map[string]lexer.Token{}
	for _, ref := range refs {
		typ, ok := a.resolveType(ref)
		if !ok {
			continue
		}
		if typ.Kind != InterfaceType {
			a.addErrorAtToken(ref.Token, "implemented type %s on %s is not an interface", typeDisplayName(typ), targetName)
			continue
		}
		// Compiler-known interfaces have no source declaration token. Anchor
		// conformance diagnostics at the explicit implements clause instead of
		// leaking a synthetic 0:0 location to CLI and LSP consumers.
		if typ.Intrinsic {
			typ.InterfaceMethods = append([]Function(nil), typ.InterfaceMethods...)
			typ.InterfaceProperties = append([]InterfaceProperty(nil), typ.InterfaceProperties...)
			typ.InterfaceEvents = append([]InterfaceEvent(nil), typ.InterfaceEvents...)
			for index := range typ.InterfaceMethods {
				typ.InterfaceMethods[index].Token = ref.Token
			}
			for index := range typ.InterfaceProperties {
				typ.InterfaceProperties[index].Token = ref.Token
			}
			for index := range typ.InterfaceEvents {
				typ.InterfaceEvents[index].Token = ref.Token
			}
		}
		if previous, exists := seen[typ.Name]; exists {
			_ = previous
			a.addErrorAtToken(ref.Token, "duplicate implemented interface %s on %s", typeDisplayName(typ), targetName)
			continue
		}
		if _, recorded := a.implementsClauseTokens[targetName+"\x00"+typ.Name]; !recorded {
			a.implementsClauseTokens[targetName+"\x00"+typ.Name] = ref.Token
		}
		seen[typ.Name] = ref.Token
		implemented = append(implemented, typ)
	}
	return implemented
}
