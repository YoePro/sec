package sema

import (
	"path/filepath"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// isTestSourceFile reports *_test.sec source, the only place test-only
// declarations live.
func isTestSourceFile(file string) bool {
	return strings.HasSuffix(filepath.Base(file), "_test.sec")
}

// checkProductionDependsOnTest enforces the test-only dependency direction
// at every resolved name: production source may resolve production
// declarations only, while test source may resolve both. A test compilation
// analyzes production and *_test.sec source together, so without this check
// a test helper could satisfy a production reference that the production
// build cannot resolve. Each offending use is reported once.
//
// Rules:
//   - rules/tooling/testing.md — § 25.1 "Production must remain independently valid", § 25.2 "Allowed dependency direction"
//   - rules/tooling/testing.md — § 26.2 "Production graph" (test-only names must not satisfy production references), § 40.1
func (a *Analyzer) checkProductionDependsOnTest(use lexer.Token, declaration lexer.Token) {
	if use.File == "" || declaration.File == "" || isTestSourceFile(use.File) || !isTestSourceFile(declaration.File) {
		return
	}
	if a.isTestTypeMember(declaration) {
		// The owning test-only type is reported where it is named.
		return
	}
	key := sourceTokenLocation(use)
	if a.productionTestDependencies == nil {
		a.productionTestDependencies = map[sourceTokenKey]bool{}
	}
	if a.productionTestDependencies[key] {
		return
	}
	a.productionTestDependencies[key] = true
	name := use.Lexeme
	if name == "" {
		name = declaration.Lexeme
	}
	a.addErrorAtTokenWithMetadataAndPrevious(use, declaration, diagnostics.ProductionDependsOnTest,
		"Production source must compile without test-only declarations. Move the declaration into a production file, or use it only from *_test.sec source.",
		"production source cannot depend on test-only declaration %s", name)
}

// isTestTypeMember reports a field declared by a type in *_test.sec source.
func (a *Analyzer) isTestTypeMember(declaration lexer.Token) bool {
	for _, typ := range a.types {
		for _, field := range typ.Fields {
			if field.Token == declaration {
				return true
			}
		}
	}
	return false
}
