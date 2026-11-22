package sema

import (
	"fmt"
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestThreadCurrentTargetCapability preserves physical identity, denies known
// unsupported plans and leaves logical-task requirements independent. Reanalysis
// and immutable queries retain the exact selected CompilationPlan evidence.
// Rules: rules/concurrency/cancellation.md §§8–11,24; rules/platform/platform_model.md §23.
func TestThreadCurrentTargetCapability(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/physical_thread/current.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, width := range []uint16{32, 64} {
			for _, state := range []layout.PhysicalThreadAvailability{layout.PhysicalThreadUnknown, layout.PhysicalThreadSupported, layout.PhysicalThreadUnsupported} {
				t.Run(fmt.Sprintf("%s/%d/%s", depth, width, state), func(t *testing.T) {
					base := layout.ResolvedScalarPlan{TargetOS: "test-os", TargetArch: "test-arch", Profile: "selected-profile", PointerWidthBits: width, Endianness: layout.LittleEndian}
					plan, err := base.WithPhysicalThreadCapability(layout.PhysicalThreadCapability{Availability: state, Provenance: "target contract", Reason: "physical identity unavailable"})
					if err != nil {
						t.Fatal(err)
					}
					p := parser.New(lexer.NewWithFile(string(source), "current.sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(plan, depth)
					for run := 0; run < 2; run++ {
						errs := a.Analyze(program)
						if state == layout.PhysicalThreadUnsupported {
							if len(errs) != 1 || errs[0].ID != diagnostics.PhysicalThreadCapability || errs[0].Line != 4 || errs[0].Help == "" || !strings.Contains(errs[0].Message, "selected-profile") || errs[0].IsImplementationCapability() {
								t.Fatal(errs)
							}
							if len(a.resolvedThreadCurrentCapabilities) != 0 {
								t.Fatal("denied physical identity published successful facts")
							}
							continue
						}
						if len(errs) != 0 {
							t.Fatal(errs)
						}
						seen := 0
						_ = walk.Inspect(program, func(node any) error {
							call, ok := node.(*ast.CallExpression)
							if !ok {
								return nil
							}
							fact, ok := a.ResolvedThreadCurrentCapabilityOf(call)
							if !ok {
								return nil
							}
							seen++
							typ, ok := a.ResolvedTypeOf(call)
							if !ok || typ.Name != "ThreadContext" {
								t.Fatal("physical context was replaced", typ)
							}
							if fact.Capability.Availability != state || fact.PlanFingerprint != plan.Fingerprints().Plan || fact.Profile != "selected-profile" {
								t.Fatal(fact)
							}
							fact.Capability.Availability = "mutated"
							again, _ := a.ResolvedThreadCurrentCapabilityOf(call)
							if again.Capability.Availability != state {
								t.Fatal("query mutates stored evidence")
							}
							return nil
						})
						if seen != 1 {
							t.Fatal("logical context was classified as physical", seen)
						}
					}
				})
			}
		}
	}
}

// TestThreadCurrentRejectsForeignCapabilitySnapshot rejects even explicit Unknown
// evidence copied into a different profile before publishing any call facts.
// Rules: rules/platform/target_profiles.md §31; rules/concurrency/cancellation.md §24.
func TestThreadCurrentRejectsForeignCapabilitySnapshot(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/physical_thread/current.sec")
	if err != nil {
		t.Fatal(err)
	}
	base := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "first", PointerWidthBits: 64, Endianness: layout.LittleEndian}
	for _, state := range []layout.PhysicalThreadAvailability{layout.PhysicalThreadUnknown, layout.PhysicalThreadSupported, layout.PhysicalThreadUnsupported} {
		plan, err := base.WithPhysicalThreadCapability(layout.PhysicalThreadCapability{Availability: state, Provenance: "test contract", Reason: "physical identity unavailable"})
		if err != nil {
			t.Fatal(err)
		}
		plan.Profile = "second"
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(plan)
		errs := a.Analyze(program)
		if len(errs) != 1 || errs[0].ID != diagnostics.PhysicalThreadCapability || !strings.Contains(errs[0].Message, "different resolved target/profile plan") || len(a.resolvedThreadCurrentCapabilities) != 0 {
			t.Fatal(errs)
		}
	}
}
