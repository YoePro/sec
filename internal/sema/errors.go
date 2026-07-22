package sema

import "fmt"

type Error struct {
	Message        string
	File           string
	Line           int
	Column         int
	PreviousFile   string
	PreviousLine   int
	PreviousColumn int
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

func formatLocation(file string, line int, column int) string {
	if file != "" {
		return fmt.Sprintf("%s:%d:%d", file, line, column)
	}
	return fmt.Sprintf("%d:%d", line, column)
}
