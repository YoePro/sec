package semantic

import (
	"os"
	"strings"
	"testing"
)

func packForwardFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/generics/type_pack_declarations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A forwarding call demands the callee of the concrete expansion, and each
// operand keeps its pack position and the spread's consuming mode.
// Rules: rules/compiler/semantic_ir.md §27A(1),(7); rules/declarations/functions.md §38A(7).
func TestPackForwardFacts(t *testing.T) {
	module, err := buildPackage15Source(t, packForwardFixture(t, "spread_forward_owned.sec"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`call.direct "main::Count[...Types<int,bool>](int,bool)"`,
		"pack-spread(Types positions=[0, 1])",
		"pack-consuming-spread(Types positions=[0, 1, 2])",
		"pack-spread(Types positions=[-1, 0])",
		"pack-spread(Types positions=[0, 1, 0, 1])",
		"pack-spread(Types positions=[-1, 0, 1])",
		`"main::Count[...Types<>]()"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

// Each position is passed with its own loan: an owned element is lent through a
// read-only root, a shared holder copied and a mutable holder reborrowed.
func TestPackForwardReferences(t *testing.T) {
	module, err := buildPackage15Source(t, packForwardFixture(t, "spread_forward_references.sec"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"place.value-root", "ref.borrow-shared", "ref.copy-shared", "ref.reborrow-mut", "ref.reborrow-shared", `"main::Write[...Types<int,bool>](ref mut int,ref mut bool)"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

// The verifier fails closed on forwarding facts that disagree with the pack.
func TestPackForwardFactsVerification(t *testing.T) {
	build := func(t *testing.T) (*Module, *Operation) {
		t.Helper()
		module, err := buildPackage15Source(t, packForwardFixture(t, "spread_forward_owned.sec"), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for index := range block.Operations {
					op := &block.Operations[index]
					if op.PackForward != nil && op.PackForward.Consuming {
						return module, op
					}
				}
			}
		}
		t.Fatal("no consuming forward")
		return nil, nil
	}
	for name, tamper := range map[string]func(*Operation){
		"missing entry": func(op *Operation) { op.PackForward.Positions = op.PackForward.Positions[1:] },
		"out of range":  func(op *Operation) { op.PackForward.Positions[0] = 99 },
		"other pack":    func(op *Operation) { op.PackForward.Parameter = "Other" },
		"not the parameter": func(op *Operation) {
			op.Operands[0], op.Operands[1] = op.Operands[1], op.Operands[0]
		},
	} {
		module, op := build(t)
		tamper(op)
		if err := Verify(module); err == nil {
			t.Errorf("%s: verifier accepted tampered facts", name)
		}
	}
}

// Ordinary array and homogeneous spreads have no IR form yet and report a clean
// boundary, never an internal missing-type error.
func TestSpreadCallArgumentBoundaries(t *testing.T) {
	for _, source := range []string{
		"module main\nfn Two(a: int, b: int) int { return a }\nfn A(values: int[2]) int { return Two(values...) }\n",
		"module main\nfn Sum(values: ...int) int { return 0 }\nfn A(values: ...int) int { return Sum(values...) }\n",
		"module main\nfn Sum(values: ...int) int { return 0 }\nfn A(values: ...int) int { return Sum(<-values...) }\n",
		"module main\nfn Sum(values: ...int) int { return 0 }\nfn A[...T](values: ...T) int { return Sum(values...) }\nfn M() int { return A(1, 2) }\n",
	} {
		_, err := buildPackage15Source(t, source, "")
		if err == nil || !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("%s: %v", source, err)
		}
	}
}

// A reference element of an owned pack stops at a named boundary, as T = ref int
// does for one generic parameter; no pack position is mistaken for ...ref Types.
func TestOwnedReferenceHolderBoundary(t *testing.T) {
	_, err := buildPackage15Source(t, packForwardFixture(t, "owned_reference_holders.sec"), "")
	if err == nil || !strings.Contains(err.Error(), "reference-holder element") {
		t.Fatal(err)
	}
}

// A pack method of a generic owner specializes per owner, sequence and own
// parameter, with its receiver and result in concrete owner types.
func TestPackMethodOnGenericOwnerSpecializes(t *testing.T) {
	module, err := buildPackage15Source(t, packForwardFixture(t, "method_generic_owner.sec"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"V=int", "V=bool", "...Types<int,bool>", "H=bool", "H=int"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

// A forward selects a method or an overload per specialization; the receiver is
// operand 0 with no pack position.
func TestPackForwardMethodsAndOverloads(t *testing.T) {
	module, err := buildPackage15Source(t, packForwardFixture(t, "spread_forward_targets.sec"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"pack-spread(Types positions=[-1, 0, 1])",
		"pack-spread(Types positions=[0, -1])",
		`"main::Pick(int)"`, `"main::Pick(bool)"`, `"main::Pick(int,int)"`,
		`main::Box.Size[V=int,...Types<bool>]`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

// A pack function's returned reference lists every expanded position as a possible
// origin, and the returned value traces through the callee's summary to them.
func TestPackForwardReturnedReferenceOrigins(t *testing.T) {
	module, err := buildPackage15Source(t, packForwardFixture(t, "spread_forward_returns.sec"), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"returns-from=[param#0, param#1]", `"main::Shared[...Types<int,bool>](ref int,ref bool)"`, `"main::Exclusive[...Types<int,bool>](ref mut int,ref mut bool)"`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	for _, fn := range module.Functions {
		if strings.HasPrefix(string(fn.ID), "main::Shared[") {
			fn.ReturnOrigins = fn.ReturnOrigins[1:]
			if err := Verify(module); err == nil {
				t.Fatal("verifier accepted a returned origin missing from the summary")
			}
		}
	}
}

// ToString() in a specialization stops at a named boundary until P23 (MD-122).
func TestGenericToStringBoundary(t *testing.T) {
	for _, source := range []string{
		"module main\nfn A[T](value: T) Result[string, StringError] { return value.ToString() }\nfn M() Result[string, StringError] { return A(1) }\n",
		"module main\nfn A[...T](values: ...T) int {\n for ref v in values {\n  let t := try v.ToString() { Err(_) => \"e\" }\n  discard t\n }\n return 0\n}\nfn M() int { return A(1, true) }\n",
	} {
		_, err := buildPackage15Source(t, source, "")
		if err == nil || !strings.Contains(err.Error(), "ToString() needs P23") {
			t.Errorf("%s: %v", source, err)
		}
	}
}
