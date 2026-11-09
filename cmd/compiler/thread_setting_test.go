package main

import "testing"

// TestThreadSettingCLI validates the canonical generic union and rejects plain
// values, incompatible payloads and invalid generic arity through the compiler.
// Rules: rules/concurrency/threads.md — §4(1–4).
func TestThreadSettingCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_setting_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_setting_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 3 {
		t.Fatal(output)
	}
}
