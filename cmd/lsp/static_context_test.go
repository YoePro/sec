package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP projects Sema's context errors even inside an impl method, while direct
// static requirements and module storage remain valid.
// Rules: rules/declarations/static.md — §§3–4,9;
// rules/tooling/diagnostics.md — §§2(6–10),15.
func TestStaticDeclarationContextsLSP(t *testing.T) {
	for _, name := range []string{"declarations_valid", "declarations_invalid"} {
		data, err := os.ReadFile("../../testdata/sema/static_context/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-static-context/main.sec", string(data))
		errors, contexts := 0, 0
		for _, d := range values {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code == diagnostics.StaticFunctionContext {
				contexts++
				if !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character {
					t.Fatalf("context diagnostic: %+v", d)
				}
			}
		}
		if name == "declarations_valid" && errors != 0 || name == "declarations_invalid" && (errors != 5 || contexts != 5) {
			t.Fatalf("%s: %+v", name, values)
		}
	}
}
