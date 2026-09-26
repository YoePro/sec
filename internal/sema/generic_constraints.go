package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// validateGenericParameterConstraints resolves every member of each ordered
// interface-constraint conjunction. Each invalid member receives its own
// declaration-site diagnostic rather than hiding later constraints.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §13 "Constraint resolution"
//   - rules/declarations/generics.md — §33 "Sema requirements"
func (a *Analyzer) validateGenericParameterConstraints(parameters []*ast.GenericParameter) {
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		for _, reference := range parameter.Constraints {
			if reference == nil || reference.Invalid {
				continue
			}
			name := a.resolveTypeName(reference.Name)
			constraint, ok := a.types[name]
			if !ok {
				a.addErrorAtToken(reference.Token, "unknown generic constraint %s for %s", reference.Name, parameter.Name.Value)
				continue
			}
			if constraint.Kind != InterfaceType {
				a.addErrorAtToken(reference.Token, "generic constraint %s is not an interface", reference.Name)
			}
		}
	}
}

// resolvedGenericParameterConstraints retains all valid interface constraints
// in parameter and source order on a generic callable template. Concrete
// substitution can therefore recheck every conjunct without reconstructing
// the declaration from syntax.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §13 "Constraint resolution"
//   - rules/declarations/generics.md — §33 "Sema requirements"
func (a *Analyzer) resolvedGenericParameterConstraints(parameters []*ast.GenericParameter) []GenericConstraint {
	constraints := []GenericConstraint{}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		for _, reference := range parameter.Constraints {
			if reference == nil || reference.Invalid {
				continue
			}
			base, exists := a.types[a.resolveTypeName(reference.Name)]
			if !exists || base.Kind != InterfaceType {
				continue
			}
			constraint, ok := a.resolveType(reference)
			if !ok || constraint.Kind != InterfaceType {
				continue
			}
			constraints = append(constraints, GenericConstraint{
				Parameter: parameter.Name.Value,
				Interface: constraint,
				Token:     reference.Token,
			})
		}
	}
	return constraints
}

// constrainedGenericParameterType exposes exactly the instance methods that
// the parameter's declared interface constraints guarantee. This is a
// compile-time lookup surface; it does not create an interface value or imply
// runtime dispatch.
//
// Rules:
//   - rules/declarations/generics.md — §14 "Constraint satisfaction"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
func (a *Analyzer) constrainedGenericParameterType(parameter *ast.GenericParameter) Type {
	typ := Type{Name: parameter.Name.Value, Kind: GenericType}
	typ.GenericConstraints = a.resolvedGenericParameterConstraints([]*ast.GenericParameter{parameter})
	for _, constraint := range typ.GenericConstraints {
		for _, method := range constraint.Interface.InterfaceMethods {
			if method.Static || containsEquivalentConstraintMethod(typ.InterfaceMethods, method) {
				continue
			}
			typ.InterfaceMethods = append(typ.InterfaceMethods, method)
		}
	}
	return typ
}

func containsEquivalentConstraintMethod(methods []Function, candidate Function) bool {
	for _, method := range methods {
		if method.Name == candidate.Name &&
			sameInterfaceRequirementSignature(method, candidate) &&
			sameConcreteType(method.ReturnType, candidate.ReturnType) {
			return true
		}
	}
	return false
}

// constrainedGenericMethodCall resolves an instance call through the finite
// member surface carried by a constrained generic parameter. The cloned
// requirement uses the parameter itself as its compile-time receiver so the
// ordinary overload, receiver-capability, and ownership checks remain active.
func (a *Analyzer) constrainedGenericMethodCall(expr *ast.CallExpression) ([]Function, methodReceiverInfo, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || a.expressionNamesType(member.Object) {
		return nil, methodReceiverInfo{}, false
	}
	receiver, ok := a.methodCallReceiver(expr)
	if !ok {
		return nil, methodReceiverInfo{}, false
	}
	typ := dereferenceType(receiver.Type)
	if typ.Kind != GenericType {
		return nil, methodReceiverInfo{}, false
	}
	methods := []Function{}
	for _, requirement := range typ.InterfaceMethods {
		if requirement.Static || requirement.Name != member.Property.Value {
			continue
		}
		requirement.ImplTarget = typ.Name
		methods = append(methods, requirement)
	}
	return methods, receiver, len(methods) > 0
}

// validateGenericTypeConstraintArguments verifies every concrete type
// substitution against the ordered constraints retained by a generic type
// template. Explicit, valid interface conformance is required; structurally
// matching members alone are insufficient.
//
// Rules:
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §14 "Constraint satisfaction"
//   - rules/declarations/generics.md — §25 "Concrete specialization"
//   - rules/declarations/generics.md — §33 "Sema requirements"
func (a *Analyzer) validateGenericTypeConstraintArguments(token lexer.Token, template Type) bool {
	if len(template.GenericConstraints) == 0 {
		return true
	}
	substitution := make(map[string]Type, len(template.GenericParameters))
	for index, parameter := range template.GenericParameters {
		if index < len(template.TypeArgs) {
			substitution[parameter] = template.TypeArgs[index]
		}
	}
	valid := true
	for _, constraint := range template.GenericConstraints {
		argument, exists := substitution[constraint.Parameter]
		if !exists {
			continue
		}
		required := substituteGenericType(constraint.Interface, substitution)
		if a.hasValidExplicitInterfaceConformance(argument, required) {
			continue
		}
		a.addErrorAtToken(token,
			"type %s does not satisfy constraint %s for %s",
			typeDisplayName(argument), typeDisplayName(required), constraint.Parameter)
		valid = false
	}
	return valid
}
