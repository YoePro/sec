package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/diagnostics"
	"sec/internal/formatter"
	"sec/internal/sema"
)

// borrowRefactoringRequested keeps explicit lifetime restructuring out of
// quick-fix/fix-all/save requests. Rules: rules/tooling/lsp.md §§9.1,9.4.
func borrowRefactoringRequested(only []string) bool {
	if len(only) == 0 {
		return true
	}
	for _, kind := range only {
		if kind == "" || kind == "refactor" || kind == "refactor.rewrite" {
			return true
		}
	}
	return false
}

// borrowCodeActions offers an explicit refactoring that moves an existing
// reference-holder discard before a conflicting statement in the same block.
// It never changes an API, adds a move/copy, or destroys the borrowed owner.
// The canonical compiler must accept the entire formatted replacement.
// Rules: rules/memory/borrowing.md §§22.2(2),32(6),33;
// rules/tooling/lsp.md §§9.1,9.4 (explicit edits, snapshots and safe fixes).
func borrowCodeActions(uri, text string, reported []diagnostic, overlay sourceOverlay) []codeAction {
	actions := []codeAction{}
	candidates := []diagnostic{}
	for _, report := range reported {
		if borrowActionConflict(report.Code) {
			candidates = append(candidates, report)
		}
	}
	if len(candidates) == 0 {
		return actions
	}
	snapshot := ownershipActionAnalysis(uri, text, overlay)
	// No unrelated diagnostics may be repaired accidentally by the refactoring.
	if !snapshot.complete || len(snapshot.errors) != 1 {
		return actions
	}
	problem := snapshot.errors[0]
	if !borrowActionConflict(problem.ID) || !diagnosticBelongsToSource(problem, pathFromURI(uri)) {
		return actions
	}
	current := semaDiagnosticWithSources(problem, 1, uri, text, overlay)
	matched := false
	for _, report := range candidates {
		if report.Code == current.Code && report.Range == current.Range {
			matched = true
		}
	}
	if !matched {
		return actions
	}
	for _, fact := range snapshot.analyzer.ResolvedBorrowsAt(problem.PreviousFile, problem.PreviousLine, problem.PreviousColumn) {
		if fact.Holder == "" || fact.Plan.Implicit || fact.Plan.Temporary || fact.Plan.Reborrow {
			continue
		}
		replacement, ok := borrowDiscardBeforeConflict(text, snapshot.program, problem, fact)
		if !ok {
			continue
		}
		formatted, err := formatSourceForDocument(uri, replacement)
		if err != nil {
			continue
		}
		errors, complete := ownershipActionErrors(uri, formatted, overlay)
		if !complete || len(errors) != 0 {
			continue
		}
		edit, changed := formatter.MinimalEdit(text, formatted)
		edits := formattingEdits(text, edit, changed)
		if len(edits) == 0 {
			continue
		}
		actions = append(actions, codeAction{
			Title: fmt.Sprintf("End borrow '%s' before this access (move its discard)", fact.Holder),
			Kind:  "refactor.rewrite", Diagnostics: []diagnostic{current},
			Edit: workspaceEdit{Changes: map[string][]textEdit{uri: edits}},
		})
	}
	return actions
}

// borrowActionConflict limits candidates to current concrete access conflicts;
// loop-carried, defer-only, escape, authority and unknown-provenance findings
// are not invitations to change a reference's lifetime.
// Rules: rules/memory/borrowing.md §§15,21–23,32–33.
func borrowActionConflict(id string) bool {
	switch id {
	case diagnostics.BorrowSharedConflict, diagnostics.BorrowMutableConflict,
		diagnostics.BorrowOwnerReadConflict, diagnostics.BorrowMutationConflict,
		diagnostics.BorrowOwnershipEndConflict:
		return true
	}
	return false
}

