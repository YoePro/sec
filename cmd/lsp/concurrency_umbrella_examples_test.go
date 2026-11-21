package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConcurrencyUmbrellaExamplesLSP keeps unsaved source validation aligned
// with the book, including clearing diagnostics after a valid replacement.
// Rules: rules/concurrency/concurrency.md §7; rules/tooling/lsp.md — Diagnostics.
func TestConcurrencyUmbrellaExamplesLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"fallible_task_invalid", "fallible_task", "nested_result"} {
		source, err := os.ReadFile("../../testdata/concurrency/umbrella/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		messages := ""
		for _, d := range analyze(uri, string(source)) {
			if d.Severity != 1 {
				continue
			}
			count++
			messages += d.Message + "\n"
			if d.Range.Start.Line < 0 || d.Range.End.Character <= d.Range.Start.Character {
				t.Fatal(d)
			}
		}
		if name == "fallible_task_invalid" {
			if count != 4 || !strings.Contains(messages, "TaskOutcome[int]") || !strings.Contains(messages, "Result[Task[Result[int, ReadError]], TaskSpawnError]") {
				t.Fatal(count, messages)
			}
		} else if count != 0 {
			t.Fatal(count, messages)
		}
	}
}
