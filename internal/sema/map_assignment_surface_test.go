package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMapAssignmentSurface rejects Set for dynamic/bounded maps and references,
// keeps declarations and ordinary user Set methods valid, and verifies that the
// shared registry never advertises the forbidden map operation.
// Rules: rules/collections/collections.md — §14.5;
// rules/compiler/compiler_known_members.md — "Registry", "Lookup order".
func TestMapAssignmentSurface(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, invalid := range []bool{false, true} {
			suffix, want := "map_assignment_surface_valid", 0
			if invalid {
				suffix, want = "map_set_invalid", 4
			}
			data, err := os.ReadFile("../../testdata/sema/" + suffix + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			// The ordinary helper follows the same canonical compiler path.
			a := NewAnalyzerWithDepth(depth)
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errs := a.Analyze(program)
			if len(errs) != want {
				t.Fatal(depth, errs)
			}
			for _, d := range errs {
				if d.ID != diagnostics.UnknownCallTarget || !strings.Contains(d.Help, "[key] = value") || !strings.Contains(d.Message, "no Set method") {
					t.Fatal(d)
				}
			}
			if invalid {
				for _, name := range []string{"Dynamic", "Bounded", "Shared", "Mutable"} {
					typ := a.functions[name][0].Parameters[0].Type
					foundRemove := false
					for _, static := range []bool{false, true} {
						for _, member := range CompilerKnownMembersForType(typ, static) {
							if member.Name == "Set" {
								t.Fatal("map registry exposed Set", typ, member)
							}
							if !static && member.Name == "Remove" {
								foundRemove = true
							}
						}
					}
					if !foundRemove {
						t.Fatal("missing concrete map surface", name, typ)
					}
				}
			}
		}
	}
}
