package mlir

import (
	"errors"
	"fmt"
	"testing"
)

// Both gate spellings retain source coordinates and typed capability facts;
// unrelated errors and MLIR notes must never acquire capability status.
// Rules: rules/compiler/compiler_pipeline.md — §32(2–4), capability limits.
func TestLoweringCapabilityDiagnosticSpellings(t *testing.T) {
	for _, prefix := range []string{"", "[unsupported-lowering] "} {
		output := "src/a.sec:10:9: error: " + prefix + "the Sec MLIR -> LLVM track does not lower 'sec.union.is_variant' yet\n" +
			"  Circle(r) => r\n" +
			"src/a.sec:10:9: note: see current operation\n" +
			"dir:with:colons/b.sec:16:23: error: " + prefix + "the Sec MLIR -> LLVM track does not lower 'sec.union.construct' yet\n"
		limits := loweringCapabilityErrors(output)
		if len(limits) != 2 {
			t.Fatalf("%q: %v", prefix, limits)
		}
		for index, want := range []LoweringCapabilityError{
			{Feature: "'sec.union.is_variant'", File: "src/a.sec", Line: 10, Column: 9},
			{Feature: "'sec.union.construct'", File: "dir:with:colons/b.sec", Line: 16, Column: 23},
		} {
			var got *LoweringCapabilityError
			if !errors.As(limits[index], &got) || *got != want {
				t.Fatalf("%v, want %+v", limits[index], want)
			}
			if got.Error() != fmt.Sprintf("the Sec MLIR -> LLVM track does not lower %s yet at %s:%d:%d", want.Feature, want.File, want.Line, want.Column) {
				t.Fatal(got.Error())
			}
			capability := got.ImplementationCapability()
			if capability.File != want.File || capability.Line != want.Line || capability.Column != want.Column || capability.Feature != want.Feature {
				t.Fatal(capability)
			}
		}
	}
	for _, output := range []string{
		"src.sec:1:2: note: the Sec MLIR -> LLVM track does not lower X yet",
		"src.sec:1:2: error: [unsupported-lowering] invalid source type",
		"src.sec:1:2: error: [other] the Sec MLIR -> LLVM track does not lower X yet",
		"error: the Sec MLIR -> LLVM track does not lower X yet",
	} {
		if limits := loweringCapabilityErrors(output); len(limits) != 0 {
			t.Fatalf("unrelated diagnostic classified: %q", output)
		}
	}
}
