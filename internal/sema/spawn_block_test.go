package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// No tail-return inference, nested callable returns do not leak and raw
// creation errors stay distinct. Rules: rules/concurrency/spawn.md §§3A(1–6).
func TestVoidSpawnBlockTyping(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/void.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	for pass := 0; pass < 2; pass++ {
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		count := 0
		ast.Visit(program, func(node any) bool {
			if expr, ok := node.(*ast.SpawnExpression); ok {
				typ, known := a.ResolvedTypeOf(expr)
				if !known || typ.Kind != ResultType || len(typ.TypeArgs) != 2 || len(typ.TypeArgs[0].TypeArgs) != 1 || typ.TypeArgs[0].TypeArgs[0].Kind != VoidType {
					t.Fatalf("bad spawn type: %+v", typ)
				}
				if expr.Value != nil || expr.Body == nil {
					t.Fatal("source AST rewritten")
				}
				count++
			}
			return true
		})
		if count != 8 {
			t.Fatalf("%d blocks", count)
		}
	}
}

// A child gets no outer locals, loop destinations or parent error channel;
// must-use values keep their normal checks. Non-void inference stays gated.
// Rules: rules/concurrency/spawn.md §§3A(4–6),16; errorhandling.md §12.
func TestVoidSpawnBlockBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spawn_blocks/boundaries_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]string{"capture": "without explicit capture", "break": "break is only valid inside a loop", "continue": "continue is only valid inside a loop", "child-error": "void", "must-use": "must be handled explicitly", "non-void": "capability unavailable: spawned block body"}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, source)
			found := false
			for _, e := range errors {
				found = found || strings.Contains(e.Message, wants[name])
			}
			if !found {
				t.Fatalf("missing %q in %v", wants[name], errors)
			}
		})
	}
}
