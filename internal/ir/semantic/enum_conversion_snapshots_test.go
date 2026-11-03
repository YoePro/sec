package semantic

import (
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"strings"
	"testing"
)

// A consumer editing its copy of a conversion fact cannot change subsequent
// enum definitions, nominal generic identities, exact proofs or emitted IR.
// Rules: rules/compiler/semantic_ir.md — §124;
// rules/types/types.md — Type identity, Exact integer conversion proof.
func TestEnumConversionSnapshotsPreserveIR(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/enum_conversion_snapshots.sec")
	options := BuildOptions{RequestedModule: "main", MaxPackage: 16}
	before, err := Build(program, a, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(before); err != nil {
		t.Fatal(err)
	}
	text := Format(before)
	// An independently transformed IR module must not mutate Sema either.
	for _, decl := range before.Enums {
		for _, item := range decl.Cases {
			if item.Value != nil {
				item.Value.SetInt64(999)
			}
		}
	}
	_ = walk.Inspect(program, func(node any) error {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		fact, ok := a.ResolvedEnumConversionOf(call)
		if !ok {
			return nil
		}
		fact.EnumType.EnumValues[0] = "corrupted"
		for name, value := range fact.EnumType.EnumConsts {
			if value.Value != nil {
				value.Value.SetInt64(999)
			}
			delete(fact.EnumType.EnumConsts, name)
		}
		if len(fact.EnumType.TypeArgs) > 0 {
			fact.EnumType.TypeArgs[0].Name = "corrupted"
		}
		if fact.Proof.SourceMin != nil {
			fact.Proof.SourceMin.SetInt64(999)
		}
		return nil
	})
	after, err := Build(program, a, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(after); err != nil {
		t.Fatal(err)
	}
	if Format(after) != text {
		t.Fatalf("query changed emitted IR:\n%s", Format(after))
	}
	found := false
	for _, decl := range after.Enums {
		if string(decl.SymbolID) == "main::Generic<main::FixedBox>" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixed nominal generic identity lost:\n%s", text)
	}
}

// Tampering with one returned fallibility flag does not authorize an exact
// conversion. The builder reads the retained decision and stops before emission.
// Rules: rules/compiler/semantic_ir.md — §124;
// rules/declarations/enums.md — §12.1.
func TestEnumConversionSnapshotCannotBypassFallibility(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/enum_checked_boundary/fallible.sec")
	count := 0
	_ = walk.Inspect(program, func(node any) error {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		fact, ok := a.ResolvedEnumConversionOf(call)
		if !ok {
			return nil
		}
		count++
		if !fact.Fallible {
			t.Fatal("missing fallibility")
		}
		fact.Fallible = false
		return nil
	})
	if count != 1 {
		t.Fatalf("facts: %d", count)
	}
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if module != nil || err == nil || !strings.Contains(err.Error(), "fallible enum to integer conversion Wide to uint8") {
		t.Fatalf("fallible conversion escaped: %v / %v", module, err)
	}
}
