package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMutexNoCopyLSP checks unsaved-source ownership diagnostics and presents
// the real opaque @noCopy declarations with source navigation.
// Rules: rules/concurrency/mutex.md — §§4–6,28,61–62.
func TestMutexNoCopyLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "owners.sec"))
	var source string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"mutex_nocopy_valid.sec", 0}, {"mutex_nocopy_invalid.sec", 6},
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
	for _, name := range []string{"Mutex", "MutexGuard"} {
		needle := "ref " + name + "[int]"
		position := offsetPosition(source, strings.Index(source, needle)+len("ref "))
		hover, ok := hoverForSource(uri, source, position)
		if !ok || !strings.Contains(hover.Contents.Value, "@noCopy\ntype "+name+"[T]") {
			t.Fatal(name, hover, ok)
		}
		locations := definitionsForSource(uri, source, position)
		if len(locations) != 1 || !strings.HasSuffix(locations[0].URI, "/sec/core/mutex.sec") {
			t.Fatal(name, locations)
		}
	}
}
