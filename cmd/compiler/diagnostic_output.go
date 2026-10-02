package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// diagnosticOutputFormat selects how diagnostic-producing commands emit
// occurrences: the existing human-readable lines, or one machine-readable
// occurrence document.
//
// Rules:
//   - rules/tooling/diagnostics.md — §14 "Machine-readable emitted occurrences"
type diagnosticOutputFormat string

const (
	diagnosticFormatHuman diagnosticOutputFormat = "human"
	diagnosticFormatJSON  diagnosticOutputFormat = "json"
)

// emittedOccurrenceFormat identifies the emitted-occurrence document so
// machine consumers never confuse it with the catalog surface.
//
// Rule: rules/tooling/diagnostics.md — §14(5) catalog versus occurrence output.
const (
	emittedOccurrenceFormat        = "sec.diagnostic-occurrences"
	emittedOccurrenceFormatVersion = 1
)

type occurrencePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type occurrenceSpan struct {
	File  string             `json:"file"`
	Start occurrencePosition `json:"start"`
	End   occurrencePosition `json:"end"`
}

type occurrenceLocation struct {
	Span occurrenceSpan `json:"span"`
}

type occurrenceMessage struct {
	Key       string            `json:"key"`
	Arguments map[string]string `json:"arguments"`
	Text      string            `json:"text"`
}

type occurrenceRelated struct {
	Span    occurrenceSpan     `json:"span"`
	Message *occurrenceMessage `json:"message"`
}

// emittedOccurrence is the JSON form of the canonical logical occurrence.
// Unregistered marks an unmigrated host diagnostic that has no stable ID yet;
// it is a governance-tracked migration fallback, never a registered identity.
//
// Rules:
//   - rules/tooling/diagnostics.md — §8 "Canonical logical occurrence schema"
//   - rules/tooling/diagnostics.md — §9 "Source locations" (1-based, exclusive End)
//   - rules/tooling/diagnostics.md — §10 "Notes, help and messages"
//   - rules/tooling/diagnostics.md — §14(3)–(4) minimum fields and display text
//   - rules/tooling/diagnostics.md — §32(2) temporary fallback behavior in governance
type emittedOccurrence struct {
	ID           *string              `json:"id"`
	Name         *string              `json:"name"`
	Unregistered bool                 `json:"unregistered"`
	Severity     diagnostics.Severity `json:"severity"`
	Arguments    map[string]string    `json:"arguments"`
	Primary      *occurrenceLocation  `json:"primary"`
	Related      []occurrenceRelated  `json:"related"`
	Notes        []occurrenceMessage  `json:"notes"`
	Help         []occurrenceMessage  `json:"help"`
	Fixes        []any                `json:"fixes"`
	Message      occurrenceMessage    `json:"message"`
	Source       string               `json:"source"`
}

type occurrenceDocument struct {
	Format      string              `json:"format"`
	Version     int                 `json:"version"`
	Occurrences []emittedOccurrence `json:"occurrences"`
	Summary     diagnosticSummary   `json:"summary"`
}

// diagnosticReporter is the single CLI path for emitted diagnostics. Human
// mode writes each occurrence immediately in the established text form; JSON
// mode collects occurrences and writes one deterministic document when the
// command reports its summary.
//
// Rules:
//   - rules/tooling/diagnostics.md — §2(10) one identity across CLI, LSP, tests and machine output
//   - rules/tooling/diagnostics.md — §14 "Machine-readable emitted occurrences"
//   - rules/tooling/diagnostics.md — §28 "Determinism and diagnostic ordering"
type diagnosticReporter struct {
	format      diagnosticOutputFormat
	output      io.Writer
	occurrences []emittedOccurrence
	written     bool
	// file names the single input of a one-file command whose lexer tokens
	// carry no file; it fills otherwise empty span files.
	file string
}

var cliDiagnostics = &diagnosticReporter{format: diagnosticFormatHuman, output: os.Stderr}

