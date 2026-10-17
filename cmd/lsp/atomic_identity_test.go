package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicCoreIdentityLSP checks resolved type-use, binding and constructor
// hovers with exact per-T APIs; source names reused in other functions cannot
// supply a different binding's type. Navigation keeps canonical synthetic
// operation definitions where compiler-owned storage has no source body.
// Rules: rules/concurrency/atomics.md — §§4,7(8),12–13,43–44;
// rules/compiler/compiler_known_members.md — Synthetic definitions.
func TestAtomicCoreIdentityLSP(t *testing.T) {
	var source, uri string
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"atomic_identity_valid.sec", 0},
		{"atomic_identity_invalid.sec", 14},
		{"atomic_identity_spoof_invalid.sec", 1},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		u := uriFromPath(filepath.Join(t.TempDir(), "identity.sec"))
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
	for _, test := range []struct {
		function, typ string
		fetch         bool
	}{
		{"Boolean", "bool", false}, {"Numeric", "Count", true},
		{"Enumeration", "State", false}, {"Constrained", "Bounded", false},
	} {
		start := strings.Index(source, "fn "+test.function+"(")
		for _, offset := range []int{
			start + strings.Index(source[start:], "Atomic["),
			start + strings.Index(source[start:], "value: ref"),
			start + strings.Index(source[start:], "value.Load"),
		} {
			hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
			if !ok {
				t.Fatal(test.function, offset)
			}
			text := hover.Contents.Value
			for _, signature := range []string{"fn Load() " + test.typ, "fn Store(value: " + test.typ + ") void", "fn Swap(value: " + test.typ + ") " + test.typ, ") CompareExchangeResult[" + test.typ + "]"} {
				if !strings.Contains(text, signature) {
					t.Fatal(test.function, signature, text)
				}
			}
			if strings.Contains(text, "fn FetchAdd(") != test.fetch || !strings.Contains(text, "representation is private") {
				t.Fatal(test.function, text)
			}
		}
	}
	start := strings.Index(source, "fn ContextualNamed()")
	for _, offset := range []int{
		start + strings.Index(source[start:], "Atomic[Count]"),
		start + strings.Index(source[start:], "Atomic(0)"),
	} {
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "fn Load() Count") || !strings.Contains(hover.Contents.Value, "Atomic[Count]") {
			t.Fatal(hover, ok)
		}
	}
	start = strings.Index(source, "fn Methods(")
	for _, name := range []string{"Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor"} {
		offset := start + strings.Index(source[start:], "value."+name) + len("value.")
		definitions := definitionsForSource(uri, source, offsetPosition(source, offset))
		if len(definitions) != 1 || !strings.HasPrefix(definitions[0].URI, compilerKnownScheme+":") {
			t.Fatal(name, definitions)
		}
		text, ok := compilerKnownDefinitionText(definitions[0].URI)
		if !ok || !strings.Contains(text, "fn "+name+"(") || !strings.Contains(text, "Atomic[int]") {
			t.Fatal(name, text, ok)
		}
	}
}
