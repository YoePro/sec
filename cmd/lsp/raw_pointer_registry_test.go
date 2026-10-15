package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// Completion consumes Sema's applicable canonical surface, marking every raw
// operation unsafe while retaining volatile effects and nominal result types.
// Rules: rules/memory/raw_pointers.md — §§3(4),11–14,35(1–3),(6).
func TestRawPointerRegistryCompletionParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_registry_completion_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, test := range []struct {
		name, element string
		kind          sema.TypeKind
	}{
		{"bytes", "byte", sema.UintType}, {"ints", "int", sema.IntType}, {"opaque", "void", sema.VoidType}, {"readings", "Reading", sema.IntType}, {"shared", "byte", sema.UintType}, {"mutable", "byte", sema.UintType},
	} {
		offset := strings.Index(source, test.name+".\n") + len(test.name) + 1
		items := completeSource("file:///tmp/sec-raw-registry-completion/main.sec", source, offset)
		expected := map[string]sema.CompilerKnownMember{}
		pointer := sema.Type{Name: "RawPtr", Kind: sema.RawPtrType, TypeArgs: []sema.Type{{Name: test.element, Kind: test.kind}}}
		if test.name == "shared" || test.name == "mutable" {
			underlying := pointer
			pointer = sema.Type{Kind: sema.ReferenceType, Element: &underlying, ReferenceMutable: test.name == "mutable"}
		}
		for _, member := range sema.CompilerKnownMembersForType(pointer, false) {
			if strings.HasPrefix(member.ID, "CKM-RAWPTR-") {
				expected[member.Name] = member
			}
		}
		for _, name := range []string{"Read", "Write", "VolatileRead", "VolatileWrite", "Offset", "AddBytes", "Difference"} {
			member, wanted := expected[name]
			matches := 0
			for _, item := range items {
				if item.Label != name {
					continue
				}
				matches++
				if !wanted {
					t.Errorf("%s offers inapplicable %s", test.name, name)
					continue
				}
				detail := "unsafe " + lspTypeName(member.Result)
				if len(member.Effects) > 0 {
					detail += " (effectful)"
				}
				if item.Detail != detail {
					t.Errorf("%s.%s detail = %q, want %q", test.name, name, item.Detail, detail)
				}
			}
			if wanted && matches != 1 || !wanted && matches != 0 {
				t.Errorf("%s.%s matches = %d, wanted = %v", test.name, name, matches, wanted)
			}
		}
	}
}

// All ordinary, volatile and arithmetic operations navigate and hover through
// the same receiver-instantiated registry used by semantic analysis and API docs.
// Rules: rules/memory/raw_pointers.md — §35(1–3),(6);
// rules/compiler/compiler_known_members.md — "Synthetic definitions", "Documentation generation".
func TestRawPointerRegistryHoverAndDefinitionParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_registry_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const uri = "file:///tmp/sec-raw-registry/main.sec"
	path := pathFromURI(uri)
	program := parseProgramForLSP(uri, source)
	_, analyzer := analyzeLSPDocument(uri, program, sourceOverlay{})
	scanner := lexer.NewWithFile(source, path)
	previous := lexer.Token{}
	count := 0
	for token := scanner.NextToken(); token.Type != lexer.EOF; token = scanner.NextToken() {
		if previous.Type == lexer.DOT {
			member, found := analyzer.CompilerKnownMemberAt(path, token.Line, token.Column)
			if !found {
				t.Fatalf("missing semantic member for %s at %d:%d", token.Lexeme, token.Line, token.Column)
			}
			pos := offsetPosition(source, token.ByteStart)
			hover, found := hoverForSource(uri, source, pos)
			if !found || !strings.Contains(hover.Contents.Value, member.Signature) || !strings.Contains(hover.Contents.Value, member.ID) {
				t.Fatalf("hover diverged from registry: %+v; member=%+v", hover, member)
			}
			definitions := definitionsForSource(uri, source, pos)
			if len(definitions) != 1 {
				t.Fatalf("%s definitions: %+v", member.ID, definitions)
			}
			text, found := compilerKnownDefinitionText(definitions[0].URI)
			if !found || text != sema.CompilerKnownSyntheticDefinition(member) {
				t.Fatalf("%s definition diverged: %s", member.ID, text)
			}
			count++
		}
		previous = token
	}
	if count != 25 {
		t.Fatalf("exercised %d calls, want 25", count)
	}
}
