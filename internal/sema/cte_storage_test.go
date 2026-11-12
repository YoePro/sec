package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCTETransientStorageIsTargetIndependent establishes aggregate and string
// values with no target allocator available, across all configured widths.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,24–25,48–49.
func TestCTETransientStorageIsTargetIndependent(t *testing.T) {
	file := "../../testdata/sema/cte_transient_storage_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"hosted", "noalloc", "freestanding"} {
		for _, bits := range []uint16{32, 64} {
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: bits})
			a.SetAllocationProfile(profile)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(profile, bits, errors)
			}
			found := 0
			for _, static := range a.ResolvedStaticDeclarations() {
				value, ok := a.CompileTimeValueOf(static.Declaration.Value)
				if !ok {
					t.Fatal(static.Name, "missing compile-time value")
				}
				switch static.Name {
				case "Answer":
					if value.Value.Integer == nil || value.Value.Integer.String() != "42" {
						t.Fatal(value)
					}
					found++
				case "Message":
					if value.Value.String != "Sec 😀" {
						t.Fatal(value)
					}
					found++
				}
			}
			if found != 2 {
				t.Fatal(found)
			}
		}
	}
}

// TestCTETransientStringBudgets aborts before string construction can exhaust
// the host, distinguishes per-value and cumulative budgets, and keeps the
// compiler abort outside source try handlers and independent requests.
// Rules: rules/compiler/compile_time_evaluation.md — §§24,33,48–49.
func TestCTETransientStringBudgets(t *testing.T) {
	for _, mode := range []string{"strings_valid", "storage_invalid"} {
		file := "../../testdata/sema/cte_transient_" + mode + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if mode == "strings_valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
		} else {
			if len(errors) != 3 {
				t.Fatal(errors)
			}
			for index, category := range []string{"AggregateValueSize", "TransientMemory"} {
				if errors[index].ID != diagnostics.CompileTimeResourceLimit || !strings.Contains(errors[index].Message, category) {
					t.Fatal(errors)
				}
			}
			if errors[2].ID != diagnostics.CompileTimeForbiddenEffect || !strings.Contains(errors[2].Message, "explicit runtime allocator") {
				t.Fatal(errors)
			}
		}
		found := false
		for _, static := range a.ResolvedStaticDeclarations() {
			if static.Name != "Independent" {
				continue
			}
			value, ok := a.CompileTimeValueOf(static.Declaration.Value)
			want := "xxxxxxxx"
			if mode == "strings_valid" {
				want += want
			}
			if !ok || value.Value.String != want {
				t.Fatal(value, ok)
			}
			found = true
		}
		if !found {
			t.Fatal("independent request was not evaluated")
		}
	}
}

// TestCTEStorageReservationBoundaries proves failed reservations do not wrap
// or corrupt the request's accounting. Limits are compiler resource policy.
// Rules: rules/compiler/compile_time_evaluation.md — §§48–49.
func TestCTEStorageReservationBoundaries(t *testing.T) {
	for _, size := range []int{-1, int(^uint(0) >> 1), cteMaxTransientMemory + 1} {
		x := newCTEExecutor(NewAnalyzer(), nil)
		if x.reserveTransientStorage(size, lexer.Token{}) || x.transient != 0 || x.failure.Kind != cteResourceLimit {
			t.Fatal(size, x.transient, x.failure)
		}
	}
	x := newCTEExecutor(NewAnalyzer(), nil)
	if !x.reserveTransientStorage(cteMaxTransientMemory, lexer.Token{}) || x.reserveTransientStorage(1, lexer.Token{}) || x.transient != cteMaxTransientMemory {
		t.Fatal(x.transient, x.failure)
	}
	x = newCTEExecutor(NewAnalyzer(), nil)
	if !x.reserveStringStorage(cteMaxStringValueSize, 0, lexer.Token{}) || x.reserveStringStorage(cteMaxStringValueSize, 1, lexer.Token{}) || x.transient != cteMaxStringValueSize {
		t.Fatal(x.transient, x.failure)
	}
}
