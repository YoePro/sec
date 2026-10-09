package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"sec/internal/diagnostics"
)

type diagnosticCatalogField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type diagnosticCatalogDefinition struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Family          string               `json:"family"`
	DefaultSeverity diagnostics.Severity `json:"default_severity"`
	Mandatory       bool                 `json:"mandatory"`
	Retired         bool                 `json:"retired"`
}

type diagnosticCatalogSummary struct {
	Total       int `json:"total"`
	Active      int `json:"active"`
	Retired     int `json:"retired"`
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Information int `json:"information"`
}

type diagnosticCatalogCoverage struct {
	Complete bool   `json:"complete"`
	Note     string `json:"note"`
}

type diagnosticCatalog struct {
	Summary                  diagnosticCatalogSummary      `json:"summary"`
	Coverage                 diagnosticCatalogCoverage     `json:"coverage"`
	DefinitionFields         []diagnosticCatalogField      `json:"definition_fields"`
	SemanticOccurrenceFields []diagnosticCatalogField      `json:"semantic_occurrence_fields"`
	ParserOccurrenceFields   []diagnosticCatalogField      `json:"parser_occurrence_fields"`
	TokenFields              []diagnosticCatalogField      `json:"token_fields"`
	EmittedOccurrenceFields  []diagnosticCatalogField      `json:"emitted_occurrence_fields"`
	Definitions              []diagnosticCatalogDefinition `json:"definitions"`
}

var diagnosticDefinitionFields = []diagnosticCatalogField{
	{Name: "ID", Type: "string", Required: true, Description: "Stable diagnostic identifier."},
	{Name: "Name", Type: "string", Required: true, Description: "Stable symbolic diagnostic name."},
	{Name: "Family", Type: "string", Required: true, Description: "Diagnostic rule family."},
	{Name: "DefaultSeverity", Type: "Severity", Required: true, Description: "Default error, warning, or information classification."},
	{Name: "Mandatory", Type: "bool", Required: true, Description: "Whether configuration may suppress or demote the diagnostic."},
	{Name: "Retired", Type: "bool", Required: true, Description: "Whether the stable identifier is reserved for a rule that is no longer emitted."},
}

var semanticOccurrenceFields = []diagnosticCatalogField{
	{Name: "ID", Type: "string", Required: false, Description: "Registered diagnostic identifier; empty on an unmigrated diagnostic."},
	{Name: "Severity", Type: "Severity", Required: true, Description: "Effective severity for this occurrence."},
	{Name: "ProofState", Type: "ProofState", Required: false, Description: "Explicit owner-supplied Valid, Invalid or Unproven; never inferred from severity or message."},
	{Name: "Help", Type: "string", Required: false, Description: "Actionable correction or next step."},
	{Name: "Message", Type: "string", Required: true, Description: "Rendered primary diagnostic message."},
	{Name: "File", Type: "string", Required: false, Description: "Primary source file."},
	{Name: "Line", Type: "int", Required: false, Description: "One-based primary source line."},
	{Name: "Column", Type: "int", Required: false, Description: "One-based primary source column."},
	{Name: "EndLine", Type: "int", Required: false, Description: "One-based final source line of the primary range."},
	{Name: "EndColumn", Type: "int", Required: false, Description: "One-based exclusive final source column of the primary range."},
	{Name: "PreviousFile", Type: "string", Required: false, Description: "Related previous declaration source file."},
	{Name: "PreviousLine", Type: "int", Required: false, Description: "One-based related source line."},
	{Name: "PreviousColumn", Type: "int", Required: false, Description: "One-based related source column."},
	{Name: "RelatedLabel", Type: "string", Required: false, Description: "Label of the related location, such as interface requirement; empty means previous declaration."},
	{Name: "Related", Type: "[]RelatedLocation", Required: false, Description: "Additional ordered source locations; preserved alongside the previous-declaration link in CLI and LSP diagnostics."},
	{Name: "EscapeCauses", Type: "[]EscapeCausePath", Required: false, Description: "Ordered canonical origin-to-sink escape explanations with mode, destination, source-mapped steps and explicit incomplete coverage; transported as notes and related locations."},
	{Name: "AllocationCause", Type: "*AllocationCausePath", Required: false, Description: "Canonical synchronous allocation-policy witness with distinct definite/unknown evidence, root, call sites, introducing callee/operation and explicit incomplete source coverage; transported as ordered notes and navigable related locations."},
}

