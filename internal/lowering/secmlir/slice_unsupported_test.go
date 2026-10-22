package secmlir

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P16 exclusions fail at their owning stage, after a supported source function,
// without a partial module or output. Handle names here are ordinary user
// structs: the undefined public handle API must not be invented from names.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§70–75,121–124,142;
// rules/memory/reference_model.md — §§3(3),10(9),11;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestPackage16UnsupportedSourceMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, tc := range []struct{ file, stage, reason string }{
			{"package16_boundary_owner.sec", "ir", "dynamic"},
			{"package16_boundaries/pointer.sec", "ir", "slice physical pointer extraction"},
			{"package16_boundaries/raw_conversion_invalid.sec", "sema", "slice"},
			{"package16_boundaries/ffi_parameter_invalid.sec", "sema", "ABI"},
			{"package16_boundaries/ffi_return_invalid.sec", "sema", "ABI"},
			{"package16_boundaries/equality_invalid.sec", "sema", "cannot compare ref int[]"},
			{"package16_boundaries/ordering_invalid.sec", "sema", "cannot order ref int[]"},
			{"package16_boundaries/membership.sec", "ir", "slice membership lowering"},
			{"package16_boundaries/iteration.sec", "ir", "for iteration over a collection, string or iterator"},
			{"package16_boundaries/reverse.sec", "ir", "mutable slice Reverse lowering"},
			{"package16_boundaries/signed_property_invalid.sec", "sema", "int"},
			{"package16_boundaries/signed_global_invalid.sec", "sema", "int"},
			{"package16_boundaries/move_only_read_invalid.sec", "sema", "copy"},
			{"package16_boundaries/element_move_invalid.sec", "sema", "not implicitly copyable"},
			{"package16_boundaries/stable_source_invalid.sec", "sema", "slice"},
			{"package16_boundaries/weak_source_invalid.sec", "sema", "slice"},
			{"package16_unsupported/text_value_element.sec", "ir", "P15 trivial value surface"},
			{"package16_unsupported/nontrivial_replacement.sec", "ir", "custom free lifecycle destruction"},
		} {
			t.Run(fmt.Sprintf("%d/%s", width, tc.file), func(t *testing.T) {
				program := &ast.Program{}
				for _, file := range []string{"package16_source_prelude.sec", tc.file} {
					data, err := os.ReadFile("../../../testdata/semantic_ir/" + file)
					if err != nil {
						t.Fatal(err)
					}
					p := parser.New(lexer.NewWithFile(string(data), file))
					parsed := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					program.Statements = append(program.Statements, parsed.Statements...)
				}
				a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
				errs := a.Analyze(program)
				if tc.stage == "sema" {
					if len(errs) == 0 {
						t.Fatal("invalid source accepted")
					}
					for _, err := range errs {
						if err.File != tc.file || !strings.Contains(err.Message, tc.reason) {
							t.Fatal("wrong owning diagnostic", err)
						}
					}
					return // No lowerer is invoked with invalid semantic input.
				}
				if len(errs) != 0 {
					t.Fatal(errs)
				}
				m, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 16})
				var unsupported *semantic.UnsupportedFeatureError
				if m != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, tc.reason) || unsupported.Location.File != tc.file {
					t.Fatalf("lost named boundary or leaked partial module: %v", err)
				}
			})
		}
	}
}

