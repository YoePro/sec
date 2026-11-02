package sema

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

// Ordinary module bindings have static duration independently of mutability or
// redundant syntax. Local statics remain distinct from automatic locals, and
// direct impl members and interface requirements remain legal static contexts.
// Rules: rules/declarations/static.md — §§2–4,9;
// rules/declarations/interfaces.md — §3.4 (static requirements).
func TestStaticStorageAndDeclarationContexts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_context/declarations_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	expected := map[string]bool{"Inferred": false, "Typed": false, "Mutable": true, "Defaulted": true, "Explicit": false, "ExplicitMutable": true, "Table": false}
	symbols := a.Symbols()
	for name, mutable := range expected {
		symbol, ok := symbols[name]
		if !ok || symbol.Storage != StorageOriginStatic || symbol.Local || symbol.Mutable != mutable {
			t.Fatalf("module binding %s: %+v", name, symbol)
		}
	}
	for _, name := range []string{"Automatic", "AutomaticMutable"} {
		symbol, ok := symbols[name]
		if !ok || symbol.Storage != StorageOriginAutomatic || !symbol.Local {
			t.Fatalf("automatic binding %s: %+v", name, symbol)
		}
	}
	seen := map[string]bool{}
	ids := map[StaticBindingID]bool{}
	for _, fact := range a.ResolvedStaticDeclarations() {
		if fact.ID == 0 || ids[fact.ID] {
			t.Fatalf("duplicate/missing static identity: %+v", fact)
		}
		ids[fact.ID] = true
		if _, module := expected[fact.Name]; module {
			if fact.Function != "" || fact.Module != "main" || fact.Mutable != expected[fact.Name] {
				t.Fatalf("module static facts: %+v", fact)
			}
		} else if fact.Name != "Persistent" && fact.Name != "PersistentMutable" || fact.Function != "Scope" {
			t.Fatalf("unexpected static declaration: %+v", fact)
		}
		if _, ok := a.ResolvedStaticOf(fact.Declaration.Name); !ok {
			t.Fatalf("missing declaration binding: %+v", fact)
		}
		seen[fact.Name] = true
	}
	if len(seen) != len(expected)+2 {
		t.Fatalf("static declarations: %+v", seen)
	}
	for name := range expected {
		if !seen[name] {
			t.Errorf("missing static facts: %s", name)
		}
	}
	if methods := a.Types()["Factory"].InterfaceMethods; len(methods) != 1 || !methods[0].Static {
		t.Fatalf("lost static interface requirement: %+v", methods)
	}
}

// An enclosing impl scope never turns a local function into a static member.
// Invalid declarations must not enter the callable registry used by later stages.
// Rules: rules/declarations/static.md — §9;
// rules/foundations/grammar.md — "Static declarations", "Statement grammar".
func TestStaticDeclarationContextRejections(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_context/declarations_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 5 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if e.ID != diagnostics.StaticFunctionContext || e.Severity != diagnostics.SeverityError || e.Help == "" {
			t.Fatalf("incomplete context diagnostic: %+v", e)
		}
	}
	for _, name := range []string{"ModuleFunction", "LocalFunction", "NestedInstance", "NestedStatic", "NestedExtension"} {
		if functions := a.functions[name]; len(functions) != 0 {
			t.Fatalf("invalid declaration registered: %s %+v", name, functions)
		}
	}
}
