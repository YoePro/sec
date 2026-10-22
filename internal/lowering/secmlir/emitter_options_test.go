package secmlir

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// Explicit schema 12 uses the established emitter and target plan; zero and
// the public Emit entry retain schema 11 until P16 source production is ready.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1.
func TestEmitSchemaSelectionPreservesDefaultAndTargetFacts(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		m := representativeModule(t)
		ordinary, err := Emit(m, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		for _, version := range []uint32{0, 11, 12} {
			first, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			second, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: version})
			if err != nil || !bytes.Equal(first, second) {
				t.Fatal("schema selection is not deterministic")
			}
			expected := ordinary
			if version == 12 {
				expected = bytes.Replace(ordinary, []byte("sec.dialect_version = 11 : i32"), []byte("sec.dialect_version = 12 : i32"), 1)
			}
			if !bytes.Equal(first, expected) {
				t.Fatalf("schema %d changed semantic or target facts", version)
			}
		}
	}
}

// Input compatibility is wider than emission: old schemas remain readable,
// but an explicit unsupported output version must never produce bytes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1.
func TestEmitSchemaSelectionRejectsUnknownAndDowngradeVersions(t *testing.T) {
	for _, version := range []uint32{1, 10, 14, ^uint32(0)} {
		output, err := EmitWithOptions(boolModule(t), testPlan(64), EmitOptions{DialectVersion: version})
		if len(output) != 0 || err == nil || !strings.Contains(err.Error(), "unsupported Sec MLIR emission schema version") {
			t.Fatalf("version %d: output=%q err=%v", version, output, err)
		}
	}
}

// Schema 11 retains its slice gate; explicit schema 12 admits abstract slices
// while preserving IR verification and leaving physical descriptors/ABI separate.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§87,121–122;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestEmitSchemaSelectionKeepsUnsupportedSliceBoundary(t *testing.T) {
	for _, version := range []uint32{11, 12} {
		m := boolModule(t)
		element := m.Types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int32", Signed: true, BitWidth: 32})
		view, err := m.Types.InternSlice(element, false)
		if err != nil {
			t.Fatal(err)
		}
		m.Functions[0].Parameters = append(m.Functions[0].Parameters, semantic.Parameter{Name: "view", Value: semantic.Value{ID: 99, Type: view, Ownership: semantic.OwnershipSharedReference}, Reference: &semantic.ReferenceMetadata{Borrow: 1, Lifetime: 1, Validity: semantic.ReferenceProven, ProofProvenance: "parameter", AddressSpace: semantic.AddressSpaceOrdinary, AddressStability: "stable", Provenance: "parameter", OriginStorages: []uint32{1}}})
		if err := semantic.Verify(m); err != nil {
			t.Fatal(err)
		}
		output, err := EmitWithOptions(m, testPlan(64), EmitOptions{DialectVersion: version})
		if version == 12 {
			if err != nil || !strings.Contains(string(output), "!sec.slice<si32>") {
				t.Fatalf("schema 12 lost abstract slice type: %q %v", output, err)
			}
			continue
		}
		var unsupported *UnsupportedLoweringError
		if len(output) != 0 || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "slice") {
			t.Fatalf("schema %d bypassed slice gate: %q %v", version, output, err)
		}
	}
	m := boolModule(t)
	m.Functions[0].Entry = 999
	output, err := EmitWithOptions(m, testPlan(64), EmitOptions{DialectVersion: 12})
	if len(output) != 0 || err == nil || !strings.Contains(err.Error(), "verify Semantic IR") {
		t.Fatalf("schema 12 bypassed verification: %q %v", output, err)
	}
}

// The rebuilt real dialect accepts 12 and all predecessor schema headers,
// round-trips explicit emitted output, and rejects unknown 13 (not a fallback).
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — §1.
func TestEmitSchema12RoundTripWithRealTool(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		output, err := EmitWithOptions(boolModule(t), testPlan(width), EmitOptions{DialectVersion: 12})
		if err != nil {
			t.Fatal(err)
		}
		for version := 1; version <= 13; version++ {
			input := strings.Replace(string(output), "sec.dialect_version = 12 : i32", fmt.Sprintf("sec.dialect_version = %d : i32", version), 1)
			command := exec.Command(tool, "-")
			command.Stdin = strings.NewReader(input)
			parsed, err := command.CombinedOutput()
			if version == 13 {
				if err == nil || !bytes.Contains(parsed, []byte("unsupported Sec dialect schema version")) {
					t.Fatalf("unknown schema accepted: %v %s", err, parsed)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%d-bit schema %d rejected: %v %s", width, version, err, parsed)
			}
			if version == 12 {
				again := exec.Command(tool, "-")
				again.Stdin = bytes.NewReader(parsed)
				canonical, err := again.CombinedOutput()
				if err != nil || !bytes.Equal(parsed, canonical) {
					t.Fatalf("schema-12 round trip: %v %s", err, canonical)
				}
			}
		}
	}
}
