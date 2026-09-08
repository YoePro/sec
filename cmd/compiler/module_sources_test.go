package main

import (
	"path/filepath"
	"testing"

	"sec/internal/sema"
)

func TestSingleFileAnalysisLoadsModuleSiblings(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "module_siblings", "active.sec")
	source, counts := parseSourceFileWithDiagnostics(path)
	if counts.Errors != 0 {
		t.Fatalf("parse: %+v", counts)
	}
	counts = assembleCLIModuleSources(source.Program, hostCompilerTarget())
	if counts.Errors != 0 {
		t.Fatalf("siblings: %+v", counts)
	}
	if errors := sema.NewAnalyzer().Analyze(source.Program); len(errors) != 0 {
		t.Fatalf("sibling types unresolved: %+v", errors)
	}
	before := len(source.Program.Statements)
	assembleCLIModuleSources(source.Program, hostCompilerTarget())
	if len(source.Program.Statements) != before {
		t.Fatal("module assembly duplicates already included siblings")
	}
}
