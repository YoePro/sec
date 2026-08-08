package semantic

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func analyzedModule(t *testing.T, source string, maxPackage uint8) (*Module, error) {
	t.Helper()
	p := parser.New(lexer.NewWithFile(source, "sample.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatalf("sema errors: %v", errs)
	}
	return Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}, MaxPackage: maxPackage})
}

func TestTypeTableInternsStructuralIdentity(t *testing.T) {
	table := NewTypeTable()
	base := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true})
	if got := table.Intern(Type{Kind: TypeInt, Name: "int", TargetSize: true}); got != base {
		t.Fatalf("same type got %d and %d", base, got)
	}
	a := table.Intern(Type{Kind: TypeNamed, Name: "A", Identity: "main::A", Base: base})
	b := table.Intern(Type{Kind: TypeNamed, Name: "B", Identity: "main::B", Base: base})
	if a == b || a == base || b == base {
		t.Fatal("named type identities were erased")
	}
}

func TestExactConstantsAndDeterministicFormat(t *testing.T) {
	decimal, err := parseDecimal("0.10")
	if err != nil {
		t.Fatal(err)
	}
	if decimal.Coefficient.Cmp(big.NewInt(10)) != 0 || decimal.Scale != 2 || decimal.Lexeme != "0.10" {
		t.Fatalf("decimal = %#v", decimal)
	}
	module, err := analyzedModule(t, "module main\nfn Huge() int { return 42 }\n", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first, second := Format(module), Format(module); first != second {
		t.Fatal("printer is not deterministic")
	}
}

func TestPackage2RejectsPackage3Construct(t *testing.T) {
	tests := []struct{ name, source, feature string }{
		{"mutable", "module main\nfn F() int {\n  let mut x := 1\n  return x\n}\n", "mutable local storage"},
		{"call", "module main\nfn One() int { return 1 }\nfn F() int { return One() }\n", "function call"},
		{"if", "module main\nfn F(flag: bool) int {\n  if flag { return 1 }\n  return 2\n}\n", "if control flow"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analyzedModule(t, test.source, 2)
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || unsupported.Feature != test.feature {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPackage3BuildsStorageCallsAndCFG(t *testing.T) {
	source := `module main
fn One() int { return 1 }
fn Choose(flag: bool) int {
  let mut value := One()
  if flag { value = 2 } else { value = 3 }
  return value
}`
	module, err := analyzedModule(t, source, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"storage.declare", "storage.init", "storage.store", "storage.load", "call.direct", "conditional-branch"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestVerifierRejectsMissingTerminatorAndInvalidType(t *testing.T) {
	types := NewTypeTable()
	intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
	module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{{ID: "main::F()", Name: "F", ReturnType: intID, Blocks: []*Block{{ID: 0}}, Entry: 0}}}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "terminator") {
		t.Fatalf("error = %v", err)
	}
	module.Functions[0].ReturnType = 0
	if err := Verify(module); err == nil {
		t.Fatal("invalid TypeID accepted")
	}
}

func TestVerifierStorageOrderAndSSADominance(t *testing.T) {
	t.Run("init before declare", func(t *testing.T) {
		types := NewTypeTable()
		intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
		value := Value{ID: 0, Type: intID, Ownership: OwnershipImmediate}
		fn := &Function{ID: "main::F()", Name: "F", ReturnType: intID, Entry: 0,
			Storages: []Storage{{ID: 1, Type: intID, Mutable: true, Class: StorageLocalAutomatic}},
			Blocks: []*Block{{ID: 0, Operations: []Operation{
				{Kind: OpConstInt, Results: []Value{value}, Integer: big.NewInt(1)},
				{Kind: OpStorageInit, Storage: 1, Operands: []ValueID{0}},
				{Kind: OpStorageDeclare, Storage: 1},
				{Kind: OpReturn, Operands: []ValueID{0}},
			}}},
		}
		module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "before declaration") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("non dominating value", func(t *testing.T) {
		types := NewTypeTable()
		intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
		boolID := types.Intern(Type{Kind: TypeBool, Name: "bool"})
		condition := Value{ID: 0, Type: boolID, Ownership: OwnershipImmediate}
		branchValue := Value{ID: 1, Type: intID, Ownership: OwnershipImmediate}
		fn := &Function{ID: "main::F(bool)", Name: "F", ReturnType: intID, Entry: 0, Parameters: []Parameter{{Name: "flag", Value: condition}}, Blocks: []*Block{
			{ID: 0, Operations: []Operation{{Kind: OpCondBranch, Operands: []ValueID{0}, Successors: []BranchTarget{{Block: 1}, {Block: 2}}}}},
			{ID: 1, Operations: []Operation{{Kind: OpConstInt, Results: []Value{branchValue}, Integer: big.NewInt(1)}, {Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}},
			{ID: 2, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}},
			{ID: 3, Operations: []Operation{{Kind: OpReturn, Operands: []ValueID{1}}}},
		}}
		module := &Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "does not dominate") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestVerifierAcceptsBlockParameterMerge(t *testing.T) {
	types := NewTypeTable()
	intID := types.Intern(Type{Kind: TypeInt, Name: "int"})
	value := Value{ID: 0, Type: intID, Ownership: OwnershipImmediate}
	merged := Value{ID: 1, Type: intID, Ownership: OwnershipImmediate}
	fn := &Function{ID: "main::F()", Name: "F", ReturnType: intID, Entry: 0, Blocks: []*Block{
		{ID: 0, Operations: []Operation{{Kind: OpConstInt, Results: []Value{value}, Integer: big.NewInt(7)}, {Kind: OpBranch, Successors: []BranchTarget{{Block: 1, Arguments: []ValueID{0}}}}}},
		{ID: 1, Parameters: []Value{merged}, Operations: []Operation{{Kind: OpReturn, Operands: []ValueID{1}}}},
	}}
	if err := Verify(&Module{Version: Version, Identity: "main", Types: types, Functions: []*Function{fn}}); err != nil {
		t.Fatal(err)
	}
}
