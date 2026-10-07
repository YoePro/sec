// Arena frontend validity and epoch facts. This represented subset does not
// establish complete NLL, nested/captured/foreign retention or epoch exhaustion.
// Rules: rules/memory/arena.md — §§4.2–4.7, 35–46;
// rules/memory/reference_model.md — §30;
// rules/corrections/applied/correction25-20260823.md — Part II traceability.
package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// recordArenaEffect records source-ordered direct Arena effects for reachable
// final-body paths, without publishing speculative summary-pass events.
// Rules: rules/memory/arena.md — §§76–77; rules/analysis/call_graph.md — "Effect-analysis integration".
func (a *Analyzer) recordArenaEffect(kind ArenaEffectKind, arena string, source lexer.Token, mayAllocate bool) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{
		Kind:        kind,
		Arena:       arena,
		Source:      source,
		MayAllocate: mayAllocate,
	})
}

// bindArenaDomainFromExpression preserves an ArenaDomain through owner moves
// and attaches a borrowed backing dependency to the new owner. Fresh owners
// receive a distinct logical domain rather than a source-binding identity.
// Rules: rules/memory/arena.md — §§4.2(1–5), 7(6), 10(3), 45(2).
func (a *Analyzer) bindArenaDomainFromExpression(holder string, expr ast.Expression) {
	if holder == "" {
		return
	}
	symbol, ok := a.symbols[holder]
	if !ok || symbol.Type.Name != "Arena" {
		return
	}
	domain := ""
	if inferred, ok := a.expressionTypes[expr]; ok && inferred.Name == "Arena" {
		domain = inferred.ArenaDomainID
	}
	if domain == "" {
		if source, ok := expr.(*ast.Identifier); ok {
			if sourceSymbol, exists := a.symbols[source.Value]; exists && sourceSymbol.Type.Name == "Arena" {
				domain = sourceSymbol.Type.ArenaDomainID
				a.transferBorrowHolder(source.Value, holder)
			}
		}
	}
	if call, ok := expr.(*ast.CallExpression); ok {
		if member, memberOK := call.Callee.(*ast.MemberExpression); memberOK && member.Property != nil && member.Property.Value == "FromBuffer" && len(call.Arguments) == 1 {
			a.transferBorrowHolder(arenaConstructorBorrowHolder(call), holder)
			if place, placeOK := a.resolvePlace(call.Arguments[0]); placeOK {
				for _, alternative := range placeOriginAlternatives(place) {
					a.borrows[alternative.Root] = append(a.borrows[alternative.Root], borrowRecord{
						Root: alternative.Root, Place: alternative, Holder: holder, Kind: mutableBorrow, Token: call.Token,
					})
				}
			}
		}
	}
	if domain == "" {
		domain = a.newArenaDomainID()
	}
	symbol.Type.ArenaDomainID = domain
	a.symbols[holder] = symbol
}

// newArenaDomainID creates an analysis-local logical identity for a fresh
// Arena owner. This frontend ID does not implement runtime epoch exhaustion.
// Rules: rules/memory/arena.md — §§4.2(1–5), 44(3–4).
func (a *Analyzer) newArenaDomainID() string {
	a.nextArenaDomainID++
	return fmt.Sprintf("$arena-domain-%d", a.nextArenaDomainID)
}

// checkStaleArenaReference rejects a represented reference or slice whose
// expected ArenaDomain epoch differs from the current flow state. This stale-use
// check is separate from proving Reset/Release operation-point legality.
// Rules: rules/memory/reference_model.md — §30(2–6); rules/memory/arena.md — §§38(1–4), 44(1).
func (a *Analyzer) checkStaleArenaReference(symbol Symbol, token lexer.Token) bool {
	if !typeCarriesReferenceOrigin(symbol.Type) || symbol.Type.ReferenceOriginStorage != StorageOriginArena || symbol.Type.ReferenceOriginName == "" {
		return false
	}
	current := a.arenaGenerations[symbol.Type.ReferenceOriginName]
	if symbol.Type.ReferenceOriginGeneration == current {
		return false
	}
	display := symbol.Type.ReferenceOriginDisplayName
	if display == "" {
		display = symbol.Type.ReferenceOriginName
	}
	a.addErrorAtTokenWithPrevious(token, symbol.Type.ReferenceOriginToken, "cannot use %s after arena %s was reset", symbol.Name, display)
	return true
}

