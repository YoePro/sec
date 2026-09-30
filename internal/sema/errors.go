package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

type Error struct {
	ID             string
	Severity       diagnostics.Severity
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

func (e Error) Error() string {
	if e.Line > 0 && e.Column > 0 {
		if e.PreviousLine > 0 && e.PreviousColumn > 0 {
			return fmt.Sprintf(
				"%s at %s, previous declaration at %s",
				e.Message,
				formatLocation(e.File, e.Line, e.Column),
				formatLocation(e.PreviousFile, e.PreviousLine, e.PreviousColumn),
			)
		}
		return fmt.Sprintf("%s at %s", e.Message, formatLocation(e.File, e.Line, e.Column))
	}

	return e.Message
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
