package semantic

import (
	"strings"
	"testing"
)

// allocationContextFixture selects a resolved origin and forwards the capability
// to a synchronous internal callee. Source argument lists remain independent of
// the marked hidden input; this fixture does not implement Sema summary plans.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§56–60,98–99.
func allocationContextFixture(origin string) (*Module, *Operation) {
	m, _ := arenaOperationFixture(OpArenaCreateBorrowed, 64)
	fn := m.Functions[0]
	contextType := m.Types.InternAllocationContext()
	facts := &AllocationContextFacts{Value: 1000, Origin: origin}
	if origin == ContextPropagated {
		fn.RequiresAllocationContext = true
		fn.Parameters = append([]Parameter{{Name: "$context", Value: Value{ID: 1000, Type: contextType, Ownership: OwnershipCompilerTemporary}, HiddenAllocationContext: true}}, fn.Parameters...)
	} else {
		kind := OpAllocationContextTarget
		facts.OriginPlan = "resolved-origin-proof"
		operands := []ValueID(nil)
		if origin == ContextExplicitArena {
			kind = OpAllocationContextFromArena
			facts.Domain = 9
			operands = []ValueID{4}
		}
		if origin == ContextCompilerLocal {
			kind = OpAllocationContextCompilerLocal
		}
		producer := Operation{Kind: kind, Results: []Value{{ID: 1000, Type: contextType, Ownership: OwnershipCompilerTemporary}}, Operands: operands, AllocationContext: facts}
		entry := fn.Blocks[0]
		entry.Operations = append(entry.Operations[:3], producer, Operation{Kind: OpReturn})
	}
	callee := &Function{ID: "main::NeedsContext", Name: "NeedsContext", RequiresAllocationContext: true, ReturnType: fn.ReturnType, Parameters: []Parameter{{Name: "$context", Value: Value{ID: 0, Type: contextType, Ownership: OwnershipCompilerTemporary}, HiddenAllocationContext: true}}, Blocks: []*Block{{ID: 0, Operations: []Operation{{Kind: OpReturn}}}}}
	m.Functions = append(m.Functions, callee)
	selection := *facts
	call := Operation{Kind: OpDirectCall, Callee: callee.ID, Operands: []ValueID{1000}, ArgumentActions: []ArgumentAction{ArgumentAllocationContext}, AllocationContext: &selection}
	block := fn.Blocks[0]
	last := len(block.Operations) - 1
	block.Operations = append(block.Operations[:last], call, Operation{Kind: OpReturn})
	return m, &block.Operations[last]
}

// All canonical origins and internal propagation verify and print deterministically.
// Hidden arguments neither acquire a foreign ABI nor become source parameters.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§56–60,98–99.
func TestAllocationContextIR(t *testing.T) {
	for _, origin := range []string{ContextPropagated, ContextExplicitArena, ContextTarget, ContextCompilerLocal} {
		t.Run(origin, func(t *testing.T) {
			m, _ := allocationContextFixture(origin)
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			text := Format(m)
			for _, want := range []string{"hidden-allocation-context", "requires-allocation-context", "context=%1000", "origin=" + origin} {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q in %s", want, text)
				}
			}
			if text != Format(m) {
				t.Fatal("context printing is not deterministic")
			}
		})
	}
}

