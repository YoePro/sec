package main

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/sema"
	"strings"
)

// functionHoverContents renders resolved signatures with their static receiver
// category and documentation, retaining trusted foreign contract provenance.
// Rules: rules/declarations/static.md — §9; rules/tooling/lsp.md — "Hover";
// rules/platform/ffi.md — §42.
func functionHoverContents(functions []sema.Function, program *ast.Program, sourcePath string) string {
	lines := make([]string, 0, len(functions)+2)
	for _, function := range functions {
		params := make([]string, 0, len(function.Parameters))
		for _, parameter := range function.Parameters {
			params = append(params, initializerParameterLabel(parameter))
		}
		prefix := "fn"
		if function.Static {
			prefix = "static fn"
		}
		lines = append(lines, fmt.Sprintf("%s %s%s(%s) %s", prefix, function.Name, genericHeaderDisplay(function.GenericParameters, function.GenericConstraints, function.GenericTypePack), strings.Join(params, ", "), lspTypeName(function.ReturnType)))
	}
	contents := "```sec\n" + strings.Join(lines, "\n") + "\n```"
	// rules/platform/ffi.md §42: an extern @noPanic is a trusted foreign
	// contract, not a compiler-verified guarantee; hover keeps that provenance.
	if len(functions) == 1 && functions[0].TrustedNoPanic {
		contents += "\n\nTrusted foreign contract: `@noPanic` (declared, not verified by the compiler)"
	}
	if len(functions) == 1 && functionSourceMatches(functions[0], sourcePath) {
		if doc := functionDocumentation(program, functions[0].Token); doc != "" {
			contents += "\n\n" + doc
		}
	}
	return contents
}
