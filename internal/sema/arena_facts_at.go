package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// ArenaCreationFact is the creation of one Arena domain (arena.md §§ 4.2,
// 10–12, 77): the constructor, its capacity argument, the borrowed backing
// Place of FromBuffer, and the creation site.
type ArenaCreationFact struct {
	Domain      string
	Constructor string
	// Capacity is the source capacity argument, with its value when it is
	// a compile-time constant.
	Capacity      string
	CapacityValue string
	// Backing is the borrowed backing Place of FromBuffer.
	Backing string
	Source  lexer.Token
}

// ResolvedArenaFact is the tooling view of one Arena at a source position:
// the domain it operates on, the represented epoch (Reset count on the
// analyzed path) at that point, the domain's creation when it is visible in
// this analysis, and the selected allocation profile.
type ResolvedArenaFact struct {
	Name     string
	Domain   string
	Epoch    int
	Creation *ArenaCreationFact
	// Profile is the explicitly selected allocation profile, empty for the
	// hosted analysis default.
	Profile string
}

type arenaUseFact struct {
	name   string
	domain string
	epoch  int
}

// noteArenaCreation records the constructor of a new Arena domain.
func (a *Analyzer) noteArenaCreation(expr *ast.CallExpression, constructor, domain string) {
	if a.summaryPass || domain == "" || expr == nil {
		return
	}
	fact := ArenaCreationFact{Domain: domain, Constructor: constructor, Source: expr.Token}
	if len(expr.Arguments) == 1 {
		argument := expr.Arguments[0]
		if constructor == "FromBuffer" {
			if place, ok := a.resolvePlace(argument); ok {
				fact.Backing = place.String()
			} else {
				fact.Backing = argument.String()
			}
		} else {
			fact.Capacity = argument.String()
			if value, ok := a.integerConstantValue(argument); ok && value != nil {
				fact.CapacityValue = value.String()
			}
		}
	}
	if a.arenaCreations == nil {
		a.arenaCreations = map[string]ArenaCreationFact{}
	}
	a.arenaCreations[domain] = fact
}

// noteArenaUse records the domain and represented epoch of an Arena named at
// token (its declaration or the receiver of an Arena operation).
func (a *Analyzer) noteArenaUse(token lexer.Token, name, domain string) {
	if a.summaryPass || token.Line == 0 || domain == "" {
		return
	}
	if a.arenaUses == nil {
		a.arenaUses = map[sourceTokenKey]arenaUseFact{}
	}
	a.arenaUses[sourceTokenLocation(token)] = arenaUseFact{name: name, domain: domain, epoch: a.arenaGenerations[domain]}
}

// ResolvedArenaAt returns the Arena fact of the binding declared or used as
// an Arena receiver at a source position.
// Rules: rules/memory/arena.md — §§ 4.2, 10–12, 44, 77; rules/tooling/lsp.md — "Hover".
func (a *Analyzer) ResolvedArenaAt(file string, line, column int) (ResolvedArenaFact, bool) {
	if a == nil {
		return ResolvedArenaFact{}, false
	}
	use, ok := a.arenaUses[sourceTokenKey{File: file, Line: line, Column: column}]
	if !ok {
		return ResolvedArenaFact{}, false
	}
	fact := ResolvedArenaFact{Name: use.name, Domain: use.domain, Epoch: use.epoch, Profile: a.targetProfile}
	if creation, created := a.arenaCreations[use.domain]; created {
		fact.Creation = &creation
	}
	return fact, true
}
