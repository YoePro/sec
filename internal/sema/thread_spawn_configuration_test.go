package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestNamedThreadConfiguration uses ordinary consuming-argument transfer facts
// for whole and projected named configurations, with ordinary spawn controls.
// Invalid source preparation restores both config and child-argument state.
// Rules: rules/concurrency/threads.md — §§10(3–4),20;
// rules/concurrency/spawn.md — §4(1),(10).
func TestNamedThreadConfiguration(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/thread_named_config_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if mode == "invalid" {
			if len(errors) != 7 {
				t.Fatalf("invalid (%d): %v", len(errors), errors)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		count, partial := 0, 0
		ast.Visit(program, func(node any) bool {
			spawn, ok := node.(*ast.SpawnExpression)
			if !ok || spawn.Configuration == nil {
				return true
			}
			count++
			plan, ok := a.ResolvedTransferPlanOf(explicitMoveSource(spawn.Configuration))
			if !ok || plan.Action != TransferMove || plan.Site != TransferSiteArgument || plan.OwnershipAfter != TransferMoved {
				t.Fatal("missing config move", plan, ok)
			}
			if plan.Partial {
				partial++
			}
			if typ, ok := a.ResolvedTypeOf(spawn); !ok || typ.Name != "Thread" {
				t.Fatal(typ, ok)
			}
			return true
		})
		if count != 3 || partial != 1 {
			t.Fatal(count, partial)
		}
	}
}
