package secmlir

import (
	"bytes"
	"os"
	"sec/internal/ir/semantic"
	"strconv"
	"strings"
	"testing"
)

// staticPlaceRootsFixture borrows module statics and a function-local static,
// returns references to them directly, through a branch and through a call,
// and leaves one static that is never borrowed.

// Static Place roots emit as sec.place.static over a sec.static.global that
// carries its explicit StaticID and the address-taken marker; returned static
// references are summarized as static@N of that ID. An unborrowed static has
// no marker. The output is deterministic on 32- and 64-bit plans and passes
// the three schema-11 verifiers and the P5/P6/P8 preservation pipeline.
//
// Rules:
//   - rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md — §§ 2-4, 6, 9
//   - rules/mlir/sec_mlir_dialect.md — v11.41.3
//   - rules/mlir/sec_mlir_lowering.md — v11.29
func TestPackage15EmitsStaticPlaceRoots(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_place_roots.sec")
	if err != nil {
		t.Fatal(err)
	}
	staticPlaceRootsFixture := string(source)
	for _, width := range []uint16{32, 64} {
		module := buildPackage15EmitModule(t, "static_place_roots", staticPlaceRootsFixture)
		output, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatalf("width %d: %v", width, err)
		}
		again, err := Emit(buildPackage15EmitModule(t, "static_place_roots", staticPlaceRootsFixture), testPlan(width))
		if err != nil || !bytes.Equal(output, again) {
			t.Fatalf("width %d: emission is not deterministic: %v", width, err)
		}
		text := string(output)
		for _, static := range module.Statics {
			line := ""
			for _, candidate := range strings.Split(text, "\n") {
				if strings.Contains(candidate, `"sec.static.global"`) && strings.Contains(candidate, `sym_name = "`+staticSymbol(static.ID)+`"`) {
					line = candidate
				}
			}
			if !strings.Contains(line, "sec.static_id = "+strconv.Itoa(int(static.ID))+" : i32") {
				t.Errorf("width %d: %s lacks its sec.static_id: %q", width, static.Name, line)
			}
			if strings.Contains(line, "sec.address_taken = true") != static.AddressTaken {
				t.Errorf("width %d: %s address-taken=%t but line %q", width, static.Name, static.AddressTaken, line)
			}
		}
		for _, want := range []string{
			`"sec.place.static"() <{global = @sec_static_1, place_root_id = `,
			`sec.return_origins = ["static@1"]`,
			`sec.return_origins = ["static@2"]`,
		} {
			if !strings.Contains(text, want) {
				t.Errorf("width %d: missing %q in:\n%s", width, want, text)
			}
		}
		verifyPackage15MLIR(t, "static_place_roots", output)
		inherited, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12})
		if err != nil {
			t.Fatal(err)
		}
		verifyPackage15MLIR(t, "static_place_roots_schema12", inherited)

	}
}

// Corrupted independent Place/storage facts must fail before any MLIR bytes.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.3 (MD-116 §§4(6),7(7)).
func TestStaticPlaceIdentityFailurePublishesNoMLIR(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/static_place_roots.sec")
	if err != nil {
		t.Fatal(err)
	}
	module := buildPackage15EmitModule(t, "static_place_roots", string(source))
	var first uint32
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for i := range block.Operations {
				op := &block.Operations[i]
				if op.Kind != semantic.OpPlaceStaticRoot {
					continue
				}
				if first == 0 {
					first = op.Place.Storage
				} else {
					op.Place.Storage = first
				}
			}
		}
	}
	for _, schema := range []uint32{11, 12} {
		output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: schema})
		if err == nil || output != nil {
			t.Fatalf("forged static identities published schema %d output: %v", schema, err)
		}
	}
}
