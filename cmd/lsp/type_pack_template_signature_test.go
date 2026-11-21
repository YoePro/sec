package main

import (
	"strings"
	"testing"
)

// MD-117 call-site tooling (correction § 10(3)): while a pack call is
// incomplete or unresolved, or the cursor is past the positions Sema selected,
// signature help shows the declared template with the pack parameter active
// for every trailing position instead of nothing or a stale prefix.
// Rules: rules/declarations/functions.md — § 38A(10)(3); rules/tooling/lsp.md — "Signature help".
func TestTypePackTemplateSignatureHelp(t *testing.T) {
	declaration := "module main\n\nfn Trace[...Types](prefix: string, values: ...ref Types) void {}\n\nfn Use() void {\n"
	cases := []struct {
		name, call string
		active     int
	}{
		{"open call", "    Trace(", 0},
		{"incomplete trailing position", "    Trace(\"a\", 1, ", 1},
		{"unresolved argument", "    Trace(\"a\", missing", 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			source := declaration + test.call + "\n}\n"
			help, ok := signatureHelpForSource("file:///tmp/template.sec", source, position{Line: 5, Character: len(test.call)})
			if !ok || len(help.Signatures) != 1 {
				t.Fatalf("help = %+v, %v", help, ok)
			}
			signature := help.Signatures[0]
			if signature.Label != "fn Trace[...Types](prefix: string, values: ...ref Types) void" || help.ActiveParameter != test.active ||
				len(signature.Parameters) != 2 || !strings.Contains(signature.Documentation.Value, "inferred from the trailing arguments") {
				t.Fatalf("help = %+v", help)
			}
		})
	}
	// A complete resolved call keeps the concrete receiving signature.
	source := declaration + "    Trace(\"a\", 1, true)\n}\n"
	help, ok := signatureHelpForSource("file:///tmp/template.sec", source, position{Line: 5, Character: len("    Trace(\"a\", 1, t")})
	if !ok || help.Signatures[0].Label != "fn Trace(prefix: string, values[0]: ref int, values[1]: ref bool) void" || help.ActiveParameter != 2 {
		t.Fatalf("resolved help = %+v", help)
	}
}
