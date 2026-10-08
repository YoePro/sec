package formatter

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestFormatPreservesEveryDefaultForm verifies explicit expressions, omitted
// fields and derived defaults survive formatting without source expansion.
// Rules: rules/types/default_values.md — "Formatter", "LSP";
// rules/tooling/formatter.md — canonical model and type declarations.
func TestFormatPreservesEveryDefaultForm(t *testing.T) {
	input, err := os.ReadFile("../../testdata/defaults/default_presentation.sec")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../testdata/defaults/default_presentation.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string
		Value  *string
		Source sema.DefaultKind
	}
	if err := json.Unmarshal(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	original := string(input)
	formatted := Format(Source{Text: original}, Options{}).Text
	if again := Format(Source{Text: formatted}, Options{}).Text; again != formatted {
		t.Fatal("defaults are not a formatting fixed point")
	}
	tokens := func(source string) []string {
		var values []string
		lex := lexer.New(source)
		for token := lex.NextToken(); token.Type != lexer.EOF; token = lex.NextToken() {
			values = append(values, string(token.Type)+":"+token.Lexeme)
		}
		return values
	}
	if !reflect.DeepEqual(tokens(original), tokens(formatted)) {
		t.Fatal("formatting changed default expressions or expanded omitted fields")
	}
	for _, source := range []string{original, formatted} {
		parsed := parser.New(lexer.New(source)).Parse()
		if parsed.HasErrors {
			t.Fatal(parsed.Diagnostics)
		}
		analyzer := sema.NewAnalyzer()
		if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, entry := range cases {
			typ, exists := analyzer.Types()[entry.Name]
			if !exists {
				t.Fatal("missing type", entry.Name)
			}
			value, kind, ok := sema.DefaultValuePreview(typ, 8)
			if entry.Value == nil {
				if ok {
					t.Fatal(entry.Name, "fabricated default", value)
				}
				continue
			}
			if !ok || value != *entry.Value || kind != entry.Source {
				t.Fatalf("%s: %q/%s/%v, want %q/%s", entry.Name, value, kind, ok, *entry.Value, entry.Source)
			}
		}
	}
}
