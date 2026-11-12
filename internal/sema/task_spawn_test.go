package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTaskSpawnResultTyping checks the raw creation layer independently of
// await, for implicit/explicit tasks, methods, generics and callable values.
// Rules: rules/concurrency/tasks.md — §4(1–7); rules/concurrency/spawn.md — §3.
func TestTaskSpawnResultTyping(t *testing.T) {
	file := "../../testdata/sema/task_spawn_result_valid.sec"
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
	spawns, nested, empty, awaited := 0, 0, 0, 0
	ast.Visit(program, func(node any) bool {
		switch expression := node.(type) {
		case *ast.SpawnExpression:
			typ, ok := a.ResolvedTypeOf(expression)
			if !ok || typ.Kind != ResultType || len(typ.TypeArgs) != 2 {
				t.Fatalf("raw spawn at %d: %s", expression.Token.Line, typeDisplayName(typ))
			}
			owner, failure := typ.TypeArgs[0], typ.TypeArgs[1]
			if !owner.Intrinsic || !isTaskType(owner) || failure.Name != "TaskSpawnError" || !failure.Intrinsic {
				t.Fatalf("noncanonical creation types at %d: %s", expression.Token.Line, typeDisplayName(typ))
			}
			payload := typ.TypeArgs[0].TypeArgs[0]
			if payload.Kind == ResultType {
				nested++
			}
			if payload.Kind == VoidType {
				empty++
			}
			if implicitlyCopyable(typ) {
				t.Fatal("creation result copied its owning task")
			}
			spawns++
		case *ast.AwaitExpression:
			typ, ok := a.ResolvedTypeOf(expression)
			if !ok || typ.Name != "TaskOutcome" {
				t.Fatal(typ, ok)
			}
			awaited++
		}
		return true
	})
	if spawns != 13 || nested != 2 || empty != 1 || awaited != 4 {
		t.Fatal(spawns, nested, empty, awaited)
	}
}

// TestTaskSpawnResultRejections keeps raw creation distinct from its owner
// and applies ordinary Result propagation and aggregate discard restrictions.
// Rules: rules/concurrency/tasks.md — §§4–5,30; rules/control-flow/discard.md — §9.
func TestTaskSpawnResultRejections(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/task_spawn_result_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	for line, want := range map[int]string{
		8: "Task[int]", 13: "await requires an owning Task[T]",
		17: "bodyless try propagates TaskSpawnError",
		22: "cannot discard spawn result", 27: "unresolved lifecycle handle",
	} {
		found := false
		for _, diagnostic := range errors {
			found = found || diagnostic.Line == line && strings.Contains(diagnostic.Message, want)
		}
		if !found {
			t.Errorf("missing %q at line %d in %v", want, line, errors)
		}
	}
	if len(errors) != 5 {
		t.Fatalf("expected five source boundary errors, got %v", errors)
	}
}
