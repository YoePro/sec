package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MD-091: byte-to-string decoding belongs to stdlib/str. Core has no public
// string.FromByteArray, user code cannot reach the internal materialization
// capability, there is no byte[] -> string cast or implicit conversion, and
// byte[].ToString() keeps the universal Result[string, StringError] signature
// as a type summary. The internal capability is known only to core (owner
// decision 2026-10-18): the core byte formatters use it, and even stdlib
// source cannot reach it (the stdlib path is MD-109).
//
// Rules:
//   - rules/library/core-library.md — § 5.2
//   - rules/compiler/compiler_known_members.md — "Internal core string materialization"
//   - sec/stdlib/str/stdlib_str.md — FromByteArray, FromByteArrayLossy
func TestByteStringDecodingBelongsToStdlibStr(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/byte_string_decoding_invalid.sec")
	if code != 3 {
		t.Fatalf("invalid byte decoding accepted: %d %s", code, stderr)
	}
	for _, want := range []string{
		"string.FromByteArray is not a core API; byte-to-string decoding belongs to stdlib/str at ../../testdata/sema/byte_string_decoding_invalid.sec:4:19",
		"help: decode bytes with str.FromByteArray(ref bytes) or str.FromByteArrayLossy(ref bytes) from stdlib/str",
		"_stringFromUtf8 is a compiler-internal operation available only to privileged core source",
		"cannot convert byte[] to string",
		"cannot initialize string with byte[]",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "sema error"); count != 4 {
		t.Errorf("want exactly 4 diagnostics (ToString summary stays valid), got %d:\n%s", count, stderr)
	}

	// A module spelled `str` is not core.
	impostor := filepath.Join(t.TempDir(), "str.sec")
	if err := os.WriteFile(impostor, []byte("module str\n\nfn Sneak(bytes: ref byte[]) Result[string, StringError] {\n    return _stringFromUtf8(bytes)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runCLIForDiagnostics(t, "check", impostor); code != 3 || !strings.Contains(stderr, "_stringFromUtf8 is a compiler-internal operation") {
		t.Fatalf("user module named str reached the internal capability: %d %s", code, stderr)
	}

	if stdout, stderr, code := runCLIForDiagnostics(t, "check", "../../sec/core/byte.sec"); code != 0 {
		t.Errorf("core byte formatters rejected: %d %s %s", code, stdout, stderr)
	}
}
