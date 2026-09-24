package sema

import (
	"math/big"
	"reflect"
	"testing"

	"sec/internal/layout"
)

// ProcessID follows the target-sized uint representation while retaining a
// distinct nominal identity, and ProcessStatus exposes exactly the portable
// lifecycle states.
//
// Rules:
//   - rules/concurrency/processes.md — §§5.1–5.2 "ProcessID", "ProcessStatus"
func TestCompilerKnownProcessIdentityAndStatus(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		processID := analyzer.types["ProcessID"]
		wantMaximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
		if processID.Kind != UintType || !processID.Named || !processID.Intrinsic || processID.Underlying != "uint" ||
			processID.BitWidth != int64(width) || processID.MaxInteger == nil || processID.MaxInteger.Cmp(wantMaximum) != 0 {
			t.Fatalf("width %d ProcessID = %+v, want nominal target uint", width, processID)
		}
		if sameConcreteType(processID, analyzer.types["uint"]) {
			t.Fatalf("width %d ProcessID lost nominal identity", width)
		}
	}

	status := builtinTypes()["ProcessStatus"]
	wantVariants := []string{"Created", "Running", "Completed", "CompletionFailed", "Panicked", "Terminated"}
	if status.Kind != EnumType || !status.Named || !status.Intrinsic || status.Underlying != "uint" ||
		status.EnumDefault != "Created" || !reflect.DeepEqual(status.EnumValues, wantVariants) {
		t.Fatalf("ProcessStatus = %+v, want exact portable lifecycle enum", status)
	}
	for index, name := range wantVariants {
		value, ok := status.EnumConsts[name]
		if !ok || value.Value.Cmp(big.NewInt(int64(index))) != 0 {
			t.Fatalf("ProcessStatus.%s = %+v, want ordinal %d", name, value, index)
		}
	}
}

func TestCompilerKnownProcessTypesResolveInSource(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn SameProcess(left: ProcessID, right: ProcessID) bool {
	return left == right
}

fn IsTerminal(status: ProcessStatus) bool {
	return status == ProcessStatus.Completed || status == ProcessStatus.CompletionFailed || status == ProcessStatus.Panicked || status == ProcessStatus.Terminated
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestCompilerKnownProcessTypesCannotBeRedeclared(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type ProcessID uint

enum ProcessStatus {
	Replacement,
}
`)
	want := []string{
		"type name ProcessID is compiler-known and cannot be redeclared",
		"type name ProcessStatus is compiler-known and cannot be redeclared",
	}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v, want %v", errors, want)
	}
	for _, fragment := range want {
		if !errorsContainMessage(errors, fragment) {
			t.Fatalf("errors = %v, missing %q", errors, fragment)
		}
	}
}
