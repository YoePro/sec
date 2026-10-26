package main

import (
	"strings"
	"testing"
)

const expectedValueCompletionSource = `module main

type FileMode enum {
    Read,
    Write,
}

type File struct {
    _mode: FileMode,
    _fallback: FileMode,
    _path: string,
    _size: int,
}

type Options struct {
    Mode: FileMode,
    Label: string,
}

fn Apply(mode: FileMode, size: int) void {
}

impl File {
    fn DefaultMode() FileMode {
        return FileMode.Read
    }

    fn Length() int {
        return self._size
    }

    fn Use(flag: bool) void {
        CASE
    }
}
`

// lsp.md expected-type member completion: a selector written directly as the
// value of a typed let, an assignment, a call argument, a struct literal
// field or a match arm offers only members of the expected type; an untyped
// let and a selector nested in a larger expression stay unfiltered.
func TestMemberCompletionUsesExpectedValueTypes(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"typed let":       {"let mode: FileMode := self.|", "DefaultMode,_fallback,_mode"},
		"assignment":      {"let mut size := 0\n        size = self.|", "Length,_size"},
		"call argument":   {"Apply(FileMode.Read, self.|)", "Length,_size"},
		"first argument":  {"Apply(self.|, 1)", "DefaultMode,_fallback,_mode"},
		"struct field":    {"let options := Options { Mode: self.|, Label: \"x\" }", "DefaultMode,_fallback,_mode"},
		"match arm value": {"let path := match self._mode {\n            FileMode.Read => \"a\"\n            FileMode.Write => self.|\n        }", "_path"},
	} {
		t.Run(name, func(t *testing.T) {
			source := strings.Replace(expectedValueCompletionSource, "CASE", test.body, 1)
			got := strings.Join(completionLabelsAt(t, source, "|"), ",")
			if got != test.want {
				t.Fatalf("%s completion = %s, want %s", name, got, test.want)
			}
		})
	}
	for name, body := range map[string]string{
		"untyped let": "let any := self.|",
		"nested":      "let total: int := 1 + self.|",
	} {
		t.Run(name, func(t *testing.T) {
			source := strings.Replace(expectedValueCompletionSource, "CASE", body, 1)
			labels := completionLabelsAt(t, source, "|")
			if !strings.Contains(strings.Join(labels, ","), "_path") || !strings.Contains(strings.Join(labels, ","), "_mode") {
				t.Fatalf("%s completion must stay unfiltered, got %v", name, labels)
			}
		})
	}
}

// Expected-type filtering uses Sema's assignability, not name equality: a
// concrete error enum value is assignable to `error` (one-way widening), so
// it is offered for an `error` destination, while Option[T] admits no bare T
// because Sec has no implicit Some wrapping.
func TestExpectedTypeCompletionFollowsSemaAssignability(t *testing.T) {
	source := `module main

enum ReadError error {
    Failed,
}

type Job struct {
    _failure: ReadError,
    _count: int,
    _maybe: Option[int],
}

impl Job {
    fn Report() void {
        let failure: error := self.|
        let maybe: Option[int] := self.@
    }
}
`
	got := strings.Join(completionLabelsAt(t, strings.Replace(source, "@", "", 1), "|"), ",")
	if got != "_failure" {
		t.Fatalf("error destination completion = %s, want _failure", got)
	}
	got = strings.Join(completionLabelsAt(t, strings.Replace(source, "|", "", 1), "@"), ",")
	if got != "_maybe" {
		t.Fatalf("Option destination completion = %s, want _maybe", got)
	}
}
