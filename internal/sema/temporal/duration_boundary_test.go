package temporal_test

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestDurationCallBoundariesHaveNoUnitSpellingShortcut verifies that the same
// canonical duration parameter behaves identically for mutex-like, context-like
// and ordinary names, including user-defined member calls. Familiar unit names
// with a non-time dimension must never become a hidden timeout overload.
// Rules: rules/concurrency/mutex.md — §12(2)-(6);
// rules/types/temporal.md — §1 (nominal temporal identities);
// rules/types/units.md — Dimensions; MD-015 (conversion remains undecided).
func TestDurationCallBoundariesHaveNoUnitSpellingShortcut(t *testing.T) {
	template := fixture(t, "duration_call_boundary.sec.in")
	for _, symbol := range []string{"s", "ms", "us", "ns", "beat", "arbitraryTick"} {
		for _, axis := range []string{"time", "length"} {
			for _, scale := range []string{"1", "1 / 1000"} {
				for _, kind := range []string{"duration", "quantity", "plain_integer"} {
					t.Run(symbol+"/"+axis+"/"+scale+"/"+kind, func(t *testing.T) {
						typ := "int<" + symbol + ">"
						if kind == "duration" {
							typ = "duration"
						}
						if kind == "plain_integer" {
							typ = "int"
						}
						source := strings.NewReplacer("__UNIT__", symbol, "__AXIS__", axis, "__SCALE__", scale, "__TYPE__", typ).Replace(template)
						parsed := parser.New(lexer.NewWithFile(source, "duration_boundary.sec")).Parse()
						if parsed.HasErrors {
							t.Fatal(parsed.Diagnostics)
						}
						analyzer := sema.NewAnalyzer()
						errors := analyzer.Analyze(parsed.Program)
						if kind == "duration" {
							if len(errors) > 0 {
								t.Fatal(errors)
							}
							return
						}
						if len(errors) != 4 {
							t.Fatalf("want four nominal argument errors, got %+v", errors)
						}
						for _, err := range errors {
							if !strings.Contains(err.Message, "must be duration") || err.File != "duration_boundary.sec" {
								t.Fatalf("unexpected rejection: %+v", err)
							}
						}
						// Prove that all metadata was accepted before checking the call boundary;
						// a parser/unit-metadata failure must not masquerade as timeout rejection.
						parameter := analyzer.Functions()["Check"][0].Parameters[1].Type
						if kind == "quantity" {
							expectedScale, ok := new(big.Rat).SetString(strings.ReplaceAll(scale, " ", ""))
							if !ok || parameter.Dimension.Base[axis] != 1 || parameter.UnitSemantics.Scale == nil || parameter.UnitSemantics.Scale.Cmp(expectedScale) != 0 {
								t.Fatalf("quantity facts missing: %+v", parameter)
							}
						}
					})
				}
			}
		}
	}
}

// TestRepresentedMutexCallsDoNotInventUnitOverloads closes the actual intrinsic
// dispatch path: its represented zero-argument forms reject all timeout inputs
// equally until the owning mutex integration implements canonical overloads.
// Rules: rules/concurrency/mutex.md — §12(2)-(6);
// correction mutex-v2-cross-rulebook-correction-20260907.md — §§6,7.
func TestRepresentedMutexCallsDoNotInventUnitOverloads(t *testing.T) {
	template := fixture(t, "duration_mutex_boundary.sec.in")
	for _, symbol := range []string{"s", "ms", "us", "ns", "beat", "arbitraryTick"} {
		source := strings.NewReplacer("__UNIT__", symbol, "__AXIS__", "time", "__SCALE__", "1").Replace(template)
		parsed := parser.New(lexer.NewWithFile(source, "mutex_boundary.sec")).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		errors := sema.NewAnalyzer().Analyze(parsed.Program)
		if len(errors) != 2 {
			t.Fatal(errors)
		}
		for _, err := range errors {
			if !strings.Contains(err.Message, "expects 0 arguments, got 1") {
				t.Fatal(err)
			}
		}
	}
}
