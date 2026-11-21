package main

import (
	"strings"
	"testing"
)

// TestConcurrencyUmbrellaExamplesCLI checks the published fallible-task and
// nested-outcome sources at the public source boundary and rejects legacy types.
// Rules: rules/concurrency/concurrency.md §7; rules/concurrency/tasks.md §§4,12.
func TestConcurrencyUmbrellaExamplesCLI(t *testing.T) {
	const root = "../../testdata/concurrency/umbrella/"
	for _, name := range []string{"fallible_task", "nested_result"} {
		if _, output, code := runCLIForDiagnostics(t, "check", root+name+".sec"); code != 0 {
			t.Fatal(code, output)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", root+"fallible_task_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || doc.Summary.Errors != 4 {
		t.Fatal(code, output)
	}
	for _, want := range []string{"Result[Task[int], TaskSpawnError]", "TaskOutcome[int]", "Result[Task[Result[int, ReadError]], TaskSpawnError]"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing type layer %s: %s", want, output)
		}
	}
}
