package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateInterruptEntryCalls rejects ordinary invocations of bound and unbound
// ISR entries. It consumes selected overloads and canonical callable targets;
// a matching spelling or an unknown callback is not evidence of an ISR target.
// Attribute capability checks remain independent of this source restriction.
// Rules: rules/platform/interrupts.md — §§2–4,25;
// rules/analysis/closure_analysis.md — "Soundness of target sets".
func (a *Analyzer) validateInterruptEntryCalls(program *ast.Program) {
	entries := map[sourceTokenKey]lexer.Token{}
	var collect func(ast.Statement)
	collect = func(statement ast.Statement) {
		switch declaration := statement.(type) {
		case *ast.FunctionDeclaration:
			if declaration != nil && declaration.Name != nil && (hasAttribute(declaration.Attributes, "isr") || hasAttribute(declaration.Attributes, "interrupt")) {
				entries[sourceTokenLocation(declaration.Name.Token)] = declaration.Name.Token
			}
		case *ast.ImplStatement:
			if declaration != nil {
				for _, member := range declaration.Members {
					if function, ok := member.(*ast.FunctionDeclaration); ok {
						collect(function)
					}
				}
			}
		}
	}
	if program == nil {
		return
	}
	for _, statement := range program.Statements {
		collect(statement)
	}
	if len(entries) == 0 {
		return
	}
	type violation struct{ call, entry lexer.Token }
	var violations []violation
	seen := map[[2]sourceTokenKey]bool{}
	add := func(call, declaration lexer.Token) {
		entry, exists := entries[sourceTokenLocation(declaration)]
		key := [2]sourceTokenKey{sourceTokenLocation(call), sourceTokenLocation(entry)}
		if !exists || seen[key] {
			return
		}
		seen[key] = true
		violations = append(violations, violation{call, entry})
	}
	// Resolved calls also retain source-invalid calls in unreachable branches;
	// the effect graph correctly prunes those branches for other analyses.
	for call, target := range a.resolvedCalls {
		add(callCalleeDefinitionToken(call), target.Function.Token)
	}
	if a.callGraph != nil {
		for _, site := range a.callGraph.sites {
			for _, target := range site.Targets {
				if node, ok := a.callGraph.nodes[target]; ok {
					add(site.Source, node.Declaration)
				}
			}
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		left, right := violations[i], violations[j]
		if left.call.File != right.call.File {
			return left.call.File < right.call.File
		}
		if left.call.Line != right.call.Line {
			return left.call.Line < right.call.Line
		}
		if left.call.Column != right.call.Column {
			return left.call.Column < right.call.Column
		}
		if left.entry.File != right.entry.File {
			return left.entry.File < right.entry.File
		}
		if left.entry.Line != right.entry.Line {
			return left.entry.Line < right.entry.Line
		}
		return left.entry.Column < right.entry.Column
	})
	for _, violation := range violations {
		a.addErrorAtTokenWithPreviousMetadata(violation.call, violation.entry, diagnostics.InterruptEntryNotCallable,
			"Move reusable work into an ordinary helper and call that helper from both the ISR entry and ordinary code. @interruptSafe helpers remain ordinary-callable; unsafe and tests do not authorize calling an ISR entry.",
			"interrupt entry %s cannot be called as an ordinary Sec function", violation.entry.Lexeme)
	}
}
