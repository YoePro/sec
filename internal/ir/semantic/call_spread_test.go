package semantic

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Constant indexed projections normalize only the source/slot plan Sema
// selected. Ordinary and zero-length sources retain their evaluation order.
// Rules: rules/declarations/spread.md §§3,4;
// rules/corrections/applied/semantic-ir-spread-correction-20260813.md.
func TestCallSpreadConsumesPreparedSources(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/spread_call_arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	var order []string
	var indices []string
	var arrays []ValueID
	for _, fn := range module.Functions {
		if fn.Name != "main" {
			continue
		}
		constants := map[ValueID]string{}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpConstInt {
					constants[op.Results[0].ID] = op.Integer.String()
				}
				if op.Kind == OpDirectCall {
					name := strings.Split(strings.Split(string(op.Callee), "::")[1], "(")[0]
					order = append(order, name)
				}
				if op.Kind == OpArrayExtract {
					// The two authored values[i] checks also have constant
					// projections; all projection bounds must remain proven.
					if op.ArrayCheckKind != ArrayIndexProvenSafe || op.ArrayProofKind != ArrayIndexProofConstant {
						t.Fatalf("lost projection proof: %+v", op)
					}
					indices = append(indices, constants[op.Operands[1]])
					arrays = append(arrays, op.Operands[0])
				}
			}
		}
	}
	want := []string{"Mark", "PairSource", "Mark", "Four", "PairSource", "PairSource", "Four", "EmptySource", "Mark", "One", "Consume", "Pick", "Generic[T=int]"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("call order %v, want %v", order, want)
	}
	if len(indices) < 6 || !reflect.DeepEqual(indices[:6], []string{"0", "1", "0", "1", "0", "1"}) {
		t.Fatalf("projection order: %v", indices)
	}
	if arrays[0] != arrays[1] || arrays[2] != arrays[3] || arrays[4] != arrays[5] || arrays[2] == arrays[4] {
		t.Fatalf("spread sources not captured exactly once per occurrence: %v", arrays)
	}
}

// Prepared type identity survives concrete generic substitution and aggregate
// projection on both target widths; the plans select no host byte layout.
// Rules: spread.md §§4.2–4.3; generics_lowering.md — concrete specializations.
func TestCallSpreadAggregateAndGenericShapes(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/spread_call_shapes.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), "spread-shapes.sec"))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", MaxPackage: 20})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Invalid or unavailable plans fail before partial source evaluation. Runtime
// homogeneous segments stay explicit capabilities; no static length is guessed.
// Rules: rules/declarations/spread.md §§4.1–4.3; functions.md §38A.
func TestCallSpreadPlanRejectsMissingAndReorderedSlots(t *testing.T) {
	// Reuse real Sema plans so the tests exercise their consumer boundary.
	data, err := os.ReadFile("../../../testdata/semantic_ir/spread_call_arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "spread-call.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	var target *ast.CallExpression
	walk.Inspect(parsed.Program, func(node any) error {
		if call, ok := node.(*ast.CallExpression); ok && call.Callee.String() == "Consume" {
			target = call
		}
		return nil
	})
	if target == nil {
		t.Fatal("missing source call")
	}
	for _, test := range []struct {
		name   string
		change func(*sema.ResolvedCallArgumentPlan)
	}{
		{"runtime", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded[0].RuntimeSpread = true }},
		{"missing source", func(p *sema.ResolvedCallArgumentPlan) { p.Sources[0].Expression = nil }},
		{"missing slot", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded = p.Expanded[:1] }},
		{"wrong source", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded[0].Source = 1 }},
		{"reordered", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded[0], p.Expanded[1] = p.Expanded[1], p.Expanded[0] }},
		{"element move", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded[0].Operation = sema.ArgumentMove }},
		{"unprepared", func(p *sema.ResolvedCallArgumentPlan) { p.Expanded[0].Prepared = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fact, known := a.ResolvedCallTarget(target)
			if !known || fact.Arguments == nil {
				t.Fatal("missing Sema plan")
			}
			test.change(fact.Arguments)
			if err := validateCallArgumentPlan(fact.Arguments, fact.Function.Parameters); err == nil {
				t.Fatal("invalid plan was admitted")
			}
		})
	}
}
