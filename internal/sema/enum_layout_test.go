package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// Source enums retain exact resolved aliases, values and underlying widths;
// native-size enums and enclosing structs use the selected target's facts.
// Rules: rules/memory/layout.md — §§14,47; rules/declarations/enums.md — §14.1.
func TestSourceIntegerEnumLayouts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_layout.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "armv7"} {
		t.Run(arch, func(t *testing.T) {
			definition, _ := target.Find(target.Target{OS: "linux", Arch: arch})
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), "enum_layout.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(plan)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			r, err := definition.LayoutResolver()
			if err != nil {
				t.Fatal(err)
			}
			req := LayoutRequirementOf(a.types["Status"])
			status := r.Resolve(req)
			if !status.Complete() || status.SizeBytes.Int64() != int64(plan.PointerWidthBits/8) || len(status.Variants) != 3 || status.Variants[2].Tag.Int64() != 9 || status.ValidBitPattern != "closed enum values {-2,9}" {
				t.Fatalf("status %+v", status)
			}
			req.EnumMembers[0].Value.SetInt64(100)
			if a.types["Status"].EnumConsts["Low"].Value.Int64() != -2 {
				t.Fatal("requirement changed Sema enum")
			}
			wide := r.Resolve(LayoutRequirementOf(a.types["Wide"]))
			if !wide.Complete() || wide.SizeBytes.Int64() != 32 || !strings.Contains(wide.ValidBitPattern, "340282366920938463463374607431768211456") {
				t.Fatalf("wide %+v", wide)
			}
			holder := r.Resolve(LayoutRequirementOf(a.types["Holder"]))
			if !holder.Complete() || holder.Fields[1].Offset.Int64() != int64(plan.PointerWidthBits/8) {
				t.Fatalf("holder %+v", holder)
			}
			allocations := 0
			for _, operation := range a.arenaOperationPlans {
				if operation.Operation != ArenaOperationNew {
					continue
				}
				allocations++
				if operation.ElementLayout == nil || operation.ElementLayout.SizeBytes.Cmp(status.SizeBytes) != 0 || operation.ElementLayout.AlignBytes.Cmp(status.AlignBytes) != 0 {
					t.Fatalf("Arena did not consume the shared enum layout: %+v", operation)
				}
			}
			if allocations != 1 {
				t.Fatalf("got %d enum allocations", allocations)
			}
			for _, name := range []string{"Flag", "Text"} {
				blocked := r.Resolve(LayoutRequirementOf(a.types[name]))
				if blocked.Completeness != layout.LayoutUnsupported || blocked.SizeBytes != nil {
					t.Fatalf("invented %s layout: %+v", name, blocked)
				}
			}
		})
	}
}
