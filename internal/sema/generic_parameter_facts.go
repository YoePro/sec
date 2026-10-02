package sema

import (
	"strings"

	"sec/internal/lexer"
)

// GenericParameterFact is the resolved compile-time view of one generic
// parameter declaration: its ordered interface-constraint conjunction and the
// instance-method surface those constraints guarantee inside the generic
// body. Tooling presents it instead of reconstructing constraints from syntax.
type GenericParameterFact struct {
	Name              string
	Token             lexer.Token
	Constraints       []GenericConstraint
	GuaranteedMethods []Function
	// ImplTarget names the type whose declaration owns the constraints when
	// the parameter is introduced by a generic impl target such as
	// `impl Pair[K, V]`; it is empty for a declaration's own parameter list.
	ImplTarget string
}

// recordGenericParameterFact retains the constrained parameter type built for
// a generic scope, keyed by the parameter's declaration token.
//
// Rules:
//   - rules/declarations/generics.md — §8 "Generic impl blocks"
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
//   - rules/tooling/lsp.md — "Hover"
func (a *Analyzer) recordGenericParameterFact(typ Type, token lexer.Token, implTarget string) {
	if !validDefinitionToken(token) {
		return
	}
	a.genericParameterFacts[sourceTokenLocation(token)] = GenericParameterFact{
		Name:              typ.Name,
		Token:             token,
		Constraints:       append([]GenericConstraint(nil), typ.GenericConstraints...),
		GuaranteedMethods: append([]Function(nil), typ.InterfaceMethods...),
		ImplTarget:        implTarget,
	}
}

// GenericParameterFactAt returns the resolved fact for a generic parameter
// declaration token, such as a definition returned by DefinitionsAt.
func (a *Analyzer) GenericParameterFactAt(definition lexer.Token) (GenericParameterFact, bool) {
	fact, ok := a.genericParameterFacts[sourceTokenLocation(definition)]
	return fact, ok
}

// GenericParameterDisplays renders ordered generic parameters with their
// resolved constraint conjunctions, preserving parameter order and the source
// order of each parameter's conjuncts: `T: Named & Ranked`, `U`.
//
// Rules:
//   - rules/declarations/generics.md — §11 "Constraints", §12 "Multiple constraints"
func GenericParameterDisplays(parameters []string, constraints []GenericConstraint) []string {
	displays := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		display := parameter
		if conjuncts := GenericConstraintDisplays(parameter, constraints); len(conjuncts) > 0 {
			display += ": " + strings.Join(conjuncts, " & ")
		}
		displays = append(displays, display)
	}
	return displays
}

// GenericConstraintDisplays returns the resolved interface names constraining
// one parameter, in source order.
func GenericConstraintDisplays(parameter string, constraints []GenericConstraint) []string {
	conjuncts := []string{}
	for _, constraint := range constraints {
		if constraint.Parameter == parameter {
			conjuncts = append(conjuncts, typeDisplayName(constraint.Interface))
		}
	}
	return conjuncts
}
