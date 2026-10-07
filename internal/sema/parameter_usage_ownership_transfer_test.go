package sema

import "testing"

// Revision 2.0 ownership-transfer regressions: consumption demand follows the
// resolved transfer and callee contract, never the presence of a `<-` token.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Ownership-transfer syntax boundary", "Required ownership-transfer tests"
//   - rules/analysis/parameter_usage_analysis.md — "Move use", "Calls propagate demand", "Forwarding", "Control-flow joins"
//   - rules/analysis/parameter_usage_analysis.md — "Returning a parameter by value"
func TestParameterUsageOwnershipTransferFollowsResolvedSemantics(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

@noCopy
type Buffer struct {
    data: int[64],
}

fn Sink(-> value: Buffer) void {
}

fn Inspect(value: ref Buffer) void {
}

fn Forward(-> value: Buffer) void {
    Sink(<-value)
}

fn Branch(-> value: Buffer, store: bool) void {
    if store {
        Sink(<-value)
    } else {
        Inspect(value)
    }
}

fn Fresh() void {
    Sink(Buffer{})
}

fn ReadOnly(value: Buffer) int {
    return value.data[0]
}

fn ReturnPlain(-> value: Buffer) Buffer {
    return value
}

fn ReturnMarked(-> value: Buffer) Buffer {
    return <-value
}
`)
	assertSemaErrors(t, errors, nil)
	analysis := analyzer.ParameterUsageAnalysis()
	ownership := func(function string) ParameterOwnershipDemand {
		t.Helper()
		return parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, function), "value").Demand.Ownership
	}

	if got := ownership("Forward"); got != ParameterConsumptionRequired {
		t.Fatalf("consuming forwarding ownership = %s, want %s", got, ParameterConsumptionRequired)
	}
	if got := ownership("Branch"); got != ParameterConsumptionRequired {
		t.Fatalf("branch join ownership = %s, want %s", got, ParameterConsumptionRequired)
	}
	// A move-only by-value parameter is not consuming from its type alone.
	if got := ownership("ReadOnly"); got != ParameterBorrowSufficient {
		t.Fatalf("by-value move-only read ownership = %s, want %s", got, ParameterBorrowSufficient)
	}
	// The optional return marker yields the same demand as the intrinsic
	// result-value transfer.
	plain, marked := ownership("ReturnPlain"), ownership("ReturnMarked")
	if plain != marked || plain == ParameterBorrowSufficient {
		t.Fatalf("return value ownership = %s, return <-value ownership = %s; want equal non-borrow demand", plain, marked)
	}
	fresh := parameterUsageSummaryNamed(t, analysis, "Fresh")
	if len(fresh.Parameters) != 0 {
		t.Fatalf("fresh temporary caller summary = %#v", fresh.Parameters)
	}
}

// A scalar copied out through a borrowed parameter is not a returned
// dependency, so neither the callee nor a caller passing its own parameter
// gains returned-lifetime demand.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Returning a parameter by value", "Calls propagate demand"
//   - rules/analysis/escape_analysis.md — "Call-boundary semantic classification"
func TestParameterUsageCopyThroughBorrowIsNotReturned(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Buffer struct {
    data: int[8],
}

fn First(value: ref Buffer) int {
    return value.data[0]
}

fn Caller(buffer: Buffer) void {
    discard First(buffer)
}
`)
	assertSemaErrors(t, errors, nil)
	analysis := analyzer.ParameterUsageAnalysis()
	first := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, "First"), "value")
	if first.Demand.Lifetime != ParameterLifetimeCallOnly {
		t.Fatalf("copied borrowed read lifetime = %s, want %s", first.Demand.Lifetime, ParameterLifetimeCallOnly)
	}
	caller := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, "Caller"), "buffer")
	if caller.Demand.Lifetime != ParameterLifetimeCallOnly {
		t.Fatalf("caller lifetime = %s, want %s", caller.Demand.Lifetime, ParameterLifetimeCallOnly)
	}
}
