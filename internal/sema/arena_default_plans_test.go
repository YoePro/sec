package sema

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Nonzero explicit defaults and aggregate field/array defaults must survive
// source analysis without becoming zero-fill assumptions or mutable aliases.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§31,73,76;
// rules/types/default_values.md — §§7,18,28.
func TestArenaElementDefaultPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/arena_default_plans.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{0, 32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(data), "arena_default_plans.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				identities := fmt.Sprintf("%#v", a.placeIdentities)
				checked := 0
				for call, stored := range a.arenaOperationPlans {
					plan, _ := a.ResolvedArenaOperationPlanOf(call)
					if plan.Operation != ArenaOperationNew && plan.Operation != ArenaOperationAlloc {
						if plan.ElementDefault != nil {
							t.Fatal("nonallocation acquired an element default")
						}
						continue
					}
					checked++
					value := plan.ElementDefault
					if value == nil || !plan.ElementDefaultable {
						t.Fatal("missing resolved element default")
					}
					if plan.ElementType.Name == "Port" {
						if value.Kind != ExplicitTypeDefault || value.Value.Integer == nil || value.Value.Integer.Int64() != 8080 {
							t.Fatal("nonzero explicit default lost", value)
						}
						value.Value.Integer.SetInt64(0)
					} else {
						if value.Kind != StructDefault || len(value.Fields) != 3 || value.Fields[0].Value.Value.Integer.Int64() != 8080 {
							t.Fatal("struct default lost", value)
						}
						array := &value.Fields[2].Value
						if array.Kind != ArrayDefault || array.ArrayLengthDecimal != "3" || array.ArrayElementDefault == nil {
							t.Fatal("compact fixed-array default lost", array)
						}
						value.Fields[0].Value.Value.Integer.SetInt64(0)
						array.ArrayElementDefault.Kind = NoDefault
						value.Fields[0].Name = "corrupt"
					}
					value.Kind = NoDefault
					again, _ := a.ResolvedArenaOperationPlanOf(call)
					if !reflect.DeepEqual(again, stored) {
						t.Fatal("query mutation changed stored default")
					}
				}
				if checked != 3 || identities != fmt.Sprintf("%#v", a.placeIdentities) {
					t.Fatal("missing allocation plans or query allocated identities", checked)
				}
			})
		}
	}
}

// Exact explicit CTE defaults can contain mutable nested payloads; detachment
// must cover those values as well as structural defaults from the source test.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §76;
// rules/types/default_values.md — §§7,18.
func TestArenaDefaultConstantSnapshot(t *testing.T) {
	original := DefaultResolution{Kind: ExplicitTypeDefault, Value: DefaultConstant{
		Exact: big.NewRat(1, 3),
		Aggregate: &CompileTimeAggregate{
			Names:    []string{"value"},
			Fields:   map[string]DefaultConstant{"value": {Integer: big.NewInt(7)}},
			Payload:  &DefaultConstant{Integer: big.NewInt(9)},
			Elements: []DefaultConstant{{Integer: big.NewInt(11)}},
		},
	}}
	snapshot := snapshotArenaElementDefault(original)
	snapshot.Value.Exact.SetInt64(0)
	snapshot.Value.Aggregate.Names[0] = "corrupt"
	snapshot.Value.Aggregate.Fields["value"].Integer.SetInt64(0)
	snapshot.Value.Aggregate.Fields["other"] = DefaultConstant{}
	snapshot.Value.Aggregate.Payload.Integer.SetInt64(0)
	snapshot.Value.Aggregate.Elements[0].Integer.SetInt64(0)
	if original.Value.Exact.Cmp(big.NewRat(1, 3)) != 0 || original.Value.Aggregate.Names[0] != "value" ||
		original.Value.Aggregate.Fields["value"].Integer.Int64() != 7 || len(original.Value.Aggregate.Fields) != 1 ||
		original.Value.Aggregate.Payload.Integer.Int64() != 9 || original.Value.Aggregate.Elements[0].Integer.Int64() != 11 {
		t.Fatal("snapshot aliases original exact/aggregate default")
	}
}
