package main

import (
	"os"
	"strings"
	"testing"
)

// genericErrorInterfaceFixture keeps actual Sec sources in testdata and verifies
// that each baseline uses legal MD-120 template operations before editor recovery.
// Rules: rules/declarations/generics.md — §15A; MD-120 §§2–7,11(6).
func genericErrorInterfaceFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/generic_error_parameters/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, d := range analyze("file:///tmp/sec-generic-error-interface/"+name+".sec", source) {
		if d.Severity == 1 {
			t.Fatalf("invalid baseline: %+v", d)
		}
	}
	return source
}

// Interface declarations, requirement declarations and specialized calls expose
// unknown E, explicit error and a concrete error as three different channels.
// Rules: rules/tooling/lsp.md — "Hover", "Signature help"; MD-120 §§10(7),11(6).
func TestGenericErrorInterfaceHoverAndSignatures(t *testing.T) {
	source := genericErrorInterfaceFixture(t, "interface")
	uri := "file:///tmp/sec-generic-error-interface/interface.sec"
	for _, test := range []struct{ needle, want string }{
		{"MyInterf[E: error]", "interface MyInterf[E: error]"},
		{"E: error]", "generic E: error"},
		{"error] {", "Compile-time error-category requirement"},
		{"myFunc() Result", "fn myFunc() Result[void, E]"},
		{"MyInterf[IOError]", "interface MyInterf[IOError]"},
		{"MyInterf[error]", "interface MyInterf[error]"},
		{"error])", "Open error root"},
	} {
		i := strings.Index(source, test.needle)
		if i < 0 {
			t.Fatal(test.needle)
		}
		hover, ok := hoverForSource(uri, source, offsetPosition(source, i))
		if !ok || !strings.Contains(hover.Contents.Value, test.want) {
			t.Fatalf("%s: %v %+v", test.needle, ok, hover)
		}
		if test.needle == "MyInterf[IOError]" && !strings.Contains(hover.Contents.Value, "myFunc() Result[void, IOError]") {
			t.Fatal("instantiated interface lost its concrete requirement", hover)
		}
		if test.needle == "error])" && strings.Contains(hover.Contents.Value, "FormatError") {
			t.Fatal("root error shadowed by unrelated binding", hover)
		}
	}
	start := 0
	for _, channel := range []string{"E", "IOError", "error", "IOError"} {
		i := strings.Index(source[start:], "value.myFunc()")
		if i < 0 {
			t.Fatal("missing call")
		}
		i += start
		want := "fn myFunc() Result[void, " + channel + "]"
		items := completeSource(uri, source, i+len("value."))
		found := false
		for _, item := range items {
			if item.Label == "myFunc" {
				found = true
				if !strings.Contains(item.Detail, "Result[void, "+channel+"]") {
					t.Errorf("wrong completion channel: %+v", item)
				}
			}
		}
		if !found {
			t.Errorf("no interface member completion: %+v", items)
		}
		hover, ok := hoverForSource(uri, source, offsetPosition(source, i+len("value.")))
		if !ok || !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("channel %s: %v %+v", channel, ok, hover)
		}
		help, ok := signatureHelpForSource(uri, source, offsetPosition(source, i+len("value.myFunc(")))
		if !ok || len(help.Signatures) != 1 || help.Signatures[0].Label != want || help.ActiveParameter != 0 {
			t.Fatalf("channel %s: %v %+v", channel, ok, help)
		}
		targets := definitionsForSource(uri, source, offsetPosition(source, i+len("value.")))
		declaration := offsetPosition(source, strings.Index(source, "myFunc() Result"))
		if len(targets) != 1 || targets[0].URI != uri || targets[0].Range.Start != declaration {
			t.Fatalf("lost requirement navigation: %+v", targets)
		}
		if channel == "E" {
			incomplete := source[:i+len("value.myFunc(")]
			pending, known := signatureHelpForSource(uri, incomplete, offsetPosition(incomplete, len(incomplete)))
			if !known || len(pending.Signatures) != 1 || pending.Signatures[0].Label != want {
				t.Fatalf("incomplete interface call: %v %+v", known, pending)
			}
		}
		start = i + 1
	}
}

