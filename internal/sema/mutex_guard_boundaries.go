package sema

import (
	"sec/internal/lexer"
	"sort"
)

// checkMutexGuardsAcrossSelect validates the enclosing wait boundary once.
// Rules: rules/concurrency/mutex.md — §41(3).
func (a *Analyzer) checkMutexGuardsAcrossSelect(token lexer.Token) {
	a.checkLiveMutexGuardsAcrossBoundary(token, "select")
}

// checkLiveMutexGuardsAcrossBoundary enforces suspension/blocking boundaries
// for compiler-known MutexGuard[T] bindings. A guard explicitly consumed before
// the boundary no longer holds the acquisition and therefore does not diagnose.
//
// Rules:
//   - rules/concurrency/mutex.md — §41(1–4) "Suspension restriction"
//   - rules/concurrency/await.md — §42 "Mutex guards across await"
//   - rules/concurrency/select.md — "Mutex guards across select"
func (a *Analyzer) checkLiveMutexGuardsAcrossBoundary(token lexer.Token, boundary string) {
	names := []string{}
	for name := range a.symbols.all() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		symbol, _ := a.symbols.lookup(name)
		if !isMutexGuardType(symbol.Type) {
			continue
		}
		if _, moved := a.moved[name]; moved {
			continue
		}
		a.addErrorAtTokenWithPrevious(token, symbol.Token, "mutex guard %s remains active across %s", name, boundary)
	}
}