// checkArenaBackingBorrowRead rejects reads of backing still exclusively
// borrowed by a represented Arena owner. Release ends that backing borrow.
// Rules: rules/memory/arena.md — §§10(6–9), 45(2).
func (a *Analyzer) checkArenaBackingBorrowRead(name string, token lexer.Token) bool {
	for _, record := range a.borrows[name] {
		if record.Kind != mutableBorrow {
			continue
		}
		holder, ok := a.symbols[record.Holder]
		if !ok || holder.Type.Name != "Arena" {
			continue
		}
		a.addErrorAtTokenWithPrevious(token, record.Token, "cannot read %s while its backing is exclusively borrowed by arena %s", name, record.Holder)
		return true
	}
	return false
}

// arenaResultType represents the fallible result of safe typed Arena
// allocation and owned/growable Arena construction.
// Rules: rules/memory/arena.md — §§12, 13, 20–21.
func arenaResultType(value Type, err Type) Type {
	return Type{Name: "Result[" + typeDisplayName(value) + ", " + typeDisplayName(err) + "]", Kind: ResultType, TypeArgs: []Type{value, err}}
}

// arenaConstructorBorrowHolder identifies the temporary holder of a
// borrowed backing dependency until constructor-result ownership is bound.
// Rules: rules/memory/arena.md — §§10, 45(2).
func arenaConstructorBorrowHolder(expr *ast.CallExpression) string {
	if expr == nil {
		return "$arena-backing"
	}
	return fmt.Sprintf("$arena-backing:%s:%d:%d", expr.Token.File, expr.Token.Line, expr.Token.Column)
}

// validateArenaAllocationElement checks sized layout, an infallible valid
// default and trivial destruction before safe typed allocation. Unresolved
// generic elements defer these requirements to specialization; this check does
// not supply runtime byte-size, alignment or overflow proof.
// Rules: rules/memory/arena.md — §§19(1–4), 20(2), 21(2), 120(1);
// rules/memory/layout.md — §2(7); rules/types/default_values.md — canonical default resolution.
func (a *Analyzer) validateArenaAllocationElement(method string, reference *ast.TypeReference, element Type) bool {
	token := reference.Token
	if element.Kind == GenericType {
		return true
	}
	display := typeDisplayName(element)
	if !compilerKnownSizedType(element) {
		a.addErrorAtTokenWithMetadata(token, diagnostics.ArenaUnsizedAllocationType,
			"Allocate a concrete type with complete sized layout.",
			"Arena.%s requires a sized type with complete layout, got %s", method, display)
		return false
	}
	if !IsDefaultable(element) {
		a.addErrorAtTokenWithMetadata(token, diagnostics.ArenaAllocationMissingDefault,
			"Safe Arena allocation fully initializes every value with its compiler-defined default; give the type a valid default.",
			"Arena.%s requires a type with a valid infallible default, but %s has no default", method, display)
		return false
	}
	if !TriviallyDestructible(element) {
		a.addErrorAtTokenWithMetadata(token, diagnostics.ArenaNonTrivialDestructionType,
			"Safe Arena allocation never runs destructors; allocate only trivially destructible types.",
			"Arena.%s cannot allocate %s because it requires destruction", method, display)
		return false
	}
	return true
}