// Context requirement, SSA selection and ABI boundaries are checked, rather
// than silently inferring an allocator or copying a capability as ordinary data.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§54–60,98–99,109.
func TestAllocationContextContracts(t *testing.T) {
	tests := []struct {
		name, origin string
		change       func(*Module, *Operation)
	}{
		{"context used before selection", ContextTarget, func(m *Module, o *Operation) {
			ops := m.Functions[0].Blocks[0].Operations
			ops[3], ops[4] = ops[4], ops[3]
		}},
		{"context escapes through branch", ContextPropagated, func(m *Module, o *Operation) {
			fn := m.Functions[0]
			ctx := fn.Parameters[0].Value
			ctx.ID = 2000
			block := fn.Blocks[0]
			block.Operations[len(block.Operations)-1] = Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{1000}}}}
			fn.Blocks = append(fn.Blocks, &Block{ID: 1, Parameters: []Value{ctx}, Operations: []Operation{{Kind: OpReturn}}})
		}},

		{"missing requirement", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].RequiresAllocationContext = false }},
		{"missing input", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].Parameters = m.Functions[0].Parameters[1:] }},
		{"unmarked input", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].Parameters[0].HiddenAllocationContext = false }},
		{"hidden source scalar", ContextPropagated, func(m *Module, o *Operation) {
			m.Functions[0].Parameters[0].Value.Type = m.Types.Intern(Type{Kind: TypeUint, TargetSize: true})
		}},
		{"mispositioned hidden input", ContextPropagated, func(m *Module, o *Operation) { p := m.Functions[0].Parameters; p[0], p[1] = p[1], p[0] }},
		{"owned capability input", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].Parameters[0].Value.Ownership = OwnershipOwned }},
		{"foreign ABI input", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].ABI = "C" }},
		{"foreign declaration input", ContextPropagated, func(m *Module, o *Operation) { m.Functions[0].Extern = true; m.Functions[0].Blocks = nil }},
		{"missing call selection", ContextPropagated, func(m *Module, o *Operation) { o.AllocationContext = nil }},
		{"missing call input", ContextPropagated, func(m *Module, o *Operation) { o.Operands = nil; o.ArgumentActions = nil }},
		{"ordinary argument action", ContextPropagated, func(m *Module, o *Operation) { o.ArgumentActions[0] = ArgumentCopyTrivial }},
		{"invented context origin", ContextPropagated, func(m *Module, o *Operation) { o.AllocationContext.Origin = ContextTarget }},
		{"undefined selected capability", ContextPropagated, func(m *Module, o *Operation) { o.AllocationContext.Value = 999 }},
		{"unsolicited context", ContextPropagated, func(m *Module, o *Operation) {
			m.Functions[1].RequiresAllocationContext = false
			m.Functions[1].Parameters = nil
			o.Operands = nil
			o.ArgumentActions = nil
		}},
		{"producer lacks origin plan", ContextTarget, func(m *Module, o *Operation) {
			m.Functions[0].Blocks[0].Operations[3].AllocationContext.OriginPlan = ""
		}},
		{"unknown producer origin", ContextTarget, func(m *Module, o *Operation) {
			m.Functions[0].Blocks[0].Operations[3].AllocationContext.Origin = "guess"
		}},
		{"call changes origin plan", ContextTarget, func(m *Module, o *Operation) { o.AllocationContext.OriginPlan = "other" }},
		{"producer manufactures owned data", ContextTarget, func(m *Module, o *Operation) {
			m.Functions[0].Blocks[0].Operations[3].Results[0].Ownership = OwnershipOwned
		}},
		{"target discovers lexical Arena", ContextTarget, func(m *Module, o *Operation) { m.Functions[0].Blocks[0].Operations[3].Operands = []ValueID{4} }},
		{"explicit Arena lacks domain", ContextExplicitArena, func(m *Module, o *Operation) { m.Functions[0].Blocks[0].Operations[3].AllocationContext.Domain = 0 }},
		{"explicit Arena is read-only", ContextExplicitArena, func(m *Module, o *Operation) {
			id := m.Functions[0].Blocks[0].Operations[0].Results[0].Type
			m.Types.types[id-1].Mutable = false
		}},
		{"context stored as ordinary SSA data", ContextTarget, func(m *Module, o *Operation) {
			block := m.Functions[0].Blocks[0]
			own := Operation{Kind: OpOwnMove, Operands: []ValueID{1000}, Results: []Value{{ID: 1001, Type: m.Types.InternAllocationContext(), Ownership: OwnershipCompilerTemporary}}, Ownership: &OwnershipFacts{}}
			block.Operations = append(block.Operations[:4], own, Operation{Kind: OpReturn})
		}},
		{"context passed over foreign call", ContextPropagated, func(m *Module, o *Operation) {
			callee := m.Functions[1]
			callee.Extern = true
			callee.Blocks = nil
			o.Kind = OpForeignCall
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, call := allocationContextFixture(test.origin)
			test.change(m, call)
			if err := Verify(m); err == nil {
				t.Fatal("invalid allocation context accepted")
			}
		})
	}
}

// Implicit owned backing creation carries its selected SSA context explicitly.
// An opaque numeric backing Context ID alone is not a forwarding capability.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§90–91,99.
func TestArenaCreationRequiresSelectedContext(t *testing.T) {
	m, create := arenaOperationFixture(OpArenaCreateOwnedFixed, 64)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	create.AllocationContext = nil
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "selected allocation context") {
		t.Fatalf("implicit context guessed: %v", err)
	}
}

// A callee with no ambient requirement receives no mandatory hidden argument,
// even when the caller already has an ambient context available.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§56,58,98.
func TestAllocationContextNotAddedToOrdinaryCall(t *testing.T) {
	m, call := allocationContextFixture(ContextPropagated)
	m.Functions[1].RequiresAllocationContext = false
	m.Functions[1].Parameters = nil
	call.Operands = nil
	call.ArgumentActions = nil
	call.AllocationContext = nil
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}
