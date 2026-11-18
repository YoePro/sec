package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestThreadSpawnResultTyping keeps creation failure outside the complete
// child type for calls, generics, methods, lambdas and callable values.
// Rules: rules/concurrency/threads.md — §§2,19,26,34;
// rules/concurrency/spawn.md — §3.
func TestThreadSpawnResultTyping(t *testing.T) {
	file := "../../testdata/sema/thread_spawn_result_valid.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	spawns, nested, empty := 0, 0, 0
	ast.Visit(program, func(node any) bool {
		expr, ok := node.(*ast.SpawnExpression)
		if !ok {
			return true
		}
		typ, ok := a.ResolvedTypeOf(expr)
		if !ok || typ.Kind != ResultType || len(typ.TypeArgs) != 2 {
			t.Fatalf("raw thread at %d: %s", expr.Token.Line, typeDisplayName(typ))
		}
		owner, failure := typ.TypeArgs[0], typ.TypeArgs[1]
		if !owner.Intrinsic || !isThreadType(owner) || len(owner.TypeArgs) != 1 || failure.Name != "ThreadSpawnError" || failure.Module != "core" || !failure.Intrinsic {
			t.Fatalf("noncanonical creation layer: %s", typeDisplayName(typ))
		}
		switch owner.TypeArgs[0].Kind {
		case ResultType:
			nested++
		case VoidType:
			empty++
		}
		if implicitlyCopyable(typ) {
			t.Fatal("creation result copied its thread owner")
		}
		spawns++
		return true
	})
	if spawns != 11 || nested != 1 || empty != 1 {
		t.Fatal(spawns, nested, empty)
	}
}

// TestThreadSpawnResultRejections requires creation handling before thread
// lifecycle operations, compatible error propagation and explicit owner disposal.
// Rules: rules/concurrency/threads.md — §§2,19,34; discard.md — §9.
func TestThreadSpawnResultRejections(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/thread_spawn_result_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	for line, want := range map[int]string{7: "Thread[int]", 12: "join requires Task[T] or Thread[T]", 16: "bodyless try propagates ThreadSpawnError", 22: "cannot discard spawn result", 27: "unresolved lifecycle handle"} {
		found := false
		for _, e := range errors {
			found = found || e.Line == line && strings.Contains(e.Message, want)
		}
		if !found {
			t.Errorf("missing %q at %d in %v", want, line, errors)
		}
	}
	if len(errors) != 7 {
		t.Fatalf("want seven errors, got %v", errors)
	}
}