// arenaDependencyEscapesImmediateLocal recognizes represented deferred
// uses and additional borrow/reference-container holders of an Arena-backed
// local. Absence here does not prove absence of unrepresented dependencies.
// Rules: rules/memory/arena.md — §36(2–4), §44(1); rules/control-flow/defer.md — deferred execution.
func (a *Analyzer) arenaDependencyEscapesImmediateLocal(name, owner string) bool {
	for _, records := range a.borrows {
		for _, record := range records {
			if isDeferredUseKind(record.Kind) && record.Root == name || record.Holder != "" && record.Holder != name && record.Holder != owner && record.Root == name {
				return true
			}
		}
	}
	for holder, origin := range a.localRefContainers {
		if holder != name && holder != owner && origin.Name == name {
			return true
		}
	}
	return false
}

// inferArenaConstructorCall validates compiler-known borrowed and owned/growable
// construction, assigning a fresh domain and recording backing/effect facts.
// Rules: rules/memory/arena.md — §§5(1–5), 10–13, 19–21, 35–38, 43–45;
// rules/corrections/applied/correction28-20260823.md — Reset/Release dependency legality.
func (a *Analyzer) inferArenaConstructorCall(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "Arena."+member.Name, 1, 1) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	argumentType, _ := a.inferExpression(expr.Arguments[0])
	if argumentType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if member.Name == "FromBuffer" {
		if argumentType.Kind != ReferenceType || !argumentType.ReferenceMutable || argumentType.Element == nil || argumentType.Element.Kind != SliceType || argumentType.Element.Element == nil || argumentType.Element.Element.Name != "byte" {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.FromBuffer requires ref mut byte[], got %s", typeDisplayName(argumentType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if origin, ok := a.directReferenceOrigin(expr.Arguments[0]); ok {
			origin.Mutable = true
			a.expressionReferenceOrigins[expr] = origin
		}
		if place, ok := a.resolvePlace(expr.Arguments[0]); ok {
			holder := arenaConstructorBorrowHolder(expr)
			for _, alternative := range placeOriginAlternatives(place) {
				a.borrows[alternative.Root] = append(a.borrows[alternative.Root], borrowRecord{
					Root: alternative.Root, Place: alternative, Holder: holder, Kind: mutableBorrow, Token: expr.Token,
				})
			}
		}
		a.recordArenaEffect(ArenaEffectCreateBorrowed, "", callCalleeDefinitionToken(expr), false)
		arena := a.types["Arena"]
		arena.ArenaDomainID = a.newArenaDomainID()
		return arena, expressionValue{Display: expr.String()}, true
	}
	if !a.canInitialize(a.types["uint"], argumentType, expr.Arguments[0]) {
		a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.%s capacity must be uint, got %s", member.Name, typeDisplayName(argumentType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	effect := ArenaEffectCreateOwned
	if member.Name == "Growable" {
		effect = ArenaEffectCreateGrowable
	}
	a.recordArenaEffect(effect, "", callCalleeDefinitionToken(expr), true)
	arena := a.types["Arena"]
	arena.ArenaDomainID = a.newArenaDomainID()
	return arenaResultType(arena, a.types["AllocationError"]), expressionValue{Display: expr.String()}, true
}

