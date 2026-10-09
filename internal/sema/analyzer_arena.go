// Arena frontend validity and epoch facts. This represented subset does not
// establish complete NLL, nested/captured/foreign retention or epoch exhaustion.
// Rules: rules/memory/arena.md — §§4.2–4.7, 35–46;
// rules/memory/reference_model.md — §30;
// rules/corrections/applied/correction25-20260823.md — Part II traceability.
package sema

import (
	"fmt"
	"sort"

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
	// An owner move carries the borrowed backing with it, whether or not the
	// domain was already known from the expression type (rules/memory/arena.md
	// § 4.2(3), § 10(6)).
	if source, ok := expr.(*ast.Identifier); ok && source.Value != holder {
		if sourceSymbol, exists := a.symbols[source.Value]; exists && sourceSymbol.Type.Name == "Arena" {
			if domain == "" {
				domain = sourceSymbol.Type.ArenaDomainID
			}
			a.transferBorrowHolder(source.Value, holder)
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
		domain = a.newArenaDomainID(expressionToken(expr), "binding:"+holder)
	}
	symbol.Type.ArenaDomainID = domain
	a.symbols[holder] = symbol
}

// newArenaDomainID records the abstract lexical creation/owner site with its
// callable context. Re-analysis of one site retains its identity; distinct sites
// stay distinct. Runtime allocation instances and epoch exhaustion remain separate.
// Rules: rules/memory/arena.md — §§4.2(1–5), 44(3–4); compiler/compiler.md — §71.
func (a *Analyzer) newArenaDomainID(source lexer.Token, role string) string {
	record, ok := a.generatedIdentities.Resolve("arena-domain", string(a.currentCallable)+"/"+role, source, "")
	if !ok {
		a.addErrorAtToken(source, "Arena domain requires a prepared lexical source origin")
		return ""
	}
	return "$" + record.ID
}

// GeneratedIdentities returns sorted compiler-owned resource provenance.
// These are abstract frontend creation-site identities, not runtime allocation IDs.
// Rules: rules/compiler/compiler.md — §71; compiler_pipeline.md — §76(2);
// rules/memory/arena.md — §§4.2,44.
func (a *Analyzer) GeneratedIdentities() []GeneratedIdentity {
	records := a.generatedIdentities.Records()
	for index := range records {
		if records[index].Kind == "arena-domain" {
			records[index].ID = "$" + records[index].ID
		}
	}
	return records
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
// It is the canonical compiler-known Result, so it compares equal to a
// declared Result[T, AllocationError] return type.
func arenaResultType(value Type, err Type) Type {
	return compilerKnownResult(value, err)
}

// validArenaBackingArgument accepts the `ref mut byte[]` backing of
// Arena.FromBuffer: a mutable reference value to byte storage, or a mutable
// byte-array or byte-slice Place that the call borrows implicitly as a
// borrowed parameter does (no call-site `ref mut` marker is required).
//
// Rules:
//   - rules/memory/arena.md — § 10 "Borrowed fixed Arena" (mutable, contiguous, addressable backing)
//   - rules/analysis/escape_analysis.md — "Call-boundary semantic classification" (call-bounded implicit borrow)
func (a *Analyzer) validArenaBackingArgument(argument ast.Expression, argumentType Type) bool {
	if argumentType.Kind == ReferenceType {
		if argumentType.ReferenceMutable && argumentType.Element != nil && isByteSequenceType(*argumentType.Element) {
			return true
		}
	} else if isByteSequenceType(argumentType) {
		place, ok := a.callArgumentBorrowPlace(argument)
		if !ok {
			a.addErrorAtToken(expressionToken(argument), "Arena.FromBuffer backing must be addressable storage that outlives the Arena, not a temporary %s", typeDisplayName(argumentType))
			return false
		}
		return !a.checkBorrowCreationPlace(place, true, expressionToken(argument))
	}
	a.addErrorAtToken(expressionToken(argument), "Arena.FromBuffer requires mutable contiguous byte storage borrowed as ref mut byte[], such as a let mut byte[] or byte[N], got %s", typeDisplayName(argumentType))
	return false
}

func isByteSequenceType(typ Type) bool {
	return (typ.Kind == ArrayType || typ.Kind == SliceType) && typ.Element != nil && typ.Element.Name == "byte"
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
		if !a.validArenaBackingArgument(expr.Arguments[0], argumentType) {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
		}
		if origin, ok := a.directReferenceOrigin(expr.Arguments[0]); ok {
			origin.Mutable = true
			a.expressionReferenceOrigins[expr] = origin
		} else if place, ok := a.callArgumentBorrowPlace(expr.Arguments[0]); ok {
			// The implicit call-bounded borrow of a backing Place becomes the
			// Arena's dependency on that storage for its whole lifetime.
			origin := localOriginWithPlaces(localReferenceOrigin{Name: place.Root, Token: place.RootToken, Mutable: true}, []Place{place})
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
		arena.ArenaDomainID = a.newArenaDomainID(expr.Token, "creation")
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
	arena.ArenaDomainID = a.newArenaDomainID(expr.Token, "creation")
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
	if !ok {
		return Type{}, expressionValue{}, false
	}
	// An Arena reached through `ref`/`ref mut` (for example a helper
	// parameter) supports the same operations; mutating ones require
	// `ref mut`, and only the owning Arena can be released.
	// Rules: rules/memory/arena.md — § 68(3)–(4) "fn Fill(arena: ref mut Arena)", § 43 "Release()".
	throughReference := symbol.Type.Kind == ReferenceType
	if throughReference {
		if symbol.Type.Element == nil || symbol.Type.Element.Name != "Arena" {
			return Type{}, expressionValue{}, false
		}
	} else if symbol.Type.Name != "Arena" {
		return Type{}, expressionValue{}, false
	}
	domain := symbol.Type.ArenaDomainID
	if throughReference {
		// The referenced Arena belongs to the caller: storage allocated from it
		// is rooted in the reference parameter, which may be returned, and Reset
		// through the reference advances that same identity's epoch.
		domain = receiver.Value
	} else if domain == "" {
		domain = a.newArenaDomainID(symbol.Token, "owner:"+receiver.Value)
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
		refType := Type{Name: "ref mut " + typeDisplayName(elementType), Kind: ReferenceType, Element: &elementType, ReferenceMutable: true, ReferenceOriginName: domain, ReferenceOriginDisplayName: receiver.Value, ReferenceOriginToken: receiver.Token, ReferenceOriginLocal: symbol.Local && !throughReference, ReferenceOriginStorage: StorageOriginArena, ReferenceOriginGeneration: a.arenaGenerations[domain]}
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
			ReferenceOriginLocal:       symbol.Local && !throughReference,
			ReferenceOriginStorage:     StorageOriginArena,
			ReferenceOriginGeneration:  a.arenaGenerations[domain],
		}
		errType := a.types["AllocationError"]
		a.recordArenaEffect(ArenaEffectAllocate, receiver.Value, member.Property.Token, true)
		return arenaResultType(refSliceType, errType), expressionValue{Display: expr.String()}, true
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
		if throughReference {
			a.addErrorAtTokenWithMetadata(receiver.Token, "", "Release ends the Arena and consumes its owner; call it where the Arena is owned, or use Reset through the reference.",
				"Arena.Release cannot consume arena %s through a borrowed reference", receiver.Value)
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

// arenaBorrowedBacking returns the backing borrow an Arena owner binding
// still controls, choosing the earliest source position deterministically.
func (a *Analyzer) arenaBorrowedBacking(holder string) (borrowRecord, bool) {
	var records []borrowRecord
	for _, list := range a.borrows {
		for _, record := range list {
			if record.Holder == holder && record.Kind == mutableBorrow {
				records = append(records, record)
			}
		}
	}
	if len(records) == 0 {
		return borrowRecord{}, false
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Token.Line != records[j].Token.Line {
			return records[i].Token.Line < records[j].Token.Line
		}
		return records[i].Token.Column < records[j].Token.Column
	})
	return records[0], true
}

const borrowedArenaHelp = "An Arena from Arena.FromBuffer controls its buffer for its whole lifetime, and that dependency cannot travel with the Arena value. Keep the Arena in the function that borrows the buffer and pass it on as ref mut Arena, or Release it first."

// rejectBorrowedArenaMove rejects moving an Arena that controls borrowed
// backing out of its binding to anything but another local binding (which
// takes over the borrow before the move is marked). A consuming call, a field
// store or an aggregate would otherwise end the borrow while the Arena lives.
//
// Rules:
//   - rules/memory/arena.md — § 10(6)–(9) "Borrowed fixed Arena", § 4.2(3), § 45(2)
func (a *Analyzer) rejectBorrowedArenaMove(name string, token lexer.Token) bool {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Type.Name != "Arena" {
		return false
	}
	record, borrowed := a.arenaBorrowedBacking(name)
	if !borrowed {
		return false
	}
	a.addErrorAtTokenWithMetadataAndPrevious(token, record.Token, "", borrowedArenaHelp,
		"cannot move arena %s out of its binding while it controls borrowed backing %s", name, record.Root)
	return true
}

// rejectBorrowedArenaReturn rejects returning an Arena whose backing is
// borrowed in this function: the backing must stay live and exclusively
// controlled for the complete Arena lifetime, which the caller cannot see.
//
// Rules:
//   - rules/memory/arena.md — § 10(8)–(9)
func (a *Analyzer) rejectBorrowedArenaReturn(value ast.Expression) bool {
	if move, ok := explicitMoveArgument(value); ok {
		value = move.Right
	}
	switch value := value.(type) {
	case *ast.Identifier:
		symbol, ok := a.symbols[value.Value]
		if !ok || symbol.Type.Name != "Arena" {
			return false
		}
		if record, borrowed := a.arenaBorrowedBacking(value.Value); borrowed {
			a.addErrorAtTokenWithMetadataAndPrevious(value.Token, record.Token, "", borrowedArenaHelp,
				"cannot return arena %s: its backing %s is borrowed here and must outlive the Arena", value.Value, record.Root)
			return true
		}
	case *ast.CallExpression:
		member, ok := value.Callee.(*ast.MemberExpression)
		if ok && member.Property != nil && member.Property.Value == "FromBuffer" && a.expressionNamesType(member.Object) {
			a.addErrorAtTokenWithMetadata(value.Token, "", borrowedArenaHelp,
				"cannot return Arena.FromBuffer(...): its borrowed backing must outlive the Arena")
			return true
		}
	}
	return false
}
