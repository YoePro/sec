package formatter

import "testing"

// Rules:
//   - rules/tooling/formatter.md — § 15(1), § 8(7), § 11(2)
func TestFormatMakesStructuralDeclarationBlocksMultiline(t *testing.T) {
	input := "module main\n\ntype P struct { a: int, b: int }\n\ntype E struct {}\n\nenum Color { Red, Green, }\n\ntype F union error { Broken, Closed }\n\ninterface Shape { fn Area() int }\n\nimpl P { fn Sum() int { return self.a } }\n\ntype C struct { a: int /* note */ }\n"
	want := "module main\n\ntype P struct {\n    a: int,\n    b: int,\n}\n\ntype E struct {}\n\nenum Color {\n    Red,\n    Green,\n}\n\ntype F union error {\n    Broken,\n    Closed,\n}\n\ninterface Shape {\n    fn Area() int\n}\n\nimpl P {\n    fn Sum() int {\n        return self.a\n    }\n}\n\ntype C struct { a: int /* note */ }\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}
