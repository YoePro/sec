package sema

import "sec/internal/ast"

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
