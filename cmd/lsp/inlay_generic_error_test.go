package main

import (
	"strings"
	"testing"
)

const genericErrorHintSource = `module main

type ParseError enum error { Empty }

fn Parse(text: string) Result[int, ParseError] {
    if text == "" {
        return Err(ParseError.Empty)
    }
    return Ok(1)
}

fn First[T](a: T, b: T) T { return a }

fn Use() Result[int, ParseError] {
    let x := try Parse("1")
    let y := First(x, 2)
    let z := First[int](x, y)
    let handled := try Parse("") {
        Err(_) => 0
    }
    return Ok(z + handled)
}

fn Find(values: int[3]) Option[int] {
    let first := try values.First()
    return Some(first)
}
`

// TestInlayGenericAndErrorHints: an inferred generic call shows its type
// arguments after the callee, an explicit one gets none; a propagating try
// shows the error type (or None) that leaves the function, a locally handled
// try gets none; both categories switch off independently.
// Rules: rules/tooling/lsp.md — § 17(1), § 17(7a).
func TestInlayGenericAndErrorHints(t *testing.T) {
	labels := func(settings inlayHintSettings) map[string]int {
		out := map[string]int{}
		for _, hint := range inlayHintsForSource("file:///tmp/generic_error.sec", genericErrorHintSource, allLines(), settings) {
			out[hint.Label]++
		}
		return out
	}
	all := labels(defaultInlayHintSettings())
	if all["[int]"] != 1 {
		t.Fatalf("one inferred generic hint expected (explicit First[int] has none): %v", all)
	}
	if all["↑ ParseError"] != 1 {
		t.Fatalf("one propagating try (the handled one has none): %v", all)
	}
	for label := range all {
		if strings.HasPrefix(label, "↑ ") && label != "↑ ParseError" && label != "↑ None" {
			t.Fatalf("unexpected error hint %q: %v", label, all)
		}
	}
	off := defaultInlayHintSettings()
	off.Generics, off.Errors = false, false
	if got := labels(off); got["[int]"] != 0 || got["↑ ParseError"] != 0 {
		t.Fatalf("settings must switch the categories off: %v", got)
	}
	settings := inlayHintSettingsFrom(defaultInlayHintSettings(), []byte(`{"inlayHints":{"generics":false}}`))
	if settings.Generics || !settings.Errors {
		t.Fatalf("configuration payload: %+v", settings)
	}
}
