package formatter

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestFormatInvalidInterfaceReceiver preserves rejected receiver syntax rather
// than silently changing its contract to a shared receiver during formatting.
// Rules: rules/declarations/interfaces.md — §3.3;
// rules/tooling/formatter.md — §27 Invalid code.
func TestFormatInvalidInterfaceReceiver(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/interface_consuming_receiver_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := Format(Source{Text: string(data)}, Options{}).Text
	for _, fragment := range []string{"-> fn Detach() int", "-> fn Drop(-> value: int) void"} {
		if !strings.Contains(result, fragment) {
			t.Fatal("formatter changed the invalid receiver contract", result)
		}
	}
	if again := Format(Source{Text: result}, Options{}).Text; again != result {
		t.Fatal("not idempotent", again)
	}
	p := parser.New(lexer.New(result))
	p.ParseProgram()
	if len(p.Diagnostics()) != 2 {
		t.Fatal(p.Errors())
	}
	for _, d := range p.Diagnostics() {
		if d.ID != diagnostics.ParserConsumingInterfaceReceiver {
			t.Fatal(d)
		}
	}
}
