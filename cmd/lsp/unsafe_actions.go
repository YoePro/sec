package main

import (
	"fmt"
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
// add call-form `unsafe` to a registry-authorized call that needs it, and
// remove an unnecessary or redundant call-form prefix. A wrap needs a missing
// context, never for a proven-invalid operation (§ 52(4)), and every edit
// is reanalyzed and kept only when it removes the diagnostic without adding
// errors.
func unsafeCodeActions(uri, text string, reported []diagnostic, overlay sourceOverlay) []codeAction {
	actions := []codeAction{}
	path := pathFromURI(uri)
	var authorization *sema.Analyzer
	var currentDiagnostics []diagnostic
	for _, item := range reported {
		var start, end int
		var title, replacement string
		authorizing := false
		offset := lineCharToOffset(text, item.Range.Start.Line, item.Range.Start.Character)
		switch item.Code {
		case diagnostics.UnsafeCallRequiresContext, diagnostics.UnsafeOperationRequiresContext, diagnostics.RawPointerUnsafeRequired:
			call := callAtOffset(text, path, offset)
			if call == nil {
				continue
			}
			// The registry distinguishes call-form authorization from block-only
			// properties; a missing context can also mask a further invalidity.
			if authorization == nil {
				program := parser.New(lexer.NewWithFile(text, path)).ParseProgram()
				prepareProgramForLSP(program, path, overlay)
				authorization = newLSPAnalyzerWithOverlay(uri, program, overlay)
				authorization.Analyze(program)
			}
			token := call.Token
			if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property.Token.ByteStart == offset {
				token = member.Property.Token
			}
			site, operation, known := authorization.UnsafeOperationAt(path, token.Line, token.Column)
			if !known || site.Authorized || !operation.CallForm || operation.Diagnostic != item.Code {
				continue
			}
			authorizing = true
			first := callStartToken(call)
			start, end = first.ByteStart, first.ByteStart
			title, replacement = "Authorize this call with unsafe", "unsafe "
		case diagnostics.UnsafeUnnecessary, diagnostics.UnsafeRedundant:
			// A client may retain an old advisory after the call changes. Never
			// remove authorization unless that exact advisory still holds now.
			if currentDiagnostics == nil {
				currentDiagnostics = analyze(uri, text, overlay)
			}
			current := false
			for _, candidate := range currentDiagnostics {
				if candidate.Code == item.Code && candidate.Range == item.Range {
					current = true
					break
				}
			}
			if !current {
				continue
			}
			if offset < 0 || offset+len("unsafe ") > len(text) || text[offset:offset+len("unsafe ")] != "unsafe " {
				continue
			}
			// Removing a block marker would leave unsupported bare-block
			// syntax; only a call-form prefix is removed.
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
		if remaining && authorizing {
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

// callAtOffset finds a call by its callee or member diagnostic token at a
// byte offset of the document.
func callAtOffset(text, path string, offset int) *ast.CallExpression {
	program := parser.New(lexer.NewWithFile(text, path)).ParseProgram()
	var found *ast.CallExpression
	ast.Visit(program, func(node any) bool {
		if call, ok := node.(*ast.CallExpression); ok && found == nil {
			if call.Token.ByteStart == offset {
				found = call
			}
			if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property.Token.ByteStart == offset {
				found = call
			}
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
		case *ast.IndexExpression:
			callee = expr.Left
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

// trustProvenanceHoverLines presents a callable's trust provenance from the
// shared Sema summary: caller-facing safety, the unsafe operations in its own
// body, the trusted foreign declarations it calls and the path through which
// a safe wrapper still depends on unsafe code. Open implementation targets
// remain unknown. A safe callable with neither recorded trust dependencies
// nor open-target uncertainty gets no lines.
// Rules: rules/memory/unsafe.md — §§ 14(1)–(3), 52(1), (5).
func trustProvenanceHoverLines(analyzer *sema.Analyzer, graph *sema.CallGraph, id sema.CallableID) []string {
	provenance := analyzer.TrustProvenance(id)
	if !provenance.CallerFacingUnsafe && !provenance.DependsOnTrust() && len(provenance.UnknownPath) == 0 {
		return nil
	}
	safety := "safe"
	if provenance.CallerFacingUnsafe {
		safety = "unsafe (callers accept its obligations)"
	}
	lines := []string{"**Trust provenance**"}
	if !provenance.ForeignDeclaration {
		lines = append(lines, "Caller-facing safety: "+safety)
	}
	// unsafe.md §15: safety and effect summaries are separate dimensions.
	if len(provenance.Effects.Effects) > 0 {
		lines = append(lines, "Effects remain active through unsafe: "+unsafeEffectNames(provenance.Effects.Effects))
	}
	if len(provenance.Effects.UnknownEffects) > 0 {
		lines = append(lines, "Effects unknown through unsafe: "+unsafeEffectNames(provenance.Effects.UnknownEffects)+"; no effect-freedom proof")
	}
	if !provenance.DependsOnTrust() && len(provenance.UnknownPath) == 0 {
		return append(lines, "Implementation provenance: no unsafe operation or foreign call")
	}
	if provenance.ForeignDeclaration {
		lines = append(lines, "Implementation provenance: trusted foreign declaration outside Sec source verification")
	}
	if len(provenance.UnknownPath) > 0 {
		path := make([]string, 0, len(provenance.UnknownPath))
		for _, step := range provenance.UnknownPath {
			if node, ok := graph.Node(step); ok {
				path = append(path, node.Name)
			}
		}
		lines = append(lines, "Implementation trust unknown via `"+strings.Join(path, "` -> `")+"`; open callable contract retained")
	}
	if len(provenance.Direct) > 0 {
		kinds := []string{}
		seen := map[sema.UnsafeOperationKind]bool{}
		for _, site := range provenance.Direct {
			if !seen[site.Kind] {
				seen[site.Kind] = true
				kinds = append(kinds, string(site.Kind))
			}
		}
		lines = append(lines, fmt.Sprintf("Implementation provenance: `%d` unsafe operations here (`%s`)", len(provenance.Direct), strings.Join(kinds, "`, `")))
	}
	if len(provenance.Foreign) > 0 {
		names := make([]string, 0, len(provenance.Foreign))
		for _, foreign := range provenance.Foreign {
			if node, ok := graph.Node(foreign); ok {
				names = append(names, node.Name)
			}
		}
		lines = append(lines, "Implementation provenance: calls trusted foreign `"+strings.Join(names, "`, `")+"`")
	}
	if last := len(provenance.Path) - 1; last > 1 || last == 1 && !containsCallable(provenance.Foreign, provenance.Path[last]) {
		path := make([]string, 0, len(provenance.Path))
		for _, step := range provenance.Path {
			if node, ok := graph.Node(step); ok {
				path = append(path, node.Name)
			}
		}
		lines = append(lines, "Depends on unsafe code via `"+strings.Join(path, "` -> `")+"`")
	}
	return lines
}

func containsCallable(ids []sema.CallableID, id sema.CallableID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// unsafeEffectNames presents canonical effect domains without deriving a new
// effect or interpreting absence as an effect-free guarantee.
// Rules: rules/memory/unsafe.md §15(1–5), §52(1).
func unsafeEffectNames(effects []sema.SummaryEffect) string {
	names := make([]string, len(effects))
	for index, effect := range effects {
		names[index] = "`" + string(effect) + "`"
	}
	return strings.Join(names, ", ")
}
