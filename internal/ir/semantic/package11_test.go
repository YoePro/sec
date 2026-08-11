package semantic

import (
	"errors"
	"strings"
	"testing"
)

func TestPackage11BuildsEnumAndUnionValues(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum Status int { unknown = 0, active = 10, aliasActive = 10, }
enum Wide uint256 { huge = 340282366920938463463374607431768211456, }
type State union { idle count(int128) point { x: int, y: int } }
type Maybe[T] union { Some(T) None }
fn Identity(value: Status) Status { return value }
fn EnumValue(value: int) bool {
  let restored := Status(value)
  let numeric := int(restored)
  return Identity(Status.aliasActive) == Status.active
}
fn Different(value: Status) bool { return value != Status.unknown }
fn WideValue() uint256 { return uint256(Wide.huge) }
fn Empty() State { return State.idle }
fn Single(value: int128) State { return State.count(value) }
fn Fields(x: int, y: int) State { return State.point { y: y, x: x } }
fn First() int { return 1 }
fn Second() int { return 2 }
fn OrderedFields() State { return State.point { y: Second(), x: First() } }
fn Some(value: int) Maybe[int] { return Maybe.Some(value) }
fn None() Maybe[int] { return Maybe.None }
`, 11)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := map[OpKind]int{}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
	}
	for kind, minimum := range map[OpKind]int{
		OpEnumConstant: 4, OpEnumFromInteger: 1, OpEnumToInteger: 2,
		OpEnumCompare: 2, OpUnionConstruct: 5,
	} {
		if counts[kind] < minimum {
			t.Errorf("%s count = %d, want at least %d\n%s", kind, counts[kind], minimum, Format(module))
		}
	}
	if len(module.Enums) != 2 || len(module.Unions) != 2 {
		t.Fatalf("definitions: enums=%d unions=%d\n%s", len(module.Enums), len(module.Unions), Format(module))
	}
	var aliasPreserved, widePreserved, genericConcrete bool
	for _, definition := range module.Enums {
		if len(definition.Cases) == 3 && definition.Cases[1].Value.Cmp(definition.Cases[2].Value) == 0 && definition.Cases[1].ID != definition.Cases[2].ID {
			aliasPreserved = true
		}
		if len(definition.Cases) == 1 && definition.Cases[0].Value.BitLen() > 128 {
			widePreserved = true
		}
	}
	for _, definition := range module.Unions {
		typ, _ := module.Types.Lookup(definition.TypeID)
		if strings.Contains(typ.Identity, "Maybe<int>") && len(definition.TypeArguments) == 1 {
			genericConcrete = true
		}
	}
	if !aliasPreserved || !widePreserved || !genericConcrete {
		t.Fatalf("lost package-11 identity: alias=%t wide=%t generic=%t\n%s", aliasPreserved, widePreserved, genericConcrete, Format(module))
	}
	for _, function := range module.Functions {
		if function.Name != "OrderedFields" {
			continue
		}
		var calls []FunctionID
		var construction Operation
		for _, operation := range function.Blocks[0].Operations {
			if operation.Kind == OpDirectCall {
				calls = append(calls, operation.Callee)
			}
			if operation.Kind == OpUnionConstruct {
				construction = operation
			}
		}
		if len(calls) != 2 || !strings.Contains(string(calls[0]), "Second") || !strings.Contains(string(calls[1]), "First") {
			t.Fatalf("source field evaluation order = %v", calls)
		}
		if len(construction.UnionFields) != 2 || construction.UnionFields[0] != "x" || construction.UnionFields[1] != "y" ||
			construction.Operands[0] != function.Blocks[0].Operations[1].Results[0].ID || construction.Operands[1] != function.Blocks[0].Operations[0].Results[0].ID {
			t.Fatalf("canonical field assembly lost source-evaluated values: %+v", construction)
		}
	}
}

func TestPackage11RejectsNonTrivialUnionPayloadTransfer(t *testing.T) {
	_, err := analyzedModule(t, `module main
type Holder struct { text: string }
type Box union { Item(Holder) }
fn Build() Box { return Box.Item(Holder { text: "value" }) }
`, 11)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v", err)
	}
}

func TestPackage11UsesOrdinaryEnumForUserErrorHandler(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum Failure { failed }
fn Source(value: int) Result[int, Failure] { return Ok(value) }
fn Handle(value: int) int {
  return try Source(value) {
    Err(Failure.failed) => 0
    Err(error) => 1
  }
}
`, 11)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if !strings.Contains(text, string(OpEnumConstant)) || !strings.Contains(text, string(OpEnumCompare)) || strings.Contains(text, string(OpCoreErrorIsVariant)) {
		t.Fatalf("user error handler did not use ordinary enum operations:\n%s", text)
	}
}
