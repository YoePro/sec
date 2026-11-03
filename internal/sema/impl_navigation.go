package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// recordImplNavigation links the impl keyword of each validated extension to
// its primary block and the primary keyword to all its extensions. Target
// identifiers retain ordinary type-definition navigation. Reuse the validated
// same-module composition rather than resolving names again in the editor.
// Rules: rules/declarations/impl.md — §§4–5, §22;
// rules/tooling/lsp.md — "One semantic source of truth", "Navigation".
func (a *Analyzer) recordImplNavigation(program *ast.Program) {
	type identity struct{ module, target string }
	type fragments struct {
		primary    lexer.Token
		extensions []lexer.Token
	}
	groups := map[identity]*fragments{}
	a.withProgramModules(program, func(statement ast.Statement) {
		impl, ok := statement.(*ast.ImplStatement)
		if !ok || impl == nil || impl.Target == nil || !a.validImplStatements[impl] || !validDefinitionToken(impl.Token) {
			return
		}
		key := identity{a.currentModule, a.implBlockKey(impl)}
		group := groups[key]
		if group == nil {
			group = &fragments{}
			groups[key] = group
		}
		if impl.Extends {
			group.extensions = append(group.extensions, impl.Token)
		} else {
			group.primary = impl.Token
		}
	})
	for _, group := range groups {
		if !validDefinitionToken(group.primary) {
			continue
		}
		// Navigation order follows source locations, not file discovery order.
		sort.Slice(group.extensions, func(i, j int) bool {
			left, right := group.extensions[i], group.extensions[j]
			if left.File != right.File {
				return left.File < right.File
			}
			if left.Line != right.Line {
				return left.Line < right.Line
			}
			return left.Column < right.Column
		})
		// These links describe source organization, not semantic uses. Do not
		// run bindDefinitions' production-to-test dependency checks for them.
		if len(group.extensions) != 0 {
			a.definitionTokens[sourceTokenLocation(group.primary)] = group.extensions
		}
		for _, extension := range group.extensions {
			a.definitionTokens[sourceTokenLocation(extension)] = []lexer.Token{group.primary}
		}
	}
}
