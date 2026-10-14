package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestGenericStorageDependencyPaths verifies one owning layout error per
// source cycle, carrying full generic source arguments and navigable carriers.
// Rules: rules/memory/layout.md — §25(1–5), §42(4);
// rules/compiler/monomorphization.md — §18.4–18.6.
func TestGenericStorageDependencyPaths(t *testing.T) {
	path := "../../testdata/generics/storage_paths_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var baseline []Error
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		for attempt := 0; attempt < 3; attempt++ {
			errors := a.Analyze(program)
			if len(errors) != 7 {
				t.Fatal(errors)
			}
			for _, e := range errors {
				if (e.ID != diagnostics.RecursiveStructLayout && e.ID != diagnostics.RecursiveUnionLayout) || e.ProofState != diagnostics.ProofInvalid || len(e.RelatedLocations()) == 0 || !strings.Contains(e.Message, " -> ") || strings.Contains(e.Message, "does not converge") {
					t.Fatal(e)
				}
				for _, related := range e.RelatedLocations() {
					if related.File != path || related.Line == 0 || related.EndColumn <= related.Column {
						t.Fatal(e, related)
					}
				}
			}
			if !strings.Contains(errors[3].Message, "Stabilizes[int]") || !strings.Contains(errors[3].Help, "not a proof of non-converging") {
				t.Fatal(errors[3])
			}
			wrapper := errors[6].RelatedLocations()
			if len(wrapper) != 3 || wrapper[0].Label != "by-value generic carrier Outer[U].inner" || wrapper[1].Label != "by-value generic carrier Inner[T].value" {
				t.Fatal(wrapper)
			}
			if baseline == nil {
				baseline = errors
			} else if !reflect.DeepEqual(errors, baseline) {
				t.Fatal("unstable layout witnesses", errors)
			}
		}
	}
}

// TestGenericStoragePathsAcrossSources ensures every cycle edge retains its
// owning file in either declaration order, without materializing recursive types.
// Rules: rules/memory/layout.md — §25(3), §42(4); rules/tooling/diagnostics.md — §9.
func TestGenericStoragePathsAcrossSources(t *testing.T) {
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		program := &ast.Program{}
		for _, suffix := range order {
			path := "../../testdata/generics/storage_paths_cross_" + suffix + ".sec"
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
		}
		errors := NewAnalyzer().Analyze(program)
		if len(errors) != 1 || errors[0].ID != diagnostics.RecursiveStructLayout {
			t.Fatal(errors)
		}
		files := map[string]bool{errors[0].File: true}
		for _, related := range errors[0].RelatedLocations() {
			files[related.File] = true
		}
		if len(files) != 2 || !strings.Contains(errors[0].Message, "Cross[T]") || !strings.Contains(errors[0].Message, "Cross[int]") {
			t.Fatal(errors)
		}
	}
}

// TestGenericStoragePathsOutsideModuleGraph retains direct storage validation
// for callable-local and associated declarations using the same owning reporter.
// Rules: rules/memory/layout.md — §25(1–5), §42(4).
func TestGenericStoragePathsOutsideModuleGraph(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/storage_paths_scoped_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.RecursiveStructLayout || e.ProofState != diagnostics.ProofInvalid || len(e.RelatedLocations()) != 1 {
			t.Fatal(e)
		}
	}
	if !strings.Contains(errors[0].Message, "Owner.Nested[T].next") || !strings.Contains(errors[1].Message, "Local[T].next") {
		t.Fatal(errors)
	}
}
