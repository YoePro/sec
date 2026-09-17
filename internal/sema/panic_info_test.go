package sema

import (
	"math/big"
	"os"
	"testing"
)

// TestCompilerKnownPanicInfoHasCanonicalShape verifies the exact shared
// compiler-known PanicID representation and portable PanicInfo field surface.
//
// Rules:
//   - rules/errors/panic.md — § 13(2)–(4) "Panic information and reason IDs"
//   - rules/corrections/applied/panic-info-compiler-known-correction-20260907.md — §§ 2.1, 3.1 "Exact declaration"
func TestCompilerKnownPanicInfoHasCanonicalShape(t *testing.T) {
	types := builtinTypes()
	id, ok := types["PanicID"]
	if !ok || id.Kind != UintType || !id.Named || id.Underlying != "uint32" || id.BitWidth != 32 ||
		id.MaxInteger == nil || id.MaxInteger.Cmp(new(big.Int).SetUint64(1<<32-1)) != 0 {
		t.Fatalf("PanicID = %+v, want nominal uint32", id)
	}

	info, ok := types["PanicInfo"]
	if !ok || info.Kind != StructType || !info.Named {
		t.Fatalf("PanicInfo = %+v, want named compiler-known struct", info)
	}
	want := []struct {
		name string
		typ  string
	}{
		{"ID", "PanicID"},
		{"File", "string"},
		{"Line", "uint"},
		{"Column", "uint"},
		{"Function", "string"},
	}
	if len(info.Fields) != len(want) {
		t.Fatalf("PanicInfo fields = %+v, want exactly %d fields", info.Fields, len(want))
	}
	for index, expected := range want {
		field := info.Fields[index]
		if field.Name != expected.name || field.Type.Name != expected.typ {
			t.Errorf("PanicInfo field %d = %s: %s, want %s: %s", index, field.Name, field.Type.Name, expected.name, expected.typ)
		}
	}
	outcome := types["TaskOutcome"]
	for _, variant := range outcome.UnionVariants {
		if variant.Name == "Panicked" {
			if variant.Payload == nil || !sameConcreteType(*variant.Payload, info) || len(variant.Payload.Fields) != len(info.Fields) {
				t.Fatalf("TaskOutcome.Panicked payload = %+v, want canonical PanicInfo", variant.Payload)
			}
			return
		}
	}
	t.Fatal("TaskOutcome lacks Panicked(PanicInfo)")
}

// TestCompilerKnownPanicInfoFieldsResolve verifies ordinary Sema member lookup
// against the same canonical field facts consumed by tooling.
//
// Rules:
//   - rules/errors/panic.md — § 13(3)–(7) "Panic information and reason IDs"
func TestCompilerKnownPanicInfoFieldsResolve(t *testing.T) {
	path := "../../testdata/sema/panic_info_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(source)), nil)
}
