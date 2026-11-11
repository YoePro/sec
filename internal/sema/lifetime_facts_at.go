package sema

import (
	"fmt"
	"sort"

	"sec/internal/diagnostics"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// ArenaInvalidation is one Reset or Release of an Arena domain.
type ArenaInvalidation struct {
	Kind   ArenaEffectKind
	Source lexer.Token
}

// ResolvedLifetimeFact is the tooling view of the lifetime of one reference
// holder declared by `let` (lifetime_analysis.md §§ 2, 6, 12; arena.md § 44):
// its origin and storage, the call whose returned-reference summary
// produced it, the Arena domain and epoch it depends on with the later
// invalidating operations of that domain, and the resolved ends of the
// holder's retained borrow. It projects facts Sema already established; no
// second lifetime engine runs in tooling.
type ResolvedLifetimeFact struct {
	Holder      string
	HolderToken lexer.Token
	Origin      string
	OriginToken lexer.Token
	Storage     StorageOrigin
	// ReturnedBy is the callee whose returned-reference summary was
	// instantiated over the actual arguments; ReturnedFrom the resulting
	// origin Places.
	ReturnedBy   string
	ReturnedFrom []string
	// ArenaDomain and Generation identify the Arena incarnation the
	// reference depends on; Invalidations are that domain's Reset/Release
	// operations after the declaration, in source order.
	ArenaDomain   string
	Generation    int
	Invalidations []ArenaInvalidation
	Ends          []ReferenceLifetimeEnd
	Facts         ReferenceFacts
}

// recordLifetimeBinding retains the lifetime fact of an accepted reference
// holder declaration.
func (a *Analyzer) recordLifetimeBinding(stmt *ast.LetStatement, errorsBefore int) {
	if a.summaryPass || stmt == nil || stmt.Name == nil || len(a.errors) != errorsBefore {
		return
	}
	symbol, ok := a.symbols.lookup(stmt.Name.Value)
	if !ok || symbol.Token != stmt.Name.Token || !typeCarriesReferenceOrigin(symbol.Type) {
		return
	}
	typ := symbol.Type
	fact := ResolvedLifetimeFact{
		Holder: stmt.Name.Value, HolderToken: stmt.Name.Token,
		Origin: typ.ReferenceOriginDisplayName, OriginToken: typ.ReferenceOriginToken, Storage: typ.ReferenceOriginStorage,
	}
	if fact.Origin == "" {
		fact.Origin = typ.ReferenceOriginName
	}
	if typ.ReferenceOriginStorage == StorageOriginArena {
		fact.ArenaDomain, fact.Generation = typ.ReferenceOriginName, typ.ReferenceOriginGeneration
	}
	value := stmt.Value
	if try, isTry := value.(*ast.TryExpression); isTry {
		value = try.Expression
	}
	if call, isCall := value.(*ast.CallExpression); isCall {
		if facts, returned := a.resolvedCallReferences[call]; returned {
			fact.ReturnedBy = call.Callee.String()
			fact.Facts = cloneReferenceFacts(facts)
			for _, place := range facts.OriginPlaces {
				fact.ReturnedFrom = append(fact.ReturnedFrom, place.String())
			}
		}
	}
	if a.lifetimeFacts == nil {
		a.lifetimeFacts = map[sourceTokenKey]ResolvedLifetimeFact{}
	}
	a.lifetimeFacts[sourceTokenLocation(stmt.Name.Token)] = fact
}

// noteArenaInvalidation retains a Reset/Release of a domain.
func (a *Analyzer) noteArenaInvalidation(kind ArenaEffectKind, domain string, source lexer.Token) {
	if domain == "" || kind != ArenaEffectReset && kind != ArenaEffectRelease {
		return
	}
	if a.arenaInvalidations == nil {
		a.arenaInvalidations = map[string][]ArenaInvalidation{}
	}
	for _, existing := range a.arenaInvalidations[domain] {
		if existing.Kind == kind && sourceTokenLocation(existing.Source) == sourceTokenLocation(source) {
			return
		}
	}
	a.arenaInvalidations[domain] = append(a.arenaInvalidations[domain], ArenaInvalidation{Kind: kind, Source: source})
}

// ResolvedLifetimeAt returns the lifetime fact of the reference holder
// declared at a source position.
// Rules: rules/memory/lifetime_analysis.md — §§ 2, 6, 12; rules/memory/arena.md — § 44;
// rules/tooling/lsp.md — "Hover".
func (a *Analyzer) ResolvedLifetimeAt(file string, line, column int) (ResolvedLifetimeFact, bool) {
	if a == nil {
		return ResolvedLifetimeFact{}, false
	}
	fact, ok := a.lifetimeFacts[sourceTokenKey{File: file, Line: line, Column: column}]
	if !ok {
		return ResolvedLifetimeFact{}, false
	}
	fact.ReturnedFrom = append([]string(nil), fact.ReturnedFrom...)
	fact.Facts = cloneReferenceFacts(fact.Facts)
	fact.Invalidations = nil
	for _, invalidation := range a.arenaInvalidations[fact.ArenaDomain] {
		if tokenBefore(fact.HolderToken, invalidation.Source) {
			fact.Invalidations = append(fact.Invalidations, invalidation)
		}
	}
	sort.Slice(fact.Invalidations, func(i, j int) bool { return tokenBefore(fact.Invalidations[i].Source, fact.Invalidations[j].Source) })
	binding := a.bindingIDs[sourceTokenLocation(fact.HolderToken)]
	fact.Ends = nil
	for _, end := range a.referenceLifetimeEnds {
		if end.Holder == fact.Holder && (binding == 0 || end.Binding == binding) {
			end.Place = clonePlace(end.Place)
			fact.Ends = append(fact.Ends, end)
		}
	}
	return fact, true
}

// reportArenaUseAfterInvalidation is the mentor diagnostic of a stale Arena
// reference use (S1222): the rejected use, the allocation origin of the
// reference, the Reset/Release that ended its epoch on the analyzed path and
// practical restructurings. The invalidating site is the latest recorded
// Reset/Release of the domain before the use; when none is recorded (an
// invalidation through a callee), the help says so instead of inventing one.
// Rules: rules/memory/arena.md — §§ 38(1–4), 44(1); rules/memory/lifetime_analysis.md
// — mentor diagnostics; rules/tooling/diagnostics.md — §§ 7, 12.
func (a *Analyzer) reportArenaUseAfterInvalidation(symbol Symbol, domain, arena string, origin lexer.Token, generation int, use lexer.Token) {
	var invalidation *ArenaInvalidation
	for index, candidate := range a.arenaInvalidations[domain] {
		if tokenBefore(candidate.Source, use) && (invalidation == nil || tokenBefore(invalidation.Source, candidate.Source)) {
			invalidation = &a.arenaInvalidations[domain][index]
		}
	}
	endLine, endColumn := use.EndPosition()
	err := Error{
		ID: diagnostics.ArenaUseAfterInvalidation, Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofInvalid,
		Message: fmt.Sprintf("cannot use %s after arena %s was reset", symbol.Name, arena),
		File:    use.File, Line: use.Line, Column: use.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: origin.File, PreviousLine: origin.Line, PreviousColumn: origin.Column,
		RelatedLabel: fmt.Sprintf("%s refers to storage allocated from arena %s in epoch %d", symbol.Name, arena, generation),
	}
	what := "A Reset or Release of arena " + arena + " reached through a call"
	if invalidation != nil {
		operation := "Reset"
		if invalidation.Kind == ArenaEffectRelease {
			operation = "Release"
		}
		err.Related = []RelatedLocation{arenaRetentionLocation(invalidation.Source, operation+" of arena "+arena+" ends that epoch here")}
		what = "The " + operation + " of arena " + arena + " at the related location"
	}
	err.Help = fmt.Sprintf("%s ends the epoch %s belongs to, so the storage it refers to may already be reused. Read or copy what you need from %s before that operation, allocate a fresh value from the arena after it, or keep the data in an owned value instead of Arena storage.", what, symbol.Name, symbol.Name)
	a.appendError(err)
}
