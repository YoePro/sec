package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestThreadStorageConstructor checks compile-time capacity, ordinary explicit
// ownership transfer, immutable public metadata and rejection of legacy generics.
// Rules: rules/concurrency/threads.md — §8(1–7).
func TestThreadStorageConstructor(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{{"thread_storage_valid.sec", 0}, {"thread_storage_invalid.sec", 11}} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.file, errors)
		}
		typ := a.Types()["ThreadStorage"]
		if !typ.Intrinsic || !typ.ExplicitlyNonCopyable || typ.Module != "core" || len(typ.GenericParameters) != 0 || implicitlyCopyable(typ) {
			t.Fatal(typ)
		}
		if tc.errors == 0 {
			count := 0
			for expression, capacity := range a.threadStorageCapacities {
				if capacity != "4096" {
					t.Fatal(capacity)
				}
				got, ok := a.ThreadStorageCapacityOf(expression)
				if !ok || got != capacity {
					t.Fatal(got, ok)
				}
				count++
			}
			if count != 1 {
				t.Fatal("capacity facts", count)
			}
		}
	}
}

// TestThreadStorageCoreDeclaration checks exact trusted metadata and noCopy
// identity, with no public backing representation or value-generic parameter.
// Rules: rules/concurrency/threads.md — §8(1–2),(6–7).
func TestThreadStorageCoreDeclaration(t *testing.T) {
	file := "../../testdata/sema/thread_storage_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ old, new string }{{"", ""}, {"@noCopy\n", ""}, {"ThreadStorage struct", "ThreadStorage[T] struct"}, {"_capacity: uint", "Capacity: uint"}, {"_capacity: uint", "_capacity: int"}} {
		source := string(data)
		if tc.old != "" {
			source = strings.Replace(source, tc.old, tc.new, 1)
		}
		p := parser.New(lexer.NewWithFile(source, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if tc.old == "" {
			if len(errors) != 0 || a.Types()["ThreadStorage"].DeclarationToken.File != file {
				t.Fatal(errors)
			}
			program.SourceProvenance = nil
			if errors := NewAnalyzer().Analyze(program); len(errors) != 1 {
				t.Fatal("untrusted replacement", errors)
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical non-generic @noCopy") {
			t.Fatal(errors)
		}
	}
}

// TestThreadStorageCapacityFactsReset prevents syntax facts from an older
// analysis snapshot from supplying capacity evidence for a later compilation.
// Rules: rules/concurrency/threads.md — §8(3–4);
// rules/compiler/compiler_analysis.md — immutable snapshot results.
func TestThreadStorageCapacityFactsReset(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_storage_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	var old *ast.CallExpression
	for expression := range a.threadStorageCapacities {
		old = expression
	}
	if old == nil {
		t.Fatal("missing constructor fact")
	}
	empty, err := os.ReadFile("../../testdata/tooling/for_binding_empty.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(empty)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if capacity, ok := a.ThreadStorageCapacityOf(old); ok {
		t.Fatal("stale capacity", capacity)
	}
	var absent *Analyzer
	if _, ok := absent.ThreadStorageCapacityOf(old); ok {
		t.Fatal("nil analyzer has capacity")
	}
}
