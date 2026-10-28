package secmlir

import (
	"bytes"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
	"strings"
	"testing"
)

// MD-114 §§2–7,11: source-produced typed/constant/interval register updates
// retain layout/identity through deterministic schema-11/12 native round trips.
func TestEmitDetachedRegisters(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/register_detached_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	tool := testsupport.RequireSecMLIROptPath(t)
	module := buildPackage15EmitModule(t, "register", string(source))
	for _, width := range []uint16{32, 64} {
		for _, schema := range []uint32{11, 12} {
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
			if err != nil {
				t.Fatal(err)
			}
			again, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
			if err != nil || !bytes.Equal(output, again) {
				t.Fatalf("nondeterministic emission: %v", err)
			}
			verifyPackage15MLIR(t, "register", output)
			command := exec.Command(tool, "-")
			command.Stdin = bytes.NewReader(output)
			canonical, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("native rejected registers: %v\n%s\n%s", err, canonical, output)
			}
			repeated := exec.Command(tool, "-")
			repeated.Stdin = bytes.NewReader(canonical)
			second, err := repeated.CombinedOutput()
			if err != nil || !bytes.Equal(canonical, second) {
				t.Fatalf("unstable native round trip: %v\n%s", err, second)
			}
		}
	}
}

// MD-114 §9(6)/§11: a forged table/path/proof must not publish native output,
// including when a later scalar/integer pass is requested.
func TestNativeRejectsForgedRegisters(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/register_detached_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	module := buildPackage15EmitModule(t, "register", string(source))
	output, err := Emit(module, testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, tc := range []struct{ name, old, replacement string }{
		{"layout", "offset = 31, width = 1", "offset = 30, width = 1"},
		{"constant", "basis = \"constant\", low = \"1\", high = \"1\"", "basis = \"constant\", low = \"0\", high = \"0\""},
		{"guard", `predicate = "lt"`, `predicate = "gt"`},
		{"reserved", "sec.field_path = [0 : i32]", "sec.field_path = [5 : i32]"},
		{"proof-family", "sec.write_proof =", "sec.register_fit_proof ="},
		{"old-schema", "sec.dialect_version = 11", "sec.dialect_version = 10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forged := bytes.Replace(output, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(output, forged) {
				t.Fatal("mutation missed target")
			}
			for _, pipeline := range []string{"", "--sec-lower-scalar-core", "--sec-lower-integer-core"} {
				path := filepath.Join(t.TempDir(), "rejected.mlir")
				args := []string{"-", "-o", path}
				if pipeline != "" {
					args = append(args, pipeline)
				}
				command := exec.Command(tool, args...)
				command.Stdin = bytes.NewReader(forged)
				if combined, err := command.CombinedOutput(); err == nil {
					t.Fatalf("forged register accepted: %s", combined)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("failed verification published an artifact: %v", err)
				}
			}
		})
	}
}

// Go pre-emission verification rejects missing registries and malformed or
// fabricated numeric evidence before any MLIR bytes are returned.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§7,9).
func TestEmitRejectsMalformedRegisterFacts(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/register_detached_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "duplicate", "access", "layout", "constant", "guard"} {
		t.Run(kind, func(t *testing.T) {
			module := buildPackage15EmitModule(t, "register", string(source))
			switch kind {
			case "missing":
				module.Registers = nil
			case "duplicate":
				module.Registers = append(module.Registers, module.Registers[0])
			case "access":
				module.Registers[0].Fields[0].Access = "unknown"
			case "layout":
				module.Registers[0].Fields[0].BitOffset--
			case "guard":
				for _, fn := range module.Functions {
					for _, block := range fn.Blocks {
						for i := range block.Operations {
							if block.Operations[i].Kind == semantic.OpIntCompare {
								block.Operations[i].IntegerCompare = "gt"
							}
						}
					}
				}
			case "constant":
				mutated := false
				for _, fn := range module.Functions {
					for _, block := range fn.Blocks {
						for i := range block.Operations {
							op := &block.Operations[i]
							if op.Kind == semantic.OpRegisterValueUpdate && op.RegisterWrite.Basis == "constant" && !mutated {
								op.RegisterWrite.Low = big.NewInt(0)
								op.RegisterWrite.High = big.NewInt(0)
								mutated = true
							}
						}
					}
				}
				if !mutated {
					t.Fatal("no constant update")
				}
			}
			output, err := Emit(module, testPlan(64))
			if err == nil || output != nil {
				t.Fatalf("invalid register %s published output: %v", kind, err)
			}
		})
	}
}

// The existing MD-110 bit-extraction gap remains a named no-output boundary;
// the MD-114 table must not fabricate a fourth conversion-proof basis.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a; MD-114 §8(4).
func TestRegisterBitExtractionTransportRemainsClosed(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/register_bit_extraction_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	module := buildPackage15EmitModule(t, "boundary", string(source))
	output, err := Emit(module, testPlan(64))
	if err == nil || output != nil || !strings.Contains(err.Error(), "MD-110 bit-extraction proof transport") {
		t.Fatalf("missing precise conversion boundary: %v", err)
	}
}
