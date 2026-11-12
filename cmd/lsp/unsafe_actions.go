package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// unsafeOperationHover explains an unsafe operation from the shared
// registry: what it does, whether an unsafe context authorizes it, the
// caller obligations that context accepts and a safe alternative.
// Rules: rules/memory/unsafe.md — §§ 47(4), 52(1), (5).
func unsafeOperationHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	site, operation, ok := analyzer.UnsafeOperationAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	status := "authorized by an unsafe context"
	if !site.Authorized {
		status = "not authorized: it needs an unsafe context"
	}
	lines := []string{"\n\n**Unsafe operation** (`" + string(operation.Kind) + "`)", "", "- `" + site.Operation + "` " + operation.Explanation + "; " + status}
	if len(site.Obligations) > 0 {
		lines = append(lines, "- caller obligations: "+strings.Join(site.Obligations, "; "))
	}
	if operation.SafeAlternative != "" {
		lines = append(lines, "- safe alternative: "+operation.SafeAlternative)
	}
	lines = append(lines, "- unsafe accepts these obligations; it does not make a compiler-proven invalid operation valid")
	return strings.Join(lines, "\n")
}

// unsafeCodeActions offers the exact unsafe edits of unsafe.md § 52(3):
// add call-form `unsafe` to a call that needs it, and remove an unnecessary
// or redundant call-form `unsafe`. A wrap is offered only for a missing
// context, never for a proven-invalid operation (§ 52(4)), and every edit
// is reanalyzed and kept only when it removes the diagnostic without adding
// errors.
func unsafeCodeActions(uri, text string, reported []diagnostic, overlay sourceOverlay) []codeAction {
	actions := []codeAction{}
	path := pathFromURI(uri)
	for _, item := range reported {
		var start, end int
		var title, replacement string
		offset := lineCharToOffset(text, item.Range.Start.Line, item.Range.Start.Character)
		switch item.Code {
		case diagnostics.UnsafeCallRequiresContext:
			call := callAtOffset(text, path, offset)
			if call == nil {
				continue
			}
			first := callStartToken(call)
			start, end = first.ByteStart, first.ByteStart
			title, replacement = "Authorize this call with unsafe", "unsafe "
		case diagnostics.UnsafeUnnecessary, diagnostics.UnsafeRedundant:
			if offset < 0 || offset+len("unsafe ") > len(text) || text[offset:offset+len("unsafe ")] != "unsafe " {
				continue
			}
			// A block keeps its braces only as an ordinary block statement;
			// only the call-form prefix is removed.
			rest := strings.TrimLeft(text[offset+len("unsafe "):], " ")
			if strings.HasPrefix(rest, "{") {
				continue
			}
			start, end = offset, offset+len("unsafe ")
			title, replacement = "Remove unnecessary unsafe", ""
		default:
			continue
		}
		edited := text[:start] + replacement + text[end:]
		checked := parser.New(lexer.NewWithFile(edited, path)).Parse()
		if checked.Fatal || len(checked.Diagnostics) != 0 {
			continue
		}
		updated := sourceOverlay{Sources: map[string]string{}, Targets: overlay.Targets}
		for file, source := range overlay.Sources {
			updated.Sources[file] = source
		}
		updated.Sources[normalizedSourcePath(path)] = edited
		prepareProgramForLSP(checked.Program, path, updated)
		analyzer := newLSPAnalyzerWithOverlay(uri, checked.Program, updated)
		remaining := false
		for _, err := range analyzer.Analyze(checked.Program) {
			if diagnosticBelongsToSource(err, path) && err.Severity == diagnostics.SeverityError {
				remaining = true
			}
		}
		if remaining && item.Code == diagnostics.UnsafeCallRequiresContext {
			// The call may still be invalid for another reason; never imply
			// that unsafe repairs it.
			continue
		}
		actions = append(actions, codeAction{Title: title, Kind: "quickfix", Diagnostics: []diagnostic{item}, Edit: workspaceEdit{Changes: map[string][]textEdit{
			uri: {{Range: lspRange{Start: offsetPosition(text, start), End: offsetPosition(text, end)}, NewText: replacement}},
		}}})
	}
	return actions
}

// callAtOffset finds the call expression whose diagnostic token starts at a
// byte offset of the document.
func callAtOffset(text, path string, offset int) *ast.CallExpression {
	program := parser.New(lexer.NewWithFile(text, path)).ParseProgram()
	var found *ast.CallExpression
	ast.Visit(program, func(node any) bool {
		if call, ok := node.(*ast.CallExpression); ok && found == nil && call.Token.ByteStart == offset {
			found = call
		}
		return found == nil
	})
	return found
}

// callStartToken is the first token of a call: the leftmost token of its
// callee path.
func callStartToken(call *ast.CallExpression) lexer.Token {
	callee := call.Callee
	for {
		switch expr := callee.(type) {
		case *ast.MemberExpression:
			callee = expr.Object
			continue
		case *ast.Identifier:
			return expr.Token
		case *ast.CallExpression:
			callee = expr.Callee
			continue
		}
		return call.Token
	}
}
