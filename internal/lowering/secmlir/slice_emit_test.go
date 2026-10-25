package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// Build directly from target-aware source analysis; generated IR is never
// patched to make emission pass. Both schemas use the same verified emitter.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/sec_mlir_lowering.md — v12.1–21,31.
func sliceEmitModule(t *testing.T, file string, width uint16) *semantic.Module {
	t.Helper()
	source, err := os.ReadFile("../../../testdata/semantic_ir/" + file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(testPlan(width))
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	m, err := semantic.Build(parsed.Program, a, semantic.BuildOptions{RequestedModule: "main", MaxPackage: 16, SourceFiles: []string{file}})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Supported source operations emit their exact typed SSA/CFG, identities,
// provenance and locations, round-trip through the real dialect, and pass P42
// before and after scalar layout, together with all three P15 verifiers.
// Rules: rules/mlir/sec_mlir_lowering.md — v12.1–21,29–31;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§87,91,105–110,141.
func TestPackage16EmitSourceSlices(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		for _, file := range []string{"slice_source_fixed.sec", "slice_source_reborrow.sec", "slice_source_length.sec", "slice_source_index.sec", "slice_source_wide.sec", "slice_emit_nominal.sec", "slice_aggregate_arrays.sec", "slice_disjoint_mutable.sec", "slice_empty_mutable.sec", "slice_source_parameters.sec", "slice_source_transfers.sec", "slice_source_returns.sec", "slice_source_element_origins.sec", "slice_source_aggregates.sec", "slice_source_automatic_ends.sec"} {
			t.Run(fmt.Sprintf("%d/%s", width, file), func(t *testing.T) {
				m := sliceEmitModule(t, file, width)
				before := semantic.Format(m)
				output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil {
					t.Fatal(err)
				}
				again, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil || !bytes.Equal(output, again) || before != semantic.Format(m) {
					t.Fatal("emission mutated input or became nondeterministic", err)
				}
				if !bytes.Contains(output, []byte(`sec.dialect_version = 12 : i32`)) || !bytes.Contains(output, []byte(`loc("`+file+`":`)) {
					t.Fatal("lost schema or source locations")
				}
				static, dynamic := 0, 0
				for _, fn := range m.Functions {
					for _, b := range fn.Blocks {
						for _, op := range b.Operations {
							if op.Slice == nil {
								continue
							}
							if r := op.Slice.StaticRange; r != nil {
								static++
								for _, want := range []string{"sec.slice_static_start = " + mlirString(r.Start.String()), "sec.slice_static_end_exclusive = " + mlirString(r.EndExclusive.String())} {
									if !bytes.Contains(output, []byte(want)) {
										t.Fatal("lost exact range", want)
									}
								}
							}
							if op.Slice.DynamicRange != 0 {
								dynamic++
								if !bytes.Contains(output, []byte(fmt.Sprintf("sec.slice_range_id = %d : i32", op.Slice.DynamicRange))) {
									t.Fatal("lost dynamic range identity")
								}
							}
						}
					}
				}
				if static == 0 || ((file == "slice_source_fixed.sec" || file == "slice_source_reborrow.sec" || file == "slice_source_wide.sec") && dynamic == 0) {
					t.Fatal("fixture lost static/dynamic provenance coverage", static, dynamic)
				}
				for _, args := range [][]string{{"--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics"}, {"--sec-resolve-scalar-layout", "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics"}} {
					cmd := exec.Command(tool, append(args, "-")...)
					cmd.Stdin = bytes.NewReader(output)
					parsed, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("real dialect %v: %v\n%s", args, err, parsed)
					}
					round := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-")
					round.Stdin = bytes.NewReader(parsed)
					canonical, err := round.CombinedOutput()
					if err != nil || !bytes.Equal(parsed, canonical) {
						t.Fatalf("round trip: %v\n%s", err, canonical)
					}
				}
				if legacy, err := Emit(m, testPlan(width)); err == nil || legacy != nil {
					t.Fatal("default schema 11 unexpectedly admitted slices")
				}
				artifact := filepath.Join(t.TempDir(), "unsupported.mlir")
				gate := exec.Command(tool, "--sec-check-lowering-support", "-o", artifact, "-")
				gate.Stdin = bytes.NewReader(output)
				if diagnosis, err := gate.CombinedOutput(); err == nil || !bytes.Contains(diagnosis, []byte("does not lower 'sec.slice.")) {
					t.Fatalf("physical lowering boundary lost: %v\n%s", err, diagnosis)
				}
				if _, err := os.Stat(artifact); !os.IsNotExist(err) {
					t.Fatal("unsupported physical lowering left an artifact", err)
				}
			})
		}
	}
}