// extractDiagnosticFormat removes `--diagnostic-format <human|json>` (or the
// `=` form) from anywhere in the argument list, so every command accepts it
// without changing its own option grammar.
//
// Rule: rules/tooling/diagnostics.md — §14(1) the command's diagnostic-output option.
func extractDiagnosticFormat(args []string) ([]string, diagnosticOutputFormat, error) {
	format := diagnosticFormatHuman
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		value := ""
		switch {
		case arg == "--diagnostic-format":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("--diagnostic-format requires human or json")
			}
			index++
			value = args[index]
		case len(arg) > len("--diagnostic-format=") && arg[:len("--diagnostic-format=")] == "--diagnostic-format=":
			value = arg[len("--diagnostic-format="):]
		default:
			remaining = append(remaining, arg)
			continue
		}
		switch diagnosticOutputFormat(value) {
		case diagnosticFormatHuman, diagnosticFormatJSON:
			format = diagnosticOutputFormat(value)
		default:
			return nil, "", fmt.Errorf("unknown diagnostic format %q; use human or json", value)
		}
	}
	return remaining, format, nil
}

func registeredIdentity(id string) (*string, *string, bool) {
	if id == "" {
		return nil, nil, true
	}
	definition, ok := diagnostics.Lookup(id)
	if !ok {
		return &id, nil, true
	}
	name := definition.Name
	return &id, &name, false
}

func newOccurrence(id string, severity diagnostics.Severity, source string, text string) emittedOccurrence {
	identity, name, unregistered := registeredIdentity(id)
	key := "unregistered.message"
	if name != nil {
		key = *name + ".message"
	}
	return emittedOccurrence{
		ID:           identity,
		Name:         name,
		Unregistered: unregistered,
		Severity:     severity,
		Arguments:    map[string]string{},
		Related:      []occurrenceRelated{},
		Notes:        []occurrenceMessage{},
		Help:         []occurrenceMessage{},
		Fixes:        []any{},
		Message:      occurrenceMessage{Key: key, Arguments: map[string]string{}, Text: text},
		Source:       source,
	}
}

func (o *emittedOccurrence) addHelp(text string) {
	if text == "" {
		return
	}
	key := "unregistered.help"
	if o.Name != nil {
		key = *o.Name + ".help"
	}
	o.Help = append(o.Help, occurrenceMessage{Key: key, Arguments: map[string]string{}, Text: text})
}

func tokenSpan(token lexer.Token) occurrenceSpan {
	endLine, endColumn := token.EndPosition()
	return occurrenceSpan{
		File:  token.File,
		Start: occurrencePosition{Line: token.Line, Column: token.Column},
		End:   occurrencePosition{Line: endLine, Column: endColumn},
	}
}

func (r *diagnosticReporter) record(occurrence emittedOccurrence, human string) {
	if occurrence.Primary != nil && occurrence.Primary.Span.File == "" {
		occurrence.Primary.Span.File = r.file
	}
	for index := range occurrence.Related {
		if occurrence.Related[index].Span.File == "" {
			occurrence.Related[index].Span.File = r.file
		}
	}
	if occurrence.ID != nil {
		// rules/tooling/diagnostics.md §5: the ID namespace identifies the
		// producing stage, e.g. lexer diagnostics surfaced through the parser.
		switch (*occurrence.ID)[0] {
		case 'L':
			occurrence.Source = "lexer"
		case 'P':
			occurrence.Source = "parser"
		case 'S':
			occurrence.Source = "sema"
		}
	}
	if r.format == diagnosticFormatJSON {
		r.occurrences = append(r.occurrences, occurrence)
		return
	}
	fmt.Fprint(r.output, human)
}

