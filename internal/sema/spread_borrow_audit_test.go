package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The audit compares prepared sources with call reservations and checks that
// a copied element is never given its source array's return-reference origin.
// Rules: rules/declarations/spread.md §§3,4.3;
// rules/memory/borrowing.md §15.4; rules/memory/reference_model.md §30.
func TestSpreadBorrowAudit(t *testing.T) {
	for _, file := range []string{"spread_borrow_audit_invalid.sec", "spread_borrow_audit_valid.sec"} {
		source, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", file, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), file))
					parsed := p.Parse()
					if parsed.HasErrors {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(parsed.Program)
					if strings.HasSuffix(file, "_valid.sec") {
						assertSemaErrors(t, errors, nil)
						return
					}
					ids := []string{diagnostics.BorrowOwnerReadConflict, diagnostics.CompilerCapabilityUnavailable, diagnostics.CompilerCapabilityUnavailable, diagnostics.CompilerCapabilityUnavailable, diagnostics.BorrowOwnerReadConflict, diagnostics.BorrowOwnerReadConflict, diagnostics.CompilerCapabilityUnavailable, diagnostics.CompilerCapabilityUnavailable}
					if len(errors) != len(ids) {
						t.Fatalf("diagnostics: %+v", errors)
					}
					for i, e := range errors {
						if e.ID != ids[i] || e.Help == "" || e.File != file || e.Line == 0 {
							t.Fatalf("lost diagnostic identity or help: %+v", e)
						}
						if e.IsImplementationCapability() {
							if e.ProofState != "" || !strings.Contains(e.Message, "provenance") {
								t.Fatalf("capability represented as validity proof: %+v", e)
							}
						} else if e.PreviousLine != e.Line || e.PreviousColumn >= e.Column || !strings.Contains(e.RelatedLabel, "argument 1") {
							t.Fatalf("lost earlier borrow reservation: %+v", e)
						}
					}
					for call, fact := range a.resolvedCalls {
						if fact.Arguments != nil {
							t.Fatalf("invalid spread published call facts: %s", call)
						}
					}
				})
			}
		}
	}
}
