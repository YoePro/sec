package sema

import "fmt"

type Error struct {
	Message        string
	Line           int
	Column         int
	PreviousLine   int
	PreviousColumn int
}

func (e Error) Error() string {
	if e.Line > 0 && e.Column > 0 {
		if e.PreviousLine > 0 && e.PreviousColumn > 0 {
			return fmt.Sprintf(
				"%s at %d:%d, previous declaration at %d:%d",
				e.Message,
				e.Line,
				e.Column,
				e.PreviousLine,
				e.PreviousColumn,
			)
		}
		return fmt.Sprintf("%s at %d:%d", e.Message, e.Line, e.Column)
	}

	return e.Message
}
