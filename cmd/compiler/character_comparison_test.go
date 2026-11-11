package main

import (
	"regexp"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/testsupport"
)

// Public emission retains unsigned character comparisons through the single
// compilation track, independently of the target's pointer/int width. Runtime
// behavior is tested by the character_predicates native fixture.
// Rules: rules/compiler/semantic_ir.md §125; rules/foundations/operators.md §45;
// rules/corrections/applied/md080-md082-md096-md098-md104-md105-correction-20261018.md §7.
func TestCharacterComparisonLLVMTargets(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		output, diagnosis, code := runCLIForDiagnostics(t, "emit-llvm", "../../testdata/build/native/character_predicates.sec", "--target", target, "-o", "-")
		if code != 0 {
			t.Fatal(target, code, diagnosis)
		}
		for _, bits := range []string{"8", "32"} {
			for _, predicate := range []string{"eq", "ne", "ult", "ule", "ugt", "uge"} {
				pattern := `icmp ` + predicate + ` i` + bits + ` `
				if !regexp.MustCompile(pattern).MatchString(output) {
					t.Fatal(target, "missing", pattern, output)
				}
			}
		}
	}
}

// Mixed character identities and character/integer pairs are rejected before
// representation in every output route, with registered mandatory diagnostics.
// Rules: rules/foundations/operators.md §45; rules/compiler/semantic_ir.md §125;
// rules/tooling/diagnostics.md — shared diagnostic model.
func TestCharacterComparisonMixedTypesCLI(t *testing.T) {
	for _, command := range []string{"check", "emit-ir", "emit-llvm"} {
		args := []string{command, "../../testdata/sema/character_comparisons_invalid.sec", "--diagnostic-format=json"}
		if command != "check" {
			args = append(args, "-o", "-")
		}
		out, errText, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, errText)
		if code != 3 || out != "" || doc.Summary.Errors != 8 || len(doc.Occurrences) != 8 {
			t.Fatal(command, code, out, errText)
		}
		for _, problem := range doc.Occurrences {
			if problem.ID == nil || (*problem.ID != diagnostics.OperatorNonComparable && *problem.ID != diagnostics.OperatorNonOrderable) || problem.Unregistered || problem.Primary == nil || len(problem.Help) == 0 {
				t.Fatal(problem)
			}
		}
	}
}
