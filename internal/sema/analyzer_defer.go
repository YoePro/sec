package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzeDeferStatement validates a deferred cleanup in an isolated snapshot,
// then registers its outer-Place dependencies on the enclosing flow state.
// Dependencies retain field/index/slice precision when overlap is provable.
//
// Rules:
//   - rules/control-flow/defer.md — §§8–11 and §29
//   - rules/memory/lifetime_analysis.md — §17 "Defer and delayed lifetime dependencies"
//   - rules/memory/borrowing.md — §21 "Defer and delayed use"
func (a *Analyzer) analyzeDeferStatement(stmt *ast.DeferStatement) {
	if !a.inFunctionBody {
		a.addErrorAtToken(stmt.Token, "defer is only valid inside functions")
		return
	}
	if a.inDeferBlock {
		a.addErrorAtToken(stmt.Token, "defer is not allowed inside defer")
		return
	}
	if stmt.Body == nil {
		a.addErrorAtToken(stmt.Token, "defer requires a block")
		return
	}
	if a.loopDepth > 0 {
		a.addWarningAtToken(stmt.Token, "defer inside loop registers once per execution and runs at function exit")
	}
	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.moveReasons
	previousClosedResources := a.closedResources
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousInDeferBlock := a.inDeferBlock
	previousDeferOuterSymbols := a.deferOuterSymbols
	previousDeferCaptures := a.deferCaptures
	a.symbols = copySymbols(previousSymbols)
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = copyMoved(previousMoved)
	a.moveReasons = copyMoveReasons(previousMoveReasons)
	a.closedResources = copyMoved(previousClosedResources)
	a.borrows = copyBorrows(previousBorrows)
	a.localRefContainers = copyLocalRefContainers(previousLocalRefContainers)
	a.arenaGenerations = copyArenaGenerations(previousArenaGenerations)
	a.inDeferBlock = true
	a.deferOuterSymbols = previousSymbols
	a.deferCaptures = map[string]borrowRecord{}
	defer func() {
		keys := make([]string, 0, len(a.deferCaptures))
		for key := range a.deferCaptures {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			capture := a.deferCaptures[key]
			previousBorrows[capture.Root] = append(previousBorrows[capture.Root], capture)
		}
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.moveReasons = previousMoveReasons
		a.closedResources = previousClosedResources
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.inDeferBlock = previousInDeferBlock
		a.deferOuterSymbols = previousDeferOuterSymbols
		a.deferCaptures = previousDeferCaptures
	}()

	a.analyzeBlockStatements(stmt.Body)
}

// recordDeferCapture retains a direct outer-binding dependency. Nested Place
// traversal is suppressed because the complete member/index/slice expression
// records the narrower canonical Place after successful analysis.
//
// Rules:
//   - rules/memory/lifetime_analysis.md — §17.2 "Place sensitivity"
func (a *Analyzer) recordDeferCapture(name string, symbol Symbol, token lexer.Token) {
	if a.suppressPlaceRootRead != 0 {
		return
	}
	place, ok := a.rootPlace(name)
	if !ok || place.RootToken != symbol.Token {
		return
	}
	a.recordDeferPlace(place, token)
}

// recordDeferPlace records one future deferred use only when its root denotes
// the same outer binding visible when the defer was registered.
//
// Rules:
//   - rules/control-flow/defer.md — §§9–11
//   - rules/memory/borrowing.md — §21(1)–(5)
func (a *Analyzer) recordDeferPlace(place Place, token lexer.Token) {
	if !a.inDeferBlock || a.deferCaptures == nil || a.suppressPlaceRootRead != 0 || place.Root == "" {
		return
	}
	outer, ok := a.deferOuterSymbols[place.Root]
	if !ok || outer.Token != place.RootToken {
		return
	}
	key := place.String()
	if _, exists := a.deferCaptures[key]; exists {
		return
	}
	a.deferCaptures[key] = borrowRecord{
		Root: place.Root, Place: place, Holder: "$defer", Kind: deferredUse, Token: token,
	}
}

// recordDeferPlaceExpression records deferred writes and other target uses that
// do not necessarily pass through ordinary expression-read inference.
//
// Rules:
//   - rules/control-flow/defer.md — §29 "Sema and flow-analysis requirements"
func (a *Analyzer) recordDeferPlaceExpression(expr ast.Expression) {
	if place, ok := a.resolvePlace(expr); ok {
		a.recordDeferPlace(place, expressionToken(expr))
	}
}

// checkDeferredUsePlace rejects ownership invalidation only when it overlaps a
// Place needed by a registered defer; proven-disjoint siblings remain usable.
//
// Rules:
//   - rules/memory/lifetime_analysis.md — §17.1–§17.2
//   - rules/memory/borrowing.md — §21(2), §21(5)
func (a *Analyzer) checkDeferredUsePlace(place Place, token lexer.Token, action string) bool {
	for _, candidate := range placeOriginAlternatives(place) {
		for _, record := range a.borrows[candidate.Root] {
			if record.Kind != deferredUse || !borrowPlacesOverlap(candidate, record) {
				continue
			}
			a.addErrorAtTokenWithPrevious(token, record.Token, "cannot %s %s while it is required by defer", action, place.String())
			return true
		}
	}
	return false
}
