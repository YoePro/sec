package sema

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/lexer"
)

// MapIterationEntries is the compiler-owned entry-iteration contract. The two
// component positions are key then value, never index then value. Unspecified
// order promises neither insertion, sorted nor hash-bucket order. This is a
// frontend fact, not evidence that a backend can represent map storage.
// Rules: rules/collections/collections.md — §§14.2,14.6;
// rules/control-flow/flowcontrol_for.md — §21 "Maps".
type MapIterationEntries struct {
	Iterable   lexer.Token
	Order      MapIterationOrder
	Components [2]MapIterationComponent
}

type MapIterationOrder string

const MapIterationOrderUnspecified MapIterationOrder = "unspecified"

// MapIterationComponent retains the canonical component type and source mode.
// Plain bindings require an implicit copy, ref modes borrow, and discard does
// not copy or move a component. Mutable borrowing is valid only for the value.
// Rules: rules/collections/collections.md — §14.6;
// rules/control-flow/flowcontrol_for.md — §§5–7,21.
type MapIterationComponent struct {
	Type    Type
	Mode    ast.ForBindingMode
	Discard bool
	Source  lexer.Token
}

// mapIterationEntries records validated map binding decisions without inferring
// types again or imposing an iteration order. Invalid binding headers publish
// no dependency fact; body validity remains separate on ForIterationDependencies.
// Rules: rules/collections/collections.md — §14.6;
// rules/compiler/compiler_analysis.md — §18(2),(6).
func (a *Analyzer) mapIterationEntries(stmt *ast.ForStatement, source Type) *MapIterationEntries {
	source = dereferenceType(source)
	if !isForCollectionFamily(source) || source.Name != "map" || len(stmt.Bindings) != 2 {
		return nil
	}
	fact := &MapIterationEntries{Order: MapIterationOrderUnspecified, Iterable: expressionToken(stmt.Iterable)}
	for i, binding := range stmt.Bindings {
		fact.Components[i] = MapIterationComponent{Type: source.TypeArgs[i], Mode: binding.Mode, Discard: binding.Discard, Source: binding.Token}
	}
	return fact
}

// cloneMapIterationEntries detaches canonical component type surfaces for shared
// compiler and tooling consumers; observing a fact never re-runs type inference.
// Rules: rules/compiler/compiler_analysis.md — §18, immutable analysis facts.
func cloneMapIterationEntries(fact *MapIterationEntries) *MapIterationEntries {
	if fact == nil {
		return nil
	}
	result := *fact
	for i := range result.Components {
		result.Components[i].Type = semanticSnapshotType(result.Components[i].Type)
	}
	return &result
}

// MapIterationAt returns shared entry facts at a loop keyword, source head or
// component binding, without type/member inference or protocol discovery.
// Rules: rules/compiler/compiler_analysis.md — §18(6);
// rules/collections/collections.md — §14.6; rules/tooling/lsp.md — "Hover".
func (a *Analyzer) MapIterationAt(file string, line, column int) (ForIterationDependencies, bool) {
	if a == nil {
		return ForIterationDependencies{}, false
	}
	matches := func(token lexer.Token) bool {
		return token.File == file && token.Line == line && token.Column == column
	}
	for stmt, fact := range a.iterationDependencies {
		if fact.MapEntries == nil {
			continue
		}
		if matches(fact.Loop) || matches(fact.MapEntries.Iterable) || matches(fact.MapEntries.Components[0].Source) || matches(fact.MapEntries.Components[1].Source) {
			return a.ForIterationDependenciesOf(stmt)
		}
	}
	return ForIterationDependencies{}, false
}

// Display describes the already resolved component modes and unspecified order
// for tooling. It never derives container capabilities from method spellings.
// Rules: rules/collections/collections.md — §§14.2,14.6;
// rules/tooling/lsp.md — "Hover".
func (fact MapIterationEntries) Display() string {
	describe := func(component MapIterationComponent) string {
		if component.Discard {
			return "discard (" + typeDisplayName(component.Type) + ")"
		}
		prefix := ""
		if component.Mode != ast.ForBindingValue {
			prefix = string(component.Mode) + " "
		}
		return prefix + typeDisplayName(component.Type)
	}
	return fmt.Sprintf("Map entry iteration\n\n```sec\nkey: %s\nvalue: %s\n```\n\nIteration order: %s.", describe(fact.Components[0]), describe(fact.Components[1]), fact.Order)
}
