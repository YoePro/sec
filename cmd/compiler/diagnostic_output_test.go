package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// runCLIForDiagnostics runs the compiler CLI in a subprocess and returns its
// stdout, stderr, and exit code.
func runCLIForDiagnostics(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	process := exec.Command(os.Args[0], append([]string{"-test.run=^TestLexerCLIProcess$", "--"}, args...)...)
	process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1")
	var stdout, stderr bytes.Buffer
	process.Stdout, process.Stderr = &stdout, &stderr
	err := process.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

func decodeOccurrenceDocument(t *testing.T, text string) occurrenceDocument {
	t.Helper()
	var document occurrenceDocument
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("diagnostic stream is not one occurrence document: %v\n%s", err, text)
	}
	if decoder.More() {
		t.Fatalf("diagnostic stream contains more than one JSON value:\n%s", text)
	}
	if document.Format != emittedOccurrenceFormat || document.Version != emittedOccurrenceFormatVersion {
		t.Fatalf("document header = %q v%d", document.Format, document.Version)
	}
	return document
}

// The diagnostic-format option is accepted anywhere, in both spellings, and
// rejects unknown formats.
//
// Rule: rules/tooling/diagnostics.md — §14(1) diagnostic-output option.
func TestExtractDiagnosticFormat(t *testing.T) {
	args, format, err := extractDiagnosticFormat([]string{"sema", "--diagnostic-format", "json", "a.sec"})
	if err != nil || format != diagnosticFormatJSON || !reflect.DeepEqual(args, []string{"sema", "a.sec"}) {
		t.Fatalf("separate form = %v %q %v", args, format, err)
	}
	args, format, err = extractDiagnosticFormat([]string{"--diagnostic-format=human", "parse", "a.sec"})
	if err != nil || format != diagnosticFormatHuman || !reflect.DeepEqual(args, []string{"parse", "a.sec"}) {
		t.Fatalf("equals form = %v %q %v", args, format, err)
	}
	if _, _, err := extractDiagnosticFormat([]string{"--diagnostic-format", "xml"}); err == nil {
		t.Fatal("unknown format accepted")
	}
	if _, _, err := extractDiagnosticFormat([]string{"sema", "--diagnostic-format"}); err == nil {
		t.Fatal("missing format value accepted")
	}
}

// Semantic diagnostics are emitted as one deterministic document with
// registered identity, effective severity, exclusive primary spans, related
// locations, help, and explicit unregistered migration fallbacks. The same
// occurrences rendered for humans carry the same identities and positions.
//
// Rules:
//   - rules/tooling/diagnostics.md — §§ 8-10 occurrence, location, and message schemas
//   - rules/tooling/diagnostics.md — §14 machine-readable emitted occurrences
//   - rules/tooling/diagnostics.md — §28 deterministic ordering
//   - rules/tooling/diagnostics.md — §30(7) logical equivalence with human output
func TestSemaEmitsStructuredOccurrenceDocument(t *testing.T) {
	fixture := "../../testdata/diagnostics/emitted_occurrences_invalid.sec"
	stdout, stderr, code := runCLIForDiagnostics(t, "sema", "--diagnostic-format", "json", fixture)
	if code != 3 || strings.Contains(stdout, "OK") {
		t.Fatalf("exit = %d stdout = %q", code, stdout)
	}
	document := decodeOccurrenceDocument(t, stderr)
	if document.Summary != (diagnosticSummary{Errors: 3}) || len(document.Occurrences) != 3 {
		t.Fatalf("summary = %+v occurrences = %d", document.Summary, len(document.Occurrences))
	}
	first, overflow, duplicate := document.Occurrences[0], document.Occurrences[1], document.Occurrences[2]
	if first.ID == nil || *first.ID != diagnostics.DefaultViolatesContract || first.Name == nil || *first.Name != "types.default-violates-contract" ||
		first.Unregistered || first.Severity != diagnostics.SeverityError || first.Source != "sema" || len(first.Help) != 1 || first.Help[0].Text == "" {
		t.Fatalf("registered occurrence = %+v", first)
	}
	if first.Primary == nil || first.Primary.Span.File != fixture || first.Primary.Span.Start != (occurrencePosition{Line: 3, Column: 38}) || first.Primary.Span.End != (occurrencePosition{Line: 3, Column: 39}) {
		t.Fatalf("primary span = %+v", first.Primary)
	}
	if overflow.ID != nil || overflow.Name != nil || !overflow.Unregistered || overflow.Message.Text != "value 300 overflows int8" {
		t.Fatalf("unregistered occurrence = %+v", overflow)
	}
	if duplicate.ID == nil || *duplicate.ID != diagnostics.DuplicateLocalVariable || len(duplicate.Related) != 1 || duplicate.Related[0].Span.Start.Line != 7 {
		t.Fatalf("related occurrence = %+v", duplicate)
	}

	_, again, _ := runCLIForDiagnostics(t, "sema", "--diagnostic-format=json", fixture)
	if again != stderr {
		t.Fatal("occurrence document is not deterministic")
	}

	_, human, humanCode := runCLIForDiagnostics(t, "sema", fixture)
	if humanCode != code {
		t.Fatalf("human exit = %d, json exit = %d", humanCode, code)
	}
	lines := regexp.MustCompile(`(?m)^sema error(?:\[(\w+)\])?: .*? at \S+?:(\d+):(\d+)`).FindAllStringSubmatch(human, -1)
	if len(lines) != len(document.Occurrences) {
		t.Fatalf("human lines = %d, occurrences = %d\n%s", len(lines), len(document.Occurrences), human)
	}
	for index, line := range lines {
		occurrence := document.Occurrences[index]
		id := ""
		if occurrence.ID != nil {
			id = *occurrence.ID
		}
		lineNumber, _ := strconv.Atoi(line[2])
		column, _ := strconv.Atoi(line[3])
		if line[1] != id || occurrence.Primary.Span.Start != (occurrencePosition{Line: lineNumber, Column: column}) {
			t.Fatalf("human line %q does not match occurrence %+v", line[0], occurrence)
		}
	}
}

