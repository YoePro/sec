package sema

import (
	"sort"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// confusableSkeleton caches the UTS #39 skeleton of an identifier spelling.
func (a *Analyzer) confusableSkeleton(name string) string {
	if skeleton, ok := a.confusableSkeletons[name]; ok {
		return skeleton
	}
	skeleton := lexer.ConfusableSkeleton(name)
	a.confusableSkeletons[name] = skeleton
	return skeleton
}

// reportConfusableIdentifier emits the non-suppressible confusable-collision
// error naming both declarations.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.8–2.10
//   - rules/foundations/lexical_structure.md — "Visually confusable identifiers"
func (a *Analyzer) reportConfusableIdentifier(name string, token lexer.Token, previousName string, previous lexer.Token) {
	a.addErrorAtTokenWithMetadataAndPrevious(token, previous, diagnostics.ConfusableIdentifier,
		"Rename one of the declarations so the two names do not look alike.",
		"identifier %s is confusable with %s declared here: the spellings are distinct, but their UTS #39 confusable forms collide in the same declaration domain",
		name, previousName)
}

// checkLocalConfusableIdentifier compares a new local declaration with every
// declaration it would conflict with by spelling: the visible local,
// parameter, and module bindings, and the module types, functions, and
// generic parameters a local may not shadow. Implicit member aliases may be
// shadowed and unit symbols live in a separate namespace, so neither is
// compared. Locals in unrelated scopes are never visible together and are
// therefore never compared.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.5–2.10, 8.10
//   - rules/foundations/names_scopes_visibility.md — no shadowing of visible declarations
func (a *Analyzer) checkLocalConfusableIdentifier(name string, token lexer.Token) bool {
	skeleton := a.confusableSkeleton(name)
	for other, symbol := range a.symbols {
		if other == name || symbol.ImplicitMember || !validDefinitionToken(symbol.Token) {
			continue
		}
		if a.confusableSkeleton(other) == skeleton {
			a.reportConfusableIdentifier(name, token, other, symbol.Token)
			return true
		}
	}
	for other, definition := range a.genericTypeDefinitions {
		if other != name && validDefinitionToken(definition) && a.confusableSkeleton(other) == skeleton {
			a.reportConfusableIdentifier(name, token, other, definition)
			return true
		}
	}
	for _, other := range a.moduleConfusableIndex()[skeleton] {
		if other.name != name {
			a.reportConfusableIdentifier(name, token, other.name, other.token)
			return true
		}
	}
	return false
}

type confusableDeclaration struct {
	name  string
	token lexer.Token
}

// moduleConfusableIndex indexes, by skeleton, the module-level types and free
// functions of the current module that a local may not shadow.
func (a *Analyzer) moduleConfusableIndex() map[string][]confusableDeclaration {
	if index, ok := a.confusableModuleIndex[a.currentModule]; ok {
		return index
	}
	index := map[string][]confusableDeclaration{}
	for name, typ := range a.types {
		if typ.Module != a.currentModule || a.isUnitSymbol(name) {
			continue
		}
		if token, ok := a.typeDefinitionTokens[name]; ok && validDefinitionToken(token) {
			skeleton := a.confusableSkeleton(name)
			index[skeleton] = append(index[skeleton], confusableDeclaration{name: name, token: token})
		}
	}
	for name, functions := range a.functions {
		for _, function := range a.accessibleFunctions(functions) {
			if function.ImplTarget != "" || !validDefinitionToken(function.Token) {
				continue
			}
			skeleton := a.confusableSkeleton(name)
			index[skeleton] = append(index[skeleton], confusableDeclaration{name: name, token: function.Token})
			break
		}
	}
	a.confusableModuleIndex[a.currentModule] = index
	return index
}

// checkConfusableInDomains compares a new declaration with the declarations
// already in its conflicting declaration domain: the members of one type
// (fields, methods, properties, events), the parameters of one callable, the
// fields of one struct, or the variants of one enum or union. Any distinct
// spelling with the same UTS #39 skeleton is reported once, deterministically
// against the alphabetically first collision. The declaration itself is kept
// so later analysis does not cascade into undefined-name errors.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 2.5–2.10
//   - rules/foundations/lexical_structure.md — "Visually confusable identifiers"
func (a *Analyzer) checkConfusableInDomains(name string, token lexer.Token, domains ...map[string]lexer.Token) bool {
	skeleton := a.confusableSkeleton(name)
	collisions := []string{}
	tokens := map[string]lexer.Token{}
	for _, domain := range domains {
		for other, otherToken := range domain {
			if other == name || !validDefinitionToken(otherToken) || a.confusableSkeleton(other) != skeleton {
				continue
			}
			if _, seen := tokens[other]; !seen {
				collisions = append(collisions, other)
				tokens[other] = otherToken
			}
		}
	}
	if len(collisions) == 0 {
		return false
	}
	sort.Strings(collisions)
	a.reportConfusableIdentifier(name, token, collisions[0], tokens[collisions[0]])
	return true
}

// typeMethodTokens returns the methods already registered on target by
// earlier impl blocks, keyed by method name.
func (a *Analyzer) typeMethodTokens(target string) map[string]lexer.Token {
	methods := map[string]lexer.Token{}
	prefix := target + "."
	for key, functions := range a.functions {
		if !strings.HasPrefix(key, prefix) || strings.Contains(key[len(prefix):], ".") {
			continue
		}
		for _, function := range functions {
			if function.ImplTarget == target && validDefinitionToken(function.Token) {
				methods[key[len(prefix):]] = function.Token
				break
			}
		}
	}
	return methods
}
