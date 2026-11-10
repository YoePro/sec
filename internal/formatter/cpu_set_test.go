package formatter

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestCpuSetFormatting preserves positional CPU-set literals and is idempotent.
// Rules: rules/concurrency/threads.md — §7(2); rules/tooling/formatter.md — §§6,31.
func TestCpuSetFormatting(t *testing.T) {
	source, err := os.ReadFile("../../testdata/formatter/cpu_set.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/cpu_set.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{string(source), string(want)} {
		p := parser.New(lexer.New(input))
		p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		if got := Format(Source{Text: input}, Options{}).Text; got != string(want) {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	}
}