// Unknown E never supplies a concrete variant or a legal Err(_) discard.
// Whole-channel completion replaces the invalid selector with a fresh binding;
// accepting the edit is re-analyzed rather than merely checking its label.
// Rules: rules/tooling/lsp.md — "Completion"; MD-120 §§4(4–7),10(7).
func TestGenericErrorWholeChannelCompletion(t *testing.T) {
	source := genericErrorInterfaceFixture(t, "patterns")
	start := 0
	for n := 0; n < 2; n++ {
		i := strings.Index(source[start:], "Err(cause)")
		if i < 0 {
			t.Fatal("missing pattern")
		}
		i += start
		begin := i + len("Err(")
		edited := source[:begin] + ".Cl" + source[begin+len("cause"):]
		items := completeSource("file:///tmp/sec-generic-error-interface/patterns.sec", edited, begin+len(".Cl"))
		if len(items) != 1 || !strings.HasPrefix(items[0].Label, "Err(") || items[0].Kind == 20 || items[0].TextEdit == nil {
			t.Fatalf("items: %+v", items)
		}
		edit := items[0].TextEdit
		if edit.Range != offsetsRange(edited, begin, begin+3) || edit.NewText == "_" || edit.NewText == "cause" {
			t.Fatalf("unsafe binding edit: %+v", edit)
		}
		accepted := edited[:begin] + edit.NewText + edited[begin+3:]
		for _, d := range analyze("", accepted) {
			if d.Severity == 1 {
				t.Fatalf("accepted completion invalid: %+v", d)
			}
		}
		start = i + 1
	}
}

// Shared views offer only shared requirements; exclusive views expose the
// mutable requirement with its exact borrowed parameter and concrete error.
// Rules: rules/declarations/interfaces.md — §§3,7.1,9; MD-120 §6.
func TestGenericErrorInterfaceReceiverCapabilities(t *testing.T) {
	source := genericErrorInterfaceFixture(t, "receivers")
	uri := "file:///tmp/sec-generic-error-interface/receivers.sec"
	at := strings.Index(source, "view.Read()")
	items := completeSource(uri, source, at+len("view."))
	found := false
	for _, item := range items {
		if item.Label == "Write" {
			t.Fatal("shared reference offers mutable receiver", items)
		}
		found = found || item.Label == "Read"
	}
	if !found {
		t.Fatal("shared requirement absent", items)
	}
	at = strings.Index(source, "view.Write(1, other)")
	items = completeSource(uri, source, at+len("view."))
	found = false
	for _, item := range items {
		found = found || item.Label == "Write"
	}
	if !found {
		t.Fatal("mutable requirement absent", items)
	}
	help, ok := signatureHelpForSource(uri, source, offsetPosition(source, at+len("view.Write(1, ")))
	if !ok || help.ActiveParameter != 1 || len(help.Signatures) != 1 || help.Signatures[0].Label != "mut fn Write(first: int, second: ref int) Result[void, IOError]" || len(help.Signatures[0].Parameters) != 2 {
		t.Fatalf("mutable signature: %v %+v", ok, help)
	}
}

// MD-120 never supplies a default error argument or accepts a non-error type
// merely to make editor features available. The error root remains explicit.
// Rules: rules/declarations/generics.md — §§14,15A; MD-120 §§3,6,7.
func TestGenericErrorInterfaceDiagnostics(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/generic_error_parameters/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	values := analyze("file:///tmp/sec-generic-error-interface/invalid.sec", string(data))
	for _, want := range []string{"requires 1 generic arguments", "does not satisfy constraint error for E"} {
		found := false
		for _, d := range values {
			found = found || d.Severity == 1 && strings.Contains(d.Message, want)
		}
		if !found {
			t.Fatalf("missing %q: %+v", want, values)
		}
	}
	if len(values) != 2 {
		t.Fatalf("unexpected diagnostics: %+v", values)
	}
}
