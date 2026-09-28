package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// CompilerKnownValueAt returns the stable intrinsic identity resolved at one
// source identifier. Semantic IR and later analyses consume this fact rather
// than reconstructing compiler ownership from the spelling `_now`.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
func (a *Analyzer) CompilerKnownValueAt(file string, line int, column int) (CompilerKnownValue, bool) {
	value, ok := a.compilerKnownValueFacts[sourceTokenKey{File: file, Line: line, Column: column}]
	value.Effects = append([]EffectKind(nil), value.Effects...)
	return value, ok
}

// inferCompilerKnownValue resolves compiler-owned value expressions without
// adding them to ordinary name lookup or completion. `_now` is accepted only
// for loader-proven core source and records one nondeterministic-input effect
// for every evaluation.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
//   - rules/corrections/applied/temporal-now-correction-20260928.md
func (a *Analyzer) inferCompilerKnownValue(expr *ast.Identifier) (Type, expressionValue, bool) {
	known, ok := compilerKnownValue(expr.Value)
	if !ok {
		return Type{}, expressionValue{}, false
	}
	if known.Internal && !a.isTrustedCoreSourceToken(expr.Token) {
		a.addErrorAtToken(expr.Token, "%s is a compiler-internal value available only to loader-proven core source", known.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}

	a.compilerKnownValueFacts[sourceTokenLocation(expr.Token)] = known
	a.recordCompilerKnownValueEffects(known, expr.Token)
	return known.Result, expressionValue{Display: expr.String()}, true
}

// recordCompilerKnownValueEffects preserves each intrinsic evaluation as a
// separate source effect; callers must not infer purity from value syntax.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — `_now`: `MayUseNondeterministicInput`
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
func (a *Analyzer) recordCompilerKnownValueEffects(value CompilerKnownValue, source lexer.Token) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	for _, kind := range value.Effects {
		a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: kind, Source: source})
	}
}
