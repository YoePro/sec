package semantic

import (
	"strings"
	"testing"
)

func TestPackage12SynthesizedUnreachable(t *testing.T) {
	module := package12UnreachableModule(Operation{
		Kind: OpUnreachable, Synthesized: true, Reason: "exhaustive-match-fallthrough",
	})
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if !strings.Contains(text, `unreachable synthesized=true reason="exhaustive-match-fallthrough"`) {
		t.Fatalf("missing unreachable provenance:\n%s", text)
	}

	for _, operation := range []Operation{
		{Kind: OpUnreachable, Reason: "exhaustive-match-fallthrough"},
		{Kind: OpUnreachable, Synthesized: true},
	} {
		if err := Verify(package12UnreachableModule(operation)); err == nil || !strings.Contains(err.Error(), "invalid synthesized unreachable") {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestPackage12BuildsSourceOrderEnumMatchCFG(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum Flag: bit[1] { Off = 0, Disabled = 0, On = 1 }
fn Choose(flag: Flag, enabled: bool) int {
  return match flag {
    Flag.Off where enabled => 10
    Flag.Disabled => 20
    Flag.On => 30
  }
}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if len(module.Functions) != 1 || len(module.Functions[0].Matches) != 1 {
		t.Fatalf("match records = %#v", module.Functions)
	}
	match := module.Functions[0].Matches[0]
	if match.ID == 0 || !match.Exhaustive || !match.ValueContext || len(match.Arms) != 3 {
		t.Fatalf("match = %#v", match)
	}
	if match.Arms[0].EnumValue.String() != "0" || !match.Arms[0].Guarded || match.Arms[1].EnumValue.String() != "0" || match.Arms[2].EnumValue.String() != "1" {
		t.Fatalf("arms = %#v", match.Arms)
	}

	compares, residuals := 0, 0
	for _, block := range module.Functions[0].Blocks {
		for _, operation := range block.Operations {
			switch operation.Kind {
			case OpEnumCompare:
				compares++
				if len(operation.Operands) != 2 || operation.Operands[0] != match.Subject {
					t.Fatalf("enum comparison does not reuse subject: %#v", operation)
				}
			case OpUnreachable:
				if operation.Reason == "exhaustive-match-fallthrough" && operation.MatchID == match.ID {
					residuals++
				}
			}
		}
	}
	if compares != 3 || residuals != 1 {
		t.Fatalf("compares=%d residuals=%d\n%s", compares, residuals, Format(module))
	}
}

func TestPackage12BuildsUnionAndOptionMatchExpressions(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Number union { Integer(int) Missing }
fn Read(number: Number) int {
  return match number {
    Integer(value) => value
    Missing => 0
  }
}
fn ReadOptional(number: Option[int]) int {
  return match number {
    Some(value) where value > 0 => value
    Some(value) => value
    None => 0
  }
}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := package12OperationCounts(module)
	if counts[OpUnionIsVariant] != 5 || counts[OpUnionUnwrapPayload] != 3 {
		t.Fatalf("union operations = %#v\n%s", counts, Format(module))
	}
	for _, function := range module.Functions {
		if len(function.Matches) != 1 || !function.Matches[0].ValueContext {
			t.Fatalf("%s matches = %#v", function.Name, function.Matches)
		}
	}
}

func TestPackage12BuildsResultMatchWithBindingDiscardAndGuard(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum Failure error { Missing, Other, }
fn Read(result: Result[int, Failure], enabled: bool) int {
  return match result {
    Ok(value) where enabled => value
    Ok(value) => value
    Err(error) => 0
  }
}
fn Ignore(result: Result[int, Failure]) int {
  return match result {
    Ok(_) => 1
    Err(error) => 0
  }
}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := package12OperationCounts(module)
	if counts[OpResultIsErr] != 5 || counts[OpResultUnwrapOk] != 2 || counts[OpResultUnwrapErr] != 2 {
		t.Fatalf("Result operations = %#v\n%s", counts, Format(module))
	}
}

func TestPackage12BuildsContinuingAndTerminatingMatchStatements(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum Direction { North, South, }
fn Continuing(direction: Direction) int {
  let mut result: int := 0
  match direction {
    Direction.North => { result = 1 }
    Direction.South => { result = 2 }
  }
  return result
}
fn Terminating(direction: Direction) int {
  match direction {
    Direction.North => { return 1 }
    Direction.South => { return 2 }
  }
}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, function := range module.Functions {
		if len(function.Matches) != 1 || function.Matches[0].ValueContext {
			t.Fatalf("%s matches = %#v", function.Name, function.Matches)
		}
		if function.Name == "Continuing" && function.Matches[0].MergeBlock == 0 {
			t.Fatal("continuing match has no continuation")
		}
		if function.Name == "Terminating" && function.Matches[0].MergeBlock != 0 {
			t.Fatal("all-terminating match unexpectedly has a continuation")
		}
	}
}

func package12OperationCounts(module *Module) map[OpKind]int {
	counts := map[OpKind]int{}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
	}
	return counts
}

func package12UnreachableModule(operation Operation) *Module {
	types := NewTypeTable()
	voidType := types.Intern(Type{Kind: TypeVoid, Name: "void"})
	return &Module{
		Version: Version, Identity: "main", Types: types,
		Functions: []*Function{{
			ID: "main::Impossible()", Name: "Impossible", ReturnType: voidType, Entry: 1,
			Blocks: []*Block{{ID: 1, Operations: []Operation{operation}}},
		}},
	}
}
