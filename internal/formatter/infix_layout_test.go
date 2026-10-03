package formatter

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/tooling/formatter.md — § 9(13)–(15)
func TestFormatMultilineInfixUsesLeadingOperatorColumn(t *testing.T) {
	input := "fn F(basePrice: int, shipping: int, discount: int, a: bool, b: bool) int {\n    let total := basePrice\n    + shipping\n    - discount\n    let mut sum := 0\n    sum = basePrice +\n        shipping * 2 +\n        discount\n    let ready :=\n        a\n        && b\n    return total\n}\n"
	want := "fn F(basePrice: int, shipping: int, discount: int, a: bool, b: bool) int {\n    let total :=\n          basePrice\n        + shipping\n        - discount\n    let mut sum := 0\n    sum =\n          basePrice\n        + shipping * 2\n        + discount\n    let ready :=\n          a\n       && b\n    return total\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
	two := Format(Source{Text: input}, Options{IndentationWidth: 2}).Text
	if again := Format(Source{Text: two}, Options{IndentationWidth: 2}).Text; again != two {
		t.Fatalf("width 2 is not idempotent:\n%s", again)
	}
}

func TestFormatMultilineInfixLeavesUnclearLayoutsAlone(t *testing.T) {
	input := "fn F(a: int, b: int, c: int) int {\n    let partial := a + b\n        + c\n    let commented := a // first\n        + b\n    let single := a + b + c\n    return partial\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	for _, unchanged := range []string{"let partial := a + b\n", "let commented := a // first\n", "let single := a + b + c\n"} {
		if !strings.Contains(got, unchanged) {
			t.Fatalf("layout changed a value that is not one operand per line (%q):\n%s", unchanged, got)
		}
	}
}

// Rules:
//   - rules/tooling/formatter.md — § 17(6)–(7)
func TestFormatMultilineMemberChainUsesLeadingDots(t *testing.T) {
	input := "fn F(client: Client) void {\n    let result := client.\n        Request().\n        WithHeader(name, value).\n        Send()\n    let half := client.Request()\n        .Send()\n    let single := client.Request().Send()\n    let call := client.Request(\n        name,\n    )\n}\n"
	want := "fn F(client: Client) void {\n    let result :=\n        client\n            .Request()\n            .WithHeader(name, value)\n            .Send()\n    let half :=\n        client\n            .Request()\n            .Send()\n    let single := client.Request().Send()\n    let call := client.Request(\n        name,\n    )\n}\n"
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("Format() =\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("second pass changed output:\n%s", again)
	}
}
