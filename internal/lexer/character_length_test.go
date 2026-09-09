package lexer

import (
	"os"
	"sec/internal/diagnostics"
	"testing"
)

func TestCharacterLiteralScalarCount(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
	}{
		{"character_length_valid", 0}, {"character_length_invalid", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "../../testdata/lexer/" + tc.name + ".sec"
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			l := NewWithFile(string(input), path)
			snapshot := l.Snapshot()
			for attempt := 0; attempt < 2; attempt++ {
				following := false
				literals := map[int]Token{}
				for token := l.NextToken(); token.Type != EOF; token = l.NextToken() {
					if token.Type == CHAR {
						literals[token.Line] = token
					}
					if token.Lexeme == "following" {
						following = true
					}
				}
				ds := l.Diagnostics()
				if len(ds) != tc.count {
					t.Fatalf("diagnostics: %+v, want %d", ds, tc.count)
				}
				for _, d := range ds {
					if d.ID != diagnostics.LexerCharacterLiteralLength || d.Primary != literals[d.Primary.Line] {
						t.Fatalf("wrong diagnostic/span: %+v", d)
					}
				}
				if tc.count > 0 && !following {
					t.Fatal("lost following tokens")
				}
				l.Restore(snapshot)
				if len(l.Diagnostics()) != 0 {
					t.Fatal("restore retained diagnostics")
				}
			}
		})
	}
}
