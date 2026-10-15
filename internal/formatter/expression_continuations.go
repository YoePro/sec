package formatter

import (
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatExpressionContinuations consumes CST-owned return/if continuation relations
// after structural indentation. Each relation adds one configured indentation
// level; nested expressions compose without altering operators or literals.
// Rules: rules/tooling/formatter.md — §4(8), §5, §6(3), §9(15), §25, §29.
func formatExpressionContinuations(text string, indentationWidth int) string {
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 || program == nil {
		return text
	}
	document.ApplyProgramRoles(program)
	levels := map[int]int{}
	for _, continuation := range document.ExpressionContinuations {
		levels[document.Elements[continuation.Element].Token.Line]++
	}
	if len(levels) == 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	for line, extra := range levels {
		lines[line-1] = strings.Repeat(" ", extra*indentationWidth) + lines[line-1]
	}
	return strings.Join(lines, "\n")
}
