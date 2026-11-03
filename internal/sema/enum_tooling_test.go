package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Tooling queries retain accepted conversion results and detached proof data;
// user calls returning enums and invalid constants cannot masquerade as casts.
// Rules: rules/declarations/enums.md — §§9–10,12,18;
// rules/compiler/compiler_analysis.md — immutable analysis facts.
func TestEnumConversionToolingFacts(t *testing.T) {
	for _, name := range []string{"valid", "conversion_invalid"} {
		data, err := os.ReadFile("../../testdata/tooling/enum_domains/" + name + ".sec")
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
		if name == "valid" && len(errors) != 0 || name != "valid" && len(errors) != 3 {
			t.Fatal(errors)
		}
		conversions, checked, proven := 0, 0, 0
		_ = walk.Inspect(program, func(node any) error {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return nil
			}
			info, found := a.EnumConversionInfoOf(call)
			if !found {
				return nil
			}
			if name != "valid" {
				t.Fatalf("invalid conversion has facts: %+v", info)
			}
			if info.Enum.Kind != EnumType {
				t.Fatal(info)
			}
			conversions++
			if info.Checked {
				checked++
			}
			if info.Proof.Proven() {
				proven++
			}
			before, _ := a.EnumConversionInfoOf(call)
			info.Enum.EnumValues[0] = "Changed"
			for key, value := range info.Enum.EnumConsts {
				if value.Value != nil {
					value.Value.SetInt64(999)
				}
				if value.StringValue != nil {
					*value.StringValue = "changed"
				}
				delete(info.Enum.EnumConsts, key)
			}
			if info.Proof.SourceMin != nil {
				info.Proof.SourceMin.SetInt64(999)
			}
			if len(info.Result.TypeArgs) > 0 {
				info.Result.TypeArgs[0].Name = "Changed"
			}
			after, _ := a.EnumConversionInfoOf(call)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("tooling query aliases analysis data: %+v / %+v", before, after)
			}
			return nil
		})
		if name == "valid" && (conversions != 12 || checked != 5 || proven != 3) {
			t.Fatalf("conversions=%d checked=%d proven=%d", conversions, checked, proven)
		}
	}
}

// Unresolved hardware widths remain open, so a tooling caller never infers a
// closed domain from an incomplete named case list. Non-enums have no domain.
// Rules: rules/declarations/enums.md — §§9–10,18.
func TestEnumDomainToolingFacts(t *testing.T) {
	for _, test := range []struct {
		typ         Type
		known, open bool
	}{
		{Type{Kind: EnumType, Underlying: "uint16"}, true, false},
		{Type{Kind: EnumType, Underlying: "string"}, true, false},
		{Type{Kind: EnumType, Underlying: "bit[8]", BitWidth: 8}, true, true},
		{Type{Kind: EnumType, Underlying: "bit[N]"}, true, true},
		{Type{Kind: IntType, Underlying: "int"}, false, false},
		{Type{Kind: EnumType}, false, false},
	} {
		info, known := EnumDomainOf(test.typ)
		if known != test.known || info.Open != test.open {
			t.Fatalf("%+v -> %+v, %v", test.typ, info, known)
		}
	}
}

// A compiler-known method taking an enum and returning an integer is still a
// method call, even when it has no ordinary ResolvedCallTarget. Presentation
// requires the actual conversion producer's identity, not type-shape inference.
// Rules: rules/declarations/enums.md — §12; rules/concurrency/atomics.md — §§24,43;
// rules/tooling/lsp.md — "One semantic source of truth", "Unknown facts".
func TestEnumConversionToolingRejectsIntrinsicCalls(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomic_orders_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	loads := 0
	_ = walk.Inspect(program, func(node any) error {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		if info, found := a.EnumConversionInfoOf(call); found {
			t.Fatalf("intrinsic call mistaken for cast: %s %+v", call.String(), info)
		}
		member, ok := call.Callee.(*ast.MemberExpression)
		if ok && member.Property != nil && member.Property.Value == "Load" {
			loads++
		}
		return nil
	})
	if loads != 7 {
		t.Fatalf("expected seven actual atomic Load calls, got %d", loads)
	}
}
