package sema

import (
	"os"
	"strings"
	"testing"
)

// TestMutexCanonicalNaming retains acquisition result types and the blocking
// distinction while rejecting the old public spellings.
// Rules: rules/concurrency/mutex.md — §§3,20–22;
// rules/concurrency/blocking.md — "Mutex acquisition".
func TestMutexCanonicalNaming(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"mutex_naming_valid.sec", 0}, {"mutex_naming_invalid.sec", 2},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != test.errors {
			t.Fatal(test.file, errors)
		}
		if test.errors != 0 {
			for _, diagnostic := range errors {
				if !strings.Contains(diagnostic.Message, "unknown function or type value.") {
					t.Fatal(diagnostic)
				}
			}
			continue
		}
		if got := typeDisplayName(a.completionSymbols["guard"].Type); got != "MutexGuard[int]" {
			t.Fatal(got)
		}
	}
	typ := builtinType("Mutex")
	typ.TypeArgs = []Type{builtinType("int")}
	for _, test := range []struct{ name, result string }{{"Lock", "MutexGuard[int]"}, {"TryLock", "Option[MutexGuard[int]]"}} {
		member, ok := compilerKnownMember(typ, test.name, false)
		if !ok || typeDisplayName(member.Result) != test.result || len(member.LegacyNames) != 0 {
			t.Fatal(member, ok)
		}
	}
	for _, name := range []string{"lock", "tryLock"} {
		if _, ok := compilerKnownMember(typ, name, false); ok {
			t.Fatal("legacy alias", name)
		}
	}
}
