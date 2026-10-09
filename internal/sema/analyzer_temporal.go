package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema/temporal"
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

// compileTimeClockRead identifies prohibited ambient-clock operands by the
// compiler intrinsic registry or a resolved intrinsic temporal type identity.
// Rules: rules/types/temporal.md — §3 "UTC wall-clock access";
// rules/corrections/applied/temporal-now-correction-20260928.md — §7.
func (a *Analyzer) compileTimeClockRead(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.Identifier:
		known, ok := compilerKnownValue(expr.Value)
		return ok && known.ID == "CKV-TEMPORAL-NOW"
	case *ast.MemberExpression:
		if expr.Property == nil {
			return false
		}
		path, ok := typePathFromExpression(expr.Object)
		if !ok {
			return false
		}
		typ, found := a.types[a.resolveTypeName(path)]
		return found && temporal.IsWallClockProperty(typ.Intrinsic, typ.Name, expr.Property.Value)
	}
	return false
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

	// Trusted core declarations refine the same compiler-owned identity. Read
	// the resolved type table so intrinsic results preserve that refinement.
	// Rules: rules/types/temporal.md — §2; types/types.md — Type identity.
	if resolved, found := a.types[known.Result.Name]; found && resolved.Kind == known.Result.Kind {
		known.Result = resolved
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

// registerInstantDeclaration retains the compiler-owned monotonic identity,
// allowing only its exact opaque declaration in loader-proven core source.
// Rules: rules/concurrency/mutex.md §13(1)-(5); cancellation.md §43(4).
func (a *Analyzer) registerInstantDeclaration(stmt *ast.TypeDeclStatement) bool {
	if stmt.Name.Value != "Instant" || !a.isTrustedCoreBuiltinDeclaration("Instant", stmt.Name.Token) {
		return false
	}
	if stmt.BaseType != nil || stmt.AssignedType != nil || stmt.StructType != nil ||
		stmt.RegisterType != nil || stmt.Union || len(stmt.Variants) != 0 ||
		len(stmt.GenericParameters) != 0 || len(stmt.Attributes) != 0 || stmt.ErrorType ||
		stmt.Contract != nil || stmt.Default != nil || len(stmt.Implements) != 0 {
		a.addErrorAtToken(stmt.Name.Token, "Instant requires its canonical opaque declaration: type Instant")
	}
	typ := a.types["Instant"]
	typ.Module = "core"
	typ.Declared = true
	typ.DeclarationToken = stmt.Name.Token
	a.types["Instant"] = typ
	return true
}

// rejectInstantNonOpaqueDeclaration prevents enum/interface declarations from
// replacing the runtime-owned monotonic identity, including in trusted core.
// Rules: rules/concurrency/mutex.md §13(1)-(3); cancellation.md §43(4).
func (a *Analyzer) rejectInstantNonOpaqueDeclaration(name string, token lexer.Token) bool {
	if name != "Instant" {
		return false
	}
	a.addErrorAtToken(token, "Instant requires its canonical opaque declaration: type Instant")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}
