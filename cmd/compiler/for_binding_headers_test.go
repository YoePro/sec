package main

import "testing"

// TestForBindingHeadersCLI uses the shared canonical header golden and checks
// that all binding modes still type-check through the compiler entry point.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10;
// rules/tooling/formatter.md — §§6,28–29.
func TestForBindingHeadersCLI(t *testing.T) {
	assertFormattingGoldenCLI(t, "for_binding_headers")
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/formatter/for_binding_headers.expected.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
}
