package main

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// testingNamespaceCompletionItems completes `testing.` inside a test
// declaration with exactly the compiler-known testing surface, one item per
// overload, and nothing else. Outside a test body the selector is left to
// ordinary member completion.
//
// Rules:
//   - rules/tooling/testing.md — §11 "Compiler-known testing namespace"
func testingNamespaceCompletionItems(program *ast.Program, text string, dotOffset int, prefix string) ([]completionItem, bool) {
	if program == nil || dotOffset < len("testing") || text[dotOffset-len("testing"):dotOffset] != "testing" {
		return nil, false
	}
	if start := dotOffset - len("testing"); start > 0 && (isIdentifierByte(text[start-1]) || text[start-1] == '.') {
		return nil, false
	}
	if !offsetInsideTestBody(program, text, dotOffset) {
		return nil, false
	}
	items := []completionItem{}
	for _, member := range sema.TestingNamespaceMembers() {
		if !completionLabelMatches(member.Name, prefix) {
			continue
		}
		items = append(items, completionItem{
			Label:         member.Name,
			Kind:          2,
			Detail:        "testing." + member.Signature,
			Documentation: member.Summary,
		})
	}
	return items, true
}

func offsetInsideTestBody(program *ast.Program, text string, offset int) bool {
	for _, statement := range program.Statements {
		test, ok := statement.(*ast.TestDeclaration)
		if !ok || test.Body == nil {
			continue
		}
		open, close := bodyRangeAfter(text, test.Body.Token)
		if open < 0 || offset <= open {
			continue
		}
		if close < 0 || offset < close {
			return true
		}
	}
	return false
}
