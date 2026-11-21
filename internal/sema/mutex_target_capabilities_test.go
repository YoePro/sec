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

// TestMutexTargetCapabilities consumes frozen CompilationPlan contracts for
// each acquisition while preserving source result/error types and Unknown.
// Rules: rules/concurrency/mutex.md §§20–27,60,63; rules/platform/platform_model.md §23.
func TestMutexTargetCapabilities(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/mutex_target/acquisitions.sec")
	if err != nil {
		t.Fatal(err)
	}
	modes := []layout.MutexAcquisition{layout.MutexBlocking, layout.MutexTry, layout.MutexTimeout, layout.MutexContext}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, width := range []uint16{32, 64} {
			for _, availability := range []layout.MutexAvailability{layout.MutexUnknown, layout.MutexSupported, layout.MutexUnsupported} {
				t.Run(fmt.Sprintf("%s/%d/%s", depth, width, availability), func(t *testing.T) {
					base := layout.ResolvedScalarPlan{TargetOS: "test-os", TargetArch: "test-arch", Profile: "test-profile", PointerWidthBits: width, Endianness: layout.LittleEndian}
					var facts []layout.MutexCapability
					for _, mode := range modes {
						facts = append(facts, layout.MutexCapability{Acquisition: mode, Availability: availability, Provenance: "verified test contract", Reason: "no conforming acquisition"})
					}
					plan, err := base.WithMutexCapabilities(facts)
					if err != nil {
						t.Fatal(err)
					}
					p := parser.New(lexer.NewWithFile(string(source), "mutex-target.sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(plan, depth)
					for run := 0; run < 2; run++ {
						errs := a.Analyze(program)
						if availability == layout.MutexUnsupported {
							if len(errs) != 4 {
								t.Fatal(errs)
							}
							for _, err := range errs {
								if err.ID != diagnostics.MutexTargetCapability || err.Help == "" || !strings.Contains(err.Message, "test-profile") || err.IsImplementationCapability() {
									t.Fatal(err)
								}
							}
							if len(a.resolvedMutexTargetCapabilities) != 0 {
								t.Fatal("denied acquisition published positive facts")
							}
							continue
						}
						if len(errs) != 0 {
							t.Fatal(errs)
						}
						seen := map[layout.MutexAcquisition]bool{}
						_ = walk.Inspect(program, func(node any) error {
							call, ok := node.(*ast.CallExpression)
							if !ok {
								return nil
							}
							fact, ok := a.ResolvedMutexTargetCapabilityOf(call)
							if !ok {
								return nil
							}
							seen[fact.Capability.Acquisition] = true
							if fact.Capability.Availability != availability || fact.Profile != "test-profile" || fact.PlanFingerprint != plan.Fingerprints().Plan {
								t.Fatal("lost selected plan", fact)
							}
							original := fact.Capability.Acquisition
							fact.Capability.Acquisition = "changed"
							again, _ := a.ResolvedMutexTargetCapabilityOf(call)
							if again.Capability.Acquisition != original {
								t.Fatal("query mutates analyzer")
							}
							typ, ok := a.ResolvedTypeOf(call)
							if !ok {
								t.Fatal("lost acquisition result")
							}
							switch original {
							case layout.MutexBlocking:
								if !isMutexGuardType(typ) {
									t.Fatal(typ)
								}
							case layout.MutexTry, layout.MutexTimeout:
								if typ.Name != "Option" {
									t.Fatal(typ)
								}
							case layout.MutexContext:
								if typ.Kind != ResultType || typ.TypeArgs[1].Name != "ContextError" {
									t.Fatal(typ)
								}
							}
							return nil
						})
						if len(seen) != 4 {
							t.Fatal("missing acquisition modes", seen)
						}
					}
				})
			}
		}
	}
}
