// Package fixes materializes compiler-authorized edits with exact source spans.
package fixes

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Edit is a half-open UTF-8 source replacement, independent of LSP coordinates.
type Edit struct {
	Start, End  int
	Replacement string
}

// Pitfall materializes an existing certified action. Syntax reconstruction
// verifies the complete expression and delimiters; it never derives a new
// semantic repair. Ambiguous or recovered source is left for manual editing.
// Rules: rules/analysis/pitfall_analysis.md — "ProvenFix", "Fix safety";
// rules/tooling/lsp.md — "Safe fixes"; rules/tooling/diagnostics.md — §9.
func Pitfall(source, file string, finding sema.PitfallFinding, action sema.PitfallSuggestedAction) (Edit, bool) {
	if finding.Subject.Source.File != file {
		return Edit{}, false
	}
	proof := action.Safety
	if finding.State != sema.PitfallStateFinding || action.Kind != sema.PitfallProvenFix || action.Replacement == "" || proof.Rule != finding.Rule || proof.Replacement != action.Replacement || proof.EvaluationOrder == "" || proof.EvaluationCount == "" || proof.Effects == "" || proof.Failure == "" || proof.Ownership == "" || proof.Borrow == "" || proof.ControlFlow == "" {
		return Edit{}, false
	}
	// Actions replacing a different subexpression need an explicit producer
	// span; the current value-only action model cannot certify that range.
	if action.Source.File != finding.Subject.Source.File || action.Source.Line != finding.Subject.Source.Line || action.Source.Column != finding.Subject.Source.Column {
		return Edit{}, false
	}
	parsed := parser.New(lexer.NewWithFile(source, file)).Parse()
	if len(parsed.Diagnostics) != 0 || parsed.Fatal {
		return Edit{}, false
	}
	var expression ast.Expression
	matches := 0
	visitSyntax(reflect.ValueOf(parsed.Program), func(value reflect.Value) {
		candidate, ok := value.Interface().(ast.Expression)
		if !ok || value.Kind() != reflect.Pointer || value.IsNil() {
			return
		}
		token := value.Elem().FieldByName("Token")
		if !token.IsValid() {
			return
		}
		anchor, ok := token.Interface().(lexer.Token)
		if ok && anchor.Line == finding.Subject.Source.Line && anchor.Column == finding.Subject.Source.Column && candidate.String() == finding.Subject.Expression {
			expression = candidate
			matches++
		}
	})
	if matches != 1 {
		return Edit{}, false
	}
	start, end := len(source), 0
	visitSyntax(reflect.ValueOf(expression), func(value reflect.Value) {
		if token, ok := value.Interface().(lexer.Token); ok && token.Line > 0 && token.ByteEnd > token.ByteStart {
			if token.ByteStart < start {
				start = token.ByteStart
			}
			if token.ByteEnd > end {
				end = token.ByteEnd
			}
		}
	})
	if start >= end {
		return Edit{}, false
	}
	tokens := []lexer.Token{}
	lex := lexer.NewWithFile(source, file)
	for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
		tokens = append(tokens, token)
	}
	first, last := -1, -1
	for index, token := range tokens {
		if token.ByteStart == start {
			first = index
		}
		if token.ByteEnd == end {
			last = index
		}
	}
	if first < 0 || last < first {
		return Edit{}, false
	}
	// AST nodes omit some written closing delimiters and ordinary grouping.
	// Complete only immediately adjacent delimiters, then parse the exact
	// fragment again to reject accidental widening into another expression.
	for attempts := 0; attempts < len(tokens); attempts++ {
		needBefore, needAfter, valid := delimiterBalance(tokens[first : last+1])
		if !valid {
			return Edit{}, false
		}
		if needBefore {
			if first == 0 || tokens[first-1].Type != lexer.LPAREN {
				return Edit{}, false
			}
			first--
			continue
		}
		if needAfter != "" {
			if last+1 >= len(tokens) || string(tokens[last+1].Type) != needAfter {
				return Edit{}, false
			}
			last++
			continue
		}
		start, end = tokens[first].ByteStart, tokens[last].ByteEnd
		fragment := parser.New(lexer.New("module fix\nfn Fix() bool { return " + source[start:end] + "\n}")).Parse()
		if len(fragment.Diagnostics) != 0 || fragment.Fatal || len(fragment.Program.Statements) != 2 {
			return Edit{}, false
		}
		fn, ok := fragment.Program.Statements[1].(*ast.FunctionDeclaration)
		if !ok || fn.Body == nil || len(fn.Body.Statements) != 1 {
			return Edit{}, false
		}
		returned, ok := fn.Body.Statements[0].(*ast.ReturnStatement)
		if !ok || returned.Value == nil || returned.Value.String() != finding.Subject.Expression {
			return Edit{}, false
		}
		return Edit{Start: start, End: end, Replacement: action.Replacement}, true
	}
	return Edit{}, false
}

// visitSyntax visits detached parser structure, including tokens, without
// following semantic graphs or inferring language behavior.
// Rules: rules/tooling/lsp.md — "Safe fixes", "Recovery nodes".
func visitSyntax(value reflect.Value, visit func(reflect.Value)) {
	if !value.IsValid() {
		return
	}
	if value.Kind() == reflect.Interface {
		if !value.IsNil() {
			visitSyntax(value.Elem(), visit)
		}
		return
	}
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return
	}
	if value.CanInterface() {
		visit(value)
	}
	switch value.Kind() {
	case reflect.Pointer:
		visitSyntax(value.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			visitSyntax(value.Field(i), visit)
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			visitSyntax(value.Index(i), visit)
		}
	}
}

// delimiterBalance identifies missing parentheses preceding the AST's first
// leaf and written closing delimiters following its final leaf.
// Rules: rules/tooling/lsp.md — "Safe fixes" (exact affected source ranges).
func delimiterBalance(tokens []lexer.Token) (bool, string, bool) {
	stack := []string{}
	for _, token := range tokens {
		spelling := string(token.Type)
		switch token.Type {
		case lexer.LPAREN:
			stack = append(stack, string(lexer.RPAREN))
		case lexer.LBRACKET:
			stack = append(stack, string(lexer.RBRACKET))
		case lexer.LBRACE:
			stack = append(stack, string(lexer.RBRACE))
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			if len(stack) == 0 {
				return token.Type == lexer.RPAREN, "", token.Type == lexer.RPAREN
			}
			if stack[len(stack)-1] != spelling {
				return false, "", false
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return false, stack[len(stack)-1], true
	}
	return false, "", true
}
