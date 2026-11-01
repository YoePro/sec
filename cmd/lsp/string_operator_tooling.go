package main

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// stringOperatorHover presents the source-selected failure mode of a string `+`
// or `+=` from Sema's plan, so the three cases read differently: ordinary
// materialization may panic, try reports StringError, and an unhandled Result
// operand or interpolation is invalid.
// Rules: rules/foundations/operators.md — "Runtime allocation and failure policy",
// "Compound concatenation assignment" (MD-039); correction MD-004-MD-039 § 6(6).
func stringOperatorHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	switch hovered.Type {
	case lexer.PLUS:
		for _, expression := range astExpressionsInProgram(program) {
			infix, ok := expression.(*ast.InfixExpression)
			if !ok {
				continue
			}
			plan, planned := analyzer.StringConcatPlanOf(infix)
			if planned && concatChainHasOperator(infix, hovered) {
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: "### `operator +`\n\nString concatenation, one materialization.\n\n" + stringFailureText(plan, false)}, Range: tokenRange(text, hovered)}, true
			}
		}
	case lexer.PLUS_ASSIGN:
		found := hoverResult{}
		matched := false
		ast.Visit(program, func(node any) bool {
			stmt, ok := node.(*ast.AssignmentStatement)
			if !ok || matched || stmt.Operator != "+=" || !operatorBetween(hovered, stmt) {
				return !matched
			}
			if resolved, known := analyzer.ResolvedStringAppendOf(stmt.Value); known {
				matched = true
				found = hoverResult{Contents: markupContent{Kind: "markdown", Value: "### `operator +=`\n\nString append: commits only after the new value is prepared.\n\n" + stringFailureText(resolved.Plan, resolved.Fallible)}, Range: tokenRange(text, hovered)}
			}
			return !matched
		})
		return found, matched
	}
	return hoverResult{}, false
}

// operatorBetween recognizes the operator token of an assignment, which lies
// between its target and its value.
func operatorBetween(operator lexer.Token, stmt *ast.AssignmentStatement) bool {
	value := leftmostExpressionToken(stmt.Value)
	target := leftmostExpressionToken(stmt.Target)
	return operator.File == value.File && target.ByteStart < operator.ByteStart && operator.ByteStart < value.ByteStart
}

func concatChainHasOperator(infix *ast.InfixExpression, operator lexer.Token) bool {
	if sameSourceToken(infix.Token, operator) {
		return true
	}
	for _, side := range []ast.Expression{infix.Left, infix.Right} {
		if nested, ok := side.(*ast.InfixExpression); ok && nested.Operator == "+" && concatChainHasOperator(nested, operator) {
			return true
		}
	}
	return false
}

func stringFailureText(plan sema.StringConcatPlan, fallibleOperand bool) string {
	switch {
	case !plan.Runtime:
		return "Compile-time text: no allocation and no failure path."
	case plan.Failure == sema.StringMaterializationPanic:
		return "May allocate. Without `try`, a failed materialization **panics** with `StringMaterializationFailure`. Write `try` to get a recoverable `StringError` instead."
	case plan.Failure == sema.StringMaterializationTry && fallibleOperand:
		return "May allocate. The `try` covers the `Result[string, StringError]` operand and the materialization as one `StringError` channel."
	case plan.Failure == sema.StringMaterializationTry:
		return "May allocate. The `try` reports a failed materialization as `StringError.Allocation`."
	case plan.Interpolated:
		return "Contains an interpolation, which requires `try`."
	}
	return "Has an unhandled `Result[string, StringError]` operand, which is invalid: a `Result` is never unwrapped implicitly. Write `try` to handle it."
}

// stringTryCandidate is one `try` insertion, offered only if the edited source
// proves it: a fix must remove an error, a refactor must add none.
type stringTryCandidate struct {
	title  string
	kind   string
	at     lexer.Token
	report *diagnostic
}

