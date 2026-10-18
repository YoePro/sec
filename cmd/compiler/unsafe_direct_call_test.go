package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MD-079: `unsafe Call()` is the preferred single-call authorization. The
// prefix binds to the following complete call at prefix precedence and
// authorizes exactly that call: its arguments, its receiver, and neighboring
// operands keep their own requirements, while ordinary type checking still
// applies inside the authorized call. Anything other than a call after a
// block-less `unsafe` is a parse error.
//
// Rules:
//   - rules/memory/unsafe.md — § 4 single-operation syntax
//   - rules/foundations/grammar.md — UnsafeCallExpression
//   - rules/corrections/applied/md079-md083-md089-md091-md094-md097-correction-20261018.md — § 2
func TestUnsafeDirectCallAuthorizesExactlyOneCall(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/unsafe_direct_call_valid.sec")
	if code != 0 {
		t.Fatalf("valid direct unsafe calls rejected: %d %s %s", code, stdout, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/unsafe_direct_call_invalid.sec")
	if code != 3 {
		t.Fatalf("invalid direct unsafe calls accepted: %d %s", code, stderr)
	}
	for _, want := range []string{
		"calling unsafe function Danger requires unsafe at ../../testdata/sema/unsafe_direct_call_invalid.sec:12:24",
		"calling unsafe function Danger requires unsafe at ../../testdata/sema/unsafe_direct_call_invalid.sec:16:31",
		"RawPtr.Read requires unsafe because it reads through a raw pointer at ../../testdata/sema/unsafe_direct_call_invalid.sec:20:40",
		"argument 1 to Danger must be int, got string",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "sema error"); count != 4 {
		t.Errorf("want exactly 4 diagnostics, got %d:\n%s", count, stderr)
	}
	_, stderr, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/unsafe_direct_call_syntax_invalid.sec")
	if code != 2 || !strings.Contains(stderr, "`unsafe` without a block authorizes exactly one call") {
		t.Fatalf("unsafe without a call accepted: %d %s", code, stderr)
	}
}

// The direct prefix is preserved as source provenance on the Semantic IR call.
//
// Rules:
//   - rules/corrections/applied/md079-md083-md089-md091-md094-md097-correction-20261018.md — § 2.8
func TestUnsafeDirectCallSemanticIRProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.sec")
	source := "module main\n\nunsafe fn Danger(x: int) int {\n    return x + 1\n}\n\nfn main() int {\n    let a := unsafe Danger(1)\n    return a\n}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", path)
	if code != 0 {
		t.Fatalf("emit-ir failed: %d %s", code, stderr)
	}
	if !strings.Contains(stdout, `call.direct "main::Danger(int)"(%0[copy-trivial]) unsafe-direct@loc(`) || !strings.Contains(stdout, `:8:14)`) {
		t.Fatalf("direct unsafe provenance missing:\n%s", stdout)
	}
}
