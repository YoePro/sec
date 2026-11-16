package sema

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestArenaCapacityPlans keeps initial byte backing/request evidence distinct
// from element counts and provider-delivered capacity, across moves and Reset.
// Empty, dynamic and unknown backing must remain three different states.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§12–20,39,76.
func TestArenaCapacityPlans(t *testing.T) {
	for _, file := range []string{"arena_capacity_plans.sec", "arena_capacity_large.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{0, 32, 64} {
			if file == "arena_capacity_large.sec" && width != 64 {
				continue
			}
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", file, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(data), file))
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
					for _, statement := range program.Statements {
						function, ok := statement.(*ast.FunctionDeclaration)
						if !ok {
							continue
						}
						for call, stored := range a.arenaOperationPlans {
							if stored.Source.Line < function.Token.Line || stored.Source.Line > function.EndToken.Line {
								continue
							}
							checked++
							plan, _ := a.ResolvedArenaOperationPlanOf(call)
							capacity := plan.Capacity
							name := function.Name.Value
							wantSource := ArenaCapacityBuffer
							if name == "Owned" || name == "Growing" || name == "Large" {
								wantSource = ArenaCapacityRequest
							} else if name == "Parameter" {
								wantSource = ArenaCapacityUnknown
							}
							if capacity.Source != wantSource || capacity.GrowthAllowed != (name == "Growing") || capacity.RetainsBackingOnReset != (name != "Parameter") {
								t.Fatal("backing policy lost", name, capacity)
							}
							if name == "Fixed" || name == "Empty" || name == "Sliced" {
								want := "32"
								if name == "Empty" {
									want = "0"
								} else if name == "Sliced" {
									want = "8"
								}
								if capacity.BackingBytes == nil || capacity.BackingBytes.String() != want || capacity.RequestedBytes != nil {
									t.Fatal("exact byte backing lost", capacity)
								}
								capacity.BackingBytes.SetInt64(999)
							} else if name == "Owned" || name == "Large" {
								want := "64"
								if name == "Large" {
									want = "9223372036854775808"
								}
								if capacity.RequestedBytes == nil || capacity.RequestedBytes.String() != want || capacity.BackingBytes != nil {
									t.Fatal("request narrowed or became provider extent", capacity)
								}
								capacity.RequestedBytes.SetInt64(999)
							} else if capacity.RequestedBytes != nil || capacity.BackingBytes != nil {
								t.Fatal("runtime/unknown capacity became constant", capacity)
							}
							if wantSource != ArenaCapacityUnknown && capacity.SourceToken.Line == 0 {
								t.Fatal("capacity source lost across move/reset", capacity)
							}
							again, _ := a.ResolvedArenaOperationPlanOf(call)
							if !reflect.DeepEqual(again, stored) {
								t.Fatal("capacity snapshot aliases stored plan")
							}
						}
					}
					if checked != len(a.arenaOperationPlans) || checked == 0 || identities != fmt.Sprintf("%#v", a.placeIdentities) {
						t.Fatal("missing coverage or query allocated identities", checked)
					}
				})
			}
		}
	}
}
