package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAtomicOrderMatrix checks every object-operation/order pair, all 25
// CompareExchange order pairs, defaults, nominal order typing and local aliases
// through source analysis on both scalar plans, before target lowering.
// Rules: rules/concurrency/atomics.md — §§12–25,45(5–8).
func TestAtomicOrderMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"atomic_orders_valid.sec", 0},
			{"atomic_orders_invalid.sec", 43},
		} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian})
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatalf("%d %s: %d errors: %v", width, test.file, len(errors), errors)
			}
			invalid, wrongType, unresolved, runtime, execution := 0, 0, 0, 0, 0
			for _, diagnostic := range errors {
				switch {
				case diagnostic.ID == diagnostics.AtomicOrderNotCompileTime:
					runtime++
				case diagnostic.ID == diagnostics.SemanticCompileTimeExecutionUnavailable:
					execution++
				case strings.Contains(diagnostic.Message, "is not valid for"):
					if diagnostic.ID != diagnostics.AtomicInvalidMemoryOrder || diagnostic.Help == "" {
						t.Fatal(diagnostic)
					}
					invalid++
				case strings.Contains(diagnostic.Message, "must be MemoryOrder"):
					if diagnostic.ID != diagnostics.AtomicInvalidMemoryOrder {
						t.Fatal(diagnostic)
					}
					wrongType++
				default:
					unresolved++
				}
			}
			if test.errors != 0 && (invalid != 17 || wrongType != 10 || unresolved != 2 || runtime != 12 || execution != 2) {
				t.Fatal(errors)
			}
		}
	}
}

// TestAtomicFenceOrderRow keeps the fence row of the shared normative matrix
// exact while the separately tracked fence source/IR/lowering pipeline awaits
// integration. This test makes no claim about fence execution support.
// Rules: rules/concurrency/atomics.md — §§24(1),26(4–6).
func TestAtomicFenceOrderRow(t *testing.T) {
	want := []string{"Acquire", "Release", "AcqRel", "SeqCst"}
	if got := atomicOrderNames("Fence", false); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}
