package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAtomicRealizationDoesNotChangeEligibility checks the separation through
// source analysis: missing/denied/native/emulated target facts neither remove
// uint64 from Atomic's type set nor admit float64, even on a 32-bit target.
// These tests do not claim concrete target realization or lowering is available.
// Rules: rules/concurrency/atomics.md — §§8–9,37–39;
// rules/platform/target_profiles.md — atomic capability separation.
func TestAtomicRealizationDoesNotChangeEligibility(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, implementation := range []layout.AtomicImplementation{
			{},
			{Kind: layout.AtomicNativeInstruction, Identity: "test-native"},
			{Kind: layout.AtomicEmulated, Identity: "test-emulated", LockFree: layout.AtomicPropertyYes, Blocking: layout.AtomicPropertyNo},
		} {
			for _, availability := range []layout.AtomicAvailability{layout.AtomicUnknown, layout.AtomicUnsupported, layout.AtomicSupported} {
				if (availability == layout.AtomicSupported) != (implementation.Kind != "") {
					continue
				}
				for _, fixture := range []struct {
					name, typeID string
					errors       int
				}{
					{"realization_valid.sec", "core::uint64", 0},
					{"realization_invalid.sec", "core::float64", 1},
				} {
					source, err := os.ReadFile("../../testdata/sema/atomics/" + fixture.name)
					if err != nil {
						t.Fatal(err)
					}
					p := parser.New(lexer.NewWithFile(string(source), fixture.name))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					request := layout.AtomicRequirement{TypeID: fixture.typeID, SizeBytes: 8, AlignmentBytes: 8, Operation: "Load", Order: "SeqCst", ContextID: "test::ordinary"}
					fact := layout.AtomicCapability{Requirement: request, Availability: availability, Provenance: "test-only-plan", Implementation: implementation}
					plan, err := (layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "hosted", PointerWidthBits: width, Endianness: layout.LittleEndian}).WithAtomicCapabilities([]layout.AtomicCapability{fact})
					if err != nil {
						t.Fatal(err)
					}
					a := NewAnalyzerWithScalarPlan(plan)
					if errors := a.Analyze(program); len(errors) != fixture.errors {
						t.Fatal(width, fixture.name, fact, errors)
					}
					if got, err := plan.AtomicCapability(request); err != nil || got != fact {
						t.Fatal(got, err)
					}
				}
			}
		}
	}
}
