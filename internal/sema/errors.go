package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// RelatedLocation is one source-mapped explanation of a diagnostic.
// Rules: rules/tooling/diagnostics.md — §§8(2),9(1–7).
type RelatedLocation struct {
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	Label     string
}

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
	// Related retains additional ordered locations without replacing the legacy first link.
	Related []RelatedLocation
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
	message := e.DisplayMessage()
	if e.Line > 0 && e.Column > 0 {
		message += " at " + formatLocation(e.File, e.Line, e.Column)
	}
	for _, related := range e.RelatedLocations() {
		message += ", " + related.Label + " at " + formatLocation(related.File, related.Line, related.Column)
	}
	return message
}

// RelatedLocations exposes the ordered diagnostic provenance, including the
// compatibility previous-declaration link, without duplicate or invalid points.
// Rules: rules/tooling/diagnostics.md — §§8(2),9(2,5,7).
func (e Error) RelatedLocations() []RelatedLocation {
	locations := make([]RelatedLocation, 0, len(e.Related)+1)
	appendLocation := func(location RelatedLocation) {
		if location.Line <= 0 || location.Column <= 0 {
			return
		}
		for _, existing := range locations {
			if existing.File == location.File && existing.Line == location.Line && existing.Column == location.Column {
				return
			}
		}
		if location.Label == "" {
			location.Label = "previous declaration"
		}
		locations = append(locations, location)
	}
	appendLocation(RelatedLocation{File: e.PreviousFile, Line: e.PreviousLine, Column: e.PreviousColumn, Label: e.RelatedLocationLabel()})
	for _, location := range e.Related {
		appendLocation(location)
	}
	return locations
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
