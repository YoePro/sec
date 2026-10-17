package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicOrderMatrixLSP uses the compiler's matrix for unsaved source and
// exposes its exact operation-specific orders through compiler member hover.
// Rules: rules/concurrency/atomics.md — §§24,43–45.
func TestAtomicOrderMatrixLSP(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"atomic_orders_valid.sec", 0},
		{"atomic_orders_invalid.sec", 43},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		uri := uriFromPath(filepath.Join(t.TempDir(), "atomic_orders.sec"))
		count := 0
		for _, diagnostic := range analyze(uri, source) {
			if diagnostic.Severity == 1 {
				count++
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
		if test.errors != 0 {
			continue
		}
		for _, test := range []struct{ call, documentation string }{
			{"value.Load", "Valid explicit orders: Relaxed, Acquire, SeqCst."},
			{"value.Store", "Valid explicit orders: Relaxed, Release, SeqCst."},
			{"value.CompareExchange", "Failure orders: Relaxed, Acquire, SeqCst. Failure order need not be weaker than success order."},
		} {
			position := offsetPosition(source, strings.Index(source, test.call)+len("value."))
			hover, ok := hoverForSource(uri, source, position)
			if !ok || !strings.Contains(hover.Contents.Value, test.documentation) {
				t.Fatal(test.call, hover, ok)
			}
		}
	}
}
