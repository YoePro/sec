package main

import (
	"strings"
	"testing"
)

// MD-074/MD-088: len(value) is uint and the same count as value.Len, with an
// explicit checked conversion for signed use. MD-051: string(numericValue) is
// the numeric ToString() with the same Result[string, StringError] result;
// implicit numeric-to-string coercion stays invalid.
//
// Rules:
//   - rules/library/core-library.md — §§ 1.4, 15
//   - rules/types/types.md — "Numeric-to-text conversion"
//   - rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md — §§ 2, 3, 7, 8
func TestLenIsUintAndNumericStringConversionIsFallible(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/len_uint_numeric_string_valid.sec"); code != 0 {
		t.Fatal(code, stderr)
	}
	for file, want := range map[string]string{
		"len_uint_invalid_signed.sec":          "cannot initialize int with uint",
		"numeric_string_invalid_unwrapped.sec": "cannot initialize string with Result[string, StringError]",
		"numeric_string_invalid_implicit.sec":  "cannot initialize string with int",
	} {
		t.Run(file, func(t *testing.T) {
			_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+file)
			if code == 0 || !strings.Contains(stderr, want) {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
		})
	}
}
