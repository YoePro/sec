package lexer

import "testing"

// Documentation comments are ordinary comment tokens that retain their exact
// source spelling and ranges; `///` is an ordinary line comment; comments
// separate tokens and never join a multi-character token across them.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §5.4 "Documentation comments"
//   - rules/foundations/lexical_structure.md — §5.5 "Comment preservation"
//   - rules/foundations/lexical_structure.md — §25 "Required tests"
func TestDocumentationCommentsAndCommentBoundaries(t *testing.T) {
	source := "/**\n * Doc.\n */\n/// line\nvalue/* c */+1\n1./* c */.<10\n"
	want := []struct {
		typ       TokenType
		lexeme    string
		line      int
		column    int
		byteStart int
	}{
		{COMMENT, "/**\n * Doc.\n */", 1, 1, 0},
		{COMMENT, "/// line", 4, 1, 16},
		{IDENT, "value", 5, 1, 25},
		{COMMENT, "/* c */", 5, 6, 30},
		{PLUS, "+", 5, 13, 37},
		{INT, "1", 5, 14, 38},
		{INT, "1", 6, 1, 40},
		{DOT, ".", 6, 2, 41},
		{COMMENT, "/* c */", 6, 3, 42},
		{DOT, ".", 6, 10, 49},
		{LT, "<", 6, 11, 50},
		{INT, "10", 6, 12, 51},
		{EOF, "", 7, 1, 54},
	}
	l := NewWithFile(source, "doc.sec")
	for index, expected := range want {
		token := l.NextToken()
		if token.Type != expected.typ || token.Lexeme != expected.lexeme || token.Line != expected.line ||
			token.Column != expected.column || token.ByteStart != expected.byteStart ||
			token.Type != EOF && token.ByteEnd != expected.byteStart+len(expected.lexeme) {
			t.Fatalf("token %d = %+v, want %+v", index, token, expected)
		}
	}
	if diagnostics := l.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}
