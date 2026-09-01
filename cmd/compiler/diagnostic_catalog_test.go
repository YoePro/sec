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
		{name: "definition", typ: reflect.TypeOf(diagnostics.Definition{}), fields: diagnosticDefinitionFields},
		{name: "semantic occurrence", typ: reflect.TypeOf(sema.Error{}), fields: semanticOccurrenceFields},
		{name: "parser occurrence", typ: reflect.TypeOf(parser.Diagnostic{}), fields: parserOccurrenceFields},
		{name: "token", typ: reflect.TypeOf(lexer.Token{}), fields: diagnosticTokenFields},
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
	if catalog.Summary.Retired != 1 {
		t.Fatalf("retired definitions = %d, want 1", catalog.Summary.Retired)
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
	if err := runDiagnosticCatalogCommand([]string{"--yaml"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unknown diagnostics argument to fail")
	}
}
