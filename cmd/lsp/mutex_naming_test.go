package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMutexCanonicalNamingLSP consumes the shared compiler catalog for basic
// acquisition hover, completion and synthetic navigation, without legacy names.
// Rules: rules/concurrency/mutex.md — §§3,20–22,61–62.
func TestMutexCanonicalNamingLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/mutex_naming_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "mutex.sec"))
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatal(diagnostic)
		}
	}
	for _, test := range []struct{ name, signature string }{{"Lock", "fn Lock() MutexGuard[int]"}, {"TryLock", "fn TryLock() Option[MutexGuard[int]]"}} {
		position := offsetPosition(source, strings.Index(source, "value."+test.name)+len("value."))
		hover, ok := hoverForSource(uri, source, position)
		if !ok || !strings.Contains(hover.Contents.Value, test.signature) {
			t.Fatal(hover, ok)
		}
		locations := definitionsForSource(uri, source, position)
		if len(locations) != 1 {
			t.Fatal(locations)
		}
		definition, ok := compilerKnownDefinitionText(locations[0].URI)
		if !ok || !strings.Contains(definition, test.signature) {
			t.Fatal(definition, ok)
		}
	}
	completion := source + "\nfn Complete(value: ref Mutex[int]) void {\n value.\n}\n"
	offset := strings.LastIndex(completion, "value.\n") + len("value.")
	labels := map[string]bool{}
	for _, item := range completeSource(uri, completion, offset) {
		labels[item.Label] = true
	}
	if !labels["Lock"] || !labels["TryLock"] || labels["lock"] || labels["tryLock"] {
		t.Fatal(labels)
	}
	invalid, err := os.ReadFile("../../testdata/sema/mutex_naming_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, diagnostic := range analyze(uri, string(invalid)) {
		if diagnostic.Severity == 1 {
			count++
		}
	}
	if count != 2 {
		t.Fatal(count)
	}
}
