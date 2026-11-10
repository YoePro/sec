package sema

import (
	"os"
	"strings"
	"testing"
)

// TestYieldSurface checks exact static signatures and distinct semantic identities,
// rejection of arguments/type arguments/instance calls, and the main task boundary.
// Rules: rules/concurrency/scheduling.md — §§7,11,22,55–56.
func TestYieldSurface(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"yield_surface_valid.sec", 0}, {"yield_surface_invalid.sec", 7},
	} {
		source, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.file, errors)
		}
		if tc.errors != 0 {
			for _, want := range []string{
				"Task.Yield expects 0 arguments", "Thread.Yield expects 0 arguments",
				"Task.Yield does not take type arguments", "Thread.Yield does not take type arguments",
				"work.Yield", "worker.Yield", "requires a current logical task context",
			} {
				found := false
				for _, diagnostic := range errors {
					found = found || strings.Contains(diagnostic.Message, want)
				}
				if !found {
					t.Fatalf("missing %q: %v", want, errors)
				}
			}
			continue
		}
		for _, name := range []string{"Task", "Thread"} {
			typ := a.Types()[name]
			member, ok := compilerKnownMember(typ, "Yield", true)
			if !ok || member.Kind != CompilerKnownAssociatedFunction || member.Signature != "static fn Yield() void" || member.Result.Kind != VoidType || len(member.Parameters) != 0 || member.Unsafe {
				t.Fatalf("%s: %+v", name, member)
			}
			if member.ID != "CKM-"+strings.ToUpper(name)+"-YIELD" {
				t.Fatal(member.ID)
			}
			if _, ok := compilerKnownMember(typ, "Yield", false); ok {
				t.Fatal("instance yield", name)
			}
			for line, text := range strings.Split(string(source), "\n") {
				needle := name + ".Yield"
				if col := strings.Index(text, needle); col >= 0 {
					fact, ok := a.CompilerKnownSymbolAt("", line+1, col+len(name)+2)
					if !ok || fact.ID != member.ID {
						t.Fatalf("lost %s identity on line %d: %+v", name, line+1, fact)
					}
				}
			}
		}
	}
}
