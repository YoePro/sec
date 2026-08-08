package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// BindingID is the stable identity assigned by one successful analysis to a
// parameter or local declaration. Zero is reserved for unresolved bindings.
type BindingID uint32

type BindingKind string

const (
	BindingParameter BindingKind = "parameter"
	BindingLocal     BindingKind = "local"
)

type ResolvedBinding struct {
	ID      BindingID
	Kind    BindingKind
	Name    string
	Type    Type
	Mutable bool
}

type ResolvedCallKind string

const (
	ResolvedDirectCall       ResolvedCallKind = "direct"
	ResolvedForeignCall      ResolvedCallKind = "foreign"
	ResolvedStaticMethodCall ResolvedCallKind = "static-method"
)

type ResolvedCall struct {
	Function Function
	Kind     ResolvedCallKind
}

// ResolvedTypeOf returns only facts recorded by the completed analysis. It
// never invokes inference and does not mutate Analyzer state.
func (a *Analyzer) ResolvedTypeOf(expr ast.Expression) (Type, bool) {
	if a == nil || expr == nil {
		return Type{}, false
	}
	typ, ok := a.expressionTypes[expr]
	return typ, ok && typ.Kind != InvalidType
}

// ResolvedFunctionForDeclaration returns the already registered declaration.
func (a *Analyzer) ResolvedFunctionForDeclaration(decl *ast.FunctionDeclaration) (Function, bool) {
	if a == nil || decl == nil || decl.Name == nil {
		return Function{}, false
	}
	return a.lookupFunctionByToken(decl.Name.Value, decl.Name.Token)
}

// ResolvedBindingOf returns the declaration identity selected by Sema for an
// identifier occurrence.
func (a *Analyzer) ResolvedBindingOf(identifier *ast.Identifier) (ResolvedBinding, bool) {
	if a == nil || identifier == nil {
		return ResolvedBinding{}, false
	}
	use := sourceTokenLocation(identifier.Token)
	definitions := a.definitionTokens[use]
	if len(definitions) != 1 {
		return ResolvedBinding{}, false
	}
	declaration := definitions[0]
	key := sourceTokenLocation(declaration)
	id, ok := a.bindingIDs[key]
	if !ok {
		return ResolvedBinding{}, false
	}
	fact := a.bindingFacts[key]
	fact.ID = id
	return fact, true
}

func (a *Analyzer) ResolvedCallTarget(call *ast.CallExpression) (ResolvedCall, bool) {
	if a == nil || call == nil {
		return ResolvedCall{}, false
	}
	resolved, ok := a.resolvedCalls[call]
	return resolved, ok
}

func (a *Analyzer) recordBinding(token lexer.Token, kind BindingKind, name string, typ Type, mutable bool) {
	key := sourceTokenLocation(token)
	if !validDefinitionToken(token) {
		return
	}
	if _, exists := a.bindingIDs[key]; exists {
		return
	}
	a.bindingIDs[key] = a.nextBindingID
	a.bindingFacts[key] = ResolvedBinding{ID: a.nextBindingID, Kind: kind, Name: name, Type: typ, Mutable: mutable}
	a.nextBindingID++
}

func resolvedCallKind(dispatch CallDispatchKind) ResolvedCallKind {
	switch dispatch {
	case CallDispatchForeign:
		return ResolvedForeignCall
	case CallDispatchStaticMethod:
		return ResolvedStaticMethodCall
	default:
		return ResolvedDirectCall
	}
}
