package semantic

import (
	"errors"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// P17-35: union constructions, try-handler payload bindings and Option try
// lower the transfer Sema resolved for them. The builder reads the published
// plans instead of classifying copy versus move again; a fact it has no
// lowering for, or a missing one, is a structured unsupported boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 17–18, 65–69.
func TestBuilderConsumesResolvedTransferPlans(t *testing.T) {
	const source = `module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

type Slot union {
    Full(Handle)
    Pair {
        count: int,
        owner: Handle,
    }
    Empty
}

type LoadError union error {
    Busy(Handle)
    Stop
}

fn Make() Handle {
    return Handle { id: 1 }
}

fn Load(flag: bool) Result[int, LoadError] {
    if flag {
        return Err(LoadError.Stop)
    }
    return Ok(1)
}

fn Consume(handle: Handle) int {
    return handle.id
}

fn Wrap(count: int) Option[int] {
    return Some(count)
}

fn Hold(handle: Handle) Slot {
    return Slot.Full(<-handle)
}

fn Fields(count: int) Slot {
    return Slot.Pair { count: count, owner: Make() }
}

fn Handled(flag: bool) int {
    let value := try Load(flag) {
        Err(.Busy(h)) => Consume(<-h)
        Err(_) => 0
    }
    return value
}

fn Unwrap(maybe: Option[Handle]) Option[Handle] {
    let handle := try maybe
    return Some(<-handle)
}
`
	parsed := parser.New(lexer.NewWithFile(source, "plans.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	body := func(name string) []ast.Statement {
		for _, statement := range parsed.Program.Statements {
			if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name.Value == name {
				return function.Body.Statements
			}
		}
		t.Fatalf("missing function %s", name)
		return nil
	}
	returned := func(name string) ast.Expression {
		statements := body(name)
		return statements[len(statements)-1].(*ast.ReturnStatement).Value
	}
	for name, want := range map[string]string{
		"Wrap":   "copy-trivial",
		"Hold":   "move",
		"Fields": "copy-trivial,construct-direct",
	} {
		plan, ok := analyzer.ResolvedUnionConstructionOf(returned(name))
		if !ok || strings.Join(fieldActionNames(plan.PayloadActions), ",") != want {
			t.Errorf("%s payload actions = %v (%v), want %s", name, plan.PayloadActions, ok, want)
		}
	}
	handler := body("Handled")[0].(*ast.LetStatement).Value.(*ast.TryExpression).Handlers[0]
	optionTry := body("Unwrap")[0].(*ast.LetStatement).Value.(*ast.TryExpression)
	if transfer, ok := analyzer.ResolvedTryHandlerBindingTransferOf(handler); !ok || transfer.Action != sema.TransferMove {
		t.Errorf("handler payload binding transfer = %+v, %v; want a move", transfer, ok)
	}
	if resolved, ok := analyzer.ResolvedTryOf(optionTry); !ok || resolved.SuccessTransfer != sema.TransferMove {
		t.Errorf("Option try success transfer = %+v, %v; want a move", resolved, ok)
	}

	module, err := Build(parsed.Program, analyzer, BuildOptions{RequestedModule: "main", SourceFiles: []string{"plans.sec"}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{"union.construct", "construct-direct", "union.unwrap-payload"} {
		if !strings.Contains(text, want) {
			t.Errorf("module does not contain %q", want)
		}
	}

	// An action without a lowering is a structured boundary, not a fallback.
	fb := functionBuilder{owner: &builder{analyzer: analyzer, maxPackage: 17}}
	_, err = fb.unionPayloadAction(sema.StructFieldCopySemanticInfallible, &ast.Identifier{Value: "x"}, sema.Type{Kind: sema.IntType})
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Errorf("semantic-copy payload transfer = %v, want structured unsupported", err)
	}
	if _, err := fb.tryBindingMoves(&ast.TryHandler{}, sema.ResolvedTryHandler{BindingName: "e", BindingType: sema.Type{Kind: sema.StructType}}); !errors.As(err, &unsupported) {
		t.Errorf("try binding without a transfer plan = %v, want structured unsupported", err)
	}
}

func fieldActionNames(actions []sema.ResolvedStructFieldAction) []string {
	names := make([]string, len(actions))
	for index, action := range actions {
		names[index] = string(action)
	}
	return names
}
