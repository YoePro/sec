package main

import (
	"os"
	"strings"
	"testing"
)

// TestYieldSurfaceLSP projects static signatures, cancellation distinctions and
// synthetic navigation from compiler facts, without an editor-owned yield model.
// Rules: rules/concurrency/scheduling.md — §§7,11–13,22,55–56.
func TestYieldSurfaceLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/yield_surface_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///yield-surface/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, name := range []string{"Task", "Thread"} {
		offset := strings.Index(source, name+".Yield") + len(name) + 2
		pos := offsetPosition(source, offset)
		hover, ok := hoverForSource(uri, source, pos)
		if !ok {
			t.Fatal(name, "missing hover")
		}
		wants := []string{"static fn Yield() void", "synchronization edge", "rules/concurrency/scheduling.md"}
		if name == "Task" {
			wants = append(wants, "cancellation point", "ordinary main has none")
		} else {
			wants = append(wants, "Not a cancellation point", "scheduler hint")
		}
		for _, want := range wants {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Fatalf("%s lacks %q: %s", name, want, hover.Contents.Value)
			}
		}
		locations := definitionsForSource(uri, source, pos)
		if len(locations) != 1 {
			t.Fatalf("%s navigation: %+v", name, locations)
		}
		text, ok := compilerKnownDefinitionText(locations[0].URI)
		if !ok || !strings.Contains(text, "static fn Yield() void") || !strings.Contains(text, "rules/concurrency/scheduling.md") {
			t.Fatal(name, text)
		}
		cold, ok := compilerKnownDefinitionText("sec-compiler-known:/CKM-" + strings.ToUpper(name) + "-YIELD.sec")
		if !ok || !strings.Contains(cold, "static fn Yield() void") || !strings.Contains(cold, "rules/concurrency/scheduling.md") {
			t.Fatal("cold navigation", name, cold)
		}
		// An incomplete member expression uses the same associated-member catalog.
		completion := source[:strings.Index(source, name+".Yield")+len(name)+1] + "\n}\n"
		cursor := strings.LastIndex(completion, name+".") + len(name) + 1
		assertCompletionLabels(t, completeSource(uri, completion, cursor), []string{"Yield"})
	}
	invalid, err := os.ReadFile("../../testdata/sema/yield_surface_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity == 1 {
			count++
		}
	}
	if count != 7 {
		t.Fatal("invalid diagnostic count", count)
	}
}
