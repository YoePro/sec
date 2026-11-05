package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// stringRangeFixture analyzes MD-126 source through the ordinary parser/Sema.
// Rules: rules/collections/collections.md — §7.10; rules/types/types.md — StringView.
func stringRangeFixture(t *testing.T, name string) (*Analyzer, []Error) {
	t.Helper()
	path := "../../testdata/sema/string_ranges/" + name + ".sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	return a, a.Analyze(program)
}

// MD-126 ranges are borrowed views in rune coordinates, with explicit guarded
// runtime plans and by-value scalar iteration; try uses RangeError.
// Rules: rules/collections/collections.md — §7.10; flowcontrol_for.md — §19;
// rules/errors/errorhandling.md — §9; rules/compiler/semantic_ir.md — §17(9–10).
func TestStringRangeFrontend(t *testing.T) {
	a, errors := stringRangeFixture(t, "valid")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(a.stringRanges) < 10 {
		t.Fatal("missing range plans")
	}
	safe := 0
	for expr, original := range a.stringRanges {
		plan, ok := a.ResolvedStringRangeOf(expr)
		if !ok || plan.Range.Mode != SliceShared || plan.ViewPlace.Type.Kind != StringViewType || plan.Range.ElementType.Kind != RuneType {
			t.Fatalf("invalid plan: %+v", plan)
		}
		if plan.Range.SourceLength != nil && plan.Range.SourceKind == "string" {
			if plan.Range.SourceLength.Int64() != 4 {
				t.Fatalf("bounds used bytes rather than runes: %+v", plan)
			}
			safe++
		}
		plan.ViewPlace.Root = "changed"
		if plan.Range.StaticRange != nil {
			plan.Range.StaticRange.Start.SetInt64(999)
		}
		again, _ := a.ResolvedStringRangeOf(expr)
		if again.ViewPlace.Root != original.ViewPlace.Root || again.Range.StaticRange != nil && again.Range.StaticRange.Start.Int64() == 999 {
			t.Fatal("query aliases state")
		}
	}
	if safe < 5 {
		t.Fatal("Unicode ranges lack proofs")
	}
	for expr, typ := range a.expressionTypes {
		if _, slice := expr.(*ast.SliceExpression); slice && typ.Kind != StringViewType {
			t.Fatal("string range became an array/string")
		}
	}
}

// Invalid ranges expose stable diagnostic identities/help rather than the old
// anonymous cannot-be-sliced error; byte length must not admit rune index 4.
// Rules: rules/collections/collections.md — §§7.5,7.10; diagnostics.md — §5.
func TestStringRangeDiagnostics(t *testing.T) {
	_, errors := stringRangeFixture(t, "range_invalid")
	if len(errors) != 6 {
		t.Fatal(errors)
	}
	for i, e := range errors {
		want := diagnostics.InvalidSliceRange
		if i == 5 {
			want = diagnostics.InvalidRangeSource
		}
		if e.ID != want || e.Help == "" {
			t.Fatalf("missing identity/help: %+v", e)
		}
	}
}

// Borrowed views may not escape local/owned input storage, materialize implicitly,
// expose mutable text/rune references or remain live across source replacement.
// Rules: rules/memory/borrowing.md — §5.5; references.md — §3.3(5–6);
// rules/control-flow/flowcontrol_for.md — §19 (MD-126).
func TestStringRangeLifetimeAndModes(t *testing.T) {
	_, errors := stringRangeFixture(t, "lifetime_invalid")
	if len(errors) != 10 {
		t.Fatalf("missing rejection: %v", errors)
	}
	for _, e := range errors {
		t.Log(e)
	}
}

// Reader call mistakes keep their call diagnostic and do not masquerade as
// an invalid bracket interval.
// Rules: rules/library/core-library.md — §5.8; rules/tooling/diagnostics.md — §5.
func TestStringRangeMemberDiagnostics(t *testing.T) {
	_, errors := stringRangeFixture(t, "member_invalid")
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.InvalidStringViewCall || e.Help == "" {
			t.Fatal(e)
		}
	}
}
