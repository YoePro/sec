package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/sema"
)

// P16-48 (package16 §§ 116, 140): element references through slice
// parameters are returned with the conservative element origin
// `param#N[*]` (a slice element is bounds-checked even for a constant
// index, § 67), a field of a slice element extends it with `.field#K`, and
// a borrow of a slice element or its field is passed to a ref parameter.
func TestPackage16SliceElementReferenceOrigins(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_element_origins.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		m, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		for name, want := range map[string]string{
			"First(ref int[])":        "returns-from=[param#0[*]]",
			"Slot(ref mut int[],int)": "returns-from=[param#0[*]]",
			"RightOf(ref Pair[],int)": "returns-from=[param#0[*].field#1]",
		} {
			start := strings.Index(text, `func "main::`+name)
			if start < 0 {
				t.Fatalf("missing %s in:\n%s", name, text)
			}
			header := text[start:]
			header = header[:strings.Index(header, "\n")]
			if !strings.Contains(header, want) {
				t.Fatalf("%s: %s, want %s", name, header, want)
			}
		}
		right := text[strings.Index(text, `func "main::RightOf(`):]
		right = right[:strings.Index(right, "\n  }")]
		for _, operation := range []string{"slice.index-in-bounds", "slice.element-place", "place.field", "ref.borrow-shared"} {
			if !strings.Contains(right, operation) {
				t.Fatalf("RightOf misses %s:\n%s", operation, right)
			}
		}
	}
}

// P16-48 (package16 §§ 45, 116, 136): when a slice parameter's facts are
// dynamic-epoch, every implicit full-range reborrow that passes it on is
// validated first; the length and normalized range observe only the guarded
// holder. Epoch-injected white-box evidence: source Arena slices are P18.
func TestPackage16SliceArgumentDynamicGuardOrder(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_parameters.sec")
	m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
	if err != nil {
		t.Fatal(err)
	}
	var decl *ast.FunctionDeclaration
	for _, statement := range program.Statements {
		if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Forward" {
			decl = fn
		}
	}
	resolved, _ := a.ResolvedFunctionForDeclaration(decl)
	b := &builder{module: m, analyzer: a, maxPackage: 16}
	if err := b.prepareSliceLowering(program); err != nil {
		t.Fatal(err)
	}
	facts, ok := a.ReferenceParameterFactsOf(resolved.Parameters[0])
	if !ok {
		t.Fatal("missing parameter facts")
	}
	facts.Validity, facts.ProofProvenance = sema.ReferenceValidityDynamicEpoch, ""
	facts.InvalidationDomain, facts.EpochDependency = 4, 7
	var forward *Function
	for _, fn := range m.Functions {
		if fn.Name == "Forward" {
			forward = fn
		}
	}
	fn := &Function{ID: forward.ID, Name: forward.Name, ReturnType: forward.ReturnType, Location: forward.Location}
	fb := &functionBuilder{owner: b, fn: fn, bindings: map[sema.BindingID]binding{}, nextStorage: 1, nextMatch: 1, nextLoop: 1}
	metadata := referenceMetadataOf(facts)
	value := fb.newValue(forward.Parameters[0].Value.Type, OwnershipMutableReference, forward.Parameters[0].Location)
	fn.Parameters = []Parameter{{Name: "values", Value: value, Location: forward.Parameters[0].Location, Reference: metadata}}
	fb.recordReference(value.ID, metadata)
	bindingFact, _ := a.ResolvedBindingOf(decl.Parameters[0].Name)
	fb.bindings[bindingFact.ID] = binding{value: value.ID, typ: value.Type}
	entry := fb.newBlock()
	fn.Entry, fb.current = entry.ID, entry
	if err := fb.buildStatements(decl.Body.Statements); err != nil {
		t.Fatal(err)
	}
	fb.emit(Operation{Kind: OpReturn, Location: fn.Location})
	for index := range m.Functions {
		if m.Functions[index] == forward {
			m.Functions[index] = fn
		}
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	reborrows := 0
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpSliceReborrowMutable {
				continue
			}
			reborrows++
			if op.ReferenceGuard == 0 || index < 2 || block.Operations[index-2].Kind != OpSliceLength || block.Operations[index-2].ReferenceGuard != op.ReferenceGuard {
				t.Fatalf("reborrow %d is not preceded by its guarded length observation:\n%s", reborrows, Format(m))
			}
		}
	}
	if reborrows != 2 {
		t.Fatalf("expected 2 guarded reborrows, got %d:\n%s", reborrows, Format(m))
	}
	for _, block := range fn.Blocks {
		for index := range block.Operations {
			if block.Operations[index].Kind == OpSliceLength {
				block.Operations[index].ReferenceGuard = 0
				if err := Verify(m); err == nil {
					t.Fatal("unguarded dynamic slice argument observation accepted")
				}
				return
			}
		}
	}
}

// P16-48 (package16 §§ 117, 140): union payloads and struct fields hold
// slice values. Construction copies the shared holder into the aggregate
// (copy-trivial: a shared slice is CopyTrivial), and the aggregate is
// returned and copied as an ordinary value. Reading the contained slice back
// needs P15 § 101 contained-reference facts, which have no Semantic IR
// representation yet, so it fails closed without a module.
func TestPackage16SliceValuesInAggregates(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_aggregates.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		m, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		text := Format(m)
		for name, want := range map[string]string{
			"Pick(ref int[],bool)": "union.construct variant=#0",
			"Frame(ref int[],int)": "struct.construct",
		} {
			body := text[strings.Index(text, `func "main::`+name):]
			body = body[:strings.Index(body, "\n  }")]
			if !strings.Contains(body, want) || strings.Contains(body, "slice.move-mut") {
				t.Fatalf("%s:\n%s", name, body)
			}
		}
	}
	module, err := buildPackage16Source(t, `
module main

type Views union {
    Values(ref int[])
    Missing
}

fn First(views: Views) int {
    return match views {
        Values(found) => found[0]
        Missing => 0
    }
}
`, 64)
	if module != nil || err == nil || !strings.Contains(err.Error(), "found without lowered reference facts") {
		t.Fatalf("reading a contained slice must fail closed: %v", err)
	}
}
