package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// stateTestCompletionItems offers only the Sema-resolved subject's variants
// in a non-binding is/is not test, including a partially typed qualifier.
// Recovery replaces only the unfinished pattern, preserving subject offsets.
// Unknown or non-union subjects produce no guessed variants.
// Rules: rules/declarations/unions.md — §§8.1,8.2a,12;
// rules/foundations/operators.md — "State tests with is";
// rules/tooling/lsp.md — "Completion", expected subject types;
// rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §7.
func stateTestCompletionItems(uri, text string, offset int, overlay sourceOverlay) ([]completionItem, bool) {
	patternStart, isToken, ok := stateTestCompletionPosition(uri, text, offset)
	if !ok {
		return nil, false
	}
	prefix := strings.TrimSpace(text[patternStart:offset])
	parseText := text[:patternStart] + " " + subjectCompletionPlaceholder + text[offset:]
	program := parseProgramForLSP(uri, parseText)
	if program == nil {
		return []completionItem{}, true
	}
	program, analyzer, release := cachedLSPAnalysis(uri, parseText, program, overlay)
	defer release()
	var subject ast.Expression
	walkLSPNodes(program, func(node any) {
		test, ok := node.(*ast.StateTestExpression)
		if ok && sameSourceToken(test.Token, isToken) {
			subject = test.Subject
		}
	})
	if subject == nil {
		return nil, false
	}
	typ, ok := subjectExpressionType(analyzer, subject)
	if !ok {
		return []completionItem{}, true
	}
	owner := lspTypeName(typ)
	qualifier := ""
	if dot := strings.IndexByte(prefix, '.'); dot >= 0 {
		qualifier, prefix = strings.TrimSpace(prefix[:dot]), strings.TrimSpace(prefix[dot+1:])
		// The source grammar permits one type qualifier; do not infer arbitrary
		// selector chains or a different union's variants.
		if qualifier != typ.Name && qualifier != owner {
			return []completionItem{}, true
		}
	}
	variants := []string{}
	switch typ.Kind {
	case sema.UnionType:
		for _, variant := range typ.UnionVariants {
			variants = append(variants, variant.Name)
		}
	case sema.ResultType:
		variants = []string{"Ok", "Err"}
	default:
		return []completionItem{}, true
	}
	items := []completionItem{}
	for _, variant := range variants {
		if !completionLabelMatches(variant, prefix) {
			continue
		}
		items = append(items, completionItem{Label: variant, Kind: 20, Detail: owner})
	}
	sortCompletionItems(items)
	return items, true
}

// stateTestCompletionPosition recognizes a lexical is/is not operator followed
// only by an unfinished variant name or owner.name. Tokens prevent strings,
// comments, completed tests and payload bindings from becoming state positions.
// Rules: rules/tooling/lsp.md — "Completion", "Recovery nodes";
// rules/foundations/operators.md — "State tests with is".
func stateTestCompletionPosition(uri, text string, offset int) (int, lexer.Token, bool) {
	if offset < 0 || offset > len(text) {
		return 0, lexer.Token{}, false
	}
	tokens := sourceTokens(uri, text[:offset])
	for i := len(tokens) - 1; i >= 0; i-- {
		token := tokens[i]
		if token.Type != lexer.IDENT || token.Lexeme != "is" {
			continue
		}
		if i == 0 || tokens[i-1].Type == lexer.DOT {
			continue
		}
		start := token.ByteEnd
		if i+1 < len(tokens) && tokens[i+1].Type == lexer.IDENT && tokens[i+1].Lexeme == "not" {
			start = tokens[i+1].ByteEnd
		}
		// At the end of the operator word the editor would replace is/not
		// with the variant label. Wait for a separating gap or a variant.
		if offset <= start {
			return 0, lexer.Token{}, false
		}
		patternTokens := sourceTokens(uri, text[start:offset])
		if len(patternTokens) > 3 {
			return 0, lexer.Token{}, false
		}
		for index, part := range patternTokens {
			expected := lexer.IDENT
			if index == 1 {
				expected = lexer.DOT
			}
			if part.Type != expected {
				return 0, lexer.Token{}, false
			}
		}
		return start, token, true
	}
	return 0, lexer.Token{}, false
}
