package sema

import (
	"os"
	"strings"
	"testing"
)

// Using a Result or Option carrier where its success value is expected gets
// a mentor explanation of the missing try: initialization, return, call
// argument, and arithmetic operand, for both carrier families.
//
// Rules:
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor", §12, §15
func TestMissingTryDiagnosticsExplainTheFix(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/missing_try_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(source))
	if len(errors) != 5 {
		t.Fatalf("errors = %v, want five carrier mismatches", errors)
	}
	for index, err := range errors {
		want := "Write `try Read()` to use the int and propagate or handle ReadError"
		if index == 3 {
			want = "Write `try Find()` to use the int and propagate None, or handle absence with `try Find() { None => ... }`"
		}
		if !strings.Contains(err.Help, want) {
			t.Fatalf("error %d = %q help %q, want %q", index, err.Message, err.Help, want)
		}
	}
}
