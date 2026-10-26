package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// A structurally returning suffix excludes the outer continuation, while
// publishing separate holder ends for the return and continuation paths.
// Rules: rules/memory/borrowing.md — §8.1(9)–(14);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§2–4,6.
func TestSliceEarlyReturnLifetimeSchedule(t *testing.T) {
	source, err := os.ReadFile("../../testdata/semantic_ir/slice_branch_ends.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(source), "slice_branch_ends.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		checked := 0
		for _, stmt := range parsed.Program.Statements {
			fn, ok := stmt.(*ast.FunctionDeclaration)
			if !ok || (fn.Name.Value != "Early" && fn.Name.Value != "NestedEarly") {
				continue
			}
			view := fn.Body.Statements[1].(*ast.LetStatement)
			binding, ok := a.ResolvedBindingOf(view.Name)
			if !ok {
				t.Fatal("missing holder identity")
			}
			ends, nested := 0, 0
			for _, end := range a.ReferenceLifetimeEnds() {
				if end.Binding != binding.ID || end.Before.Line == 0 {
					continue
				}
				ends++
				if end.EndDepth > end.HolderDepth {
					nested++
				}
			}
			if ends != 2 || nested != 1 {
				t.Fatalf("%s: ends=%d nested=%d", fn.Name.Value, ends, nested)
			}
			checked++
		}
		if checked != 2 {
			t.Fatal("missing early-return fixture functions")
		}
	}
}

// Returning a holder, deferred uses and continue/backedge paths still require
// authority. A possible return on another path cannot release those borrows.
// Rules: rules/memory/borrowing.md — §8.1(7),(9)–(14), §17(4);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§2(4),3(3)–(5),4(1).
func TestSliceEarlyReturnRetainsDependencies(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/slice_early_return_liveness_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 3 {
		t.Fatalf("expected three live-dependency conflicts: %v", errors)
	}
	for _, err := range errors {
		if !strings.Contains(err.Message, "shared borrowed") {
			t.Fatalf("lost authority restriction: %v", errors)
		}
	}
}