// Unsupported handler schemas, target mismatches and invalid provenance must
// return no bytes, even after earlier supported functions could be emitted.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/sec_mlir_lowering.md — v12.29–31; missing-decisions.yaml — MD-031.
func TestPackage16EmitRetainsFailureBoundaries(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		m := sliceEmitModule(t, "slice_source_fallible.sec", width)
		out, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if out != nil || err == nil || !strings.Contains(err.Error(), "guarded try handlers") {
			t.Fatal("lost MD-031 boundary", err)
		}
		m = sliceEmitModule(t, "slice_source_reborrow.sec", width)
		other := uint16(32)
		if width == 32 {
			other = 64
		}
		out, err = EmitWithOptions(m, testPlan(other), EmitOptions{DialectVersion: 12})
		if out != nil || err == nil || !strings.Contains(err.Error(), "scalar plan") {
			t.Fatal("stale target plan accepted", err)
		}
		for _, fn := range m.Functions {
			for _, b := range fn.Blocks {
				for i := range b.Operations {
					if facts := b.Operations[i].Slice; facts != nil {
						facts.Mutable = !facts.Mutable
					}
				}
			}
		}
		out, err = EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if out != nil || err == nil || !strings.Contains(err.Error(), "slice value facts") {
			t.Fatal("invalid detached provenance accepted", err)
		}
	}
}