// inferArenaCall validates typed allocation and explicit Reset/Release
// calls. Reset advances the represented epoch; Release also consumes the owner
// and ends backing borrows. Dependency completeness and epoch exhaustion remain
// separate implementation obligations.
// Rules: rules/memory/arena.md — §§5(1–5), 10–13, 19–21, 35–38, 43–45;
// rules/corrections/applied/correction28-20260823.md — Reset/Release dependency legality.
func (a *Analyzer) inferArenaCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return Type{}, expressionValue{}, false
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	symbol, ok := a.symbols[receiver.Value]
	if !ok || symbol.Type.Name != "Arena" {
		return Type{}, expressionValue{}, false
	}
	domain := symbol.Type.ArenaDomainID
	if domain == "" {
		domain = a.newArenaDomainID()
		symbol.Type.ArenaDomainID = domain
		a.symbols[receiver.Value] = symbol
	}
	switch member.Property.Value {
	case "New":
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.New requires exactly 1 type argument, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.New expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.New requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, resolved := a.resolveType(expr.GenericArguments[0])
		if !resolved {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.validateArenaAllocationElement("New", expr.GenericArguments[0], elementType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.recordArenaEffect(ArenaEffectAllocate, receiver.Value, member.Property.Token, true)
		refType := Type{Name: "ref mut " + typeDisplayName(elementType), Kind: ReferenceType, Element: &elementType, ReferenceMutable: true, ReferenceOriginName: domain, ReferenceOriginDisplayName: receiver.Value, ReferenceOriginToken: receiver.Token, ReferenceOriginLocal: symbol.Local, ReferenceOriginStorage: StorageOriginArena, ReferenceOriginGeneration: a.arenaGenerations[domain]}
		return arenaResultType(refType, a.types["AllocationError"]), expressionValue{Display: expr.String()}, true
	case "Alloc":
		if len(expr.GenericArguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.Alloc requires exactly 1 type argument, got %d", len(expr.GenericArguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 1 {
			a.addErrorAtToken(expr.Token, "Arena.Alloc expects 1 argument, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Alloc requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		countType, _ := a.inferExpressionWithExpected(expr.Arguments[0], a.types["uint"])
		if countType.Kind != InvalidType && !a.canInitialize(a.types["uint"], countType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "Arena.Alloc count must be uint, got %s", typeDisplayName(countType))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		elementType, ok := a.resolveType(expr.GenericArguments[0])
		if !ok || !a.validateArenaAllocationElement("Alloc", expr.GenericArguments[0], elementType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		sliceType := Type{
			Name:    typeDisplayName(elementType) + "[]",
			Kind:    SliceType,
			Element: &elementType,
		}
		refSliceType := Type{
			Name:                       "ref mut " + typeDisplayName(sliceType),
			Kind:                       ReferenceType,
			Element:                    &sliceType,
			ReferenceMutable:           true,
			ReferenceOriginName:        domain,
			ReferenceOriginDisplayName: receiver.Value,
			ReferenceOriginToken:       receiver.Token,
			ReferenceOriginLocal:       symbol.Local,
			ReferenceOriginStorage:     StorageOriginArena,
			ReferenceOriginGeneration:  a.arenaGenerations[domain],
		}
		errType := a.types["AllocationError"]
		a.recordArenaEffect(ArenaEffectAllocate, receiver.Value, member.Property.Token, true)
		return Type{
			Name:     "Result[" + typeDisplayName(refSliceType) + ", AllocationError]",
			Kind:     ResultType,
			TypeArgs: []Type{refSliceType, errType},
		}, expressionValue{Display: expr.String()}, true
	case "Reset":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Reset does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Reset expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Reset requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.checkArenaInvalidationDependencies(member.Property.Value, domain, receiver.Value, member.Property.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.arenaGenerations[domain]++
		a.recordArenaEffect(ArenaEffectReset, receiver.Value, member.Property.Token, false)
		return Type{Name: "void", Kind: VoidType}, expressionValue{Display: expr.String()}, true
	case "Release":
		if len(expr.GenericArguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Release does not take type arguments")
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if len(expr.Arguments) != 0 {
			a.addErrorAtToken(expr.Token, "Arena.Release expects 0 arguments, got %d", len(expr.Arguments))
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if !a.canWriteThroughSymbol(symbol) {
			a.addErrorAtToken(receiver.Token, "Arena.Release requires mutable arena %s", receiver.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if a.checkArenaInvalidationDependencies(member.Property.Value, domain, receiver.Value, member.Property.Token) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		a.arenaGenerations[domain]++
		a.moved[receiver.Value] = expr.Token
		a.moveReasons[receiver.Value] = "released"
		a.endBorrowsHeldBy(receiver.Value)
		a.recordArenaEffect(ArenaEffectRelease, receiver.Value, member.Property.Token, false)
		return a.types["void"], expressionValue{Display: expr.String()}, true
	default:
		return Type{}, expressionValue{}, false
	}
}