// Lexer diagnostics surfaced through parsing keep their lexer identity and
// stage, parser diagnostics keep theirs, and single-file commands supply the
// file to every span.
//
// Rules:
//   - rules/tooling/diagnostics.md — §5 namespaces, §9 source locations, §14
func TestParseAndLexEmitStructuredOccurrences(t *testing.T) {
	source := "../../testdata/diagnostics/parse_occurrences_invalid.sec"
	if err := os.WriteFile(source, []byte("module main\nfn F( void {\n    let x := 0b2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(source)
	_, stderr, code := runCLIForDiagnostics(t, "parse", source, "--diagnostic-format", "json")
	document := decodeOccurrenceDocument(t, stderr)
	if code != 2 || len(document.Occurrences) != 2 {
		t.Fatalf("exit = %d occurrences = %+v", code, document.Occurrences)
	}
	parserOccurrence, lexerOccurrence := document.Occurrences[0], document.Occurrences[1]
	if parserOccurrence.ID == nil || (*parserOccurrence.ID)[0] != 'P' || parserOccurrence.Source != "parser" || parserOccurrence.Primary.Span.File != source {
		t.Fatalf("parser occurrence = %+v", parserOccurrence)
	}
	if lexerOccurrence.ID == nil || *lexerOccurrence.ID != diagnostics.LexerInvalidBaseDigit || lexerOccurrence.Source != "lexer" {
		t.Fatalf("lexer occurrence = %+v", lexerOccurrence)
	}

	_, lexed, lexCode := runCLIForDiagnostics(t, "lex", "--diagnostic-format", "json", source)
	lexDocument := decodeOccurrenceDocument(t, lexed)
	if lexCode != 2 || len(lexDocument.Occurrences) != 1 || lexDocument.Occurrences[0].Primary.Span.File != source {
		t.Fatalf("lex exit = %d document = %+v", lexCode, lexDocument)
	}
}

// Successful commands emit an empty occurrence list, and tool failures without
// a source span stay inside the single document as unregistered occurrences.
//
// Rules:
//   - rules/tooling/diagnostics.md — §9(6) diagnostics without a source span
//   - rules/tooling/diagnostics.md — §14 machine-readable emitted occurrences
func TestOccurrenceDocumentForSuccessAndToolErrors(t *testing.T) {
	success := "../../testdata/diagnostics/success_occurrences.sec"
	if err := os.WriteFile(success, []byte("module main\nfn main() int {\n    return 0\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(success)
	_, stderr, code := runCLIForDiagnostics(t, "sema", "--diagnostic-format", "json", success)
	document := decodeOccurrenceDocument(t, stderr)
	if code != 0 || document.Occurrences == nil || len(document.Occurrences) != 0 {
		t.Fatalf("success exit = %d document = %+v", code, document)
	}
	_, emitted, emitCode := runCLIForDiagnostics(t, "emit-ir", success, "-o", "-", "--diagnostic-format", "json")
	if emitCode != 0 || len(decodeOccurrenceDocument(t, emitted).Occurrences) != 0 {
		t.Fatalf("emit-ir exit = %d stderr = %s", emitCode, emitted)
	}

	toolFailure := "../../testdata/diagnostics/tool_error_occurrences.sec"
	if err := os.WriteFile(toolFailure, []byte("module main\nfn main() void {\n    let e := list[int] {}\n    discard e\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(toolFailure)
	_, failed, failedCode := runCLIForDiagnostics(t, "emit-ir", toolFailure, "-o", "-", "--diagnostic-format", "json")
	failure := decodeOccurrenceDocument(t, failed)
	if failedCode == 0 || len(failure.Occurrences) != 1 || !failure.Occurrences[0].Unregistered || failure.Occurrences[0].Primary != nil ||
		failure.Occurrences[0].Source != "semantic IR" || failure.Summary.Errors != 1 {
		t.Fatalf("tool failure exit = %d document = %+v", failedCode, failure)
	}
}
