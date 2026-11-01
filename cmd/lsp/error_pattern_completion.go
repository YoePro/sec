package main

import (
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// errorPatternCompletionItems recovers only an unfinished Err(. selector,
// then presents the enclosing pattern's compiler-resolved failure channel.
// It never chooses a channel from a candidate variant name.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/errors/errorhandling.md — §§15.3,27.2b;
// rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §5(1–5).
func errorPatternCompletionItems(uri, text string, offset int, overlay sourceOverlay) ([]completionItem, bool) {
	errToken, dot, prefix, ok := errorPatternCompletionPosition(uri, text, offset)
	if !ok {
		return nil, false
	}
	// A fresh whole-channel binding is valid even for an unknown E whose
	// discardability is not proven. Err(_) would invalidate that recovery.
	binding := freshErrorPatternBinding(uri, text)
	insertion := binding + strings.Repeat(" ", offset-dot-1)
	if rest := strings.TrimSpace(text[offset:]); !strings.HasPrefix(rest, ")") {
		insertion += ") => " + subjectCompletionPlaceholder
	}
	parseText := text[:dot] + insertion + text[offset:]
	program := parseProgramForLSP(uri, parseText)
	if program == nil {
		return []completionItem{}, true
	}
	program, analyzer, release := cachedLSPAnalysis(uri, parseText, program, overlay)
	defer release()
	channel, found := errorPatternChannelAt(program, analyzer, errToken)
	if !found {
		return []completionItem{}, true
	}
	items := []completionItem{}
	if channel.Kind == sema.GenericType && channel.ErrorAssignable {
		return []completionItem{{Label: "Err(" + binding + ")", Kind: 6,
			Detail:        "Bind the whole error channel " + lspTypeName(channel),
			Documentation: "The constrained error type is unknown in this template. Bind the whole error; no concrete variants or discardability are implied.",
			FilterText:    "." + prefix, TextEdit: &textEdit{Range: offsetsRange(text, dot, offset), NewText: binding}}}, true
	}
	if channel.Kind == sema.ErrorRootType {
		// An open channel supplies no variant context. Offer explicit visible
		// concrete type qualifications, not variants from a workspace search.
		for name, typ := range analyzer.Types() {
			if !typ.ErrorAssignable || (typ.Kind != sema.EnumType && typ.Kind != sema.UnionType) {
				continue
			}
			items = append(items, completionItem{Label: name, Kind: typeCompletionKind(typ),
				Detail:        "Concrete error type; qualify the variant",
				Documentation: "This failure channel does not supply one concrete error type. Use Err(" + name + ".Variant).",
				FilterText:    "." + prefix,
				TextEdit:      &textEdit{Range: offsetsRange(text, dot, offset), NewText: name + "."}})
		}
	} else {
		for _, item := range memberCompletionItems(channel, analyzer, prefix, true, programModulePath(program), nil) {
			if item.Kind == 20 {
				item.TextEdit = &textEdit{Range: offsetsRange(text, dot+1, offset), NewText: item.Label}
				items = append(items, item)
			}
		}
	}
	sortCompletionItems(items)
	return items, true
}

// errorPatternCompletionPosition recognizes tokens, excluding strings,
// comments and ordinary qualified selectors. The returned Err token must
// subsequently belong to a parsed try handler or Result match arm.
// Rules: rules/tooling/lsp.md — "Completion", "Recovery nodes";
// rules/foundations/grammar.md — ErrPattern.
func errorPatternCompletionPosition(uri, text string, offset int) (lexer.Token, int, string, bool) {
	if offset < 0 || offset > len(text) {
		return lexer.Token{}, 0, "", false
	}
	tokens := sourceTokens(uri, text[:offset])
	prefix := ""
	i := len(tokens) - 1
	if i >= 0 && tokens[i].Type == lexer.IDENT {
		prefix = tokens[i].Lexeme
		i--
	}
	if i < 2 || tokens[i].Type != lexer.DOT || tokens[i-1].Type != lexer.LPAREN || tokens[i-2].Lexeme != "Err" || tokens[i-2].Type != lexer.IDENT {
		return lexer.Token{}, 0, "", false
	}
	return tokens[i-2], tokens[i].ByteStart, prefix, true
}

// errorPatternChannelAt obtains a try's entire protected channel, including
// argument/receiver failures, or the Result subject's error argument. Ordinary
// call expressions named Err do not establish a pattern context.
// Rules: rules/errors/errorhandling.md — §§11,15.3,27.2b;
// rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`".
func errorPatternChannelAt(program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (sema.Type, bool) {
	for _, expression := range astExpressionsInProgram(program) {
		switch expression := expression.(type) {
		case *ast.TryExpression:
			for _, handler := range expression.Handlers {
				if handler != nil && sameSourceToken(handler.Token, token) {
					fact, ok := analyzer.ResolvedTryOf(expression)
					return fact.ErrorType, ok
				}
			}
		case *ast.MatchExpression:
			for _, arm := range expression.Arms {
				if arm == nil || arm.Pattern == nil || arm.Pattern.Name != "Err" || !sameSourceToken(arm.Pattern.Token, token) {
					continue
				}
				typ, ok := analyzer.ResolvedTypeOf(expression.Subject)
				if ok && typ.Kind == sema.ResultType && len(typ.TypeArgs) == 2 {
					return typ.TypeArgs[1], true
				}
			}
		}
	}
	return sema.Type{}, false
}

// freshErrorPatternBinding chooses an unused source spelling for the inserted
// binding without performing name resolution or assuming an error variant.
// Rules: rules/tooling/lsp.md — "Completion"; MD-120 §§4(4–7),10(7).
func freshErrorPatternBinding(uri, text string) string {
	used := map[string]bool{}
	for _, token := range sourceTokens(uri, text) {
		if token.Type == lexer.IDENT {
			used[token.Lexeme] = true
		}
	}
	for index := 0; ; index++ {
		name := "cause"
		if index > 0 {
			name += strconv.Itoa(index)
		}
		if !used[name] {
			return name
		}
	}
}
