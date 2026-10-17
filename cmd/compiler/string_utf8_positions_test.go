package main

import (
	"strings"
	"testing"
)

// MD-050/MD-071/MD-072: the trusted core string surface uses rune positions
// for IndexOf/LastIndexOf/Slice/GetRune and byte positions for GetByte,
// borrows its input in FromByteArray/FromRuneArray and materializes owning
// arrays through Result[..., CollectionError]; infallible uses are rejected.
//
// Rules:
//   - rules/library/core-library.md — §§ 5.2, 5.3, 5.7
//   - rules/compiler/compiler_known_members.md — `ToByteArray()`, `ToRuneArray()`
//   - rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md
func TestStringUTF8PositionsAndMaterializationSurface(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/string_utf8_positions_valid.sec"); code != 0 {
		t.Fatal(code, stderr)
	}
	for file, want := range map[string]string{
		"string_utf8_positions_invalid_byte_array.sec":   "cannot initialize byte[] with Result[byte[], CollectionError]",
		"string_utf8_positions_invalid_rune_array.sec":   "cannot initialize rune[] with Result[rune[], CollectionError]",
		"string_utf8_positions_invalid_construction.sec": "cannot initialize string with Result[string, StringError]",
	} {
		t.Run(file, func(t *testing.T) {
			_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+file)
			if code == 0 || !strings.Contains(stderr, want) {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
		})
	}
}
