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

// If conditions use the ordinary explicit unsafe null-test rules. Neither the
// condition nor a nested unsafe branch grants authority to surrounding code.
// Rules: rules/control-flow/flowcontrol_if.md — §23;
// rules/memory/raw_pointers.md — §7(3–9a); MD-024 correction — §5.
func TestIfRawNullUnsafeBoundary(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/control_flow/if_raw_null/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(valid), "if_raw_null.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	tests, negated := 0, 0
	ast.Visit(program, func(node any) bool {
		if test, ok := node.(*ast.NullTestExpression); ok {
			typ, found := a.ResolvedTypeOf(test)
			if !found || typ.Kind != BoolType {
				t.Fatalf("null test lost bool type: %v %t", typ, found)
			}
			if subject, found := a.ResolvedTypeOf(test.Subject); !found || subject.Kind != RawPtrType {
				t.Fatalf("null subject lost raw pointer type: %v %t", subject, found)
			}
			if test.Negated {
				negated++
			}
			tests++
		}
		return true
	})
	if tests != 7 || negated != 4 {
		t.Fatalf("null tests=%d, negated=%d, want 7 and 4", tests, negated)
	}
	invalid, err := os.ReadFile("../../testdata/control_flow/if_raw_null/contexts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errs = analyzeSourceWithAnalyzerRaw(t, string(invalid))
	counts := map[string]int{}
	unsafeRead := 0
	for _, e := range errs {
		counts[e.ID]++
		if strings.Contains(e.Message, "RawPtr.Read requires unsafe") {
			unsafeRead++
		}
	}
	if len(errs) != 10 || counts[diagnostics.NullOutsideUnsafe] != 4 || counts[diagnostics.NullTestRequiresRawPointer] != 2 || counts[diagnostics.NullEquality] != 2 || counts[diagnostics.RawPointerNullDereference] != 1 || unsafeRead != 1 {
		t.Fatalf("unexpected diagnostics: %v, counts=%v", errs, counts)
	}
}
