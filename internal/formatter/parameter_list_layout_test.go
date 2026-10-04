package formatter

import "testing"

// Multiline parameter lists are normalized: a deliberate multiline form gets
// one parameter per line, a trailing comma, and `)` on its own line; a mixed
// form whose first parameter hugs `(` becomes single-line when it fits the
// preferred width and the canonical multiline form otherwise.
//
// Rules:
//   - rules/tooling/formatter.md — § 3(7), § 11(3), § 16(1)–(2)
func TestFormatNormalizesMultilineParameterLists(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "mixed form that fits collapses",
			input: "module main\n\nfn Sum(a: int,\n       b: int) int {\n    return a + b\n}\n",
			want:  "module main\n\nfn Sum(a: int, b: int) int {\n    return a + b\n}\n",
		},
		{
			name:  "mixed form with closer line collapses",
			input: "module main\n\nfn F(a: int,\n      b: int,\n) void {\n}\n",
			want:  "module main\n\nfn F(a: int, b: int) void {\n}\n",
		},
		{
			name:  "deliberate multiline gets its closer line",
			input: "module main\n\nfn Sum(\n    a: int,\n    b: int) int {\n    return a + b\n}\n",
			want:  "module main\n\nfn Sum(\n    a: int,\n    b: int,\n) int {\n    return a + b\n}\n",
		},
		{
			name:  "too wide mixed form expands",
			input: "module main\n\nfn VeryLongFunctionNameForTesting(firstArgumentWithLongName: ref mut SomeVeryLongTypeName,\n    secondArgumentWithLongName: ref mut AnotherVeryLongTypeName) Result[uint, IOError] {\n    return Ok(0)\n}\n",
			want:  "module main\n\nfn VeryLongFunctionNameForTesting(\n    firstArgumentWithLongName:  ref mut SomeVeryLongTypeName,\n    secondArgumentWithLongName: ref mut AnotherVeryLongTypeName,\n) Result[uint, IOError] {\n    return Ok(0)\n}\n",
		},
		{
			name:  "method in impl",
			input: "module main\n\ntype P struct {\n    x: int,\n}\n\nimpl P {\n    fn Move(dx: int,\n            dy: int) void {\n    }\n}\n",
			want:  "module main\n\ntype P struct {\n    x: int,\n}\n\nimpl P {\n    fn Move(dx: int, dy: int) void {\n    }\n}\n",
		},
		{
			name:  "commented list is preserved",
			input: "module main\n\nfn F(\n    a: int, // first\n    b: int,\n) void {\n}\n",
			want:  "module main\n\nfn F(\n    a: int, // first\n    b: int,\n) void {\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Format(Source{Text: test.input}, Options{}).Text
			if got != test.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, test.want)
			}
			if again := Format(Source{Text: got}, Options{}).Text; again != got {
				t.Fatalf("not idempotent:\n%s", again)
			}
		})
	}
}
