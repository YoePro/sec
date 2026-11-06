package sema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sec/internal/diagnostics"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Heterogeneous pack iteration (MD-117; functions.md §38A(5); flowcontrol_for.md
// §13A). The template body is checked once with the opaque per-position
// element type `Types[value]` (typePackElementType), an unconstrained generic
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
	return typ.Kind == GenericType && strings.HasSuffix(typ.Name, "]")
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
			if !a.recordTypePackLoopBorrow(stmt, binding, referent) {
				return result, true
			}
			originName, originToken, originLocal, originStorage, generation := a.referenceOriginForExpression(stmt.Iterable)
			result[position].ReferenceOriginName = originName
			result[position].ReferenceOriginToken = originToken
			result[position].ReferenceOriginLocal = originLocal
			result[position].ReferenceOriginStorage = originStorage
			result[position].ReferenceOriginGeneration = generation
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

// recordTypePackLoopBorrow publishes the shared borrow `for ref value` takes of
// each callee-owned element, whose Place is rooted in the pack parameter. The
// borrow's lifetime is the iteration; it cannot escape, because no source type
// names the per-position element (functions.md §38A(8)(1)).
func (a *Analyzer) recordTypePackLoopBorrow(stmt *ast.ForStatement, binding ast.ForBinding, referent Type) bool {
	place, ok := a.resolvePlace(stmt.Iterable)
	if !ok || !place.Addressable {
		a.addErrorAtToken(binding.ModeToken, "%s loop binding requires a stable addressable iterable; bind the collection to a local first", binding.Mode)
		return false
	}
	place.Type = referent
	if a.typePackLoopBorrows == nil {
		a.typePackLoopBorrows = map[*ast.ForStatement]ResolvedReferencePlan{}
	}
	a.typePackLoopBorrows[stmt] = ResolvedReferencePlan{Facts: a.referenceFactsForPlace(place, false, binding.ModeToken), Place: place, Source: binding.ModeToken}
	return true
}

// ResolvedTypePackLoopBorrowOf returns the borrow a `for ref` loop over an owned
// pack takes of each element.
func (a *Analyzer) ResolvedTypePackLoopBorrowOf(stmt *ast.ForStatement) (ResolvedReferencePlan, bool) {
	if a == nil || stmt == nil {
		return ResolvedReferencePlan{}, false
	}
	plan, ok := a.typePackLoopBorrows[stmt]
	if !ok {
		return ResolvedReferencePlan{}, false
	}
	return cloneResolvedReferencePlan(plan), true
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
func (a *Analyzer) checkTypePackByValueLoops(instances []typePackInstance) {
	if len(a.typePackByValueLoops) == 0 {
		return
	}
	for _, instance := range instances {
		call, function := instance.origin, instance.function
		loop, copies := a.typePackByValueLoops[sourceTokenLocation(function.Token)]
		if !copies {
			continue
		}
		sequence, ok := inferredTypePack(function)
		if !ok {
			continue
		}
		for position, element := range sequence {
			if classification := CopyClassificationOf(element); classification == CopyMoveOnly || classification == CopyNonCopyable {
				endLine, endColumn := call.Token.EndPosition()
				a.appendError(Error{Severity: diagnostics.SeverityError,
					Message: fmt.Sprintf("%s copies pack elements by value, but position %d is %s, which is not implicitly copyable; use a ref binding or ref values[k] in %s", function.Name, position, typeDisplayName(element), function.Name),
					File:    call.Token.File, Line: call.Token.Line, Column: call.Token.Column, EndLine: endLine, EndColumn: endColumn,
					PreviousFile: loop.File, PreviousLine: loop.Line, PreviousColumn: loop.Column, RelatedLabel: "by-value pack loop"})
				break
			}
		}
	}
}

// callsInSourceOrder lists the resolved calls selected by want deterministically.
func (a *Analyzer) callsInSourceOrder(want func(Function) bool) []*ast.CallExpression {
	calls := make([]*ast.CallExpression, 0, len(a.resolvedCalls))
	for call, resolved := range a.resolvedCalls {
		if want(resolved.Function) {
			calls = append(calls, call)
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].Token.File != calls[j].Token.File {
			return calls[i].Token.File < calls[j].Token.File
		}
		return calls[i].Token.ByteStart < calls[j].Token.ByteStart
	})
	return calls
}

// typePackIndexLimit is the highest position a pack template selects.
type typePackIndexLimit struct {
	position int
	token    lexer.Token
}

