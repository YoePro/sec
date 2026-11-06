package main

import "testing"

// The real target-aware compiler loads every generated table and the Unicode
// wrappers, including rune extensions and scalar-only category literals.
// Rules: rules/library/core-library.md — §11.3; rules/library/stdlib.md — Unicode classification.
func TestCompilerLoadsGeneratedUnicodeProperties(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "--target=linux-amd64", "../../testdata/unicode_classification_valid.sec")
	if code != 0 {
		t.Fatalf("generated Unicode classification rejected: %d %s", code, stderr)
	}
}

// Unicode import checking uses the selected target, just like the compiler CLI.
// Rules: rules/library/core-library.md — §11.3; rules/platform/target_profiles.md — target-selected scalars.
func TestCompilerLoadsUnicodeIsLetter(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "--target=linux-amd64", "../../testdata/unicode_valid.sec")
	if code != 0 {
		t.Fatalf("Unicode letter module rejected: %d %s", code, stderr)
	}
}
