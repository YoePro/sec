package lexer

import "testing"

// TestTokenByteRangesPreserveOriginalUTF8 verifies that every lexer token is
// anchored to the original source bytes rather than decoded-rune indexes.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §1.3 "Unicode scalar values"
//   - rules/foundations/lexical_structure.md — §25 "Required tests"
func TestTokenByteRangesPreserveOriginalUTF8(t *testing.T) {
	source := "\ufefflet π := `å`\r\n// ö\rvalue"
	l := NewWithFile(source, "ranges.sec")
	want := []struct {
		typ                TokenType
		lexeme             string
		start, end         int
		endLine, endColumn int
	}{
		{LET, "let", 3, 6, 1, 4},
		{IDENT, "π", 7, 9, 1, 6},
		{DECLARE, ":=", 10, 12, 1, 9},
		{RAW_STRING, "`å`", 13, 17, 1, 13},
		{COMMENT, "// ö", 19, 24, 2, 5},
		{IDENT, "value", 25, 30, 3, 6},
		{EOF, "", 30, 30, 3, 6},
	}
	for index, expected := range want {
		token := l.NextToken()
		if token.Type != expected.typ || token.Lexeme != expected.lexeme ||
			token.ByteStart != expected.start || token.ByteEnd != expected.end ||
			token.EndLine != expected.endLine || token.EndColumn != expected.endColumn {
			t.Fatalf("token %d = %+v, want %s %q [%d,%d)", index, token, expected.typ, expected.lexeme, expected.start, expected.end)
		}
		if token.ByteStart < token.ByteEnd && source[token.ByteStart:token.ByteEnd] != token.Lexeme {
			t.Fatalf("token %d source slice = %q, want lexeme %q", index, source[token.ByteStart:token.ByteEnd], token.Lexeme)
		}
	}
}

// TestDiagnosticByteRangesRetainInvalidSourceBytes covers decoder diagnostics
// whose source spelling cannot be reconstructed from the decoded rune stream.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.1, 1.3, 20
func TestDiagnosticByteRangesRetainInvalidSourceBytes(t *testing.T) {
	source := "α\xff\ufeffz"
	l := NewWithFile(source, "invalid.sec")
	diagnostics := l.Diagnostics()
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v, want invalid UTF-8 and unexpected BOM", diagnostics)
	}
	want := [][2]int{{2, 3}, {3, 6}}
	for index, span := range want {
		primary := diagnostics[index].Primary
		if primary.ByteStart != span[0] || primary.ByteEnd != span[1] {
			t.Fatalf("diagnostic %d range = [%d,%d), want [%d,%d)", index, primary.ByteStart, primary.ByteEnd, span[0], span[1])
		}
		if got := source[primary.ByteStart:primary.ByteEnd]; got != primary.Lexeme {
			t.Fatalf("diagnostic %d source slice = %q, want %q", index, got, primary.Lexeme)
		}
		if primary.EndLine != 1 || primary.EndColumn != index+3 {
			t.Fatalf("diagnostic %d end = %d:%d, want 1:%d", index, primary.EndLine, primary.EndColumn, index+3)
		}
	}

	alpha := l.NextToken()
	if alpha.Type != IDENT || alpha.Lexeme != "α" || alpha.ByteStart != 0 || alpha.ByteEnd != 2 {
		t.Fatalf("leading token = %+v, want α [0,2)", alpha)
	}
	invalid := l.NextToken()
	if invalid.Type != ILLEGAL || invalid.ByteStart != 2 || invalid.ByteEnd != 3 {
		t.Fatalf("invalid-byte recovery token = %+v, want ILLEGAL [2,3)", invalid)
	}
	identifier := l.NextToken()
	if identifier.Type != IDENT || identifier.Lexeme != "z" || identifier.ByteStart != 6 || identifier.ByteEnd != 7 {
		t.Fatalf("following token = %+v, want z [6,7)", identifier)
	}
}

// TestMultilineTokenEndPositions verifies exclusive scalar ends across CRLF
// and bare-CR physical lines without deriving them from byte widths.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.3, 2, 5.3, 14.2
//   - rules/tooling/diagnostics.md — §9 "Source locations"
func TestMultilineTokenEndPositions(t *testing.T) {
	l := New("/* a\r\nβ */`x\ry`")
	comment := l.NextToken()
	if comment.Type != COMMENT || comment.Line != 1 || comment.Column != 1 || comment.EndLine != 2 || comment.EndColumn != 5 {
		t.Fatalf("comment range = %+v, want 1:1–2:5", comment)
	}
	raw := l.NextToken()
	if raw.Type != RAW_STRING || raw.Line != 2 || raw.Column != 5 || raw.EndLine != 3 || raw.EndColumn != 3 {
		t.Fatalf("raw-string range = %+v, want 2:5–3:3", raw)
	}
	endLine, endColumn := (Token{Line: 4, Column: 2, Lexeme: "a\r\nb\rc\nd"}).EndPosition()
	if endLine != 7 || endColumn != 2 {
		t.Fatalf("compatibility end = %d:%d, want 7:2", endLine, endColumn)
	}
}

// TestSnapshotRestoreReplaysByteRanges ensures speculative parser lookahead
// cannot shift a token's original-source range.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§1.3, 21
func TestSnapshotRestoreReplaysByteRanges(t *testing.T) {
	l := New("å β")
	first := l.NextToken()
	state := l.Snapshot()
	before := l.NextToken()
	l.Restore(state)
	after := l.NextToken()
	if first.ByteStart != 0 || first.ByteEnd != 2 || before != after || before.ByteStart != 3 || before.ByteEnd != 5 {
		t.Fatalf("ranges did not survive snapshot restore: first=%+v before=%+v after=%+v", first, before, after)
	}
}
