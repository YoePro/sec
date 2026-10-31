package sema

import (
	"fmt"
	"strings"

	"sec/internal/diagnostics"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Heterogeneous pack iteration (MD-117; functions.md §38A(5); flowcontrol_for.md
// §13A). The template body is checked once with the opaque per-position
// element type `Types[i]` (typePackElementType), an unconstrained generic
// parameter, so only operations valid for every element are accepted. The
// loop binding modes follow MD-119 for reference elements: `for value` copies
// a shared `ref T` holder, `for ref value` is a shared reborrow of the
// referent (`ref T`, never `ref ref T`) and `for ref mut value` exclusively
// reborrows the referent of a `ref mut T` element. A by-value binding of an
// owned element needs an implicitly copyable concrete element, which only a
// specialization knows; it is checked at every selected call.

// isTypePackElement reports the opaque per-position element of a pack.
func isTypePackElement(typ Type) bool {
	if typ.Kind == ReferenceType && typ.Element != nil {
		typ = *typ.Element
	}
	return typ.Kind == GenericType && strings.HasSuffix(typ.Name, "[i]")
}

// packForBindingModes applies the binding modes of a loop over a
// heterogeneous pack; false means the loop iterates something else.
func (a *Analyzer) packForBindingModes(stmt *ast.ForStatement, bindingTypes []Type) ([]Type, bool) {
	if len(bindingTypes) == 0 || !isTypePackElement(bindingTypes[len(bindingTypes)-1]) {
		return nil, false
	}
	result := append([]Type{}, bindingTypes...)
	position := len(stmt.Bindings) - 1
	binding := stmt.Bindings[position]
	element := bindingTypes[position]
	holder := element.Kind == ReferenceType && element.Element != nil
	if len(stmt.Bindings) == 2 && stmt.Bindings[0].Mode != ast.ForBindingValue {
		a.addErrorAtToken(stmt.Bindings[0].ModeToken, "the pack index binding is a position, not an element reference; remove %s", stmt.Bindings[0].Mode)
	}
	switch binding.Mode {
	case ast.ForBindingValue:
		if holder && element.ReferenceMutable {
			a.addErrorAtToken(binding.Token, "for %s cannot copy a ref mut pack element; use for ref mut %s to reborrow its referent", binding.Name, binding.Name)
			return result, true
		}
		if !holder && !binding.Discard {
			a.recordTypePackByValueLoop(binding.Token)
		}
	case ast.ForBindingRef:
		if holder {
			referent := *element.Element
			result[position] = Type{Name: referenceTypeName(referent, false), Kind: ReferenceType, Element: &referent}
		} else {
			referent := element
			result[position] = Type{Name: referenceTypeName(referent, false), Kind: ReferenceType, Element: &referent}
		}
	case ast.ForBindingRefMut:
		if !holder || !element.ReferenceMutable {
			a.addErrorAtToken(binding.ModeToken, "for ref mut %s needs a ...ref mut pack; this pack does not grant mutable element authority", binding.Name)
			return result, true
		}
		referent := *element.Element
		result[position] = Type{Name: referenceTypeName(referent, true), Kind: ReferenceType, Element: &referent, ReferenceMutable: true}
	}
	return result, true
}

// recordTypePackByValueLoop remembers that the current pack template copies
// each owned element, so every selected specialization must supply
// implicitly copyable elements (functions.md §38A(5)(5)).
func (a *Analyzer) recordTypePackByValueLoop(token lexer.Token) {
	if a.summaryPass || a.currentFunctionToken.Line == 0 {
		return
	}
	if a.typePackByValueLoops == nil {
		a.typePackByValueLoops = map[sourceTokenKey]lexer.Token{}
	}
	key := sourceTokenLocation(a.currentFunctionToken)
	if _, exists := a.typePackByValueLoops[key]; !exists {
		a.typePackByValueLoops[key] = token
	}
}

// checkTypePackByValueLoops rejects each selected pack call whose template
// copies an owned element that is not implicitly copyable in that
// specialization, naming the position and concrete type (§38A(10)(1-2)).
func (a *Analyzer) checkTypePackByValueLoops() {
	if len(a.typePackByValueLoops) == 0 {
		return
	}
	for call, resolved := range a.resolvedCalls {
		loop, copies := a.typePackByValueLoops[sourceTokenLocation(resolved.Function.Token)]
		if !copies {
			continue
		}
		sequence, ok := inferredTypePack(resolved.Function)
		if !ok {
			continue
		}
		for position, element := range sequence {
			if classification := CopyClassificationOf(element); classification == CopyMoveOnly || classification == CopyNonCopyable {
				endLine, endColumn := call.Token.EndPosition()
				a.appendError(Error{Severity: diagnostics.SeverityError,
					Message: fmt.Sprintf("%s copies each pack element by value, but position %d is %s, which is not implicitly copyable; use for ref in %s", resolved.Function.Name, position, typeDisplayName(element), resolved.Function.Name),
					File:    call.Token.File, Line: call.Token.Line, Column: call.Token.Column, EndLine: endLine, EndColumn: endColumn,
					PreviousFile: loop.File, PreviousLine: loop.Line, PreviousColumn: loop.Column, RelatedLabel: "by-value pack loop"})
				break
			}
		}
	}
}