var parserOccurrenceFields = []diagnosticCatalogField{
	{Name: "Help", Type: "string", Required: false, Description: "Actionable help associated with a parser diagnostic."},
	{Name: "ID", Type: "string", Required: true, Description: "Registered parser diagnostic identifier."},
	{Name: "Message", Type: "string", Required: true, Description: "Rendered primary diagnostic message."},
	{Name: "Primary", Type: "Token", Required: true, Description: "Primary source token and location."},
	{Name: "Expected", Type: "[]TokenType", Required: false, Description: "Expected token kinds, when known."},
	{Name: "Unexpected", Type: "*Token", Required: false, Description: "Unexpected token, when known."},
	{Name: "Context", Type: "RecoveryContext", Required: true, Description: "Stable parser recovery context containing the occurrence."},
	{Name: "Episode", Type: "int", Required: true, Description: "Positive recovery episode number within the parse."},
}

// emittedOccurrenceFields describes the canonical emitted-occurrence document
// entries written by `--diagnostic-format json`, so the catalog and
// occurrence surfaces share one identity model.
//
// Rules:
//   - rules/tooling/diagnostics.md — §8 "Canonical logical occurrence schema"
//   - rules/tooling/diagnostics.md — §13(4) exported field schemas
//   - rules/tooling/diagnostics.md — §14 "Machine-readable emitted occurrences"
var emittedOccurrenceFields = []diagnosticCatalogField{
	{Name: "ID", Type: "*string", Required: true, Description: "Registered diagnostic identifier; null only on an unregistered migration-fallback occurrence."},
	{Name: "Name", Type: "*string", Required: true, Description: "Registered symbolic name for ID; null when unregistered."},
	{Name: "Unregistered", Type: "bool", Required: true, Description: "True for an unmigrated host or tool diagnostic that has no stable registered identity yet."},
	{Name: "Severity", Type: "Severity", Required: true, Description: "Effective error, warning, or information classification."},
	{Name: "Arguments", Type: "map[string]string", Required: true, Description: "Structured arguments named by the registered definition."},
	{Name: "Primary", Type: "*Location", Required: false, Description: "Primary source span with one-based start and exclusive end; null for project, build, target, or tool diagnostics."},
	{Name: "Related", Type: "[]RelatedLocation", Required: true, Description: "Ordered related source locations, such as an earlier declaration."},
	{Name: "Notes", Type: "[]Message", Required: true, Description: "Ordered explanatory notes."},
	{Name: "Help", Type: "[]Message", Required: true, Description: "Ordered actionable help messages with display text."},
	{Name: "Fixes", Type: "[]Fix", Required: true, Description: "Ordered structured fixes; empty until the fix schema is implemented."},
	{Name: "Message", Type: "Message", Required: true, Description: "Primary message key, arguments, and rendered display text."},
	{Name: "Source", Type: "string", Required: true, Description: "Producing stage: lexer, parser, sema, or the tool error kind."},
}

var diagnosticTokenFields = []diagnosticCatalogField{
	{Name: "Type", Type: "TokenType", Required: true, Description: "Token kind."},
	{Name: "Lexeme", Type: "string", Required: true, Description: "Original source lexeme."},
	{Name: "File", Type: "string", Required: false, Description: "Source file."},
	{Name: "Line", Type: "int", Required: true, Description: "One-based source line."},
	{Name: "Column", Type: "int", Required: true, Description: "One-based source column."},
	{Name: "EndLine", Type: "int", Required: true, Description: "One-based exclusive final source line."},
	{Name: "EndColumn", Type: "int", Required: true, Description: "One-based exclusive final Unicode-scalar column."},
	{Name: "ByteStart", Type: "int", Required: true, Description: "Zero-based inclusive UTF-8 byte offset in the original source."},
	{Name: "ByteEnd", Type: "int", Required: true, Description: "Zero-based exclusive UTF-8 byte offset in the original source."},
}

// runDiagnosticCatalogCommand exposes the canonical registry as either the
// complete stable catalog or one exact diagnostic definition. Detail lookup
// never invents prose outside the registry-owned definition.
//
// Rules:
//   - rules/tooling/diagnostics.md — §13 "Canonical diagnostic commands"
//   - rules/compiler/compiler_testing.md — §11(1)–(8) "Diagnostic registry and catalog conformance"
func runDiagnosticCatalogCommand(args []string, output io.Writer) error {
	switch len(args) {
	case 0:
		return writeDiagnosticCatalogText(output, buildDiagnosticCatalog())
	case 1:
		if args[0] != "--json" {
			if strings.HasPrefix(args[0], "-") {
				return fmt.Errorf("unknown argument %q; expected --json or one diagnostic ID", args[0])
			}
			return writeDiagnosticDefinitionDetail(output, args[0])
		}
		catalog := buildDiagnosticCatalog()
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	default:
		return fmt.Errorf("invalid diagnostics arguments; expected no arguments, --json, or one diagnostic ID")
	}
}

