package main

import (
	"strings"
	"testing"
)

// The editor shares Sema's pack loop rules: a supported loop reports nothing,
// and a body operation invalid for an element is reported on its source line.
// Rules: rules/declarations/functions.md §38A(4)(6),(5),(10)(3); rules/control-flow/flowcontrol_for.md §13A.
func TestPackLoopLSP(t *testing.T) {
	valid := "module main\nfn F[...Types](values: ...ref Types) uint {\n    let mut n: uint := 0\n    for ref value in values {\n        n = n + 1\n    }\n    return n\n}\n"
	for _, item := range analyze("file:///tmp/sec-lsp-pack-loop/main.sec", valid) {
		if item.Severity == 1 {
			t.Fatalf("valid pack loop: %+v", item)
		}
	}
	invalid := strings.Replace(valid, "n = n + 1", "let x := value + 1", 1)
	found := false
	for _, item := range analyze("file:///tmp/sec-lsp-pack-loop/main.sec", invalid) {
		found = found || item.Severity == 1 && item.Range.Start.Line == 4 && strings.Contains(item.Message, "operator +")
	}
	if !found {
		t.Fatal("element arithmetic not reported")
	}
}

// A forward is a supported pack use; a concrete expansion that cannot reach its
// target is reported at the source call.
// Rules: rules/declarations/functions.md §38A(7),(10)(1–3).
func TestPackForwardLSP(t *testing.T) {
	uri := "file:///tmp/sec-lsp-pack-forward/main.sec"
	valid := "module main\nfn Take[...Types](values: ...Types) uint {\n    return values.Len\n}\nfn Move[...Types](values: ...Types) uint {\n    return Take(<-values...)\n}\nfn Use() uint {\n    return Move(1, true)\n}\n"
	for _, item := range analyze(uri, valid) {
		if item.Severity == 1 {
			t.Fatalf("valid forward: %+v", item)
		}
	}
	invalid := "module main\nfn Two(a: int, b: int) int {\n    return a\n}\nfn On[...Types](values: ...Types) int {\n    return Two(values...)\n}\nfn Use() int {\n    return On(1)\n}\n"
	found := false
	for _, item := range analyze(uri, invalid) {
		found = found || item.Severity == 1 && item.Range.Start.Line == 8 && strings.Contains(item.Message, "expands to 1")
	}
	if !found {
		t.Fatal("specialization error not reported at the call")
	}
}
