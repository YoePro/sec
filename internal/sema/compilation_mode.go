package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// CompilationMode is the mode of one CompilationPlan. It is a named mode,
// not a boolean, so later modes extend it without reinterpreting Test.
// Rules: rules/tooling/testing.md — § 3.1 "Test compilation", § 26.1 "First-class compilation mode".
type CompilationMode string

const (
	// CompilationProduction is every ordinary build, check and run: no
	// *_test.sec file and no project tests/ module participates.
	CompilationProduction CompilationMode = "production"
	// CompilationTest is a TestCompilationPlan built by `sec test`.
	CompilationTest CompilationMode = "test"
)

// SetCompilationMode freezes the plan's compilation mode before analysis.
// Rules: rules/tooling/testing.md — §§ 3.1, 26.1.
func (a *Analyzer) SetCompilationMode(mode CompilationMode) {
	a.recordConfiguration(func(b *Analyzer) { b.SetCompilationMode(mode) })
	a.compilationMode = mode
}

// CompilationMode returns the plan's mode; an unconfigured analyzer is a
// production plan.
// Rules: rules/tooling/testing.md — § 3.1.
func (a *Analyzer) CompilationMode() CompilationMode {
	if a.compilationMode == "" {
		return CompilationProduction
	}
	return a.compilationMode
}

// ProjectTestRoot returns the project-root tests/ source tree of the
// project that owns file, or "" outside a project (only a directory with
// .sec/sec.toml is a project root).
// Rules: rules/tooling/testing.md — § 8.1 "Project test source tree".
func ProjectTestRoot(file string) string {
	directory, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(directory, ".sec", "sec.toml")); err == nil && !info.IsDir() {
			return filepath.Join(directory, "tests")
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return ""
		}
		directory = parent
	}
}

// IsTestOnlySource reports source that exists only in a TestCompilationPlan:
// a *_test.sec file, or any source below the project-root tests/ tree.
// Rules: rules/tooling/testing.md — §§ 4.3, 8.1, 8.2; rules/projects/modules.md — § 19.1.
func IsTestOnlySource(file string) bool {
	if isTestSourceFile(file) {
		return true
	}
	return belowTestRoot(file, ProjectTestRoot(file))
}

// belowTestRoot checks whether a source belongs to the project test-only tree.
// Rules: rules/tooling/testing.md — §8.1; rules/projects/modules.md — §19.1.
func belowTestRoot(file, root string) bool {
	if root == "" {
		return false
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, absolute)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// TestOnlyImportIssues reports an import that selects a project tests/
// module where the plan does not allow it: a production plan never includes
// the tests/ tree, and in a TestCompilationPlan production source still
// must not depend on it (Production -> Test is invalid; Test -> Test and
// Test -> Production are allowed).
// Rules: rules/tooling/testing.md — §§ 8.1, 25.2, 26.2, 26.5; rules/projects/modules.md — § 19.1.
func TestOnlyImportIssues(stmt *ast.ImportStatement, importerFallback string, sourcePaths []string, mode CompilationMode) []Error {
	if stmt == nil || len(sourcePaths) == 0 {
		return nil
	}
	testOnly := false
	for _, path := range sourcePaths {
		if belowTestRoot(path, ProjectTestRoot(path)) {
			testOnly = true
			break
		}
	}
	if !testOnly {
		return nil
	}
	importer := stmt.Token.File
	if importer == "" {
		importer = importerFallback
	}
	issue := UnresolvedSourceImport(stmt)
	issue.ID = diagnostics.ProductionDependsOnTest
	switch {
	case mode != CompilationTest:
		issue.Message = fmt.Sprintf("test-only module %q participates only in a TestCompilationPlan", stmt.Path)
		issue.Help = "Modules below the project-root tests/ tree are compiled only by sec test; production builds and checks never include them."
	case !IsTestOnlySource(importer):
		issue.Message = fmt.Sprintf("production source cannot import test-only module %q", stmt.Path)
		issue.Help = "Production source must compile without test-only modules. Move the shared code into a production module, or import it only from *_test.sec or tests/ source."
	default:
		return nil
	}
	return []Error{issue}
}
