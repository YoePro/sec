package formatter

import "testing"

// Rules:
//   - rules/tooling/formatter.md — §11(1)–(3), §11(7), §17(2)
func TestFormatAddsTrailingCommasToMultilineArgumentsAndLiterals(t *testing.T) {
	input := "fn G() int {\n    let x := F(\n        1,\n        2 // two\n    )\n    let w := [\n        1,\n        // lead\n        2\n    ]\n    let p := P{\n        a: 1,\n        b: 2\n    }\n    let layout := P{\n        a: 1\n        b: 2\n    }\n    let hug := Append(P {\n        a: 1,\n    })\n    let one := F(1, 2,)\n    return Ok(\n        x\n    )\n}\n"
	want := "fn G() int {\n    let x := F(\n        1,\n        2, // two\n    )\n    let w := [\n        1,\n        // lead\n        2,\n    ]\n    let p := P{\n        a: 1,\n        b: 2,\n    }\n    let layout := P{\n        a: 1\n        b: 2\n    }\n    let hug := Append(P {\n        a: 1,\n    })\n    let one := F(1, 2)\n    return Ok(\n        x,\n    )\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}
