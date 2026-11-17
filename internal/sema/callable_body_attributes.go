package sema

import "sec/internal/ast"

// CallableBodyAttributeSet is the attachment set written immediately before
// a callable body that carries a verified guarantee, together with the
// callable that owns the contract: a function, init, free, property accessor
// or directly initialized lambda. The contract belongs to the callable, not
// to a containing let, property or impl (MD-030).
type CallableBodyAttributeSet struct {
	Attributes []*ast.Attribute
	Callable   CallableID
	// Kind is the body kind label ("init", "free", "property get accessor",
	// "lambda", ...) and Name the callable's diagnostic name.
	Kind string
	Name string
}

// CallableBodyAttributeAt returns the attribute whose name is at the given
// source position when it belongs to a verified callable-body attachment set,
// with that set. LSP hover uses it to show the guarantee on the real
// callable and its cause path rather than on the binding.
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)", "LSP behavior"
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 8(4), 9(5)
func (a *Analyzer) CallableBodyAttributeAt(file string, line, column int) (*ast.Attribute, CallableBodyAttributeSet, bool) {
	if a == nil {
		return nil, CallableBodyAttributeSet{}, false
	}
	for _, set := range a.bodyAttributeSets {
		for _, attribute := range set.Attributes {
			if attribute == nil || attribute.Name == nil {
				continue
			}
			token := attribute.Name.Token
			if token.File == file && token.Line == line && token.Column == column {
				return attribute, set, true
			}
		}
	}
	return nil, CallableBodyAttributeSet{}, false
}
