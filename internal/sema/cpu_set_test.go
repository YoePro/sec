package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCpuSetLiteral checks nominal copyable identity, uint components, known
// duplicate rejection, and separation from ordinary generic collection types.
// Rules: rules/concurrency/threads.md — §7(1–6).
func TestCpuSetLiteral(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"cpu_set_valid.sec", 0}, {"cpu_set_invalid.sec", 8},
	} {
		source, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.file, errors)
		}
		typ := a.Types()["CpuSet"]
		if typ.Module != "core" || !typ.Named || !typ.Intrinsic || len(typ.Fields) != 0 || len(typ.GenericParameters) != 0 || !implicitlyCopyable(typ) {
			t.Fatalf("not the canonical opaque CpuSet: %+v", typ)
		}
		if tc.errors != 0 {
			for _, want := range []string{"duplicate CpuSet index 2", "CpuSet index must be uint", "positional CPU indices"} {
				found := false
				for _, d := range errors {
					found = found || strings.Contains(d.Message, want)
				}
				if !found {
					t.Fatalf("missing %q: %v", want, errors)
				}
			}
		}
	}
}

// TestCpuSetCoreDeclaration keeps trusted declaration provenance while rejecting
// shape changes, generics, attributes and untrusted replacement.
// Rules: rules/concurrency/threads.md — §7(1–3); core-library.md — core declarations.
func TestCpuSetCoreDeclaration(t *testing.T) {
	file := "../../testdata/sema/cpu_set_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range []string{"type CpuSet", "type CpuSet[T]", "@noCopy\ntype CpuSet", "type CpuSet struct {}", "enum CpuSet { A }", "interface CpuSet {}"} {
		source := strings.Replace(string(data), "type CpuSet", declaration, 1)
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(declaration, p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if declaration == "type CpuSet" {
			if len(errors) != 0 || a.Types()["CpuSet"].DeclarationToken.File != file {
				t.Fatal(errors)
			}
			program.SourceProvenance = nil
			if errors := NewAnalyzer().Analyze(program); len(errors) != 1 {
				t.Fatal("untrusted replacement accepted", errors)
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical opaque core declaration") {
			t.Fatal(declaration, errors)
		}
	}
}

// TestCpuSetTargetIndexWidth uses the selected uint carrier, without treating
// a compiler-host CPU count as the target's topology or affinity capability.
// Rules: rules/concurrency/threads.md — §7(5–6); rules/types/types.md — int and uint.
func TestCpuSetTargetIndexWidth(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/cpu_set_target_width.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		if width == 64 {
			if len(errors) != 0 {
				t.Fatal(width, errors)
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "outside the target uint range") {
			t.Fatal(width, errors)
		}
	}
}
