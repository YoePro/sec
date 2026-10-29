package sema

import (
	"os"
	"strings"
	"testing"
)

// TestGenericFunctionConcreteMemberTypes checks that specialization reaches
// field facts, including nested results and parameter types, before copying.
// Rules: rules/declarations/generics.md — §§16–17;
// rules/compiler/monomorphization.md — §8.2.
func TestGenericFunctionConcreteMemberTypes(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generic_function_members/concrete.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(source)); len(errors) != 0 {
		t.Fatalf("concrete members retain generic template types: %v", errors)
	}
}

// TestGenericFunctionMemberSubstitutionKeepsCopyRestrictions ensures concrete
// noCopy and unconstrained template fields never gain implicit copy permission.
// Rules: rules/declarations/generics.md — §§15–17;
// rules/memory/copy_move.md — copy classification.
func TestGenericFunctionMemberSubstitutionKeepsCopyRestrictions(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/generic_function_members/copy_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	for _, want := range []string{"explicitly forbids implicit copy through @noCopy", "generic copyability has not been proven"} {
		found := false
		for _, diagnostic := range errors {
			found = found || strings.Contains(diagnostic.Message, want)
		}
		if !found {
			t.Errorf("missing %q: %v", want, errors)
		}
	}
	concreteRejected := false
	for _, diagnostic := range errors {
		if diagnostic.Line == 11 && strings.Contains(diagnostic.Message, "explicitly forbids implicit copy") {
			concreteRejected = true
		}
	}
	if !concreteRejected {
		t.Errorf("noCopy field was accepted: %v", errors)
	}
}