// Holder copy/move are represented IR operations even while their general
// source producers remain P16-36. Dynamic epoch/storage/pin facts must survive
// without a backing observation, runtime helper or element copy.
// Rules: rules/mlir/sec_mlir_lowering.md — v12.10–11,21,29;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–46,98–99,111.
func TestPackage16EmitHolderTransfers(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		for _, mutable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/mutable=%v", width, mutable), func(t *testing.T) {
				types := semantic.NewTypeTable()
				element := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int128", BitWidth: 128, Signed: true})
				view, err := types.InternSlice(element, mutable)
				if err != nil {
					t.Fatal(err)
				}
				void := types.Intern(semantic.Type{Kind: semantic.TypeVoid, Name: "void"})
				facts := &semantic.ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: semantic.ReferenceDynamicEpoch, InvalidationDomain: 3, EpochDependency: 9, AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "Stable", Pinned: true, Provenance: "parameter", OriginStorages: []uint32{7, 8}}
				copied := *facts
				copied.SourceBorrow = 1
				kind, ownership, name := semantic.OpSliceCopyShared, semantic.OwnershipSharedReference, "copy_shared"
				if mutable {
					kind, ownership, name = semantic.OpSliceMoveMutable, semantic.OwnershipMutableReference, "move_mut"
				}
				fn := &semantic.Function{ID: "main::Transfer", Name: "Transfer", ReturnType: void, Entry: 0,
					Parameters: []semantic.Parameter{{Name: "view", Value: semantic.Value{ID: 1, Type: view, Ownership: ownership}, Reference: facts}}}
				ops := []semantic.Operation{
					{Kind: kind, Operands: []semantic.ValueID{1}, Results: []semantic.Value{{ID: 2, Type: view, Ownership: ownership}}, Reference: &copied},
					{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{2}, Reference: &copied},
				}
				if !mutable {
					ops = append(ops, semantic.Operation{Kind: semantic.OpReferenceEndBorrow, Operands: []semantic.ValueID{1}, Reference: facts})
				}
				fn.Blocks = []*semantic.Block{{ID: 0, Operations: append(ops, semantic.Operation{Kind: semantic.OpReturn})}}
				m := &semantic.Module{Version: semantic.Version, Identity: "main", PointerWidthBits: width, Types: types, Functions: []*semantic.Function{fn}}
				output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"sec.slice." + name, "source_borrow = 1 : i32", "invalidation_domain = 3 : i32", "epoch_dependency = 9 : i32", "sec.pinned = true", "sec.origin_storages = [7 : i32, 8 : i32]"} {
					if !bytes.Contains(output, []byte(want)) {
						t.Fatal("lost holder facts", want)
					}
				}
				for _, bad := range []string{"sec.ref.is_valid", "sec.place.read", "sec.slice.len"} {
					if bytes.Contains(output, []byte(bad)) {
						t.Fatal("holder transfer introduced backing observation", bad)
					}
				}
				cmd := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-")
				cmd.Stdin = bytes.NewReader(output)
				if parsed, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("holder dialect: %v\n%s", err, parsed)
				}

				// Corrupt only the transfer's properties after valid emission:
				// the real borrow verifier must reject without leaving output.
				for _, key := range []string{"lifetime_id = 1 : i32", "epoch_dependency = 9 : i32"} {
					lines := strings.Split(string(output), "\n")
					changed := false
					for index, line := range lines {
						if strings.Contains(line, `"sec.slice.`+name+`"`) {
							replacement := strings.Split(key, " = ")[0] + " = 11 : i32"
							lines[index] = strings.Replace(line, key, replacement, 1)
							changed = lines[index] != line
						}
					}
					if !changed {
						t.Fatal("mutation lost target property", key)
					}
					artifact := filepath.Join(t.TempDir(), "invalid-transfer.mlir")
					reject := exec.Command(tool, "--sec-verify-borrow-semantics", "-o", artifact, "-")
					reject.Stdin = strings.NewReader(strings.Join(lines, "\n"))
					diagnosis, err := reject.CombinedOutput()
					if err == nil || !bytes.Contains(diagnosis, []byte("changes source")) {
						t.Fatalf("corrupt transfer passed: %v\n%s", err, diagnosis)
					}
					if _, err := os.Stat(artifact); !os.IsNotExist(err) {
						t.Fatal("failed transfer verification left output", err)
					}
				}
				fn.Extern, fn.Blocks = true, nil
				out, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if out != nil || err == nil || !strings.Contains(err.Error(), "foreign slice ABI") {
					t.Fatal("foreign slice ABI accepted", err)
				}
				// Distinct white-box observation evidence: source Arena/signature
				// producers remain separate, but represented guards must emit.
				fn.Extern = false
				count := types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint", TargetSize: true})
				boolean := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1})
				fn.ReturnType = count
				fn.Blocks = []*semantic.Block{
					{ID: 0, Operations: []semantic.Operation{
						{Kind: semantic.OpReferenceIsValid, Operands: []semantic.ValueID{1}, Results: []semantic.Value{{ID: 2, Type: boolean, Ownership: semantic.OwnershipImmediate}}, Reference: facts},
						{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{2}, Successors: []semantic.BranchTarget{{Block: 1}, {Block: 2}}},
					}},
					{ID: 1, Operations: []semantic.Operation{
						{Kind: semantic.OpSliceLength, Operands: []semantic.ValueID{1}, Results: []semantic.Value{{ID: 3, Type: count, Ownership: semantic.OwnershipImmediate}}, ReferenceGuard: 2},
						{Kind: semantic.OpReturn, Operands: []semantic.ValueID{3}},
					}},
					{ID: 2, Operations: []semantic.Operation{{Kind: semantic.OpReferenceGenerationFailure, PanicReasonID: 8, PanicReason: "InvalidReferenceGeneration"}}},
				}
				out, err = EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil {
					t.Fatal(err)
				}
				cmd = exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-")
				cmd.Stdin = bytes.NewReader(out)
				if parsed, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dynamic observation dialect: %v\n%s", err, parsed)
				}
			})
		}
	}
}

// The emitter must reject corrupted recursive copy metadata before returning
// any bytes, even if earlier source functions in the module are supported.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§114,140;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestPackage16EmitRejectsForgedAggregateSliceCopy(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		m := sliceEmitModule(t, "slice_aggregate_arrays.sec", width)
		element := m.Structs[0].Fields[0].Type
		exclusive, err := m.Types.InternSlice(element, true)
		if err != nil {
			t.Fatal(err)
		}
		// The source struct remains marked trivial, but now secretly owns an
		// exclusive holder; semantic validation must catch this contradiction.
		m.Structs[0].Fields[0].Type = exclusive
		output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
		if output != nil || err == nil || !strings.Contains(err.Error(), "mutable slice must be move-only") {
			t.Fatal("forged aggregate metadata leaked output", err)
		}
	}
}
