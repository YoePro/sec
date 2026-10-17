package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	secformatter "sec/internal/formatter"
)

// The formatter is stable on atomic source: formatting is idempotent, keeps
// the canonical CamelCase operations, MemoryOrder arguments and atomic.Fence
// calls, never inserts an omitted MemoryOrder.SeqCst, and leaves the fence
// fixtures unchanged.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(5), 25(4)
//   - rules/tooling/formatter.md
func TestAtomicSourceFormatting(t *testing.T) {
	format := func(source string) string {
		t.Helper()
		var out strings.Builder
		if err := formatStdinWithOptions(strings.NewReader(source), &out, secformatter.Options{}); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	files, err := filepath.Glob("../../testdata/sema/atomic_*.sec")
	if err != nil || len(files) == 0 {
		t.Fatal(files, err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		once := format(source)
		if format(once) != once {
			t.Errorf("%s: formatting is not idempotent", file)
		}
		if strings.Count(once, "MemoryOrder.SeqCst") != strings.Count(source, "MemoryOrder.SeqCst") {
			t.Errorf("%s: the formatter changed the explicit SeqCst count", file)
		}
		if strings.Count(once, "atomic.Fence(") != strings.Count(source, "atomic.Fence(") {
			t.Errorf("%s: the formatter changed atomic.Fence calls", file)
		}
		if strings.Contains(filepath.Base(file), "fence") && once != source {
			t.Errorf("%s: the fence fixture is not canonically formatted:\n%s", file, once)
		}
	}
	omitted := "module main\n\nfn Read(counter: ref Atomic[uint64]) uint64 {\n    return counter.Load()\n}\n"
	if formatted := format(omitted); strings.Contains(formatted, "SeqCst") {
		t.Fatalf("an omitted order was made explicit:\n%s", formatted)
	}
}
