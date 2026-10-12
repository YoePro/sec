package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryAssignmentFlows projects only validated plans, with exact callable
// ownership, handler continuation and residual propagation, detached and stable.
// Rules: rules/errors/errorhandling.md — §§16,21,23,34,36.
func TestTryAssignmentFlows(t *testing.T) {
	for _, fixture := range []string{"valid", "invalid"} {
		file := "../../testdata/lsp/try_assignment_flow_" + fixture + ".sec"
		if fixture == "valid" {
			file = "../../testdata/lsp/try_assignment_flow_valid_test.sec"
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		flows := a.TryAssignmentFlows()
		if fixture == "invalid" {
			if len(errors) == 0 || len(flows) != 0 {
				t.Fatal(errors, flows)
			}
			continue
		}
		if len(errors) != 0 || len(flows) != 6 {
			t.Fatal(errors, flows)
		}
		boundaries := 0
		for _, flow := range flows {
			node, ok := a.callGraph.Node(flow.Callable)
			if !ok || flow.Source.File != file || flow.ErrorType != "SetterError" || !flow.SuccessContinues {
				t.Fatal(flow)
			}
			if flow.PropagationTarget == "test invocation" {
				if !flow.FailurePropagates || flow.Kind != ResolvedTryAssignmentPropagation || len(flow.HandlerFlows) != 0 {
					t.Fatal(flow)
				}
				boundaries++
				continue
			}
			switch node.Name {
			case "Propagate":
				if !flow.FailurePropagates || flow.FailureContinues || len(flow.HandlerFlows) != 0 || flow.PropagationTarget != "Result[void, SetterError]" {
					t.Fatal(flow)
				}
			case "Handle", "Caller":
				if flow.FailurePropagates || !flow.FailureContinues || !flow.Exhaustive {
					t.Fatal(flow)
				}
			case "Partial":
				if !flow.FailurePropagates || !flow.FailureContinues || flow.Exhaustive {
					t.Fatal(flow)
				}
			case "Terminate":
				if flow.FailurePropagates || flow.FailureContinues || !flow.Exhaustive || len(flow.HandlerFlows) != 1 || flow.HandlerFlows[0] != TryHandlerReturns {
					t.Fatal(flow)
				}
			default:
				t.Fatal("wrong assignment owner", node)
			}
		}
		if boundaries != 1 {
			t.Fatal("missing invocation boundary", flows)
		}
		baseline := a.TryAssignmentFlows()
		flows[1].HandlerFlows[0] = TryHandlerInvalidFlow
		flows[0].ErrorType = "modified"
		if !reflect.DeepEqual(a.TryAssignmentFlows(), baseline) {
			t.Fatal("snapshot leaked mutable state")
		}
	}
}
