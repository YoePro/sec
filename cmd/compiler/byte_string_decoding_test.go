package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MD-109 restores static core materializers, while MD-091 byte ToString
// remains a summary. Instance calls, casts, implicit decoding and direct access
// to the private materializer stay rejected, including spoofed module names.
// Rules: rules/corrections/md109-core-string-materialization-correction.md
// — §§1–2,8,10.2,11.3.4; core-library.md — §5.2.
func TestStringMaterializationAuthority(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/byte_string_decoding_invalid.sec")
	if code != 3 {
		t.Fatalf("invalid byte decoding accepted: %d %s", code, stderr)
	}
	for _, want := range []string{
		"string.FromByteArray is static and cannot be called through an instance",
		"string.FromRuneArray is static and cannot be called through an instance",
		"help: Call string.FromByteArray(data) using the type name; the input is borrowed.",
		"_stringFromUtf8 is a compiler-internal operation available only to privileged core source",
		"cannot convert byte[] to string",
		"cannot initialize string with byte[]",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in:\n%s", want, stderr)
		}
	}
	if count := strings.Count(stderr, "sema error"); count != 5 {
		t.Errorf("want exactly 5 diagnostics (ToString summary stays valid), got %d:\n%s", count, stderr)
	}

	// Neither the stdlib's spelling nor a spoofed core module grants trust.
	for _, module := range []string{"str", "core"} {
		impostor := filepath.Join(t.TempDir(), module+".sec")
		if err := os.WriteFile(impostor, []byte("module "+module+"\n\nfn Sneak(bytes: ref byte[]) Result[string, StringError] {\n    return _stringFromUtf8(bytes)\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runCLIForDiagnostics(t, "check", impostor); code != 3 || !strings.Contains(stderr, "_stringFromUtf8 is a compiler-internal operation") {
			t.Fatalf("user module named %s reached the internal capability: %d %s", module, code, stderr)
		}
	}

	if stdout, stderr, code := runCLIForDiagnostics(t, "check", "../../sec/core/byte.sec"); code != 0 {
		t.Errorf("core byte formatters rejected: %d %s %s", code, stdout, stderr)
	}
}
