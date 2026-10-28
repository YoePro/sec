package formatter

import (
	"os"
	"strings"
	"testing"
)

// Trailing-comment columns obey max_alignment_padding group-wide in every
// non-enum group: a column needing exactly the cap stays, one more space
// drops it to single spaces for the whole group while a legal type column
// stays, and rows without comments still widen the code cell. Enum
// explicit-value groups keep both columns under any cap. Every result is a
// formatting fixed point.
//
// Rules:
//   - rules/tooling/formatter.md — §§ 9(10a), 12(6a), 15(4a)
//   - rules/corrections/applied/MD-099-MD-100-MD-037-formatter-alignment-and-selection-correction-20261028.md — §§ 2–4, 10(1)–(2)
func TestFormatCapsTrailingCommentColumn(t *testing.T) {
	cases := []struct {
		name, input, want string
		cap               int
	}{
		{"at cap", "type S struct {\na: int, // one\nab: string, // two\n}\n",
			"type S struct {\n    a:  int,    // one\n    ab: string, // two\n}\n", 4},
		{"one over cap keeps type column", "type S struct {\na: int, // one\nab: float64, // two\n}\n",
			"type S struct {\n    a:  int, // one\n    ab: float64, // two\n}\n", 4},
		{"uncommented row widens the cell", "type S struct {\na: int, // one\nab: float64,\n}\n",
			"type S struct {\n    a:  int, // one\n    ab: float64,\n}\n", 4},
		{"block comments", "type S struct {\na: int, /* one */\nab: float64, /* two */\n}\n",
			"type S struct {\n    a:  int, /* one */\n    ab: float64, /* two */\n}\n", 4},
		{"dropped type column is not restored", "type S struct {\na: int, // one\nabcdef: int, // two\n}\n",
			"type S struct {\n    a: int, // one\n    abcdef: int, // two\n}\n", 4},
		{"register fields", "type R register[8] {\nA: bit, // a\nVeryLongFieldName: bit[7], // b\n}\n",
			"type R register[8] {\n    A: bit, // a\n    VeryLongFieldName: bit[7], // b\n}\n", 2},
		{"bindings", "fn F() int {\nlet a: int := 1 // one\nlet veryLongBindingName: int := 2 // two\nreturn a\n}\n",
			"fn F() int {\n    let a: int := 1 // one\n    let veryLongBindingName: int := 2 // two\n    return a\n}\n", 4},
		{"long comment does not widen the starting column", "type S struct {\na: int, // a deliberately very long trailing comment\nab: string, // b\n}\n",
			"type S struct {\n    a:  int,    // a deliberately very long trailing comment\n    ab: string, // b\n}\n", 4},
		{"implicit enum members are not exempt", "type E enum {\nA, // a\nMuchLongerName, // b\n}\n",
			"type E enum {\n    A, // a\n    MuchLongerName, // b\n}\n", 2},
		{"enum exempt", "type State enum {\nOk = 0, // Ready\nVeryLongStatusName = 1, // Busy\n}\n",
			"type State enum {\n    Ok                 = 0, // Ready\n    VeryLongStatusName = 1, // Busy\n}\n", 2},
		{"enum exempt block comments", "enum State {\nOk = 0, /* Ready */\nVeryLongStatusName = 1, /* Busy */\n}\n",
			"enum State {\n    Ok                 = 0, /* Ready */\n    VeryLongStatusName = 1, /* Busy */\n}\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			options := Options{MaxAlignmentPadding: c.cap}
			got := Format(Source{Text: c.input}, options).Text
			if got != c.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, c.want)
			}
			if again := Format(Source{Text: got}, options).Text; again != got {
				t.Fatalf("not a fixed point:\n%s", again)
			}
		})
	}
}

// Explicit enum value/comment groups end at implicit members and blank lines;
// each explicit group retains its mandatory columns even with a low cap.
// Rules: rules/tooling/formatter.md — §15(4), §15(4a) (MD-100 §3(2)).
func TestEnumCommentExceptionEndsAtImplicitMember(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/enum_comment_boundaries.sec")
	if err != nil {
		t.Fatal(err)
	}
	options := Options{MaxAlignmentPadding: 1}
	got := Format(Source{Text: string(input)}, options).Text
	lines := strings.Split(got, "\n")
	for _, names := range [][2]string{{"A ", "LongFirst "}, {"B ", "MuchLongerSecond "}, {"C ", "LongThird "}} {
		var rows [2]string
		for _, line := range lines {
			for i, name := range names {
				if strings.HasPrefix(strings.TrimSpace(line), name) {
					rows[i] = line
				}
			}
		}
		marker := "//"
		if names[0] == "B " {
			marker = "/*"
		}
		if rows[0] == "" || rows[1] == "" || strings.Index(rows[0], "=") != strings.Index(rows[1], "=") || strings.Index(rows[0], marker) != strings.Index(rows[1], marker) {
			t.Fatalf("lost enum columns across %v:\n%s", names, got)
		}
	}
	if !strings.Contains(got, "Implicit, // Separator") {
		t.Fatalf("implicit member joined the explicit group:\n%s", got)
	}
	if again := Format(Source{Text: got}, options).Text; again != got {
		t.Fatalf("not idempotent:\n%s", again)
	}
}
