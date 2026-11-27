package sema

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// An Arena allocation's reference denotes its byte referent, not the mutable
// reference holder, even when no lexical referent origin can be named.
// Reset remains forbidden while the reference is needed afterwards.
// Rules: rules/memory/reference_model.md — §§18-20,45;
// rules/mlir/packages/sec-mlir-dialect_package19.md — §102.
func TestArenaReferenceSourcePlaces(t *testing.T) {
	for _, file := range []string{"valid.sec", "stale_invalid.sec"} {
		source, err := os.ReadFile("../../testdata/semantic_ir/arena_references/" + file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if file == "stale_invalid.sec" {
			if len(errors) == 0 || !strings.Contains(joinedSemaErrors(errors), "reset") {
				t.Fatal("live allocation accepted across Reset", errors)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		uses := 0
		for _, use := range a.resolvedReferenceUses {
			if use.Facts.Provenance != ReferenceProvenanceArena || (use.Kind != ReferenceUseRead && use.Kind != ReferenceUseWrite) {
				continue
			}
			uses++
			if use.Place.Type.Name != "byte" || use.Facts.Validity != ReferenceValidityDynamicEpoch || use.Facts.InvalidationDomain == 0 || use.Facts.EpochDependency == 0 {
				t.Fatal("lost referent type/epoch", use)
			}
		}
		if uses != 5 {
			t.Fatal("missing source reference uses", uses)
		}
	}
}
