package secmlir

import (
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// TestTargetSizedSourceLayoutParity follows source boundaries through the
// frontend, nominal Semantic IR, emitted scalar plan and actual physical MLIR
// layout. Fixed widths stay independent of the selected target width.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/memory/layout.md — §18(1–3);
// rules/corrections/applied/correction5-20260823.md — Normative requirement (1–5);
// rules/mlir/packages/sec-mlir-dialect_package6.md — §§22–23.
func TestTargetSizedSourceLayoutParity(t *testing.T) {
	template := scalarParityTemplate(t, "target_scalar_boundaries.sec.in")
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			min, max, unsigned := scalarParityBounds(width)
			source := strings.NewReplacer("__SIGNED_MIN__", min.String(), "__SIGNED_MAX__", max.String(), "__UNSIGNED_MAX__", unsigned.String()).Replace(template)
			module := scalarParityBuild(t, source, testPlan(width))
			count := 0
			runtimeNeg := false
			nominal := map[string]bool{}
			for _, typ := range module.Types.All() {
				if typ.Kind == semantic.TypeNamed {
					nominal[typ.Name] = true
				}
			}
			if !nominal["Signed"] || !nominal["Unsigned"] || !nominal["Derived"] {
				t.Fatal("nominal carrier identities erased", nominal)
			}
			for _, fn := range module.Functions {
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.Kind == semantic.OpIntNegChecked {
							runtimeNeg = true
						}
						if op.Kind != semantic.OpConstInt {
							continue
						}
						if len(op.Results) != 1 {
							t.Fatal("integer constant lacks type", op)
						}
						typ := scalarParityCarrier(t, module, op.Results[0].Type)
						bits := typ.BitWidth
						if typ.TargetSize {
							bits = width
						}
						if !scalarParityFits(op.Integer, bits, typ.Signed) {
							t.Fatalf("accepted %s cannot fit physical %d-bit signed=%v", op.Integer, bits, typ.Signed)
						}
						count++
					}
				}
			}
			if !runtimeNeg {
				t.Fatal("runtime negation lost its checked operation")
			}
			if count < 14 {
				t.Fatal("missing constant consumers", count)
			}
			emitted, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			text := string(emitted)
			for _, want := range []string{fmt.Sprintf("#dlti.dl_entry<index, %d>", width), "!sec.int", "!sec.uint", "si64", "ui64", min.String(), max.String(), unsigned.String()} {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q in emitted module", want)
				}
			}
			t.Run("physical-layout", func(t *testing.T) {
				tool := testsupport.RequireSecMLIROptPath(t)
				path := filepath.Join(t.TempDir(), "boundaries.mlir")
				if err := os.WriteFile(path, emitted, 0600); err != nil {
					t.Fatal(err)
				}
				lowered, err := exec.Command(tool, path, "--sec-resolve-scalar-layout", "--verify-each").CombinedOutput()
				if err != nil {
					t.Fatalf("physical layout rejected frontend-accepted source: %v\n%s", err, lowered)
				}
				output := string(lowered)
				for _, want := range []string{fmt.Sprintf("si%d", width), fmt.Sprintf("ui%d", width), "si64", "ui64", min.String(), max.String(), unsigned.String()} {
					if !strings.Contains(output, want) {
						t.Fatalf("physical layout lost %q:\n%s", want, output)
					}
				}
				if strings.Contains(output, "!sec.int") || strings.Contains(output, "!sec.uint") {
					t.Fatal("unresolved native width survived layout")
				}
			})
		})
	}
}

