// Source-local Reset/Release diagnostics for represented validity-preserving
// dependencies. This owner does not expand dependency discovery or NLL.
// Rules: rules/memory/arena.md — §§36(1–4), 44(1–2), 119–120;
// rules/corrections/applied/correction28-20260823.md — "Bug 1" and "Bug 2".
package sema

import (
	"fmt"
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkArenaInvalidationDependencies rejects an explicitly selected Reset or
// Release when a represented current-epoch reference/slice has a dependent
// holder or deferred use. Stable declaration ordering picks one witness per
// operation rather than emitting duplicates for all aliases. Immediate locals
// still use the separate stale-use check; this is not a complete NLL proof.
// Rules: rules/memory/arena.md — §§36(1–4), 44(1–2), 119(1–3), 120(1);
// rules/tooling/diagnostics.md — §§9, 28;
// rules/corrections/applied/correction28-20260823.md — "Bug 1" and "Bug 2".
func (a *Analyzer) checkArenaInvalidationDependencies(operation, domain, owner string, token lexer.Token) bool {
	names := make([]string, 0, len(a.symbols))
	for name := range a.symbols {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := a.symbols[names[i]].Token, a.symbols[names[j]].Token
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return names[i] < names[j]
	})
	current := a.arenaGenerations[domain]
	for _, name := range names {
		symbol := a.symbols[name]
		if name == owner {
			continue
		}
		if _, unavailable := a.moved[name]; unavailable {
			continue
		}
		if !typeCarriesReferenceOrigin(symbol.Type) || symbol.Type.ReferenceOriginStorage != StorageOriginArena ||
			symbol.Type.ReferenceOriginName != domain || symbol.Type.ReferenceOriginGeneration != current {
			continue
		}
		if !a.arenaDependencyEscapesImmediateLocal(name, owner) {
			continue
		}
		a.appendError(arenaInvalidationDependencyDiagnostic(operation, owner, name, token, symbol.Token))
		return true
	}
	return false
}

// arenaInvalidationDependencyDiagnostic distinguishes Reset's epoch
// invalidation from Release's terminal domain/owner operation. The occurrence
// is a proven represented dependency violation, not a capacity failure or an
// unproven analysis result. The caller supplies only Reset or Release.
// Rules: rules/memory/arena.md — §§35–36, 43–44, 119(1–3), 120(1);
// rules/tooling/diagnostics.md — §§4(7), 5, 8–10;
// rules/compiler/compiler_analysis.md — §7(4), §58(3).
func arenaInvalidationDependencyDiagnostic(operation, owner, dependency string, source, declaration lexer.Token) Error {
	id := diagnostics.ArenaResetLiveDependency
	consequence := "Reset invalidates the current allocation epoch."
	if operation == "Release" {
		id = diagnostics.ArenaReleaseLiveDependency
		consequence = "Release ends the Arena domain and consumes its owner."
	}
	endLine, endColumn := source.EndPosition()
	return Error{
		ID: id, Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofInvalid,
		Message: fmt.Sprintf("cannot call Arena.%s on %s while dependency %s remains live across the operation", operation, owner, dependency),
		Help:    consequence + " Complete deferred uses and end dependent borrows before the operation, or keep the Arena live until those uses finish.",
		File:    source.File, Line: source.Line, Column: source.Column, EndLine: endLine, EndColumn: endColumn,
		PreviousFile: declaration.File, PreviousLine: declaration.Line, PreviousColumn: declaration.Column,
		RelatedLabel: "live Arena dependency",
	}
}
