package main

import (
	"os"
	"strings"
	"testing"
)

// State completion consumes resolved subject types without introducing payload
// bindings, broad global symbols, or another variant inference engine.
// Rules: rules/declarations/unions.md — §§8.1,8.2a,12;
// rules/tooling/lsp.md — "Completion"; rules/foundations/operators.md — "State tests with is".
func TestStateTestCompletionOffersSubjectVariants(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/state_test_completion/context.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ expression, want string }{
		{"if state is | {}", "Done,Idle,Running"},
		{"if state is |", "Done,Idle,Running"},
		{"while state is not |", "Done,Idle,Running"},
		{"if state is not\r\n R| {}", "Running"},
		{"/* 🙂 */ if state is R| {}", "Running"},
		{"if state is not | {}", "Done,Idle,Running"},
		{"return state is R|", "Running"},
		{"let ready := state is I|", "Idle"},
		{"while state is not D| {}", "Done"},
		{"if true && holder.state is | {}", "Done,Idle,Running"},
		{"if Current(state) is | {}", "Done,Idle,Running"},
		{"if (state is |) {}", "Done,Idle,Running"},
		{"if state is State.| {}", "Done,Idle,Running"},
		{"if state is not State.R| {}", "Running"},
		{"if optional is | {}", "None,Some"},
		{"if optional is not S| {}", "Some"},
		{"if optional is Option.| {}", "None,Some"},
		{"if result is | {}", "Err,Ok"},
		{"if result is Result.O| {}", "Ok"},
		{"if number is | {}", ""},
		{"if mode is | {}", ""},
		{"if absent is | {}", ""},
		{"if state is Other.| {}", ""},
		{"if state is State.No| {}", ""},
		{"if state is State.Other.| {}", ""},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := strings.Replace(string(data), "@", tc.expression, 1)
			offset := strings.Index(source, "|")
			source = strings.Replace(source, "|", "", 1)
			items := completeSource("", source, offset)
			names := []string{}
			for _, item := range items {
				names = append(names, item.Label)
			}
			if got := strings.Join(names, ","); got != tc.want {
				t.Fatalf("items = %q, want %q", got, tc.want)
			}
		})
	}
}

// Detection must not take over completed tests, payload bindings, strings,
// comments or a contextual identifier named is used as an ordinary member.
// Rules: rules/tooling/lsp.md — "Completion", "Recovery nodes";
// rules/foundations/operators.md — "State tests with is".
func TestStateTestCompletionDoesNotCaptureOtherPositions(t *testing.T) {
	for _, source := range []string{
		`let text := "state is `,
		`// state is `,
		`/* state is `,
		`if state is Running(value`,
		`if state is Idle { let value := `,
		`holder.is.`,
		`if state is`,
		`if state is not`,
		`fn is(`,
	} {
		if _, _, ok := stateTestCompletionPosition("", source, len(source)); ok {
			t.Fatalf("captured %q", source)
		}
	}
}
