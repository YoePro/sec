package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatHardwareDeclarations writes each well-formed `hardware` fact
// declaration canonically: `hardware <category> <Name> {`, one `name: value,`
// member per line in source order with a trailing comma, then `}`. Values keep
// their exact spelling (integer radix, string text, qualified enum cases) and
// lists print as `[a, b]`. A declaration containing a comment, or one the
// parser marked invalid, is left to the general token formatting so no
// comment or recovery text is lost.
//
// Rules:
//   - rules/foundations/grammar.md — HardwareDeclaration (Amendment MD-111)
//   - rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§ 3(6), 13(3)
func formatHardwareDeclarations(text string) string {
	program := parser.New(lexer.New(text)).ParseProgram()
	var comments []int
	for _, attachment := range program.Comments {
		for _, comment := range attachment.Comments {
			comments = append(comments, comment.ByteStart)
		}
	}
	for _, attachment := range program.Documentation {
		for _, comment := range attachment.Comments {
			comments = append(comments, comment.ByteStart)
		}
	}
	type replacement struct {
		start, end int
		text       string
	}
	var replacements []replacement
	for _, statement := range program.Statements {
		declaration, ok := statement.(*ast.HardwareDeclaration)
		if !ok || declaration.Invalid || declaration.Name == nil {
			continue
		}
		start := declaration.Token.ByteStart
		end, ok := hardwareDeclarationEnd(text, declaration.Name.Token.ByteEnd)
		if !ok || start < 0 || start >= end {
			continue
		}
		commented := false
		for _, offset := range comments {
			commented = commented || offset >= start && offset < end
		}
		if commented {
			continue
		}
		replacements = append(replacements, replacement{start: start, end: end, text: canonicalHardwareDeclaration(declaration)})
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, r := range replacements {
		text = text[:r.start] + r.text + text[r.end:]
	}
	return text
}

// hardwareDeclarationEnd returns the offset just after the `}` that closes the
// member block opened after offset, skipping string literals.
func hardwareDeclarationEnd(text string, offset int) (int, bool) {
	depth := 0
	for index := offset; index < len(text); index++ {
		switch text[index] {
		case '"':
			for index++; index < len(text) && text[index] != '"'; index++ {
				if text[index] == '\\' {
					index++
				}
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return 0, false
}

func canonicalHardwareDeclaration(declaration *ast.HardwareDeclaration) string {
	var out strings.Builder
	out.WriteString("hardware " + declaration.Category + " " + declaration.Name.Value + " {\n")
	for _, member := range declaration.Members {
		out.WriteString(member.Name.Value + ": " + member.Value.String() + ",\n")
	}
	out.WriteString("}")
	return out.String()
}