// TestTargetSizedRepresentabilityAgreement checks both sides of every signed
// and unsigned boundary using independent bit-domain arithmetic, including a
// value rejected on 32 bits but accepted on 64 bits and fixed-width controls.
// Rules: rules/types/types.md — "int and uint", "Context shaping";
// rules/memory/layout.md — §18(1–3);
// rules/corrections/applied/correction5-20260823.md — Normative requirement.
func TestTargetSizedRepresentabilityAgreement(t *testing.T) {
	template := scalarParityTemplate(t, "target_scalar_representability.sec.in")
	for _, width := range []uint16{32, 64} {
		for _, carrier := range []struct {
			name   string
			bits   uint16
			signed bool
		}{{"int", width, true}, {"uint", width, false}, {"int32", 32, true}, {"uint32", 32, false}, {"int64", 64, true}, {"uint64", 64, false}} {
			min, max, unsigned := scalarParityBounds(carrier.bits)
			if !carrier.signed {
				min, max = new(big.Int), unsigned
			}
			candidates := []*big.Int{new(big.Int).Sub(min, big.NewInt(1)), min, new(big.Int).Add(min, big.NewInt(1)), new(big.Int), new(big.Int).Sub(max, big.NewInt(1)), max, new(big.Int).Add(max, big.NewInt(1)), big.NewInt(3000000000)}
			for _, value := range candidates {
				t.Run(fmt.Sprintf("%d/%s/%s", width, carrier.name, value), func(t *testing.T) {
					source := strings.NewReplacer("__TYPE__", carrier.name, "__VALUE__", value.String()).Replace(template)
					p := parser.New(lexer.NewWithFile(source, "representability.sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					plan := testPlan(width)
					a := sema.NewAnalyzerWithScalarPlan(plan)
					errs := a.Analyze(program)
					want := scalarParityFits(value, carrier.bits, carrier.signed)
					if (len(errs) == 0) != want {
						t.Fatalf("frontend=%v physical=%v: %v", len(errs) == 0, want, errs)
					}
					if !want {
						return
					}
					module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"representability.sec"}})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := Emit(module, plan); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func scalarParityTemplate(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../../testdata/ir", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func scalarParityBuild(t *testing.T, source string, plan layout.ResolvedScalarPlan) *semantic.Module {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "boundaries.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"boundaries.sec"}})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// scalarParityCarrier follows nominal and array type references without erasing
// their stored identity when checking the resolved scalar representation.
// Rules: rules/types/types.md — Named types; rules/memory/layout.md — §18.
func scalarParityCarrier(t *testing.T, module *semantic.Module, id semantic.TypeID) semantic.Type {
	t.Helper()
	typ, err := module.Types.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	for typ.Kind == semantic.TypeNamed {
		typ, err = module.Types.Get(typ.Base)
		if err != nil {
			t.Fatal(err)
		}
	}
	return typ
}

// scalarParityFits derives integer representability from physical bit width.
// Rules: rules/memory/layout.md — §18(1–3).
func scalarParityFits(value *big.Int, bits uint16, signed bool) bool {
	if bits == 0 || value == nil {
		return false
	}
	if !signed {
		return value.Sign() >= 0 && value.BitLen() <= int(bits)
	}
	if value.Sign() >= 0 {
		return value.BitLen() < int(bits)
	}
	magnitude := new(big.Int).Neg(value)
	return magnitude.BitLen() < int(bits) || magnitude.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(bits-1))) == 0
}

// scalarParityBounds defines the endpoints of signed/unsigned physical storage.
// Rules: rules/memory/layout.md — §18(1–3).
func scalarParityBounds(bits uint16) (*big.Int, *big.Int, *big.Int) {
	half := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	full := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	return new(big.Int).Neg(half), new(big.Int).Sub(half, big.NewInt(1)), new(big.Int).Sub(full, big.NewInt(1))
}

// TestTargetSizedPhysicalRejectsOutOfRange prevents the physical verifier from
// accepting constants which the same target's frontend must reject, even when
// an earlier consumer bypasses Sema. No truncation may manufacture valid IR.
// Rules: rules/memory/layout.md — §18(1–3);
// rules/mlir/packages/sec-mlir-dialect_package6.md — constant representability,
// §§22–23; rules/types/types.md — "int and uint".
func TestTargetSizedPhysicalRejectsOutOfRange(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	template := scalarParityTemplate(t, "target_scalar_representability.sec.in")
	for _, width := range []uint16{32, 64} {
		for _, name := range []string{"int", "uint"} {
			min, max, unsigned := scalarParityBounds(width)
			if name == "uint" {
				min, max = new(big.Int), unsigned
			}
			for _, bad := range []*big.Int{new(big.Int).Sub(min, big.NewInt(1)), new(big.Int).Add(max, big.NewInt(1))} {
				t.Run(fmt.Sprintf("%d/%s/%s", width, name, bad), func(t *testing.T) {
					source := strings.NewReplacer("__TYPE__", name, "__VALUE__", "0").Replace(template)
					module := scalarParityBuild(t, source, testPlan(width))
					mutated := false
					for _, fn := range module.Functions {
						for _, block := range fn.Blocks {
							for i := range block.Operations {
								if block.Operations[i].Kind == semantic.OpConstInt {
									block.Operations[i].Integer = new(big.Int).Set(bad)
									mutated = true
								}
							}
						}
					}
					if !mutated {
						t.Fatal("fixture lacks integer constant")
					}
					emitted, err := Emit(module, testPlan(width))
					if err != nil {
						return
					} // Canonical IR validation may reject earlier.
					path := filepath.Join(t.TempDir(), "out-of-range.mlir")
					if err := os.WriteFile(path, emitted, 0600); err != nil {
						t.Fatal(err)
					}
					output, err := exec.Command(tool, path, "--sec-resolve-scalar-layout", "--verify-each").CombinedOutput()
					if err == nil {
						t.Fatalf("physical layout accepted %s in %d-bit %s:\n%s", bad, width, name, output)
					}
					if !strings.Contains(string(output), "representable") && !strings.Contains(string(output), "range") && !strings.Contains(string(output), "negative value") {
						t.Fatalf("unexpected verifier failure: %v\n%s", err, output)
					}
				})
			}
		}
	}
}
