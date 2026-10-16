package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMutexAcquisitionOverloadsLSP shares compiler overload results and displays
// the exact acquisition signatures and operation-local ContextError family.
// Rules: rules/concurrency/mutex.md — §§7–8,20–24,61–63.
func TestMutexAcquisitionOverloadsLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "overloads.sec"))
	var source string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"mutex_overloads_valid.sec", 0}, {"mutex_overloads_invalid.sec", 11},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, diagnostic := range analyze(uri, string(data)) {
			if diagnostic.Severity == 1 {
				count++
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
		if test.errors == 0 {
			source = string(data)
		}
	}
	position := offsetPosition(source, strings.Index(source, "value.Lock(timeout)")+len("value."))
	hover, ok := hoverForSource(uri, source, position)
	for _, signature := range []string{"fn Lock() MutexGuard[int]", "fn Lock(timeout: duration) Option[MutexGuard[int]]", "fn Lock(context: ref Context) Result[MutexGuard[int], ContextError]"} {
		if !ok || !strings.Contains(hover.Contents.Value, signature) {
			t.Fatal(hover, ok)
		}
	}
	position = offsetPosition(source, strings.Index(source, "ref Context")+len("ref "))
	hover, ok = hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "@noCopy\ntype Context") {
		t.Fatal(hover, ok)
	}
	locations := definitionsForSource(uri, source, position)
	if len(locations) != 1 || !strings.HasSuffix(locations[0].URI, "/sec/core/context.sec") {
		t.Fatal(locations)
	}
	position = offsetPosition(source, strings.Index(source, "ContextError"))
	hover, ok = hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "enum ContextError error {\n    Cancelled\n    DeadlineExceeded\n}") {
		t.Fatal(hover, ok)
	}
	completion := source + "\nfn Complete() void {\n discard ContextError.\n}\n"
	offset := strings.LastIndex(completion, "ContextError.\n") + len("ContextError.")
	assertCompletionLabels(t, completeSource(uri, completion, offset), []string{"Cancelled", "DeadlineExceeded"})
}