// borrowDiscardBeforeConflict uses exact source AST siblings and Sema's holder
// declaration identity. Only a direct reference binding and a later standalone
// discard in the same block qualify; arbitrary statements are never reordered.
// Rules: rules/memory/borrowing.md §§22.2(2),32(6); rules/tooling/lsp.md §9.4.
func borrowDiscardBeforeConflict(text string, program *ast.Program, problem sema.Error, fact sema.ResolvedBorrowFact) (string, bool) {
	result := ""
	offset := textPositionOffset(text, problem.Line, problem.Column)
	// Only locate edits in the active source, not every core/imported body
	// assembled for proof. Shared source identities still come from Sema.
	source := &ast.Program{}
	for _, statement := range program.Statements {
		if token, ok := lspStatementTokenForSource(statement); ok && normalizedSourcePath(token.File) == normalizedSourcePath(problem.File) {
			source.Statements = append(source.Statements, statement)
		}
	}
	_ = walk.Inspect(source, func(node any) error {
		block, ok := node.(*ast.BlockStatement)
		if !ok || normalizedSourcePath(block.Token.File) != normalizedSourcePath(problem.File) {
			return nil
		}
		holder, conflict := -1, -1
		for i, statement := range block.Statements {
			if binding, ok := statement.(*ast.LetStatement); ok && binding.Name != nil &&
				binding.Name.Value == fact.Holder && binding.Name.Token.File == fact.HolderToken.File &&
				binding.Name.Token.Line == fact.HolderToken.Line && binding.Name.Token.Column == fact.HolderToken.Column {
				if _, direct := binding.Value.(*ast.RefExpression); direct {
					holder = i
				}
			}
			switch statement.(type) {
			case *ast.LetStatement, *ast.AssignmentStatement, *ast.DiscardStatement:
				start, end, located := walk.Span(statement)
				if located && start <= offset && offset < end {
					conflict = i
				}
			}
		}
		if holder < 0 || conflict <= holder {
			return nil
		}
		if borrowActionHasComments(program, block.Statements[conflict]) {
			return nil
		}
		insert, _, ok := borrowActionStatementLine(text, block.Statements[conflict])
		if !ok {
			return nil
		}
		for _, statement := range block.Statements[conflict+1:] {
			discard, ok := statement.(*ast.DiscardStatement)
			if !ok {
				continue
			}
			identifier, ok := discard.Value.(*ast.Identifier)
			if !ok || identifier.Value != fact.Holder {
				continue
			}
			if borrowActionHasComments(program, statement) {
				return nil
			}
			start, end, ok := borrowActionStatementLine(text, statement)
			if !ok || start <= insert {
				return nil
			}
			// Move source text, then let the shared formatter render the structured
			// edit under project settings. Semantic admission happens afterwards.
			result = text[:insert] + text[start:end] + text[insert:start] + text[end:]
			return nil
		}
		return nil
	})
	return result, result != "" && result != text
}

// borrowActionHasComments refuses to detach parser-owned comments from either
// statement affected by the reorder. Rules: rules/tooling/lsp.md §9.1;
// rules/tooling/formatter.md §12 (comment attachment).
func borrowActionHasComments(program *ast.Program, statement ast.Statement) bool {
	for _, attachment := range program.Comments {
		if attachment.Target == statement {
			return true
		}
	}
	return false
}

// borrowActionStatementLine admits only a standalone single-line statement,
// preserving comments and refusing ambiguous same-line siblings or headers.
// Rules: rules/tooling/lsp.md §9.4 (exact source ranges); rules/memory/borrowing.md §32(6).
func borrowActionStatementLine(text string, statement ast.Statement) (int, int, bool) {
	start, end, ok := walk.Span(statement)
	if !ok || start < 0 || end > len(text) || strings.ContainsAny(text[start:end], "\r\n") {
		return 0, 0, false
	}
	first := strings.LastIndex(text[:start], "\n") + 1
	last := len(text)
	if newline := strings.IndexByte(text[end:], '\n'); newline >= 0 {
		last = end + newline
	}
	if strings.TrimSpace(text[first:start]) != "" || strings.TrimSpace(text[end:last]) != "" {
		return 0, 0, false
	}
	if last == len(text) {
		return 0, 0, false
	}
	return first, last + 1, true
}