// lexerDiagnostic records one structured lexer diagnostic.
func (r *diagnosticReporter) lexerDiagnostic(diagnostic lexer.Diagnostic) {
	location := fmt.Sprintf("%d:%d", diagnostic.Primary.Line, diagnostic.Primary.Column)
	if diagnostic.Primary.File != "" {
		location = fmt.Sprintf("%s:%s", diagnostic.Primary.File, location)
	}
	occurrence := newOccurrence(diagnostic.ID, diagnostics.SeverityError, "lexer", diagnostic.Message)
	occurrence.Primary = &occurrenceLocation{Span: tokenSpan(diagnostic.Primary)}
	r.record(occurrence, fmt.Sprintf("lex error: %s at %s: %s\n", diagnostic.ID, location, diagnostic.Message))
}

// illegalToken records an ILLEGAL recovery token that no structured lexer
// diagnostic explains.
func (r *diagnosticReporter) illegalToken(token lexer.Token) {
	occurrence := newOccurrence("", diagnostics.SeverityError, "lexer", fmt.Sprintf("illegal token %q", token.Lexeme))
	occurrence.Primary = &occurrenceLocation{Span: tokenSpan(token)}
	r.record(occurrence, "")
}

// parserDiagnostics records structured parser diagnostics. Human output keeps
// the established `parse error:` lines rendered from the compatibility
// message list, with an optional file prefix for multi-file commands.
func (r *diagnosticReporter) parserDiagnostics(file string, structured []parser.Diagnostic, messages []string) {
	prefix := ""
	if file != "" {
		prefix = file + ": "
	}
	for index, diagnostic := range structured {
		occurrence := newOccurrence(diagnostic.ID, diagnostics.SeverityError, "parser", diagnostic.Message)
		primary := diagnostic.Primary
		if primary.File == "" {
			primary.File = file
		}
		occurrence.Primary = &occurrenceLocation{Span: tokenSpan(primary)}
		occurrence.addHelp(diagnostic.Help)
		human := ""
		if index < len(messages) {
			human = fmt.Sprintf("%sparse error: %s\n", prefix, messages[index])
		}
		r.record(occurrence, human)
	}
	for index := len(structured); index < len(messages); index++ {
		occurrence := newOccurrence("", diagnostics.SeverityError, "parser", messages[index])
		r.record(occurrence, fmt.Sprintf("%sparse error: %s\n", prefix, messages[index]))
	}
}

// parserWarnings records unregistered parser warnings, which currently carry
// only message text.
func (r *diagnosticReporter) parserWarnings(file string, warnings []string) {
	prefix := ""
	if file != "" {
		prefix = file + ": "
	}
	for _, warning := range warnings {
		occurrence := newOccurrence("", diagnostics.SeverityWarning, "parser", warning)
		r.record(occurrence, fmt.Sprintf("%sWarning: %s\n", prefix, warning))
	}
}

// semaDiagnostic records one semantic diagnostic, including its related
// earlier location and help.
func (r *diagnosticReporter) semaDiagnostic(diagnostic sema.Error, human string) {
	severity := diagnostic.Severity
	if severity == "" {
		severity = diagnostics.SeverityError
	}
	occurrence := newOccurrence(diagnostic.ID, severity, "sema", diagnostic.Message)
	if diagnostic.Line > 0 && diagnostic.Column > 0 {
		endLine, endColumn := diagnostic.EndLine, diagnostic.EndColumn
		if endLine == 0 {
			endLine, endColumn = diagnostic.Line, diagnostic.Column
		}
		occurrence.Primary = &occurrenceLocation{Span: occurrenceSpan{
			File:  diagnostic.File,
			Start: occurrencePosition{Line: diagnostic.Line, Column: diagnostic.Column},
			End:   occurrencePosition{Line: endLine, Column: endColumn},
		}}
	}
	if diagnostic.PreviousLine > 0 && diagnostic.PreviousColumn > 0 {
		position := occurrencePosition{Line: diagnostic.PreviousLine, Column: diagnostic.PreviousColumn}
		occurrence.Related = append(occurrence.Related, occurrenceRelated{
			Span: occurrenceSpan{File: diagnostic.PreviousFile, Start: position, End: position},
		})
	}
	occurrence.addHelp(diagnostic.Help)
	r.record(occurrence, human)
}

