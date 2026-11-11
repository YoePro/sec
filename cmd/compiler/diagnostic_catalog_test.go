package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func TestDiagnosticCatalogSchemasContainEveryStructField(t *testing.T) {
	tests := []struct {
		name   string
		typ    reflect.Type
		fields []diagnosticCatalogField
	}{
		{name: "definition", typ: reflect.TypeOf(diagnosticCatalogDefinition{}), fields: diagnosticDefinitionFields},
		{name: "semantic occurrence", typ: reflect.TypeOf(sema.Error{}), fields: semanticOccurrenceFields},
		{name: "parser occurrence", typ: reflect.TypeOf(parser.Diagnostic{}), fields: parserOccurrenceFields},
		{name: "token", typ: reflect.TypeOf(lexer.Token{}), fields: diagnosticTokenFields},
		{name: "emitted occurrence", typ: reflect.TypeOf(emittedOccurrence{}), fields: emittedOccurrenceFields},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.fields) != test.typ.NumField() {
				t.Fatalf("catalog has %d fields, %s has %d", len(test.fields), test.typ.Name(), test.typ.NumField())
			}
			for index, field := range test.fields {
				if field.Name != test.typ.Field(index).Name {
					t.Fatalf("catalog field %d = %q, want %q", index, field.Name, test.typ.Field(index).Name)
				}
				if field.Type == "" || field.Description == "" {
					t.Fatalf("catalog field %s is missing type or description", field.Name)
				}
			}
		})
	}
}

func TestBuildDiagnosticCatalogContainsEveryRegisteredDefinition(t *testing.T) {
	catalog := buildDiagnosticCatalog()
	definitions := diagnostics.All()
	if len(catalog.Definitions) != len(definitions) {
		t.Fatalf("catalog definitions = %d, registry definitions = %d", len(catalog.Definitions), len(definitions))
	}
	if catalog.Summary.Total != len(definitions) {
		t.Fatalf("catalog total = %d, want %d", catalog.Summary.Total, len(definitions))
	}
	if catalog.Coverage.Complete {
		t.Fatal("catalog must report the current raw-diagnostic migration gap")
	}

	for index, definition := range definitions {
		entry := catalog.Definitions[index]
		if entry.ID != definition.ID ||
			entry.Name != definition.Name ||
			entry.Family != definition.Family ||
			entry.DefaultSeverity != definition.DefaultSeverity ||
			entry.Mandatory != definition.Mandatory ||
			entry.Retired != definition.Retired {
			t.Fatalf("catalog entry %d does not preserve every definition field: %#v != %#v", index, entry, definition)
		}
	}

	if got := catalog.Summary.Errors + catalog.Summary.Warnings + catalog.Summary.Information; got != catalog.Summary.Total {
		t.Fatalf("severity counts sum to %d, want %d", got, catalog.Summary.Total)
	}
	if catalog.Summary.Active+catalog.Summary.Retired != catalog.Summary.Total {
		t.Fatalf("status counts do not cover every definition: active=%d retired=%d total=%d", catalog.Summary.Active, catalog.Summary.Retired, catalog.Summary.Total)
	}
	// S1020 and the associated-static advisory, S1152 retired by MD-029 and
	// S1069 retired by MD-131.
	if catalog.Summary.Retired != 4 {
		t.Fatalf("retired definitions = %d, want 4", catalog.Summary.Retired)
	}
}

func TestDiagnosticCatalogTextAndJSONOutput(t *testing.T) {
	var textOutput bytes.Buffer
	if err := runDiagnosticCatalogCommand(nil, &textOutput); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"DEFINITION FIELDS",
		"SEMANTIC OCCURRENCE FIELDS",
		"PARSER OCCURRENCE FIELDS",
		"TOKEN FIELDS",
		"DIAGNOSTIC DEFINITIONS",
		"DEFAULT_SEVERITY",
		"RETIRED",
		diagnostics.ParserSyntaxError,
		diagnostics.OperatorInvalidMembership,
		diagnostics.OperatorInvalidConcatOperand,
		diagnostics.RedundantAssociatedStatic,
		diagnostics.LargeValueParameter,
	} {
		if !strings.Contains(textOutput.String(), required) {
			t.Fatalf("text catalog is missing %q", required)
		}
	}

	var jsonOutput bytes.Buffer
	if err := runDiagnosticCatalogCommand([]string{"--json"}, &jsonOutput); err != nil {
		t.Fatal(err)
	}
	var decoded diagnosticCatalog
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid catalog JSON: %v", err)
	}
	if !reflect.DeepEqual(decoded, buildDiagnosticCatalog()) {
		t.Fatal("JSON catalog does not preserve the complete catalog")
	}
}

