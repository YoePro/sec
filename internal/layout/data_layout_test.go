package layout

import "testing"

// MD-141 § 3(5): LLVM's defaults apply when the string names no entry, the
// integer lookup uses the next larger or else the largest entry, only the
// ABI alignment counts, and malformed entries are rejected.
func TestScalarAlignmentsFromLLVMDataLayout(t *testing.T) {
	integer := func(width uint16) StorageRepresentation { return StorageRepresentation{IntegerStorage, width} }
	binary := func(width uint16) StorageRepresentation { return StorageRepresentation{BinaryFloatStorage, width} }

	defaults, err := ScalarAlignmentsFromLLVMDataLayout("e")
	if err != nil {
		t.Fatal(err)
	}
	for representation, want := range map[StorageRepresentation]uint64{
		integer(8): 1, integer(16): 2, integer(32): 4, integer(64): 4, integer(128): 4, integer(256): 4,
		binary(32): 4, binary(64): 8,
	} {
		if got := defaults[representation]; got != want {
			t.Errorf("default %s = %d, want %d", representation, got, want)
		}
	}

	// aarch64: i8:8:32 keeps ABI 1 (the 32 is the preferred alignment).
	aarch64, err := ScalarAlignmentsFromLLVMDataLayout("e-m:e-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32")
	if err != nil {
		t.Fatal(err)
	}
	if aarch64[integer(8)] != 1 || aarch64[integer(64)] != 8 || aarch64[integer(128)] != 16 || aarch64[integer(256)] != 16 {
		t.Errorf("aarch64 facts = %v", aarch64)
	}

	// ARM without an i128 entry: i128 and i256 use the largest entry, i64:64.
	arm, err := ScalarAlignmentsFromLLVMDataLayout("e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64")
	if err != nil {
		t.Fatal(err)
	}
	if arm[integer(64)] != 8 || arm[integer(128)] != 8 || arm[integer(256)] != 8 || arm[binary(64)] != 8 {
		t.Errorf("arm facts = %v", arm)
	}

	for _, malformed := range []string{"e-i64:x", "e-i64", "e-i0:8", "e-f64:12", "e-i64:64:64:64"} {
		if _, err := ScalarAlignmentsFromLLVMDataLayout(malformed); err == nil {
			t.Errorf("malformed data layout %q was accepted", malformed)
		}
	}
}
