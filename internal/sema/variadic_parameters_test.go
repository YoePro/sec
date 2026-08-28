package sema

import "testing"

func TestNativeVariadicParameterPackSupportsIterationAndRespread(t *testing.T) {
	// rules/declarations/functions.md sections 28-30 and 35: native packs are
	// final typed parameters, iterable by element, and may be re-spread into a
	// compatible variadic destination.
	input := `
module main

fn Sum(numbers: ...int) int {
	let mut total := 0
	for number in numbers {
		total += number
	}
	return total
}

fn Forward(numbers: ...int) int {
	return Sum(numbers...)
}

fn ForwardArray(numbers: ref int[]) int {
	return Sum(1, numbers..., 2, numbers...)
}

fn Select(value: int) int {
	return value
}

fn Select(values: ...int) int {
	return 0
}

fn Use() int {
	return Sum() + Sum(1, 2, 3) + Forward(4, 5) + Select(6)
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)

	sum := analyzer.functions["Sum"][0]
	if len(sum.Parameters) != 1 || !sum.Parameters[0].Variadic {
		t.Fatalf("Sum parameters = %+v, want native variadic int pack", sum.Parameters)
	}

	for expression, resolved := range analyzer.resolvedCalls {
		if expression.String() == "Select(6)" && resolved.Function.Parameters[0].Variadic {
			t.Fatalf("Select(6) resolved to variadic overload: %+v", resolved.Function)
		}
	}
}

func TestNativeVariadicPackIsReadOnlyAndCannotEscape(t *testing.T) {
	// rules/declarations/functions.md sections 31, 32, and 34 prohibit pack
	// mutation, escape, and move-out even though indexed reads are allowed.
	errors := analyzeSourceRaw(t, `
module main

type Resource struct {
	view: ref mut int,
}

fn Escape(values: ...int) int {
	return values
}

fn Extract(values: ...Resource) Resource {
	return values[0]
}

fn Mutate(values: ...int) void {
	values[0] = 1
}
`)
	assertSemaErrors(t, errors, []string{
		"variadic parameter pack cannot escape this call at 9:9",
		"cannot move element out of variadic parameter pack at 13:15",
		"cannot mutate variadic parameter pack at 17:2",
	})
}
