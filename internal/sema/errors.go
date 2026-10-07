package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type Error struct {
	ID             string
	Severity       diagnostics.Severity
	ProofState     diagnostics.ProofState
	Help           string
	Message        string
	File           string
	Line           int
	Column         int
	EndLine        int
	EndColumn      int
	PreviousFile   string
	PreviousLine   int
	PreviousColumn int
	// RelatedLabel names the related location; empty means "previous
	// declaration".
	RelatedLabel string
	// EscapeCauses explains the existing owning error using canonical provenance.
	EscapeCauses []EscapeCausePath
	// AllocationCause is the canonical navigable witness of an allocation-policy violation.
	AllocationCause *AllocationCausePath
}

// RelatedLocationLabel is the human label of the related location.
func (e Error) RelatedLocationLabel() string {
	if e.RelatedLabel != "" {
		return e.RelatedLabel
	}
	return "previous declaration"
}

func (e Error) WithID(id string) Error {
	e.ID = id
	return e
}

func (e Error) WithSeverity(severity diagnostics.Severity) Error {
	e.Severity = severity
	return e
}

func (e Error) WithHelp(help string) Error {
	e.Help = help
	return e
}

// DisplayMessage preserves an explicit proof classification in human and
// editor output without changing the producer's underlying explanation.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §58(3).
func (e Error) DisplayMessage() string {
	if e.ProofState != "" {
		return string(e.ProofState) + ": " + e.Message
	}
	return e.Message
}

// Error renders the same proof outcome with source and related locations.
// Rules: rules/compiler/compiler_analysis.md — §58(3);
// rules/tooling/diagnostics.md — §9.
func (e Error) Error() string {
	if e.Line > 0 && e.Column > 0 {
		if e.PreviousLine > 0 && e.PreviousColumn > 0 {
			return fmt.Sprintf(
				"%s at %s, %s at %s",
				e.DisplayMessage(),
				formatLocation(e.File, e.Line, e.Column),
				e.RelatedLocationLabel(),
				formatLocation(e.PreviousFile, e.PreviousLine, e.PreviousColumn),
			)
		}
		return fmt.Sprintf("%s at %s", e.DisplayMessage(), formatLocation(e.File, e.Line, e.Column))
	}

	return e.DisplayMessage()
}

// addErrorAtTokenWithPreviousMetadata emits one registered semantic diagnostic
// with actionable help and the source location that caused the current source
// token to be rejected.
//
// Rules:
//   - rules/tooling/diagnostics.md — § 2(6)–(8), § 3(4)–(6), and §§ 5–7
func (a *Analyzer) addErrorAtTokenWithPreviousMetadata(token lexer.Token, previous lexer.Token, id string, help string, format string, args ...any) {
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID:             id,
		Severity:       diagnostics.SeverityError,
		Help:           help,
		Message:        fmt.Sprintf(format, args...),
		File:           token.File,
		Line:           token.Line,
		Column:         token.Column,
		EndLine:        endLine,
		EndColumn:      endColumn,
		PreviousFile:   previous.File,
		PreviousLine:   previous.Line,
		PreviousColumn: previous.Column,
	})
}

func formatLocation(file string, line int, column int) string {
	if file != "" {
		return fmt.Sprintf("%s:%d:%d", file, line, column)
	}
	return fmt.Sprintf("%d:%d", line, column)
}
