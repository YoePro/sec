package main

import (
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// All actual core time units identify Time and permit exact bidirectional
// conversion to seconds. Tiny units use uint128 values to prove exact integer
// conversion without requiring an unrepresentable decimal literal. Existing
// average month/year scales are checked as declared, not calendar arithmetic.
// Rules: rules/types/units.md — "Kind", "Exact fixed conversions";
// rules/corrections/applied/md015-time-unit-duration-conversion-correction-20261013.md §3;
// rules/library/core-library.md §27.1 (UnitKind.Time).
func TestCoreTimeUnitsHaveKindAndExactConversions(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/core_time_units_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	functions := map[string]*ast.FunctionDeclaration{}
	for _, statement := range program.Statements {
		if fn, ok := statement.(*ast.FunctionDeclaration); ok {
			functions[strings.TrimPrefix(fn.Name.Value, "Convert_")] = fn
		}
	}
	resolveCoreLibrary(program)
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(program); len(errors) != 0 {
		t.Fatalf("actual core catalog conversions rejected: %v", errors)
	}
	cases := []struct{ unit, original, seconds string }{
		{"s", "1", "1"}, {"min", "1", "60"}, {"h", "1", "3600"}, {"day", "1", "86400"}, {"week", "1", "604800"},
		{"ms", "1000", "1"}, {"us", "1000000", "1"}, {"ns", "1000000000", "1"},
		{"ps", "1000000000000", "1"}, {"fs", "1000000000000000", "1"}, {"as", "1000000000000000000", "1"},
		{"zs", "1000000000000000000000", "1"}, {"ys", "1000000000000000000000000", "1"},
		{"mo", "1", "2628000"}, {"yr", "1", "31540000"}, {"decade", "1", "315400000"},
		{"century", "1", "3154000000"}, {"millennia", "1", "31540000000"},
	}
	for _, tc := range cases {
		t.Run(tc.unit, func(t *testing.T) {
			fn := functions[tc.unit]
			if fn == nil {
				t.Fatal("missing source case")
			}
			kindReads := 0
			ast.Visit(fn, func(node any) bool {
				if member, ok := node.(*ast.MemberExpression); ok {
					if read, ok := analyzer.ResolvedUnitMetadataReadOf(member); ok && read.Member == "Kind" {
						kindReads++
						if read.Enum.Name != "Time" || read.Enum.EnumType.Name != "UnitKind" {
							t.Fatalf("wrong concrete Kind: %+v", read)
						}
					}
				}
				return true
			})
			if kindReads != 1 {
				t.Fatalf("resolved Kind reads = %d, want 1", kindReads)
			}
			if tc.unit == "s" {
				return
			}
			for _, boundary := range []struct {
				statement int
				want      string
			}{{1, tc.seconds}, {3, tc.original}} {
				value := fn.Body.Statements[boundary.statement].(*ast.LetStatement).Value
				plan, ok := analyzer.UnitConversionPlanOf(value)
				if !ok {
					t.Fatal("missing exact conversion plan", boundary.statement)
				}
				want, _ := new(big.Int).SetString(boundary.want, 10)
				if plan.Minimum == nil || plan.Maximum == nil || plan.Minimum.Cmp(want) != 0 || plan.Maximum.Cmp(want) != 0 || plan.Source.Kind != "UnitKind.Time" || plan.Target.Kind != "UnitKind.Time" {
					t.Fatalf("conversion %d has wrong Kind/range: %+v", boundary.statement, plan)
				}
			}
		})
	}
}

// Declaring core Time kinds must not make fractional/overflowing integer
// conversions valid or infer Time for a user unit merely from its dimension.
// Rules: rules/types/units.md — "Known and unknown Kind", "No hidden precision loss".
func TestCoreTimeUnitConversionsRetainSafetyChecks(t *testing.T) {
	for _, file := range []string{"core_time_inexact_invalid.sec", "core_time_unknown_kind_invalid.sec"} {
		t.Run(file, func(t *testing.T) {
			_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+file)
			if code == 0 || !strings.Contains(stderr, "must return") {
				t.Fatalf("check exit=%d:\n%s", code, stderr)
			}
		})
	}
}
