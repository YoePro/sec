package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	{Name: "Help", Type: "string", Required: false, Description: "Actionable correction or next step."},
	{Name: "Message", Type: "string", Required: true, Description: "Rendered primary diagnostic message."},
	{Name: "File", Type: "string", Required: false, Description: "Primary source file."},
	{Name: "Line", Type: "int", Required: false, Description: "One-based primary source line."},
	{Name: "Column", Type: "int", Required: false, Description: "One-based primary source column."},
	{Name: "PreviousFile", Type: "string", Required: false, Description: "Related previous declaration source file."},
	{Name: "PreviousLine", Type: "int", Required: false, Description: "One-based related source line."},
	{Name: "PreviousColumn", Type: "int", Required: false, Description: "One-based related source column."},
}

var parserOccurrenceFields = []diagnosticCatalogField{
	{Name: "ID", Type: "string", Required: true, Description: "Registered parser diagnostic identifier."},
	{Name: "Message", Type: "string", Required: true, Description: "Rendered primary diagnostic message."},
	{Name: "Primary", Type: "Token", Required: true, Description: "Primary source token and location."},
	{Name: "Expected", Type: "[]TokenType", Required: false, Description: "Expected token kinds, when known."},
	{Name: "Unexpected", Type: "*Token", Required: false, Description: "Unexpected token, when known."},
	{Name: "Context", Type: "RecoveryContext", Required: true, Description: "Stable parser recovery context containing the occurrence."},
	{Name: "Episode", Type: "int", Required: true, Description: "Positive recovery episode number within the parse."},
}

var diagnosticTokenFields = []diagnosticCatalogField{
	{Name: "Type", Type: "TokenType", Required: true, Description: "Token kind."},
	{Name: "Lexeme", Type: "string", Required: true, Description: "Original source lexeme."},
	{Name: "File", Type: "string", Required: false, Description: "Source file."},
	{Name: "Line", Type: "int", Required: true, Description: "One-based source line."},
	{Name: "Column", Type: "int", Required: true, Description: "One-based source column."},
}

func runDiagnosticCatalogCommand(args []string, output io.Writer) error {
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			return fmt.Errorf("unknown argument %q; expected --json", arg)
		}
	}

	catalog := buildDiagnosticCatalog()
	if jsonOutput {
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	}
	return writeDiagnosticCatalogText(output, catalog)
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
