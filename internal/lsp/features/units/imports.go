// Package units owns source edits for compiler-validated unit assistance.
package units

import (
	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"strconv"
	"strings"
)

type Edit struct {
	Start, End int
	Text       string
}

// ImportEdits qualifies unresolved unit factors and inserts one canonical import
// after the module header, preserving comments, grouping, exponents and line endings.
// It provides candidates only: the caller must analyze the complete replacement.
// Rules: rules/tooling/lsp.md — Unit actions, Safe fixes;
// rules/projects/modules.md — §7 Import paths;
// rules/types/units.md — Formatter requirements, LSP requirements.
func ImportEdits(text string, program *ast.Program, unknown, module, name, path string) []Edit {
	if strings.HasPrefix(name, "_") {
		return nil
	}
	qualifier := module
	imported := false
	for _, statement := range program.Statements {
		if imp, ok := statement.(*ast.ImportStatement); ok && imp.Path == path {
			imported = true
			if imp.Alias != "" {
				qualifier = imp.Alias
			}
		}
	}
	qualified := qualifier + "." + name
	edits := []Edit{}
	valid := true
	_ = astwalk.Inspect(program, func(node any) error {
		if unit, ok := node.(*ast.UnitExpression); ok && unit.Kind == ast.UnitExpressionName && unit.Name == unknown && unit.Name != qualified {
			start := unit.Token.ByteStart
			end := start + len(unit.Name)
			if start < 0 || end > len(text) || text[start:end] != unit.Name {
				valid = false
				return nil
			}
			edits = append(edits, Edit{start, end, qualified})
		}
		return nil
	})
	if !valid {
		return nil
	}
	if len(edits) == 0 {
		return nil
	}
	if !imported {
		insert := -1
		for _, statement := range program.Statements {
			if module, ok := statement.(*ast.ModuleStatement); ok {
				start := module.Token.ByteStart
				if start < 0 || start > len(text) {
					return nil
				}
				if end := strings.IndexByte(text[start:], '\n'); end >= 0 {
					insert = start + end + 1
				} else {
					insert = len(text)
				}
				break
			}
		}
		if insert < 0 {
			return nil
		}
		newline := "\n"
		if strings.Contains(text, "\r\n") {
			newline = "\r\n"
		}
		prefix := ""
		if insert == len(text) && !strings.HasSuffix(text, "\n") {
			prefix = newline
		}
		edits = append(edits, Edit{insert, insert, prefix + "import " + strconv.Quote(path) + newline})
	}
	return edits
}
