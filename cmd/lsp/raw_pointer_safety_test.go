package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// Every raw operation projects the same unsafe requirement and caller contract
// into hover and signature help, including nominal and borrowed receivers.
// Rules: rules/memory/raw_pointers.md — §§5.2,11–15,35(2–6);
// rules/tooling/lsp.md — "Hover", "Signature help".
func TestRawPointerSafetyHoverAndSignatureHelp(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_registry_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/sec-raw-safety/main.sec"
	path := pathFromURI(uri)
	_, analyzer := analyzeLSPDocument(uri, parseProgramForLSP(uri, source), sourceOverlay{})
	tokens := sourceTokens(uri, source)
	count := 0
	for index, token := range tokens {
		if index == 0 || tokens[index-1].Type != lexer.DOT {
			continue
		}
		member, found := analyzer.CompilerKnownMemberAt(path, token.Line, token.Column)
		if !found {
			t.Fatalf("missing member: %+v", token)
		}
		hover, found := hoverForSource(uri, source, offsetPosition(source, token.ByteStart))
		if !found {
			t.Fatalf("missing hover for %s", member.ID)
		}
		if index+1 >= len(tokens) || tokens[index+1].Type != lexer.LPAREN {
			t.Fatal("fixture call has no argument opener")
		}
		help, found := signatureHelpForSource(uri, source, offsetPosition(source, tokens[index+1].ByteEnd))
		if !found || len(help.Signatures) != 1 {
			t.Fatalf("missing signature for %s: %+v", member.ID, help)
		}
		signature := help.Signatures[0]
		assertNoCompilerKnownDisplayIDs(t, hover.Contents.Value, signature.Label, signature.Documentation.Value)
		if signature.Label != member.Signature || len(signature.Parameters) != len(member.Parameters) || help.ActiveSignature != 0 || help.ActiveParameter != 0 {
			t.Fatalf("signature diverged from registry: %+v; member=%+v", help, member)
		}
		for i, parameter := range member.Parameters {
			if signature.Parameters[i].Label != initializerParameterLabel(parameter) {
				t.Fatalf("lost parameter type: %+v", signature.Parameters)
			}
		}
		for _, document := range []string{hover.Contents.Value, signature.Documentation.Value} {
			if !strings.Contains(document, "Requires an `unsafe` context.") || !strings.Contains(document, "Caller obligations (unsafe does not prove these):") {
				t.Fatalf("missing unsafe distinction for %s: %s", member.ID, document)
			}
			for _, obligation := range member.CallerObligations {
				if !strings.Contains(document, obligation) {
					t.Fatalf("lost obligation %q for %s", obligation, member.ID)
				}
			}
		}
		count++
	}
	if count != 25 {
		t.Fatalf("covered %d raw calls, want 25", count)
	}
}

// Outside an unsafe context the same requirements remain visible, while
// Sema still reports the missing context instead of treating tooling as proof.
// Rules: rules/memory/raw_pointers.md — §§5.2(2–3),6(2–3),35(5).
func TestRawPointerSafetyOutsideUnsafeContext(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_safety_context_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/sec-raw-safety-context/main.sec"
	offset := strings.Index(source, "pointer.Write") + len("pointer.")
	hover, found := hoverForSource(uri, source, offsetPosition(source, offset))
	if !found || !strings.Contains(hover.Contents.Value, "Requires an `unsafe` context.") || !strings.Contains(hover.Contents.Value, "exclusive/mutable authority") {
		t.Fatalf("missing caller requirements: %+v", hover)
	}
	help, found := signatureHelpForSource(uri, source, offsetPosition(source, offset+len("Write(")))
	if !found || !strings.Contains(help.Signatures[0].Documentation.Value, "unsafe does not prove these") {
		t.Fatalf("missing unsafe signature contract: %+v", help)
	}
	diagnostics := analyze(uri, source)
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "requires unsafe") {
		t.Fatalf("tooling changed unsafe checking: %+v", diagnostics)
	}
}

// Nested calls select the inner API; grouping, strings and comments cannot
// turn into new call boundaries or leak an outer raw-pointer signature.
// Rules: rules/tooling/lsp.md — "Signature help";
// rules/compiler/parser_recovery.md — "Enclosing-boundary safety".
func TestRawPointerSignatureCallSelection(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_signature_nested_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/sec-raw-nested/main.sec"
	for _, test := range []struct{ marker, want string }{
		{"pointer.Write(other.Read(", "unsafe fn Read() byte"},
		{"pointer.Write(Echo(", ""},
		{"pointer.Offset(Count(", ""},
		{"pointer.Write((1u", "unsafe fn Write(value: byte) void"},
		{"/* 😀 */ pointer.Write(1u", "unsafe fn Write(value: byte) void"},
		{"reader.Read(", ""},
		{"pointer.Write(/* , ) */ ", "unsafe fn Write(value: byte) void"},
		{"new Box(", "init(value: byte)"},
		{"pointer.Write(other.Read())", ""},
	} {
		start := strings.Index(source, test.marker)
		if start < 0 {
			t.Fatalf("fixture missing %q", test.marker)
		}
		help, found := signatureHelpForSource(uri, source, offsetPosition(source, start+len(test.marker)))
		if test.want == "" {
			if found {
				t.Errorf("%q leaked an outer signature: %+v", test.marker, help)
			}
		} else if !found || len(help.Signatures) != 1 || help.Signatures[0].Label != test.want {
			t.Errorf("%q: %+v, %v; want %q", test.marker, help, found, test.want)
		}
	}
}

// An unclosed raw call remains useful at EOF, but its virtual ')' does not own
// a following statement or foreign closing brace.
// Rules: rules/compiler/parser_recovery.md — "Argument-list recovery", "Enclosing-boundary safety";
// rules/tooling/lsp.md — "Signature help", "Invalid regions".
func TestRawPointerSignatureIncompleteBoundaries(t *testing.T) {
	for _, test := range []struct{ file, marker, want string }{
		{"raw_pointer_signature_eof_invalid.sec", "", "unsafe fn Write(value: byte) void"},
		{"raw_pointer_signature_statement_invalid.sec", "return", ""},
		{"raw_pointer_signature_closer_invalid.sec", "}", ""},
		{"raw_pointer_signature_unknown_invalid.sec", "Fake(", ""},
		{"raw_pointer_registry_invalid.sec", "opaque.Read(", ""},
		{"raw_pointer_signature_unknown_invalid.sec", "@unknown(", ""},
		{"raw_pointer_signature_conversion_invalid.sec", "byte<kg>(", ""},
		{"raw_pointer_signature_conversion_invalid.sec", "bit[8](", ""},
	} {
		data, err := os.ReadFile("../../testdata/tooling/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		offset := len(source)
		if test.marker != "" {
			offset = strings.Index(source, test.marker)
			if offset < 0 {
				t.Fatalf("missing marker %q", test.marker)
			}
			if test.marker != "return" && test.marker != "}" {
				offset += len(test.marker)
			}
		}
		help, found := signatureHelpForSource("file:///tmp/sec-raw-incomplete/"+test.file, source, offsetPosition(source, offset))
		if test.want == "" && found {
			t.Errorf("%s %q leaked signature: %+v", test.file, test.marker, help)
		}
		if test.want != "" && (!found || help.Signatures[0].Label != test.want) {
			t.Errorf("%s: %+v %v", test.file, help, found)
		}
	}
}
