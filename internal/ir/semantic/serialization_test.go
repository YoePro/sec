package semantic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/sema"
)

// Transport round trips retain the complete compiler-produced facts, not just
// printable operations: identities, constants, source positions, proofs, CFG,
// aggregate definitions, reference origins and target-dependent widths.
// Rules: rules/compiler/semantic_ir.md — §§97,106–107,109,124,126.
func TestSemanticIRSerializationRoundTrip(t *testing.T) {
	for _, fixture := range []string{
		"basic_return.sec", "wide_scalars.sec", "decimal_return.sec",
		"enum_union_values.sec", "generic_storage_closure.sec",
		"fixed_array_defaults.sec", "checked_integer_ops.sec",
		"slice_source_fixed.sec", "slice_source_fallible.sec",
		"static_storage_valid.sec",
	} {
		for _, width := range []uint16{32, 64} {
			t.Run(fmt.Sprintf("%s/%d", fixture, width), func(t *testing.T) {
				program := sliceBoundaryProgram(t, "semantic_ir/"+fixture)
				analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errors := analyzer.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 17})
				if err != nil {
					t.Fatal(err)
				}
				assertSerializationRoundTrip(t, module)
			})
		}
	}
	// Current P17 ownership/destruction plans and nested defer regions are
	// serialized through the same module shape, with no alternate IR model.
	for _, module := range []*Module{ownershipModule(), cleanupModule()} {
		assertSerializationRoundTrip(t, module)
	}
}

// Compare every exported fact through the canonical wire bytes, independently
// of the human printer's supported metadata, and exercise reconstructed Intern.
// Rules: rules/compiler/semantic_ir.md — §§97,106–107.
func assertSerializationRoundTrip(t *testing.T, module *Module) {
	t.Helper()
	printed := Format(module)
	data, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Deserialize(data)
	if err != nil {
		t.Fatal(err)
	}
	if Format(restored) != printed || restored.PointerWidthBits != module.PointerWidthBits || !reflect.DeepEqual(restored.SourceFiles, module.SourceFiles) {
		t.Fatal("round trip changed IR, source provenance or target facts")
	}
	if !reflect.DeepEqual(restored.Types.All(), module.Types.All()) {
		t.Fatal("round trip changed type facts")
	}
	for index, typ := range module.Types.All() {
		if id := restored.Types.Intern(typ); id != TypeID(index+1) {
			t.Fatal("restored interning changed a type identity", index, id)
		}
	}
	again, err := Serialize(restored)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("non-deterministic round trip: %v", err)
	}
	// Decoder ownership must not alias the original compiler module.
	if len(restored.Functions) > 0 {
		restored.Functions[0].Name = "changed"
		if Format(module) != printed {
			t.Fatal("decoded module mutated original")
		}
	}
}

// Old/new schemas, malformed/ambiguous metadata and corrupted IR never publish
// a partially decoded module. Verification also runs before serialization.
// Rules: rules/compiler/semantic_ir.md — §§96–97,106(1–4).
func TestSemanticIRSerializationRejectsInvalid(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/basic_return.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(map[string]any){
		"wrong format":           func(e map[string]any) { e["format"] = "other" },
		"old transport":          func(e map[string]any) { e["version"] = 0 },
		"new transport":          func(e map[string]any) { e["version"] = SerializationVersion + 1 },
		"missing transport":      func(e map[string]any) { delete(e, "version") },
		"unknown envelope field": func(e map[string]any) { e["future"] = true },
		"null module":            func(e map[string]any) { e["module"] = nil },
		"old operation schema":   func(e map[string]any) { e["module"].(map[string]any)["Version"] = 0 },
		"new operation schema":   func(e map[string]any) { e["module"].(map[string]any)["Version"] = Version + 1 },
		"missing type table":     func(e map[string]any) { delete(e["module"].(map[string]any), "Types") },
		"null type table":        func(e map[string]any) { e["module"].(map[string]any)["Types"] = nil },
		"duplicate type ID": func(e map[string]any) {
			m := e["module"].(map[string]any)
			types := m["Types"].([]any)
			m["Types"] = append(types, types[0])
		},
		"invalid identity": func(e map[string]any) { e["module"].(map[string]any)["Identity"] = "" },
		"null function": func(e map[string]any) {
			e["module"].(map[string]any)["Functions"].([]any)[0] = nil
		},
		"null block": func(e map[string]any) {
			e["module"].(map[string]any)["Functions"].([]any)[0].(map[string]any)["Blocks"].([]any)[0] = nil
		},
		"unknown operation": func(e map[string]any) {
			fn := e["module"].(map[string]any)["Functions"].([]any)[0].(map[string]any)
			block := fn["Blocks"].([]any)[0].(map[string]any)
			block["Operations"].([]any)[0].(map[string]any)["Kind"] = "future.operation"
		},
		"unknown module fact": func(e map[string]any) { e["module"].(map[string]any)["FutureFact"] = true },
		"unknown type fact": func(e map[string]any) {
			e["module"].(map[string]any)["Types"].([]any)[0].(map[string]any)["FutureFact"] = true
		},
		"invalid result type": func(e map[string]any) {
			fn := e["module"].(map[string]any)["Functions"].([]any)[0].(map[string]any)
			fn["ReturnType"] = 999999
		},
	} {
		t.Run(name, func(t *testing.T) {
			var envelope map[string]any
			if err := json.Unmarshal(data, &envelope); err != nil {
				t.Fatal(err)
			}
			change(envelope)
			corrupt, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := Deserialize(corrupt); err == nil || got != nil {
				t.Fatalf("published invalid module: %v %v", got, err)
			}
		})
	}
	for _, corrupt := range [][]byte{
		nil, []byte("null"), []byte("[]"), data[:len(data)/2],
		append(append([]byte{}, data...), []byte("{}")...),
		[]byte(strings.Replace(string(data), `"version": 1`, `"version": 1, "version": 2`, 1)),
		[]byte(strings.Replace(string(data), `"Version": 1`, `"Version": 2, "version": 1`, 1)),
		[]byte(strings.Replace(string(data), `"Identity": "main"`, `"Identity": "main", "Identity": "other"`, 1)),
		[]byte(strings.Replace(string(data), `"SourceFiles":`, `"ſourceFiles": [], "SourceFiles":`, 1)),
		append([]byte{0xff}, data...),
	} {
		if got, err := Deserialize(corrupt); err == nil || got != nil {
			t.Fatalf("published malformed module: %v %v", got, err)
		}
	}
	module.Version++
	if data, err := Serialize(module); err == nil || data != nil {
		t.Fatal("serialized incompatible IR", err)
	}
	if data, err := Serialize(nil); err == nil || data != nil {
		t.Fatal("serialized nil IR", err)
	}
}

// BUG-000022: the module-wide static identity walk runs before per-function
// shape checks and must reject null blocks in functions and defer regions.
// Rules: rules/compiler/semantic_ir.md — §§96–97.
func TestSemanticIRSerializationRejectsNullRegionBlock(t *testing.T) {
	module := cleanupModule()
	found := false
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				if operation.Region != nil && len(operation.Region.Blocks) > 0 {
					operation.Region.Blocks[0] = nil
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("fixture has no nested region")
	}
	if err := Verify(module); err == nil {
		t.Fatal("accepted null region block")
	}
	if data, err := Serialize(module); err == nil || data != nil {
		t.Fatal("serialized null region block", err)
	}
}
