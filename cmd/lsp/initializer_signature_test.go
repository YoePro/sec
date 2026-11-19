package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// initializerSignatureFixture validates the complete source before exercising
// recovery on the same parser/compiler-owned lifecycle declarations.
// Rules: rules/declarations/impl.md — §§15–16; rules/tooling/lsp.md — §13.
func initializerSignatureFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/initializer_signatures.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, diagnostic := range analyze("", source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid baseline: %+v", diagnostic)
		}
	}
	return source
}

// TestInitializerSignatureRecovery verifies declaration previews without
// mistaking incomplete/default recovery for a selected zero-argument init.
// Rules: rules/tooling/lsp.md — §§4.1(22),13, "Recovery nodes";
// rules/declarations/impl.md — §§15.4,15.6.
func TestInitializerSignatureRecovery(t *testing.T) {
	baseline := initializerSignatureFixture(t)
	prefix := baseline[:strings.Index(baseline, "new Widget(1)")]
	want := []string{"init(value: int)", "init(value: ref int)", "init(first: int, label: string)", "implicit init()"}
	for _, test := range []struct {
		name, tail string
		active     int
	}{
		{"empty-eof", "new Widget(", 0},
		{"unfinished-name", "new Widget(unknown", 0},
		{"next-argument", "new Widget(1, ", 1},
		{"nested-call-completed", "new Widget(Call(1, 2), ", 1},
		{"comment-delimiters", "new Widget(1, /* , ) */ ", 1},
		{"unicode-prefix", "/* 😀 */ new Widget(1, ", 1},
		{"excess-arguments", "new Widget(1, 2, 3, ", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := prefix + test.tail
			help, ok := signatureHelpForSource("", source, offsetPosition(source, len(source)))
			if !ok {
				t.Fatal("no candidate signature help")
			}
			labels := []string{}
			for _, signature := range help.Signatures {
				labels = append(labels, signature.Label)
				if !strings.Contains(signature.Documentation.Value, "no overload selection is established") || !strings.Contains(signature.Documentation.Value, "Constructs: `Widget`") {
					t.Fatalf("candidate represented as resolved: %+v", signature)
				}
			}
			if !reflect.DeepEqual(labels, want) || help.ActiveParameter != test.active {
				t.Fatalf("help = %+v; want %v / active %d", help, want, test.active)
			}
			fallible := help.Signatures[2]
			if !strings.Contains(fallible.Documentation.Value, "Construction error: `InitError`") || !strings.Contains(fallible.Documentation.Value, "handling: `yes`") {
				t.Fatal("lost construction error contract", fallible)
			}
		})
	}
}

// TestInitializerSignatureOverlayAndExplicitDefault checks unsaved sibling
// declarations and document versions through the existing analysis cache. The
// explicit zero-arity overload must suppress the implicit preview.
// Rules: rules/tooling/lsp.md — "Snapshots", §13;
// rules/declarations/impl.md — §15.6.
func TestInitializerSignatureOverlayAndExplicitDefault(t *testing.T) {
	baseline := initializerSignatureFixture(t)
	declarations := baseline[:strings.Index(baseline, "fn Make()")]
	use := "module sample\n" + baseline[strings.Index(baseline, "fn Make()"):strings.Index(baseline, "new Widget(1)")] + "new Widget("
	dir := t.TempDir()
	declarationPath := filepath.Join(dir, "widget.sec")
	usePath := filepath.Join(dir, "make.sec")
	for path, text := range map[string]string{declarationPath: declarations, usePath: use} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	query := func(text string) signatureHelp {
		t.Helper()
		overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(declarationPath): text}}
		help, ok := signatureHelpForSource(uriFromPath(usePath), use, offsetPosition(use, len(use)), overlay)
		if !ok {
			t.Fatal("no cross-file construction help")
		}
		return help
	}
	first := query(declarations)
	if len(first.Signatures) != 4 || first.Signatures[0].Label != "init(value: int)" {
		t.Fatalf("wrong original candidates: %+v", first)
	}
	changed := strings.Replace(declarations, "init(value: int)", "init(changed: int)", 1)
	changed = strings.Replace(changed, "self.value = value", "self.value = 0", 1)
	changed = strings.Replace(changed, "impl Widget {", "impl Widget {\n    init() { self.value = 0 }\n", 1)
	second := query(changed)
	if len(second.Signatures) != 4 || second.Signatures[0].Label != "init()" || second.Signatures[1].Label != "init(changed: int)" {
		t.Fatalf("unsaved overloads are stale: %+v", second)
	}
	for _, signature := range second.Signatures {
		if signature.Label == "implicit init()" {
			t.Fatal("explicit zero-arity init did not suppress implicit preview")
		}
	}
	if again := query(declarations); !reflect.DeepEqual(first, again) {
		t.Fatal("overlay switch did not restore deterministic original candidates")
	}
}

// TestInitializerSignatureSelectionAndBoundaries keeps complete selections and
// parser-owned nested/following statement boundaries distinct from previews.
// Rules: rules/tooling/lsp.md — §13, "Invalid regions", "Snapshots".
func TestInitializerSignatureSelectionAndBoundaries(t *testing.T) {
	baseline := initializerSignatureFixture(t)
	prefix := baseline[:strings.Index(baseline, "new Widget(1)")]
	for _, test := range []struct {
		tail string
		want string
	}{
		{"new Widget(1)", "init(value: int)"},
		{"new Widget()", "init()"},
		{"new Widget(1, \"text\")", "init(first: int, label: string)"},
	} {
		source := prefix + test.tail + "\n}\n"
		cursor := len(prefix) + len(test.tail) - 1
		help, ok := signatureHelpForSource("", source, offsetPosition(source, cursor))
		if !ok || len(help.Signatures) != 1 || help.Signatures[0].Label != test.want || strings.Contains(help.Signatures[0].Documentation.Value, "candidate") {
			t.Fatalf("selected %s: %+v, %v", test.tail, help, ok)
		}
	}
	for _, tail := range []string{"new Widget(Call(", "new Unknown(", "new Widget(1)\n    let next := ", "new Widget(\n}\nfn After() void {\n    let next := "} {
		source := prefix + tail
		if help, ok := signatureHelpForSource("", source, offsetPosition(source, len(source))); ok {
			t.Fatalf("outer/unknown help leaked into %q: %+v", tail, help)
		}
	}
	for _, tail := range []string{"new Widget(true)", "new Widget(new Other("} {
		source := prefix + tail
		cursor := len(source)
		if strings.HasSuffix(tail, ")") {
			cursor--
		}
		help, ok := signatureHelpForSource("", source, offsetPosition(source, cursor))
		if !ok || !strings.Contains(help.Signatures[0].Documentation.Value, "candidate") {
			t.Fatalf("unresolved candidate missing in %q: %+v, %v", tail, help, ok)
		}
		if strings.Contains(tail, "new Other") && help.Signatures[0].Label != "init(flag: bool)" {
			t.Fatalf("nested constructor selected outer type: %+v", help)
		}
	}
}
