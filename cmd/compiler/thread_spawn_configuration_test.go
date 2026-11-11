package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNamedThreadConfigurationCLI shares normal consuming-argument diagnostics
// and rejects invalid configuration ownership before artifact publication.
// Rules: rules/concurrency/threads.md — §§10(3),20; spawn.md — §4.
func TestNamedThreadConfigurationCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_named_config_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/thread_named_config_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 || decodeOccurrenceDocument(t, output).Summary.Errors != 7 || !strings.Contains(output, "ThreadConfig") {
		t.Fatal(code, output)
	}
	markers := 0
	for _, occurrence := range decodeOccurrenceDocument(t, output).Occurrences {
		if strings.Contains(occurrence.Message.Text, "explicit <- ownership transfer") {
			markers++
			if occurrence.ID == nil || *occurrence.ID != "S1007" || len(occurrence.Help) == 0 {
				t.Fatal(occurrence)
			}
		}
	}
	if markers != 2 {
		t.Fatal("missing consuming guidance", output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid configuration emitted IR", err)
	}
}