// sortedOccurrences orders occurrences by source location and then stable
// identity; occurrences without a location keep their emission order after
// located ones.
//
// Rule: rules/tooling/diagnostics.md — §28(3) deterministic ordering.
func sortedOccurrences(occurrences []emittedOccurrence) []emittedOccurrence {
	sorted := append([]emittedOccurrence(nil), occurrences...)
	sort.SliceStable(sorted, func(left, right int) bool {
		a, b := sorted[left].Primary, sorted[right].Primary
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		if a.Span.File != b.Span.File {
			return a.Span.File < b.Span.File
		}
		if a.Span.Start.Line != b.Span.Start.Line {
			return a.Span.Start.Line < b.Span.Start.Line
		}
		if a.Span.Start.Column != b.Span.Start.Column {
			return a.Span.Start.Column < b.Span.Start.Column
		}
		return identityKey(sorted[left]) < identityKey(sorted[right])
	})
	return sorted
}

func identityKey(occurrence emittedOccurrence) string {
	if occurrence.ID == nil {
		return "~"
	}
	return *occurrence.ID
}

// summary reports the command's diagnostic totals. Human mode prints the
// established summary line; JSON mode writes the complete occurrence document
// once, so later exits cannot duplicate it.
func (r *diagnosticReporter) summary(summary diagnosticSummary) {
	if r.format != diagnosticFormatJSON {
		fmt.Fprintf(r.output, "summary: %s, %s", diagnosticCountLabel(summary.Errors, "error"), diagnosticCountLabel(summary.Warnings, "warning"))
		if summary.Information > 0 {
			fmt.Fprintf(r.output, ", %s", diagnosticCountLabel(summary.Information, "information diagnostic"))
		}
		fmt.Fprintln(r.output)
		return
	}
	if r.written {
		return
	}
	r.written = true
	document := occurrenceDocument{
		Format:      emittedOccurrenceFormat,
		Version:     emittedOccurrenceFormatVersion,
		Occurrences: append([]emittedOccurrence{}, sortedOccurrences(r.occurrences)...),
		Summary:     summary,
	}
	encoder := json.NewEncoder(r.output)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(document)
}

// reporterFor returns the CLI reporter for its own stream, or a human-format
// reporter for an explicit writer such as a test buffer.
func reporterFor(output io.Writer) *diagnosticReporter {
	if output == cliDiagnostics.output {
		return cliDiagnostics
	}
	return &diagnosticReporter{format: diagnosticFormatHuman, output: output}
}

// finish writes a pending JSON occurrence document for commands that end
// without reporting a summary, such as successful emit commands or tool
// errors, deriving the totals from the recorded occurrences.
//
// Rule: rules/tooling/diagnostics.md — §14 "Machine-readable emitted occurrences".
func (r *diagnosticReporter) finish() {
	if r.format != diagnosticFormatJSON || r.written {
		return
	}
	summary := diagnosticSummary{}
	for _, occurrence := range r.occurrences {
		switch occurrence.Severity {
		case diagnostics.SeverityError:
			summary.Errors++
		case diagnostics.SeverityWarning:
			summary.Warnings++
		case diagnostics.SeverityInformation:
			summary.Information++
		}
	}
	r.summary(summary)
}

// exitCLI flushes pending machine-readable diagnostics before terminating
// the process with code.
func exitCLI(code int) {
	cliDiagnostics.finish()
	os.Exit(code)
}

// reportToolError records a project, build, target, or tool failure that has
// no registered source diagnostic. Human mode keeps the established
// `<kind> error: <message>` line; JSON mode records an unregistered
// occurrence without a source span so the error stream stays one document.
//
// Rules:
//   - rules/tooling/diagnostics.md — §9(6) project/build/target diagnostics without a source span
//   - rules/tooling/diagnostics.md — §14 "Machine-readable emitted occurrences"
func reportToolError(kind string, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	occurrence := newOccurrence("", diagnostics.SeverityError, kind, message)
	cliDiagnostics.record(occurrence, fmt.Sprintf("%s error: %s\n", kind, message))
}
