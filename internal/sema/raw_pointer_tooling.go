package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// RawPointerFact is a detached source-local presentation of existing Sema
// proofs. Origin availability never establishes full lifetime, spatial or
// target-domain validity, and copying a raw address does not retain storage.
// Rules: rules/memory/raw_pointers.md §§5.1,7(6–8),23–24,32,35(4–6).
type RawPointerFact struct {
	Source        lexer.Token
	Type          string
	ProvenNull    bool
	Origin        string
	InvalidatedAt lexer.Token
}

// recordRawPointerExpressionFact freezes proofs at an identifier use rather
// than exposing final function state at every occurrence. Summary/replay and
// unreachable paths do not publish replacement facts.
// Rules: rules/memory/raw_pointers.md §§32,35(4–6);
// rules/compiler/compiler_analysis.md §18(2,6).
func (a *Analyzer) recordRawPointerExpressionFact(expr ast.Expression, typ Type) {
	identifier, ok := expr.(*ast.Identifier)
	if !ok || dereferenceType(typ).Kind != RawPtrType {
		return
	}
	a.recordRawPointerFact(identifier.Value, identifier.Token)
}

// recordRawPointerBindingFact captures immutable initialization provenance only
// after the canonical null/origin binding producers have run.
// Rules: rules/memory/raw_pointers.md §§23–24,32,35(4–6).
func (a *Analyzer) recordRawPointerBindingFact(name string) {
	if symbol, ok := a.symbols.lookup(name); ok && dereferenceType(symbol.Type).Kind == RawPtrType {
		a.recordRawPointerFact(name, symbol.Token)
	}
}

// recordRawPointerFact projects the same stable null and origin facts used by
// mandatory diagnostics; unknown address-space/range/alignment/lifetime facts
// stay unknown. An available origin does not prove a raw dereference valid.
// Rules: rules/memory/raw_pointers.md §§7(6–8),24,31–32,34–35.
func (a *Analyzer) recordRawPointerFact(name string, source lexer.Token) {
	if a.summaryPass || a.iterationDependencyProbe || !a.callGraphPathReachable {
		return
	}
	symbol, ok := a.symbols.lookup(name)
	if !ok {
		return
	}
	fact := RawPointerFact{Source: source, Type: typeDisplayName(symbol.Type)}
	_, fact.ProvenNull = a.provenNullRawPointerBinding(name)
	if !symbol.Mutable && !symbol.Volatile && symbol.RawPointerOrigin != nil {
		origin := *symbol.RawPointerOrigin
		fact.Origin = origin.String()
		if invalidated, _, _, unavailable := a.unavailablePlace(origin); unavailable {
			fact.InvalidatedAt = invalidated
		}
	}
	if a.rawPointerToolingFacts == nil {
		a.rawPointerToolingFacts = map[sourceTokenKey]RawPointerFact{}
	}
	a.rawPointerToolingFacts[sourceTokenLocation(source)] = fact
}

// RawPointerFacts returns deterministic detached facts, safe for CLI/LSP
// consumers and callers that mutate their returned slice.
// Rules: rules/compiler/compiler_analysis.md §18(2,6);
// rules/memory/raw_pointers.md §35.
func (a *Analyzer) RawPointerFacts() []RawPointerFact {
	if a == nil {
		return nil
	}
	facts := make([]RawPointerFact, 0, len(a.rawPointerToolingFacts))
	for _, fact := range a.rawPointerToolingFacts {
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		left, right := facts[i].Source, facts[j].Source
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	return facts
}

// RawPointerFactAt queries an exact source occurrence, preserving disjoint
// same-spelled locals and different path proofs of one binding.
// Rules: rules/memory/raw_pointers.md §§32,35(4–6).
func (a *Analyzer) RawPointerFactAt(file string, line, column int) (RawPointerFact, bool) {
	if a == nil {
		return RawPointerFact{}, false
	}
	fact, ok := a.rawPointerToolingFacts[sourceTokenKey{File: file, Line: line, Column: column}]
	return fact, ok
}

// Display shares the compiler's proof/unknown distinction between tools.
// Rules: rules/memory/raw_pointers.md §35(4–6).
func (fact RawPointerFact) Display() string {
	lines := []string{"Raw pointer `" + fact.Type + "`"}
	nullability := "not established"
	if fact.ProvenNull {
		nullability = "proven null"
	}
	lines = append(lines, "Nullability: "+nullability)
	if fact.Origin != "" {
		lines = append(lines, "Known origin: `"+fact.Origin+"` (address only; does not retain storage)")
	}
	if fact.InvalidatedAt.Line > 0 {
		lines = append(lines, fmt.Sprintf("Origin no longer holds a value after %s:%d:%d", fact.InvalidatedAt.File, fact.InvalidatedAt.Line, fact.InvalidatedAt.Column))
	}
	lines = append(lines, "Address space, range, alignment and lifetime: not established")
	return strings.Join(lines, "\n\n")
}
