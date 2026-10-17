package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompareExchangeResultLSP presents the source enum, instantiated payloads,
// closed completion set and canonical source definitions on unsaved files.
// Rules: rules/concurrency/atomics.md — §§4,6,43–45.
func TestCompareExchangeResultLSP(t *testing.T) {
	const root = "../../testdata/sema/atomics/compare_exchange_result/"
	var source, uri string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"compare_exchange_result_valid.sec", 0},
		{"compare_exchange_result_invalid.sec", 13},
		{"compare_exchange_result_spoof_invalid.sec", 1},
	} {
		data, err := os.ReadFile(root + test.file)
		if err != nil {
			t.Fatal(err)
		}
		u := uriFromPath(filepath.Join(t.TempDir(), "result.sec"))
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
	typeOffset := strings.Index(source, "CompareExchangeResult[Count]")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, typeOffset))
	if !ok || !strings.Contains(hover.Contents.Value, "enum CompareExchangeResult[T]") || !strings.Contains(hover.Contents.Value, "NotExchanged(T)") || !strings.Contains(hover.Contents.Value, "not an error type") {
		t.Fatal(hover, ok)
	}
	checks := []struct{ text, want string }{
		{".Exchanged", "CompareExchangeResult[Count].Exchanged"},
		{".NotExchanged(observed)", "CompareExchangeResult[Count].NotExchanged(Count)"},
		{"Exchanged =>", "CompareExchangeResult[Count].Exchanged"},
		{"NotExchanged(observed) =>", "CompareExchangeResult[Count].NotExchanged(Count)"},
	}
	for _, check := range checks {
		offset := strings.Index(source, check.text)
		if strings.HasPrefix(check.text, ".") {
			offset++
		}
		position := offsetPosition(source, offset)
		hover, ok := hoverForSource(uri, source, position)
		if !ok || !strings.Contains(hover.Contents.Value, check.want) {
			t.Fatal(check, hover, ok)
		}
		definitions := definitionsForSource(uri, source, position)
		if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/atomic.sec") {
			t.Fatal(check, definitions)
		}
	}
	definitions := definitionsForSource(uri, source, offsetPosition(source, typeOffset))
	if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/atomic.sec") {
		t.Fatal(definitions)
	}
	for _, variant := range []string{"Exchanged", "NotExchanged"} {
		offset := strings.Index(source, "return CompareExchangeResult[Count]."+variant) + len("return ")
		definitions := definitionsForSource(uri, source, offsetPosition(source, offset))
		if len(definitions) != 1 || !strings.HasSuffix(definitions[0].URI, "/sec/core/atomic.sec") {
			t.Fatal(variant, definitions)
		}
	}
	completion := source + "\nfn Complete() void {\n    discard CompareExchangeResult.\n}\n"
	offset := strings.LastIndex(completion, "CompareExchangeResult.\n") + len("CompareExchangeResult.")
	items := completeSource(uri, completion, offset)
	assertCompletionLabels(t, items, []string{"Exchanged", "NotExchanged"})
	for _, item := range items {
		if item.Label == "Ok" || item.Label == "Err" || item.Label == "Some" || item.Label == "None" {
			t.Fatal(items)
		}
	}
	start := strings.Index(source, "fn Inspect(")
	offset = start + strings.Index(source[start:], "=> observed") + len("=> ")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "observed: Count") {
		t.Fatal(hover, ok)
	}
	definitions = definitionsForSource(uri, source, offsetPosition(source, offset))
	bindingOffset := start + strings.Index(source[start:], "NotExchanged(observed)") + len("NotExchanged(")
	if len(definitions) != 1 || definitions[0].URI != uri || definitions[0].Range.Start != offsetPosition(source, bindingOffset) {
		t.Fatal(definitions)
	}
}
