package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicSurfaceLSP shares Sema's spelling and eligibility checks on unsaved
// files, and exposes the source-visible core enum and canonical member names.
// Rules: rules/concurrency/atomics.md — §§4–5,8–9,12–13,43–45.
func TestAtomicSurfaceLSP(t *testing.T) {
	var source, uri string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"atomic_surface_valid.sec", 0}, {"atomic_surface_invalid.sec", 15},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		u := uriFromPath(filepath.Join(t.TempDir(), "atomic.sec"))
		count := 0
		for _, diagnostic := range analyze(u, string(data)) {
			if diagnostic.Severity == 1 {
				count++
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
		if test.errors == 0 {
			source, uri = string(data), u
		}
	}
	position := offsetPosition(source, strings.Index(source, "MemoryOrder.SeqCst")+len("MemoryOrder."))
	hover, ok := hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "SeqCst") || !strings.Contains(hover.Contents.Value, "Sequential consistency") {
		t.Fatal(hover, ok)
	}
	definitions := definitionsForSource(uri, source, position)
	if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/atomic.sec") {
		t.Fatal(definitions)
	}
	ownerPosition := offsetPosition(source, strings.Index(source, "MemoryOrder.SeqCst"))
	ownerDefinitions := definitionsForSource(uri, source, ownerPosition)
	if len(ownerDefinitions) != 1 || !strings.HasSuffix(ownerDefinitions[0].URI, "/sec/core/atomic.sec") {
		t.Fatal(ownerDefinitions)
	}
	position = offsetPosition(source, strings.Index(source, "Orders() MemoryOrder")+len("Orders() "))
	hover, ok = hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "enum MemoryOrder {\n    Relaxed\n    Acquire\n    Release\n    AcqRel\n    SeqCst\n}") {
		t.Fatal(hover, ok)
	}
	completion := source + "\nfn Complete(value: ref Atomic[int]) void {\n value.\n}\n"
	offset := strings.LastIndex(completion, "value.\n") + len("value.")
	items := completeSource(uri, completion, offset)
	labels := map[string]bool{}
	for _, item := range items {
		labels[item.Label] = true
	}
	for _, name := range []string{"Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor"} {
		if !labels[name] || labels[strings.ToLower(name[:1])+name[1:]] {
			t.Fatal(name, items)
		}
	}
	completion = source + "\nfn CompleteOrder() void {\n discard MemoryOrder.\n}\n"
	offset = strings.LastIndex(completion, "MemoryOrder.\n") + len("MemoryOrder.")
	assertCompletionLabels(t, completeSource(uri, completion, offset), []string{"Relaxed", "Acquire", "Release", "AcqRel", "SeqCst"})
}
