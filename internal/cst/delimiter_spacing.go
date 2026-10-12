package cst

import "sec/internal/lexer"

// ApplySingleLineDelimiterRoles projects horizontal punctuation roles onto
// complete same-line lexical groups only after the shared parser confirmed the
// source syntax. Literal tokens stay opaque; comment-bearing and multiline
// groups receive no roles. The policy is uniform across parser-accepted calls,
// types, indexing, grouping, literals and compact declaration/accessor bodies.
// Rules: rules/tooling/formatter.md — §5(1–6),§8(6–8),§11(1,4–8),§12,§25.
func (d *Document) ApplySingleLineDelimiterRoles(parserConfirmed bool) {
	if d == nil || !parserConfirmed || len(d.Diagnostics) > 0 || len(d.UnmatchedClosers) > 0 {
		return
	}
	mark := func(index int, role Role) { d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, role) }
	for _, group := range d.Groups {
		if group.Close < 0 || d.Elements[group.Open].Token.Line != d.Elements[group.Close].Token.Line {
			continue
		}
		comments := false
		for index := group.Open + 1; index < group.Close; index++ {
			if d.Elements[index].Kind == Comment {
				comments = true
				break
			}
		}
		if comments {
			continue
		}
		empty := true
		for index := group.Open + 1; index < group.Close; index++ {
			if d.Elements[index].Kind == Token {
				empty = false
				break
			}
		}
		if d.Elements[group.Open].Token.Type == lexer.LBRACE {
			if !empty {
				mark(group.Open, PaddedDelimiterOpen)
				mark(group.Close, PaddedDelimiterClose)
			} else {
				mark(group.Open, CompactDelimiterOpen)
				mark(group.Close, CompactDelimiterClose)
			}
		} else {
			mark(group.Open, CompactDelimiterOpen)
			mark(group.Close, CompactDelimiterClose)
			for previous := group.Close - 1; previous > group.Open; previous-- {
				if d.Elements[previous].Kind == Whitespace {
					continue
				}
				if d.Elements[previous].Token.Type == lexer.COMMA {
					mark(previous, SingleLineTrailingComma)
				}
				break
			}
		}
		for index := group.Open + 1; index < group.Close; index++ {
			switch d.Elements[index].Token.Type {
			case lexer.COMMA:
				mark(index, SingleLineComma)
			case lexer.COLON:
				mark(index, DeclarationColon)
			}
		}
	}
}