// writeDiagnosticDefinitionDetail renders one registry definition using the
// same fields exported by the complete text and JSON catalogs.
//
// Rules:
//   - rules/tooling/diagnostics.md — §13 "Canonical diagnostic commands"
//   - rules/compiler/compiler_testing.md — §11(5)–(8)
func writeDiagnosticDefinitionDetail(output io.Writer, id string) error {
	definition, ok := diagnostics.Lookup(id)
	if !ok {
		return fmt.Errorf("unknown diagnostic ID %q", id)
	}

	status := "active"
	if definition.Retired {
		status = "retired"
	}
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "DIAGNOSTIC\t%s\n", definition.ID)
	fmt.Fprintf(w, "NAME\t%s\n", definition.Name)
	fmt.Fprintf(w, "FAMILY\t%s\n", definition.Family)
	fmt.Fprintf(w, "DEFAULT_SEVERITY\t%s\n", definition.DefaultSeverity)
	fmt.Fprintf(w, "MANDATORY\t%t\n", definition.Mandatory)
	fmt.Fprintf(w, "CONFIGURABLE\t%t\n", !definition.Mandatory)
	fmt.Fprintf(w, "STATUS\t%s\n", status)
	return w.Flush()
}

func buildDiagnosticCatalog() diagnosticCatalog {
	catalog := diagnosticCatalog{
		Coverage: diagnosticCatalogCoverage{
			Complete: false,
			Note:     "The catalog contains every registered definition. Some parser, semantic, compiler, and build diagnostics still use generic or empty IDs pending migration.",
		},
		DefinitionFields:         append([]diagnosticCatalogField(nil), diagnosticDefinitionFields...),
		SemanticOccurrenceFields: append([]diagnosticCatalogField(nil), semanticOccurrenceFields...),
		ParserOccurrenceFields:   append([]diagnosticCatalogField(nil), parserOccurrenceFields...),
		TokenFields:              append([]diagnosticCatalogField(nil), diagnosticTokenFields...),
		EmittedOccurrenceFields:  append([]diagnosticCatalogField(nil), emittedOccurrenceFields...),
	}

	for _, definition := range diagnostics.All() {
		catalog.Definitions = append(catalog.Definitions, diagnosticCatalogDefinition{
			ID:              definition.ID,
			Name:            definition.Name,
			Family:          definition.Family,
			DefaultSeverity: definition.DefaultSeverity,
			Mandatory:       definition.Mandatory,
			Retired:         definition.Retired,
		})
		catalog.Summary.Total++
		if definition.Retired {
			catalog.Summary.Retired++
		} else {
			catalog.Summary.Active++
		}
		switch definition.DefaultSeverity {
		case diagnostics.SeverityError:
			catalog.Summary.Errors++
		case diagnostics.SeverityWarning:
			catalog.Summary.Warnings++
		case diagnostics.SeverityInformation:
			catalog.Summary.Information++
		}
	}

	return catalog
}

func writeDiagnosticCatalogText(output io.Writer, catalog diagnosticCatalog) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SUMMARY")
	fmt.Fprintln(w, "TOTAL\tACTIVE\tRETIRED\tERRORS\tWARNINGS\tINFORMATION")
	fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\n\n", catalog.Summary.Total, catalog.Summary.Active, catalog.Summary.Retired, catalog.Summary.Errors, catalog.Summary.Warnings, catalog.Summary.Information)
	fmt.Fprintf(w, "COVERAGE\tcomplete=%t\n", catalog.Coverage.Complete)
	fmt.Fprintf(w, "NOTE\t%s\n\n", catalog.Coverage.Note)

	writeDiagnosticFieldTable(w, "DEFINITION FIELDS", catalog.DefinitionFields)
	writeDiagnosticFieldTable(w, "SEMANTIC OCCURRENCE FIELDS", catalog.SemanticOccurrenceFields)
	writeDiagnosticFieldTable(w, "PARSER OCCURRENCE FIELDS", catalog.ParserOccurrenceFields)
	writeDiagnosticFieldTable(w, "TOKEN FIELDS", catalog.TokenFields)

	fmt.Fprintln(w, "DIAGNOSTIC DEFINITIONS")
	fmt.Fprintln(w, "ID\tNAME\tFAMILY\tDEFAULT_SEVERITY\tMANDATORY\tRETIRED")
	for _, definition := range catalog.Definitions {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%t\t%t\n",
			definition.ID,
			definition.Name,
			definition.Family,
			definition.DefaultSeverity,
			definition.Mandatory,
			definition.Retired,
		)
	}

	return w.Flush()
}

func writeDiagnosticFieldTable(w *tabwriter.Writer, title string, fields []diagnosticCatalogField) {
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, "FIELD\tTYPE\tREQUIRED\tDESCRIPTION")
	for _, field := range fields {
		fmt.Fprintf(w, "%s\t%s\t%t\t%s\n", field.Name, field.Type, field.Required, field.Description)
	}
	fmt.Fprintln(w)
}
