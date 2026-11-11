package sema

import (
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// arenaDependencyRetentionWitnesses explains exactly the existing local
// retention predicate: deferred uses, additional borrow holders and reference
// carriers. Sorting chooses stable source witnesses independently of map order.
// Missing source tokens retain the dependency but never invent a navigation
// location; Error.RelatedLocations filters those unlocated witnesses.
// Rules: rules/memory/arena.md §§36(2–4),44(1),119(1–3);
// rules/control-flow/defer.md — deferred execution;
// rules/tooling/diagnostics.md §9.
func (a *Analyzer) arenaDependencyRetentionWitnesses(name, owner string) []RelatedLocation {
	witnesses := []RelatedLocation{}
	for _, records := range a.borrows {
		for _, record := range records {
			if record.Root != name {
				continue
			}
			if isDeferredUseKind(record.Kind) {
				witnesses = append(witnesses, arenaRetentionLocation(record.Token, "deferred use keeps the Arena dependency live"))
			} else if record.Holder != "" && record.Holder != name && record.Holder != owner {
				witnesses = append(witnesses, arenaRetentionLocation(record.Token, "retained borrow keeps the Arena dependency live"))
			}
		}
	}
	for holder, origin := range a.localRefContainers {
		if holder != name && holder != owner && origin.Name == name {
			witnesses = append(witnesses, arenaRetentionLocation(a.symbols.get(holder).Token, "reference carrier keeps the Arena dependency live"))
		}
	}
	sort.Slice(witnesses, func(i, j int) bool {
		l, r := witnesses[i], witnesses[j]
		if l.File != r.File {
			return l.File < r.File
		}
		if l.Line != r.Line {
			return l.Line < r.Line
		}
		if l.Column != r.Column {
			return l.Column < r.Column
		}
		return l.Label < r.Label
	})
	return witnesses
}

// arenaRetentionLocation preserves the complete canonical witness token span.
// Rules: rules/memory/arena.md §119(2); rules/tooling/diagnostics.md §§8–9.
func arenaRetentionLocation(token lexer.Token, label string) RelatedLocation {
	endLine, endColumn := token.EndPosition()
	return RelatedLocation{File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn, Label: label}
}

// ArenaInvalidationBlockersAt publishes detached canonical mandatory
// diagnostics at a Reset/Release operation for editor inspection. The query
// does not rerun lifetime analysis. Absence of a blocker is not a positive NLL,
// capacity, task/thread or foreign-retention proof.
// Rules: rules/memory/arena.md §§36,44,119–120;
// rules/tooling/lsp.md — shared compiler facts, Hover.
func (a *Analyzer) ArenaInvalidationBlockersAt(file string, line, column int) []Error {
	if a == nil {
		return nil
	}
	var result []Error
	for _, value := range a.errors {
		if (value.ID == diagnostics.ArenaResetLiveDependency || value.ID == diagnostics.ArenaReleaseLiveDependency) && value.File == file && value.Line == line && value.Column == column {
			result = append(result, value)
		}
	}
	result = cloneSemanticErrors(result)
	for i := range result {
		result[i].Related = append([]RelatedLocation(nil), result[i].Related...)
	}
	return result
}
