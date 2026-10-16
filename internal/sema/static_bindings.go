package sema

import "sec/internal/ast"

// StaticBindingID is the stable identity of one static-duration binding,
// allocated in declaration-analysis order.
type StaticBindingID uint32

// ResolvedStatic is the read-only resolution of a binding with static
// storage duration: a module-level `let` / `let mut` (static by storage
// duration) or a function-local `static let` / `static let mut`. A function
// -local static keeps one storage location across calls, so it must never be
// lowered as automatic local storage.
//
// Rules:
//   - rules/declarations/static.md — §§ 2, 3, 4
//   - rules/compiler/semantic_ir.md — § 28
type ResolvedStatic struct {
	ID      StaticBindingID
	Name    string
	Type    Type
	Mutable bool
	// Function names the enclosing function of a function-local static;
	// empty at module level.
	Function    string
	Declaration *ast.LetStatement
}

// recordStaticBinding publishes the static resolution of a let statement
// whose binding has static storage duration (not an addressed register).
func (a *Analyzer) recordStaticBinding(stmt *ast.LetStatement, symbol Symbol) {
	if a.summaryPass || stmt == nil || stmt.Name == nil || stmt.Address != nil || symbol.Storage != StorageOriginStatic || !validDefinitionToken(stmt.Name.Token) {
		return
	}
	key := sourceTokenLocation(stmt.Name.Token)
	if a.staticBindings == nil {
		a.staticBindings = map[sourceTokenKey]ResolvedStatic{}
	}
	if _, exists := a.staticBindings[key]; exists {
		return
	}
	function := ""
	if a.inFunctionBody {
		function = a.currentFunctionName
	}
	a.staticBindings[key] = ResolvedStatic{
		ID: StaticBindingID(len(a.staticBindings) + 1), Name: stmt.Name.Value, Type: symbol.Type,
		Mutable: symbol.Mutable, Function: function, Declaration: stmt,
	}
}

// ResolvedStaticOf returns the static binding an identifier use or
// declaration resolves to.
func (a *Analyzer) ResolvedStaticOf(identifier *ast.Identifier) (ResolvedStatic, bool) {
	if a == nil || identifier == nil {
		return ResolvedStatic{}, false
	}
	use := sourceTokenLocation(identifier.Token)
	if static, ok := a.staticBindings[use]; ok {
		return static, true
	}
	definitions := a.definitionTokens[use]
	if len(definitions) != 1 {
		return ResolvedStatic{}, false
	}
	static, ok := a.staticBindings[sourceTokenLocation(definitions[0])]
	return static, ok
}

// ResolvedStaticDeclarations returns the static bindings declared at module
// level, in identity order.
func (a *Analyzer) ResolvedStaticDeclarations() []ResolvedStatic {
	statics := make([]ResolvedStatic, len(a.staticBindings))
	for _, static := range a.staticBindings {
		statics[static.ID-1] = static
	}
	return statics
}