// stringTryCodeActions offers the `try` placements of string `+` and `+=`. The
// edit is verified by analysis, so a function whose error channel cannot take
// StringError gets no fix that would only move the error.
// Rules: correction MD-004-MD-039 §§ 3, 6(6); rules/foundations/operators.md.
func stringTryCodeActions(uri, text string, requested lspRange, reported []diagnostic, overlay sourceOverlay) []codeAction {
	path := pathFromURI(uri)
	parsed := parser.New(lexer.NewWithFile(text, path)).Parse()
	if parsed.Fatal || len(parsed.Diagnostics) != 0 {
		return nil
	}
	statements := map[*ast.AssignmentStatement]lexer.Token{}
	protected := map[*ast.AssignmentStatement]bool{}
	ast.Visit(parsed.Program, func(node any) bool {
		switch stmt := node.(type) {
		case *ast.TryAssignmentStatement:
			if stmt.Assignment != nil {
				protected[stmt.Assignment] = true
			}
		case *ast.AssignmentStatement:
			if stmt.Operator == "+=" {
				statements[stmt] = leftmostExpressionToken(stmt.Target)
			}
		}
		return true
	})
	prepareProgramForLSP(parsed.Program, path, overlay)
	analyzer := newLSPAnalyzerWithOverlay(uri, parsed.Program, overlay)
	baseline := len(analyzer.Analyze(parsed.Program))
	candidates := []stringTryCandidate{}
	for index := range reported {
		report := &reported[index]
		switch report.Code {
		case diagnostics.StringMaterializationRequiresTry:
			for _, expression := range astExpressionsInProgram(parsed.Program) {
				plan, planned := analyzer.StringConcatPlanOf(expression)
				if planned && plan.Runtime && plan.Failure == "" && expressionTokenWithin(text, expression, report.Range) {
					candidates = append(candidates, stringTryCandidate{"Write try before the expression: StringError covers it as one channel", "quickfix", leftmostExpressionToken(expression), report})
				}
			}
		case diagnostics.OperatorInvalidConcatOperand:
			for stmt, start := range statements {
				if !protected[stmt] && expressionTokenWithin(text, stmt.Value, report.Range) {
					candidates = append(candidates,
						stringTryCandidate{"Write try before the assignment: StringError covers the operand and the append", "quickfix", start, report},
						stringTryCandidate{"Write try before the operand: the append may still panic", "quickfix", leftmostExpressionToken(stmt.Value), report})
				}
			}
		}
	}
	for _, expression := range astExpressionsInProgram(parsed.Program) {
		infix, ok := expression.(*ast.InfixExpression)
		if !ok || !tokenInRange(text, infix.Token, requested) {
			continue
		}
		if plan, planned := analyzer.StringConcatPlanOf(infix); planned && plan.Failure == sema.StringMaterializationPanic {
			candidates = append(candidates, stringTryCandidate{title: "Write try: report a failed materialization as StringError instead of panicking", kind: "refactor.rewrite", at: leftmostExpressionToken(infix)})
		}
	}
	for stmt, start := range statements {
		resolved, known := analyzer.ResolvedStringAppendOf(stmt.Value)
		if known && resolved.Plan.Failure == sema.StringMaterializationPanic && tokenInRange(text, start, requested) {
			candidates = append(candidates, stringTryCandidate{title: "Write try: report a failed append as StringError instead of panicking", kind: "refactor.rewrite", at: start})
		}
	}
	return verifiedStringTryActions(uri, text, candidates, baseline, overlay)
}

func verifiedStringTryActions(uri, text string, candidates []stringTryCandidate, baseline int, overlay sourceOverlay) []codeAction {
	path := pathFromURI(uri)
	sortStringTryCandidates(candidates)
	var result []codeAction
	for _, candidate := range candidates {
		offset := lineCharToOffset(text, tokenRange(text, candidate.at).Start.Line, tokenRange(text, candidate.at).Start.Character)
		edited := text[:offset] + "try " + text[offset:]
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
		remaining := len(newLSPAnalyzerWithOverlay(uri, checked.Program, updated).Analyze(checked.Program))
		if candidate.report != nil && remaining >= baseline || candidate.report == nil && remaining > baseline {
			continue
		}
		at := offsetPosition(text, offset)
		action := codeAction{Title: candidate.title, Kind: candidate.kind, Edit: workspaceEdit{Changes: map[string][]textEdit{uri: {{Range: lspRange{Start: at, End: at}, NewText: "try "}}}}}
		if candidate.report != nil {
			action.Diagnostics = []diagnostic{*candidate.report}
		}
		result = append(result, action)
	}
	return result
}

// sortStringTryCandidates keeps action order deterministic: source order, then title.
func sortStringTryCandidates(candidates []stringTryCandidate) {
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && stringTryCandidateLess(candidates[j], candidates[j-1]); j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
}

func stringTryCandidateLess(left, right stringTryCandidate) bool {
	if left.at.ByteStart != right.at.ByteStart {
		return left.at.ByteStart < right.at.ByteStart
	}
	return fmt.Sprint(left.kind, left.title) < fmt.Sprint(right.kind, right.title)
}

func tokenInRange(text string, token lexer.Token, requested lspRange) bool {
	span := tokenRange(text, token)
	return comparePosition(span.End, requested.Start) >= 0 && comparePosition(span.Start, requested.End) <= 0
}

// expressionTokenWithin matches a diagnostic anchored at the expression's own
// token or its first token.
func expressionTokenWithin(text string, expression ast.Expression, reported lspRange) bool {
	for _, token := range []lexer.Token{expressionAnchorToken(expression), leftmostExpressionToken(expression)} {
		if token.Line > 0 && tokenRange(text, token).Start == reported.Start {
			return true
		}
	}
	return false
}

func expressionAnchorToken(expression ast.Expression) lexer.Token {
	switch node := expression.(type) {
	case *ast.InfixExpression:
		return node.Token
	case *ast.CallExpression:
		return node.Token
	case *ast.InterpolatedStringLiteral:
		return node.Token
	}
	return lexer.Token{}
}
