package main

import (
	"strings"
	"testing"
)

// TestThreadConfigurationEnumsCLI exposes exact start-mode/priority types and
// rejects unsupported variants and interchange of distinct non-error identities.
// Rules: rules/concurrency/threads.md — §§5–6.
func TestThreadConfigurationEnumsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_configuration_enums_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_configuration_enums_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 4 {
		t.Fatal(output)
	}
	for _, name := range []string{"ThreadStartMode.Immediate", "ThreadPriority.Realtime"} {
		if !strings.Contains(output, name+" has never been declared") {
			t.Fatal(output)
		}
	}
}
