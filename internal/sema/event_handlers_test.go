package sema

import (
	"strings"
	"testing"
)

// Event handlers follow the owner-approved borrowed contract (MD-134,
// 2026-11-13): the handler type is fn(ref T) void; a stateless callable is
// passed as a value, a capturing callable through `ref` (or `ref mut` when it
// mutates its environment) to its external storage; consuming and by-value
// handlers are rejected; Publish borrows a T or passes a ref T.
// Rules: rules/concurrency/events.md — §§ 6(4), 6(7), 10(1)–(4), 11(1)–(4).
func TestEventHandlerBorrowedContract(t *testing.T) {
	const prelude = `module main

type Data struct { id: int }

type Button struct {
    Pressed: Event[Data],
}

impl Button {
    fn Press(data: ref Data) void {
        Pressed.Publish(data)
    }
}

fn OnPressed(data: ref Data) void {
}

fn ByValue(data: Data) void {
}
`
	for _, test := range []struct {
		name, body, want string
	}{
		{"named function", "match button.Pressed.Subscribe(OnPressed) { Subscribed(s) => s.Close()\n Full => {} }", ""},
		{"capture-free lambda", "match button.Pressed.Subscribe(fn(data: ref Data) void {}) { Subscribed(s) => s.Close()\n Full => {} }", ""},
		{"capturing through ref", "let limit := 3\n let handler := capture(limit) fn(data: ref Data) void { let over := data.id > limit }\n match button.Pressed.Subscribe(ref handler) { Subscribed(s) => s.Close()\n Full => {} }", ""},
		{"mutating through ref mut", "let mut count := 0\n let mut handler := capture(ref mut count) fn(data: ref Data) void { *count += 1 }\n match button.Pressed.Subscribe(ref mut handler) { Subscribed(s) => s.Close()\n Full => {} }", ""},
		{"by-value handler", "match button.Pressed.Subscribe(ByValue) { Subscribed(s) => s.Close()\n Full => {} }", "handler must be fn(ref Data) void"},
		{"inline capturing lambda", "let limit := 3\n match button.Pressed.Subscribe(capture(limit) fn(data: ref Data) void { let over := data.id > limit }) { Subscribed(s) => s.Close()\n Full => {} }", "cannot retain this handler by value"},
		{"capturing handler by value", "let limit := 3\n let handler := capture(limit) fn(data: ref Data) void { let over := data.id > limit }\n match button.Pressed.Subscribe(handler) { Subscribed(s) => s.Close()\n Full => {} }", "cannot retain this handler by value"},
		{"mutating through shared ref", "let mut count := 0\n let mut handler := capture(ref mut count) fn(data: ref Data) void { *count += 1 }\n match button.Pressed.Subscribe(ref handler) { Subscribed(s) => s.Close()\n Full => {} }", "requires ref mut, not ref"},
		{"ref of a named function", "match button.Pressed.Subscribe(ref OnPressed) { Subscribed(s) => s.Close()\n Full => {} }", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := prelude + "\nfn Use(button: ref Button) void {\n " + test.body + "\n}\n"
			errors := analyzeSourceRaw(t, source)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("unexpected errors: %v", errors)
				}
				return
			}
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("missing %q in %v", test.want, errors)
		})
	}

	consuming := prelude + "\nfn Use(button: ref Button, -> handler: -> fn(ref Data) void) void {\n match button.Pressed.Subscribe(handler) { Subscribed(s) => s.Close()\n Full => {} }\n}\n"
	found := false
	for _, err := range analyzeSourceRaw(t, consuming) {
		found = found || strings.Contains(err.Message, "is consuming; event handlers must be reusable")
	}
	if !found {
		t.Fatal("consuming handler accepted")
	}

	wrongPayload := strings.Replace(prelude, "Pressed.Publish(data)", "Pressed.Publish(42)", 1)
	found = false
	for _, err := range analyzeSourceRaw(t, wrongPayload) {
		found = found || strings.Contains(err.Message, "Event.Publish payload must be Data or ref Data")
	}
	if !found {
		t.Fatal("wrong payload accepted")
	}
}
