package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTaskFailureDomainDiagnostics keeps creation, cancellation, panic, runtime
// failure and normal application Result returns distinct in every source form.
// Rules: rules/concurrency/tasks.md — §§5,11–15,30(1),(8–9),31(3–9).
func TestTaskFailureDomainDiagnostics(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		file := "../../testdata/sema/task_failure_domains_" + mode + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if mode == "invalid" {
			if len(errors) != 21 {
				t.Fatalf("%d errors: %v", len(errors), errors)
			}
			missing := 0
			for _, err := range errors {
				if err.ID == diagnostics.UnionVariantPayloadRequired {
					missing++
				} else if err.ID != diagnostics.TaskFailureDomainMismatch {
					t.Fatal(err)
				}
				for _, category := range []string{"TaskSpawnError", "Cancelled", "Panicked(PanicInfo)", "Failed(TaskError)", "Completed(Err(E))"} {
					if !strings.Contains(err.Help, category) {
						t.Fatal("missing failure domain", category, err)
					}
				}
				if err.File != file || err.EndColumn <= err.Column {
					t.Fatal("missing source span", err)
				}
			}
			if missing != 1 {
				t.Fatal("missing-payload identity changed", missing)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		completed, awaited := 0, 0
		ast.Visit(program, func(node any) bool {
			if await, ok := node.(*ast.AwaitExpression); ok {
				typ, ok := a.ResolvedTypeOf(await)
				if !ok || typ.Name != "TaskOutcome" {
					t.Fatal(typ, ok)
				}
				awaited++
			}
			if call, ok := node.(*ast.CallExpression); ok {
				typ, ok := a.ResolvedTypeOf(call)
				if ok && typ.Name == "TaskOutcome" && len(typ.TypeArgs) == 1 && typ.TypeArgs[0].Kind == ResultType {
					if len(typ.TypeArgs[0].TypeArgs) != 2 || typ.TypeArgs[0].TypeArgs[1].Name != "AppError" {
						t.Fatal("lost nested application error", typ)
					}
					completed++
				}
			}
			return true
		})
		if awaited != 2 || completed != 2 {
			t.Fatal(awaited, completed)
		}
		if TaskTypeExplanation(a.Types()["OtherOutcome"]) != "" {
			t.Fatal("classified ordinary union by variant spelling")
		}
	}
	definition, ok := diagnostics.Lookup(diagnostics.TaskFailureDomainMismatch)
	if !ok || !definition.Mandatory || definition.Name != "task.failure-domain-mismatch" {
		t.Fatal(definition, ok)
	}
}
