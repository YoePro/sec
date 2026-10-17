package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicOperationApplicabilityLSP uses shared compiler facts for per-T
// completion/hover and rejects unsupported calls in unsaved source.
// Rules: rules/concurrency/atomics.md — §§12–13,27–30,43–45.
func TestAtomicOperationApplicabilityLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "applicability.sec"))
	var source string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"atomic_applicability_valid.sec", 0},
		{"atomic_applicability_invalid.sec", 49},
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
	for _, test := range []struct {
		typ   string
		fetch bool
	}{
		{"int128", true}, {"uint128", true}, {"Count", true}, {"AliasCount", true},
		{"bool", false}, {"Flag", false}, {"State", false}, {"NamedState", false},
		{"RawPtr[byte]", false}, {"Pointer", false}, {"Bounded", false}, {"InheritedBound", false},
	} {
		completion := source + "\nfn Complete(value: ref Atomic[" + test.typ + "]) void {\n value.\n}\n"
		offset := strings.LastIndex(completion, "value.\n") + len("value.")
		labels := map[string]bool{}
		for _, item := range completeSource(uri, completion, offset) {
			labels[item.Label] = true
		}
		for _, name := range []string{"Load", "Store", "Swap", "CompareExchange"} {
			if !labels[name] {
				t.Fatal(test.typ, name, labels)
			}
		}
		for _, name := range []string{"FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor"} {
			if labels[name] != test.fetch {
				t.Fatal(test.typ, name, labels)
			}
		}
	}
	start := strings.Index(source, "fn NamedChain(")
	offset := start + strings.Index(source[start:], "value.Load()") + len("value.")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "fn Load() AliasCount") {
		t.Fatal(hover, ok)
	}
}
