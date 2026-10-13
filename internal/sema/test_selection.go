package sema

import (
	"regexp"
	"sec/internal/ast"
)

// SelectTests applies the existing --run name filter to resolved compiler test
// metadata, preserving structured identity independently of display/linker names.
// Rules: rules/tooling/testing.md — §§6,27,28,39.1–39.2;
// rules/compiler/compiler_pipeline.md — §11(3–5), §92(2).
func (a *Analyzer) SelectTests(pattern string) ([]ResolvedTestMetadata, error) {
	filter, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	selected := []ResolvedTestMetadata{}
	for _, metadata := range a.ResolvedTests() {
		if filter.MatchString(metadata.Name) {
			selected = append(selected, metadata)
		}
	}
	return selected, nil
}

// recordResolvedTestMetadata retains the stable, structured identity and source
// location of a valid top-level test without inventing a generated callable or
// linker-symbol identity.
//
// Rules:
//   - rules/tooling/testing.md — § 6 "Test identity"
//   - rules/tooling/testing.md — § 33.1 "Explicit semantic identity"
func (a *Analyzer) recordResolvedTestMetadata(declaration *ast.TestDeclaration) {
	if declaration == nil || declaration.Name == nil {
		return
	}
	endLine, endColumn := declaration.Name.Token.EndPosition()
	metadata := ResolvedTestMetadata{
		Identity: TestIdentity{
			Module: a.currentModule,
			Path:   []string{declaration.Name.Value},
		},
		Name: declaration.Name.Value,
		Source: TestSourceLocation{
			File:    declaration.Name.Token.File,
			Line:    declaration.Name.Token.Line,
			Column:  declaration.Name.Token.Column,
			EndLine: endLine, EndColumn: endColumn,
		},
		Declaration: declaration,
	}
	a.resolvedTestMetadata[declaration] = metadata
	a.resolvedTests = append(a.resolvedTests, metadata)
}
