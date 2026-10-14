package sema

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestNoAllocTargetRoots verifies root policy with transitive unknown behavior,
// recursive free code and an uncalled allocator, independent of depth/profile.
// Rules: rules/memory/allocation.md — §§6(8),22,24(6),28(4).
func TestNoAllocTargetRoots(t *testing.T) {
	for _, suffix := range []string{"valid", "invalid"} {
		path := "../../testdata/sema/allocation_noalloc_roots_valid.sec"
		if suffix == "invalid" {
			path = "../../testdata/sema/allocation_noalloc_roots.sec"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			for _, profile := range []string{"hosted", "noalloc", "freestanding"} {
				p := parser.New(lexer.NewWithFile(string(data), path))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithDepth(depth)
				a.SetAllocationProfile(profile)
				errors := a.Analyze(program)
				want := suffix == "invalid" && profile == "noalloc"
				if !want {
					if len(errors) != 0 {
						t.Fatal(suffix, depth, profile, errors)
					}
					continue
				}
				if len(errors) != 1 || errors[0].ID != diagnostics.NoAllocViolation || errors[0].ProofState != diagnostics.ProofUnproven || !strings.Contains(errors[0].Message, "main -> Forward -> Leaf") || len(errors[0].RelatedLocations()) == 0 {
					t.Fatal(errors)
				}
			}
		}
	}
}

// TestNoAllocTargetExecutionRelations keeps worker and deferred allocation in
// the root obligation and preserves a complete navigable canonical cause.
// Rules: rules/memory/allocation.md — §§6(8),24,28(4),29(4).
func TestNoAllocTargetExecutionRelations(t *testing.T) {
	for _, name := range []string{"allocate", "spawn", "defer"} {
		path := "../../testdata/sema/allocation_noalloc_" + name + ".sec"
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			a.SetAllocationProfile("noalloc")
			errors := a.Analyze(program)
			if len(errors) != 1 || errors[0].ID != diagnostics.NoAllocViolation || errors[0].ProofState != diagnostics.ProofInvalid {
				t.Fatal(name, errors)
			}
			cause := errors[0].AllocationCause
			if cause == nil || cause.Unknown || cause.Incomplete || len(cause.Path) < 2 || len(cause.Steps) != len(cause.Path)+1 {
				t.Fatal(cause)
			}
			for _, step := range cause.Steps {
				if step.Source.File != path {
					t.Fatal(step)
				}
			}
		}
	}
}
