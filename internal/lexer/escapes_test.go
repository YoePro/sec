package lexer

import (
	"strings"
	"testing"
)

func TestEscapeValidationAndRecovery(t *testing.T) {
	for _, test := range []struct{ source, id string }{
		{`\q`, "L1006"}, {`\xG1`, "L1007"}, {`\x1`, "L1007"},
		{`\u{}`, "L1007"}, {`\u{xyz}`, "L1007"}, {`\u1234`, "L1007"},
		{`\u{1234567}`, "L1007"}, {`\u{123`, "L1007"},
		{`\u{D800}`, "L1008"}, {`\u{DFFF}`, "L1008"}, {`\u{110000}`, "L1008"},
		{`\n`, ""}, {`\r`, ""}, {`\t`, ""}, {`\0`, ""}, {`\\`, ""}, {`\"`, ""}, {`\'`, ""},
		{`\x00`, ""}, {`\xFF`, ""}, {`\u{0}`, ""}, {`\u{03A9}`, ""}, {`\u{10FFFF}`, ""},
	} {
		for _, wrapper := range []struct {
			start, end string
			kind       TokenType
		}{{`"`, `"`, STRING}, {`'`, `'`, CHAR}, {`$"`, `"`, INTERPSTRING}} {
			t.Run(wrapper.start+test.source, func(t *testing.T) {
				input := wrapper.start + test.source + wrapper.end
				l := NewWithFile("\n  "+input+" following", "escapes.sec")
				state := l.Snapshot()
				for attempt := 0; attempt < 2; attempt++ {
					token := l.NextToken()
					if token.Type != wrapper.kind || token.Lexeme != input {
						t.Fatalf("lost literal boundary: %+v", token)
					}
					d := l.Diagnostics()
					if test.id == "" {
						if len(d) != 0 {
							t.Fatalf("valid escape: %+v", d)
						}
					} else {
						if len(d) != 1 || d[0].ID != test.id || d[0].Primary.Line != 2 || d[0].Primary.Column != 3+len(wrapper.start) || d[0].Primary.File != "escapes.sec" || !strings.HasPrefix(d[0].Primary.Lexeme, `\`) {
							t.Fatalf("wrong diagnostic: %+v", d)
						}
					}
					if next := l.NextToken(); next.Type != IDENT || next.Lexeme != "following" {
						t.Fatalf("lost next token: %+v", next)
					}
					l.Restore(state)
					if len(l.Diagnostics()) != 0 {
						t.Fatal("diagnostics survived restore")
					}
				}
			})
		}
	}
}

func TestEscapeValidationExcludesRawStringsAndComments(t *testing.T) {
	l := New("`\\q \\u{D800}` /* \\xG1 */ // \\q\n")
	for token := l.NextToken(); token.Type != EOF; token = l.NextToken() {
	}
	if len(l.Diagnostics()) != 0 {
		t.Fatalf("raw/comment contents validated: %+v", l.Diagnostics())
	}
}

func TestEscapeLineEndAndNestedInterpolation(t *testing.T) {
	for _, ending := range []string{"\n", "\r", "\r\n", ""} {
		l := New("\"\\" + ending)
		if token := l.NextToken(); token.Type != ILLEGAL {
			t.Fatalf("unterminated token: %+v", token)
		}
		if d := l.Diagnostics(); len(d) != 1 || d[0].ID != "L1007" {
			t.Fatalf("missing incomplete escape diagnostic: %+v", d)
		}
	}
	l := New(`$"{F("\q")}"`)
	if token := l.NextToken(); token.Type != INTERPSTRING {
		t.Fatalf("lost outer boundary: %+v", token)
	}
	if d := l.Diagnostics(); len(d) != 1 || d[0].ID != "L1006" || d[0].Primary.Column != 7 {
		t.Fatalf("wrong nested position: %+v", d)
	}
}
