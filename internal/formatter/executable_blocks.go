package formatter

import (
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatExecutableBlocks expands parser-confirmed one-line executable blocks.
// It rewrites the outermost remaining block on each pass so a nested one-line
// block first acquires its correct surrounding indentation. Aggregate literals
// and structural declaration braces have no executable-block role and remain
// outside this transformation.
//
// Rules:
//   - rules/tooling/formatter.md — §8(3–8) "Brace placement"
//   - rules/tooling/formatter.md — §29(1–3) formatter invariants
func formatExecutableBlocks(text string, indentationWidth int) string {
	for {
		program := parser.New(lexer.New(text)).ParseProgram()
		document := cst.Build(text, "")
		document.ApplyProgramRoles(program)

		groups := outermostSingleLineExecutableBlocks(document, text)
		if len(groups) == 0 {
			return text
		}

		// Groups are source ordered. Apply from right to left so every byte span
		// remains valid while independent outermost blocks are expanded together.
		for index := len(groups) - 1; index >= 0; index-- {
			group := groups[index]
			open := document.Elements[group.Open]
			close := document.Elements[group.Close]
			indent := sourceLineIndent(text, open.Span.Start)
			content := strings.TrimSpace(text[open.Span.End:close.Span.Start])
			replacement := "\n" + indent
			if content != "" {
				replacement = "\n" + indent + strings.Repeat(" ", indentationWidth) + content + "\n" + indent
			}
			text = text[:open.Span.End] + replacement + text[close.Span.Start:]
		}
	}
}

func outermostSingleLineExecutableBlocks(document cst.Document, text string) []cst.DelimiterGroup {
	candidates := make([]bool, len(document.Groups))
	for index, group := range document.Groups {
		if group.Close < 0 || !document.Elements[group.Open].HasRole(cst.ExecutableBlockOpen) ||
			!document.Elements[group.Close].HasRole(cst.ExecutableBlockClose) {
			continue
		}
		open := document.Elements[group.Open]
		close := document.Elements[group.Close]
		if !strings.Contains(text[open.Span.End:close.Span.Start], "\n") {
			candidates[index] = true
		}
	}

	groups := []cst.DelimiterGroup{}
	for index, candidate := range candidates {
		if !candidate {
			continue
		}
		nested := false
		for parent := document.Groups[index].Parent; parent >= 0; parent = document.Groups[parent].Parent {
			if candidates[parent] {
				nested = true
				break
			}
		}
		if !nested {
			groups = append(groups, document.Groups[index])
		}
	}
	return groups
}

func sourceLineIndent(text string, at int) string {
	lineStart := strings.LastIndexByte(text[:at], '\n') + 1
	indentEnd := lineStart
	for indentEnd < at && isHorizontalFormatterByte(text[indentEnd]) {
		indentEnd++
	}
	return text[lineStart:indentEnd]
}
