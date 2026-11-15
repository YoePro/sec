package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// P17-49: a foreign call never consumes an owned Sec value. Every foreign
// ABI rejects owning, non-trivially destructible Sec types in parameters and
// results, so the dialect's foreign-consumption action has no producer;
// ownership transfer to foreign code goes through an ordinary owning Sec
// wrapper whose own calls carry the ordinary P17 actions.
// Rules: rules/platform/ffi.md — §§ 13, 35–36;
// rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — § 25.
func TestForeignSignaturesRejectOwnedSecValues(t *testing.T) {
	wrapper := "type Handle struct {\n    raw: RawPtr[int32],\n}\n\nimpl Handle {\n    free {\n    }\n}\n\n"
	for _, item := range []struct{ declaration, want string }{
		{`unsafe extern "C" fn Take(values: int[]) void`, "cannot cross the C ABI boundary"},
		{`unsafe extern "system" fn Take(values: int[]) void`, "cannot cross the system ABI boundary"},
		{`unsafe extern "Sec" fn Take(values: int[]) void`, "non-ABI-compatible type"},
		{`unsafe extern "C" fn Make() int[]`, "cannot cross the C ABI boundary"},
		{`unsafe extern "Sec" fn Make() int[]`, "non-ABI-compatible return type"},
		{`unsafe extern "C" fn Take(text: string) void`, "cannot cross the C ABI boundary"},
		{`unsafe extern "C" fn Take(handle: Handle) void`, "cannot cross the C ABI boundary"},
		{`unsafe extern "system" fn Make() Handle`, "cannot cross the system ABI boundary"},
	} {
		source := "module main\n\n" + wrapper + item.declaration + "\n\nfn main() int {\n    return 0\n}\n"
		p := parser.New(lexer.NewWithFile(source, "sample.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatalf("%s: parse errors %v", item.declaration, p.Errors())
		}
		errs := NewAnalyzer().Analyze(parsed.Program)
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Error(), item.want)
		}
		if !found {
			t.Errorf("%s: want %q, got %v", item.declaration, item.want, errs)
		}
	}
}