// Verified abstract slices, including nested carriers, cannot acquire a foreign
// ABI or physical layout merely by selecting schema 12. Each mutation starts
// from fresh valid source IR containing supported functions.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§74,121–123,142;
// rules/mlir/sec_mlir_lowering.md — v12.25–31.
func TestPackage16UnsupportedRepresentationMatrix(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, carrier := range []string{"shared", "mutable", "nested-struct", "union"} {
			for _, boundary := range []string{"ffi-parameter", "ffi-return", "layout"} {
				if boundary == "layout" && (carrier == "shared" || carrier == "mutable") {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/%s", width, carrier, boundary), func(t *testing.T) {
					m := sliceEmitModule(t, "slice_source_fixed.sec", width)
					integer := m.Types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
					view, err := m.Types.InternSlice(integer, carrier == "mutable")
					if err != nil {
						t.Fatal(err)
					}
					if carrier == "nested-struct" || carrier == "union" {
						inner := m.Types.Intern(semantic.Type{Kind: semantic.TypeStruct, Name: "SliceField", Module: "main", Identity: "main::SliceField"})
						m.Structs = append(m.Structs, semantic.StructDefinition{TypeID: inner, SymbolID: "main::SliceField", Name: "SliceField", Fields: []semantic.StructFieldDefinition{{ID: 0, Name: "View", Type: view}}, CopyClassification: string(sema.CopyTrivial), TriviallyDestructible: true})
						if carrier == "nested-struct" {
							view = m.Types.Intern(semantic.Type{Kind: semantic.TypeStruct, Name: "SliceEnvelope", Module: "main", Identity: "main::SliceEnvelope"})
							m.Structs = append(m.Structs, semantic.StructDefinition{TypeID: view, SymbolID: "main::SliceEnvelope", Name: "SliceEnvelope", Fields: []semantic.StructFieldDefinition{{ID: 0, Name: "Inner", Type: inner}}, CopyClassification: string(sema.CopyTrivial), TriviallyDestructible: true})
						} else {
							view = m.Types.Intern(semantic.Type{Kind: semantic.TypeUnion, Name: "SliceEnvelope", Module: "main", Identity: "main::SliceEnvelope"})
							m.Unions = append(m.Unions, semantic.UnionDefinition{TypeID: view, SymbolID: "main::SliceEnvelope", Name: "SliceEnvelope", Variants: []semantic.UnionVariantDefinition{{Index: 0, Name: "View", Kind: semantic.UnionVariantSingle, Payload: inner}}, CopyClassification: string(sema.CopyTrivial), TriviallyDestructible: true})
						}
					}
					want := "foreign slice ABI parameter"
					if boundary == "layout" {
						want = "physical slice layout request layout::slice-envelope"
						if carrier == "union" {
							m.Unions[len(m.Unions)-1].LayoutRef = "layout::slice-envelope"
						} else {
							m.Structs[len(m.Structs)-1].LayoutRef = "layout::slice-envelope"
						}
					} else {
						void := m.Types.Intern(semantic.Type{Kind: semantic.TypeVoid, Name: "void"})
						fn := &semantic.Function{ID: "main::ForeignSlice", Name: "ForeignSlice", Extern: true, ABI: "C", ReturnType: void}
						if boundary == "ffi-return" {
							fn.ReturnType = view
							want = "foreign slice ABI return"
						} else {
							p := semantic.Parameter{Name: "value", Value: semantic.Value{ID: 1, Type: view, Ownership: semantic.OwnershipOwned}}
							if carrier == "shared" || carrier == "mutable" {
								p.Value.Ownership = semantic.OwnershipSharedReference
								if carrier == "mutable" {
									p.Value.Ownership = semantic.OwnershipMutableReference
								}
								p.Reference = &semantic.ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: semantic.ReferenceProven, AddressSpace: semantic.AddressSpaceOrdinary, Provenance: "parameter"}
							}
							fn.Parameters = []semantic.Parameter{p}
						}
						m.Functions = append(m.Functions, fn)
					}
					if err := semantic.Verify(m); err != nil {
						t.Fatal("boundary input must verify", err)
					}
					output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
					var unsupported *UnsupportedLoweringError
					if output != nil || !errors.As(err, &unsupported) || unsupported.Feature != want {
						t.Fatalf("wrong boundary or partial output: %v", err)
					}
				})
			}
		}
	}
}

// The real one-pass tool must reject physical slice lowering without a file,
// both at the support gate and at the composed source-to-LLVM boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§113,121,142;
// rules/compiler/compiler_pipeline.md — §45(6).
func TestPackage16UnsupportedPhysicalPipeline(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		m := sliceEmitModule(t, "slice_source_fixed.sec", width)
		output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if err != nil {
			t.Fatal(err)
		}
		valid := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-")
		valid.Stdin = bytes.NewReader(output)
		if diagnosis, err := valid.CombinedOutput(); err != nil {
			t.Fatalf("baseline must pass: %v\n%s", err, diagnosis)
		}
		for _, tc := range []struct {
			args   []string
			reason string
		}{
			{[]string{"--sec-check-lowering-support"}, "does not lower 'sec.slice."},
			{[]string{"--sec-lower-to-llvm"}, "does not lower 'sec.slice."},
		} {
			artifact := filepath.Join(t.TempDir(), "unsupported.mlir")
			cmd := exec.Command(tool, append(tc.args, "-o", artifact, "-")...)
			cmd.Stdin = bytes.NewReader(output)
			if diagnosis, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(diagnosis, []byte(tc.reason)) {
				t.Fatalf("lost named physical gate: %v\n%s", err, diagnosis)
			}
			if _, err := os.Stat(artifact); !os.IsNotExist(err) {
				t.Fatal("failed lowering left artifact", err)
			}
		}
	}
}
