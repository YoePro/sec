package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTestSourceVisibility preserves the source identity of a test invocation:
// module-internal helpers are available, same-file private helpers remain local,
// and trusted core authority cannot be obtained from a module spelling.
// Rules: rules/tooling/testing.md — §§4,7.1–7.4;
// rules/projects/modules.md — §6;
// rules/library/core-library.md — §1.2.
func TestTestSourceVisibility(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			for _, mode := range []string{"trusted", "private", "untrusted"} {
				t.Run(fmt.Sprintf("%d/%s/%s", width, depth, mode), func(t *testing.T) {
					program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
					files := []string{"helpers.sec", "helpers_test.sec", "sibling_test.sec"}
					if mode == "private" {
						files = append(files, "visibility_invalid_test.sec")
					}
					for _, file := range files {
						path := filepath.Join("../../testdata/core/test_visibility", file)
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						p := parser.New(lexer.NewWithFile(string(data), path))
						parsed := p.ParseProgram()
						if len(p.Errors()) != 0 {
							t.Fatal(p.Errors())
						}
						program.Statements = append(program.Statements, parsed.Statements...)
						if mode != "untrusted" || file == "helpers.sec" {
							program.SourceProvenance[path] = ast.SourceCore
						}
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					want := 0
					if mode == "private" {
						want = 2
					}
					if mode == "untrusted" {
						want = 1
					}
					if len(errors) != want {
						t.Fatalf("want %d errors, got %+v", want, errors)
					}
					for _, err := range errors {
						if !strings.Contains(err.Message, "not accessible from module core") {
							t.Fatal(err)
						}
						if mode == "private" && !strings.Contains(err.Message, "function __private") {
							t.Fatal(err)
						}
						if mode == "untrusted" && !strings.Contains(err.Message, "function _sharedProduction") {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}