// typePackIndexElement types `values[k]` on a heterogeneous pack: the index must
// be a non-negative compile-time constant, and the element is the opaque
// `Types[k]`, so equal positions unify and different positions do not. A runtime
// index never yields a universal element type (functions.md §38A(5)(2)); whether
// k exists depends on the specialization and is checked at each selected call.
func (a *Analyzer) typePackIndexElement(expr *ast.IndexExpression, packElement Type) (Type, bool) {
	if _, pack := typePackElementType(packElement, "0"); !pack {
		return Type{}, false
	}
	position, known := a.typePackConstantPosition(expr)
	if !known {
		a.addErrorAtToken(expressionToken(expr.Index), "a heterogeneous pack index must be a non-negative compile-time constant; its elements have no common type, so use a for loop over the pack")
		return Type{Kind: InvalidType}, true
	}
	element, _ := typePackElementType(packElement, strconv.Itoa(position))
	if a.typePackIndexes == nil {
		a.typePackIndexes = map[*ast.IndexExpression]int{}
	}
	a.typePackIndexes[expr] = position
	if a.summaryPass {
		return element, true
	}
	// Reading an owned element by value copies it, which only a specialization
	// can prove (§38A(5)(5)); an explicit `ref values[k]` borrows instead.
	if element.Kind != ReferenceType && !a.typePackBorrowedIndexes[expr] {
		a.recordTypePackByValueLoop(expressionToken(expr.Index))
	}
	if a.typePackIndexLimits == nil {
		a.typePackIndexLimits = map[sourceTokenKey]typePackIndexLimit{}
	}
	key := sourceTokenLocation(a.currentFunctionToken)
	if limit, seen := a.typePackIndexLimits[key]; !seen || position > limit.position {
		a.typePackIndexLimits[key] = typePackIndexLimit{position: position, token: expressionToken(expr.Index)}
	}
	return element, true
}

// ResolvedTypePackIndexOf returns the constant position of `values[k]`.
func (a *Analyzer) ResolvedTypePackIndexOf(expr *ast.IndexExpression) (int, bool) {
	if a == nil || expr == nil {
		return 0, false
	}
	position, ok := a.typePackIndexes[expr]
	return position, ok
}

// checkTypePackIndexLimits rejects each selected call whose pack is too short
// for a position its template reads, naming that position (§38A(10)(1-2)).
func (a *Analyzer) checkTypePackIndexLimits(instances []typePackInstance) {
	if len(a.typePackIndexLimits) == 0 {
		return
	}
	for _, instance := range instances {
		call, function := instance.origin, instance.function
		limit, indexed := a.typePackIndexLimits[sourceTokenLocation(function.Token)]
		sequence, ok := inferredTypePack(function)
		if !indexed || !ok || limit.position < len(sequence) {
			continue
		}
		endLine, endColumn := call.Token.EndPosition()
		a.appendError(Error{Severity: diagnostics.SeverityError,
			Message: fmt.Sprintf("%s reads pack position %d, but this call passes %d pack elements", function.Name, limit.position, len(sequence)),
			File:    call.Token.File, Line: call.Token.Line, Column: call.Token.Column, EndLine: endLine, EndColumn: endColumn,
			PreviousFile: limit.token.File, PreviousLine: limit.token.Line, PreviousColumn: limit.token.Column, RelatedLabel: "pack index"})
	}
}

// expressionNamesMutableBinding reports an index that a flow fact made constant
// through a `let mut`: such a value is not compile-time-known (§38A(5)(2)).
func (a *Analyzer) expressionNamesMutableBinding(expr ast.Expression) bool {
	mutable := false
	ast.Visit(expr, func(node any) bool {
		if identifier, ok := node.(*ast.Identifier); ok && identifier != nil {
			if symbol, found := a.symbols.lookup(identifier.Value); found && symbol.Mutable {
				mutable = true
			}
		}
		return true
	})
	return mutable
}

// typePackConstantPosition is the position a constant index selects, or false
// for a runtime, flow-known mutable or out-of-domain index.
func (a *Analyzer) typePackConstantPosition(expr *ast.IndexExpression) (int, bool) {
	value, known := a.integerConstantValue(expr.Index)
	if !known || a.expressionNamesMutableBinding(expr.Index) || value.Sign() < 0 || !value.IsInt64() || value.Int64() > 1<<30 {
		return 0, false
	}
	return int(value.Int64()), true
}

// isOwnedTypePackElement reports the opaque element of an owned pack position.
func isOwnedTypePackElement(typ Type) bool {
	return typ.Kind != ReferenceType && isTypePackElement(typ)
}
