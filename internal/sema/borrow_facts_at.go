package sema

import (
	"sort"

	"sec/internal/lexer"
)

// borrowHolderFact links a source borrow creation to the binding that holds
// it, as registerBorrow established.
type borrowHolderFact struct {
	Name  string
	Token lexer.Token
}

// ResolvedBorrowFact is the tooling view of one source reference creation
// (borrowing.md § 15; copy_move.md § 29): the reference plan Sema published,
// the binding that holds the borrow when it is bound to a local, and the
// resolved ends of that holder's borrow. It is assembled from published
// facts only.
type ResolvedBorrowFact struct {
	Plan        ResolvedReferencePlan
	Holder      string
	HolderToken lexer.Token
	Ends        []ReferenceLifetimeEnd
}

// noteBorrowHolder records the holder of a borrow created at site.
func (a *Analyzer) noteBorrowHolder(site lexer.Token, holder string) {
	if a == nil || a.summaryPass || site.Line == 0 || holder == "" {
		return
	}
	symbol, ok := a.symbols.lookup(holder)
	if !ok {
		return
	}
	if a.borrowHolders == nil {
		a.borrowHolders = map[sourceTokenKey]borrowHolderFact{}
	}
	a.borrowHolders[sourceTokenLocation(site)] = borrowHolderFact{Name: holder, Token: symbol.Token}
}

// ResolvedBorrowsAt returns the borrow facts of the reference creations at a
// source position, or of the borrow held by the binding declared there.
// Rules: rules/memory/borrowing.md — §§ 11, 15; rules/tooling/lsp.md — "Hover".
func (a *Analyzer) ResolvedBorrowsAt(file string, line, column int) []ResolvedBorrowFact {
	if a == nil {
		return nil
	}
	here := sourceTokenKey{File: file, Line: line, Column: column}
	out := []ResolvedBorrowFact{}
	for _, plan := range a.resolvedReferencePlans {
		site := sourceTokenLocation(plan.Source)
		holder, held := a.borrowHolders[site]
		if site != here && !(held && sourceTokenLocation(holder.Token) == here) {
			continue
		}
		fact := ResolvedBorrowFact{Plan: cloneResolvedReferencePlan(plan)}
		if held {
			fact.Holder, fact.HolderToken = holder.Name, holder.Token
			binding := a.bindingIDs[sourceTokenLocation(holder.Token)]
			for _, end := range a.referenceLifetimeEnds {
				if end.Holder == holder.Name && (binding == 0 || end.Binding == binding) {
					end.Place = clonePlace(end.Place)
					fact.Ends = append(fact.Ends, end)
				}
			}
		}
		out = append(out, fact)
	}
	sort.Slice(out, func(i, j int) bool {
		return tokenBefore(out[i].Plan.Source, out[j].Plan.Source)
	})
	return out
}
