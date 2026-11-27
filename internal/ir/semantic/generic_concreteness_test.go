package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

// Executable and independently loaded IR must reject template placeholders even
// when a valid generic builder produced the original nested storage graph.
// Rules: rules/compiler/generics_lowering.md — §§4–5,7–8;
// rules/compiler/semantic_ir.md — §§20,106.
func TestGenericConcretenessRejectsUnresolvedIR(t *testing.T) {
	for _, kind := range []TypeKind{"", "generic", "generic-pack", "invalid", "future-type"} {
		t.Run(string(kind), func(t *testing.T) {
			program, analyzer := sliceBoundaryInput(t, "semantic_ir/generic_storage_closure.sec")
			module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			wire, err := Serialize(module)
			if err != nil {
				t.Fatal(err)
			}
			// Corrupt the concrete leaf shared by Box[int], Choice[int], their
			// field/payload definitions and function signatures. No name-based
			// heuristic can establish its concreteness after this change.
			leaf := -1
			for index, typ := range module.Types.types {
				if typ.Kind == TypeInt {
					leaf = index
					break
				}
			}
			if leaf < 0 {
				t.Fatal("fixture has no concrete int argument")
			}
			module.Types.types[leaf].Kind = kind
			module.Types.types[leaf].TargetSize = false
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "generic concreteness:") {
				t.Fatalf("Verify accepted unresolved leaf: %v", err)
			}
			if output, err := Serialize(module); err == nil || output != nil {
				t.Fatalf("Serialize published unresolved IR: %s, %v", output, err)
			}
			var envelope map[string]any
			if err := json.Unmarshal(wire, &envelope); err != nil {
				t.Fatal(err)
			}
			types := envelope["module"].(map[string]any)["Types"].([]any)
			types[leaf].(map[string]any)["Kind"] = string(kind)
			types[leaf].(map[string]any)["TargetSize"] = false
			wire, err = json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if restored, err := Deserialize(wire); err == nil || restored != nil || !strings.Contains(err.Error(), "generic concreteness:") {
				t.Fatalf("Deserialize published unresolved IR: %v, %v", restored, err)
			}
		})
	}
}

// Phantom arguments are part of concrete identity even with no field using
// them. Dangling edges must not survive just because the owner has a known kind.
// Rules: rules/compiler/generics_lowering.md — §§4–6.
func TestGenericConcretenessRejectsDanglingTypeEdges(t *testing.T) {
	for _, edge := range []string{"base", "success", "error", "underlying", "element", "argument"} {
		t.Run(edge, func(t *testing.T) {
			module := &Module{Version: Version, Identity: "main", Types: NewTypeTable()}
			typ := Type{Kind: TypeInt, Name: "int", TargetSize: true}
			switch edge {
			case "base":
				typ.Base = 99
			case "success":
				typ.Success = 99
			case "error":
				typ.Error = 99
			case "underlying":
				typ.Underlying = 99
			case "element":
				typ.Element = 99
			case "argument":
				typ.TypeArgs = []TypeID{99}
			}
			module.Types.Intern(typ)
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "generic concreteness:") || !strings.Contains(err.Error(), edge) {
				t.Fatalf("dangling %s crossed closure gate: %v", edge, err)
			}
		})
	}
	module := &Module{Version: Version, Identity: "main", Types: NewTypeTable()}
	module.Types.Intern(Type{Kind: "generic", Name: "T"})
	module.Types.Intern(Type{Kind: TypeInt, Name: "Phantom", TypeArgs: []TypeID{1}})
	if err := Verify(module); err == nil {
		t.Fatal("unresolved phantom argument accepted")
	}
}

// Type spelling and target representation are independent from semantic closure.
// Rules: rules/compiler/generics_lowering.md — §§4–6.
func TestGenericConcretenessDoesNotRequirePhysicalLayout(t *testing.T) {
	module := &Module{Version: Version, Identity: "main", Types: NewTypeTable()}
	module.Types.Intern(Type{Kind: TypeInt, Name: "T", TargetSize: true})
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}
