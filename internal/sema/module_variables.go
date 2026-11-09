package sema

import "sec/internal/ast"

// Module variables are module-scoped (names_scopes_visibility.md: a module's
// declarations are visible in that module; another module reaches them only
// through a qualified import). The analyzer keeps them in the shared root
// symbol table for the many readers that look names up there, and makes each
// body and each module-level statement see exactly its own module's
// variables: a variable of another module with the same name is hidden, and
// the module's own one is restored where another module's variable of that
// name took the root entry.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — module scope
//   - rules/projects/modules.md — module identity and qualified access

// scopeModuleVariables adjusts a freshly opened child layer to the current
// module's view: other modules' variables are hidden and the module's own
// are visible under their names.
func (a *Analyzer) scopeModuleVariables() {
	if len(a.moduleVariables) == 0 {
		return
	}
	for module, variables := range a.moduleVariables {
		if module == a.currentModule {
			continue
		}
		for name := range variables {
			if current, ok := a.symbols.lookup(name); ok && !current.Local && current.Module == module {
				a.symbols.del(name)
				if completion, known := a.completionSymbols.lookup(name); known && completion.Module == module {
					a.completionSymbols.del(name)
				}
			}
		}
	}
	for name, symbol := range a.moduleVariables[a.currentModule] {
		if current, ok := a.symbols.lookup(name); !ok || current.Module != a.currentModule || current.Token != symbol.Token {
			a.symbols.set(name, symbol)
			a.completionSymbols.set(name, symbol)
		}
	}
}

// analyzeModuleStatement analyzes one module-level statement in a layer with
// the current module's view, then promotes the module variables it defined
// to the root table and the per-module record.
func (a *Analyzer) analyzeModuleStatement(stmt ast.Statement) {
	root, completionRoot := a.symbols, a.completionSymbols
	a.symbols, a.completionSymbols = root.child(), completionRoot.child()
	a.scopeModuleVariables()
	a.analyzeStatement(stmt)
	layer, completionLayer := a.symbols, a.completionSymbols
	a.symbols, a.completionSymbols = root, completionRoot
	for name, symbol := range layer.local {
		if symbol.Local {
			continue
		}
		if symbol.Module == "" {
			symbol.Module = a.currentModule
		}
		root.set(name, symbol)
		if completion, ok := completionLayer.local[name]; ok {
			completion.Module = symbol.Module
			completionRoot.set(name, completion)
		}
		if a.moduleVariables == nil {
			a.moduleVariables = map[string]map[string]Symbol{}
		}
		if a.moduleVariables[symbol.Module] == nil {
			a.moduleVariables[symbol.Module] = map[string]Symbol{}
		}
		a.moduleVariables[symbol.Module][name] = symbol
	}
}