func TestDiagnosticCatalogRejectsUnknownArguments(t *testing.T) {
	if err := runDiagnosticCatalogCommand([]string{"--yaml"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unknown argument") {
		t.Fatalf("unknown diagnostics argument error = %v", err)
	}
}

// Detail lookup must expose the same registry-owned identity fields as the
// complete catalog without introducing a second diagnostic data source.
//
// Rules:
//   - rules/tooling/diagnostics.md — §13 "Canonical diagnostic commands"
//   - rules/compiler/compiler_testing.md — §11(5)–(8)
func TestDiagnosticCatalogDetailMatchesRegisteredDefinition(t *testing.T) {
	definition, ok := diagnostics.Lookup(diagnostics.UnreachableStatement)
	if !ok {
		t.Fatal("S3001 is not registered")
	}

	var output bytes.Buffer
	if err := runDiagnosticCatalogCommand([]string{definition.ID}, &output); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	// The definition fields form the first block; extended explanation
	// sections (§ 13(6)) follow after a blank line.
	header, _, _ := strings.Cut(output.String(), "\n\n")
	for _, line := range strings.Split(strings.TrimSpace(header), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid detail line %q", line)
		}
		got[fields[0]] = fields[1]
	}
	want := map[string]string{
		"DIAGNOSTIC":       definition.ID,
		"NAME":             definition.Name,
		"FAMILY":           definition.Family,
		"DEFAULT_SEVERITY": string(definition.DefaultSeverity),
		"MANDATORY":        "true",
		"CONFIGURABLE":     "false",
		"STATUS":           "active",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detail fields = %#v, want %#v", got, want)
	}
}

func TestDiagnosticCatalogDetailRejectsUnknownIDAndExtraArguments(t *testing.T) {
	if err := runDiagnosticCatalogCommand([]string{"S9999"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unknown diagnostic ID") {
		t.Fatalf("unknown ID error = %v", err)
	}
	if err := runDiagnosticCatalogCommand([]string{diagnostics.UnreachableStatement, "--json"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected mixed detail and JSON arguments to fail")
	}
}

// `sec diagnostics S3001` renders the registry-owned extended sections and
// the JSON catalog carries the same explanation; a diagnostic without one
// keeps the plain detail and omits the JSON field.
// Rules: rules/tooling/diagnostics.md — §13(4)–(6).
func TestDiagnosticCatalogDetailIncludesExtendedExplanation(t *testing.T) {
	var out bytes.Buffer
	if err := runDiagnosticCatalogCommand([]string{diagnostics.UnreachableStatement}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DESCRIPTION\n", "EXPLANATION\n", "COMMON CAUSES\n", "VALID ALTERNATIVES\n", "EXAMPLE\n", "let x := 2   // S3001", "RELATED\nS1045, A2008", "SINCE\nSec 0.1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("S3001 detail lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := runDiagnosticCatalogCommand([]string{diagnostics.UseAfterDiscard}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "DESCRIPTION") {
		t.Errorf("unexplained diagnostic rendered extended sections:\n%s", out.String())
	}

	out.Reset()
	if err := runDiagnosticCatalogCommand([]string{"--json"}, &out); err != nil {
		t.Fatal(err)
	}
	var catalog diagnosticCatalog
	if err := json.Unmarshal(out.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, definition := range catalog.Definitions {
		_, explained := diagnostics.Explain(definition.ID)
		if explained != (definition.Explanation != nil) {
			t.Errorf("%s: JSON explanation presence %v, registry %v", definition.ID, definition.Explanation != nil, explained)
		}
		if definition.ID == diagnostics.UnreachableStatement && (definition.Explanation == nil || len(definition.Explanation.Causes) == 0) {
			t.Errorf("S3001 JSON explanation incomplete: %+v", definition.Explanation)
		}
	}
}
