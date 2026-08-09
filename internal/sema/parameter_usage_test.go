package sema

import (
	"reflect"
	"testing"
)

func TestParameterUsageDistinguishesUnusedAndFieldRead(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Pair struct {
    first: int,
    second: int,
}

fn First(pair: ref Pair, fallback: int) int {
    return pair.first
}
`)
	assertSemaErrors(t, errors, nil)

	summary := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "First")
	pair := parameterUsageParameterNamed(t, summary, "pair")
	if pair.Demand.Access != ParameterAccessRead || pair.Demand.Mutation != ParameterNoMutation || pair.Demand.Ownership != ParameterBorrowSufficient {
		t.Fatalf("pair = %#v", pair)
	}
	if len(pair.Uses) != 1 || pair.Uses[0].Place.String() != "pair.first" {
		t.Fatalf("pair uses = %#v", pair.Uses)
	}
	fallback := parameterUsageParameterNamed(t, summary, "fallback")
	if fallback.Demand.Access != ParameterAccessUnused || fallback.Demand.Precision != ParameterDemandExact {
		t.Fatalf("fallback demand = %#v", fallback.Demand)
	}
}

func TestParameterUsageRecordsFieldMutation(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Pair struct {
    first: int,
}

fn SetFirst(pair: ref mut Pair, value: int) void {
    pair.first = value
}
`)
	assertSemaErrors(t, errors, nil)

	summary := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "SetFirst")
	pair := parameterUsageParameterNamed(t, summary, "pair")
	if pair.Demand.Access != ParameterAccessWrite || pair.Demand.Mutation != ParameterElementOrFieldMutation {
		t.Fatalf("pair demand = %#v", pair.Demand)
	}
	if len(pair.Uses) != 1 || pair.Uses[0].Kind != ParameterUseWrite || pair.Uses[0].Place.String() != "pair.first" {
		t.Fatalf("pair uses = %#v", pair.Uses)
	}
}

func TestParameterUsageDerivesIndexAndIterationShape(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Fourth(values: ref int[]) int {
    return values[3]
}

fn Sum(values: ref int[]) int {
    let mut total := 0
    for value in values {
        total += value
    }
    return total
}
`)
	assertSemaErrors(t, errors, nil)

	fourth := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Fourth"), "values")
	if !hasParameterShape(fourth.Demand.Shapes, ParameterShapeRandomAccess) || !hasParameterShape(fourth.Demand.Shapes, ParameterShapeMinimumExtent) || fourth.Demand.MinimumExtent != 4 {
		t.Fatalf("indexed demand = %#v", fourth.Demand)
	}
	sum := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Sum"), "values")
	if !hasParameterShape(sum.Demand.Shapes, ParameterShapeSequence) || len(sum.Uses) != 1 || sum.Uses[0].Kind != ParameterUseIteration {
		t.Fatalf("iteration demand = %#v, uses = %#v", sum.Demand, sum.Uses)
	}
}

func TestParameterUsageConsumesReturnedEscapeSummary(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Identity(value: ref int) ref int {
    return value
}
`)
	assertSemaErrors(t, errors, nil)

	value := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Identity"), "value")
	if value.Demand.Lifetime != ParameterLifetimeReturned || value.Demand.Identity != ParameterAddressRequired {
		t.Fatalf("returned demand = %#v", value.Demand)
	}
}

func TestParameterUsageDerivesSliceAndReferenceCallDemand(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Read(value: ref int) void {
    discard value
}

fn Slice(values: ref int[]) ref int[] {
    return values[1..<3]
}

fn Forward(value: ref int) void {
    Read(value)
}
`)
	assertSemaErrors(t, errors, nil)

	values := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Slice"), "values")
	if !hasParameterShape(values.Demand.Shapes, ParameterShapeSequence) || !hasParameterShape(values.Demand.Shapes, ParameterShapeKnownRange) {
		t.Fatalf("slice demand = %#v", values.Demand)
	}
	if len(values.Uses) == 0 || values.Uses[0].Place.String() != "values[1..<3]" {
		t.Fatalf("slice uses = %#v", values.Uses)
	}
	value := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Forward"), "value")
	if value.Demand.Identity != ParameterAddressRequired || value.Demand.Ownership != ParameterBorrowSufficient {
		t.Fatalf("ref-call demand = %#v", value.Demand)
	}
}

func TestParameterUsageSummarizesImplicitReceiver(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Counter struct {
    value: int,
}

impl Counter {
    fn Current() int {
        return self.value
    }
}
`)
	assertSemaErrors(t, errors, nil)

	summary := parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Current")
	if summary.Receiver == nil || summary.Receiver.Demand.Access != ParameterAccessRead {
		t.Fatalf("receiver = %#v", summary.Receiver)
	}
	if len(summary.Receiver.Uses) != 1 || summary.Receiver.Uses[0].Place.String() != "self.value" {
		t.Fatalf("receiver uses = %#v", summary.Receiver.Uses)
	}
}

func TestParameterUsageSnapshotIsDefensivelyCloned(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Read(values: ref int[]) int {
    return values[0]
}
`)
	assertSemaErrors(t, errors, nil)

	first := analyzer.ParameterUsageAnalysis()
	summary := parameterUsageSummaryNamed(t, first, "Read")
	summary.Parameters[0].Demand.Shapes[0] = ParameterShapeUnknown
	summary.Parameters[0].Uses[0].Place.Root = "changed"

	again := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Read"), "values")
	if hasParameterShape(again.Demand.Shapes, ParameterShapeUnknown) || again.Uses[0].Place.Root != "values" {
		t.Fatalf("analysis snapshot was mutated: %#v", again)
	}
}

func TestParameterUsagePropagatesDemandThroughForwardingChain(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Inner(values: ref int[]) int {
    return values[4]
}

fn Middle(values: ref int[]) int {
    return Inner(values)
}

fn Outer(values: ref int[]) int {
    return Middle(values)
}
`)
	assertSemaErrors(t, errors, nil)

	outer := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "Outer"), "values")
	if !hasParameterShape(outer.Demand.Shapes, ParameterShapeRandomAccess) || outer.Demand.MinimumExtent != 5 {
		t.Fatalf("forwarded demand = %#v", outer.Demand)
	}
	if !hasParameterUsePlace(outer.Uses, "values[4]") {
		t.Fatalf("forwarded uses = %#v", outer.Uses)
	}
	iterations, converged := analyzer.ParameterUsageAnalysis().InterproceduralStatus()
	if !converged || iterations == 0 {
		t.Fatalf("interprocedural status = iterations %d, converged %t", iterations, converged)
	}
}

func TestParameterUsageSolvesMutuallyRecursiveSCC(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Alpha(values: ref int[], descend: bool) int {
    return Beta(values, descend)
}

fn Beta(values: ref int[], descend: bool) int {
    if descend {
        return Alpha(values, false)
    }
    return values[6]
}
`)
	assertSemaErrors(t, errors, nil)

	analysis := analyzer.ParameterUsageAnalysis()
	for _, name := range []string{"Alpha", "Beta"} {
		values := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analysis, name), "values")
		if !hasParameterShape(values.Demand.Shapes, ParameterShapeRandomAccess) || values.Demand.MinimumExtent != 7 {
			t.Fatalf("%s recursive demand = %#v", name, values.Demand)
		}
	}
	if _, converged := analysis.InterproceduralStatus(); !converged {
		t.Fatal("mutually recursive demand did not converge")
	}
}

func TestParameterUsagePropagatesMethodReceiverDemand(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Buffer struct {
    values: int[8],
}

impl Buffer {
    fn Fourth() int {
        return self.values[3]
    }
}

fn ReadFourth(buffer: ref Buffer) int {
    return buffer.Fourth()
}
`)
	assertSemaErrors(t, errors, nil)

	buffer := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, analyzer.ParameterUsageAnalysis(), "ReadFourth"), "buffer")
	if !hasParameterShape(buffer.Demand.Shapes, ParameterShapeRandomAccess) || buffer.Demand.MinimumExtent != 4 {
		t.Fatalf("receiver demand = %#v", buffer.Demand)
	}
	if !hasParameterUsePlace(buffer.Uses, "buffer.values[3]") {
		t.Fatalf("receiver uses = %#v", buffer.Uses)
	}
}

func TestParameterUsagePropagationIsDeclarationOrderIndependent(t *testing.T) {
	forward, forwardErrors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Inspect(values: ref int[]) int {
    return values[2]
}

fn Forward(values: ref int[]) int {
    return Inspect(values)
}
`)
	assertSemaErrors(t, forwardErrors, nil)

	reversed, reversedErrors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Forward(values: ref int[]) int {
    return Inspect(values)
}

fn Inspect(values: ref int[]) int {
    return values[2]
}
`)
	assertSemaErrors(t, reversedErrors, nil)

	forwardDemand := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, forward.ParameterUsageAnalysis(), "Forward"), "values").Demand
	reversedDemand := parameterUsageParameterNamed(t, parameterUsageSummaryNamed(t, reversed.ParameterUsageAnalysis(), "Forward"), "values").Demand
	if !reflect.DeepEqual(forwardDemand, reversedDemand) {
		t.Fatalf("forward demand = %#v, reversed demand = %#v", forwardDemand, reversedDemand)
	}
}

func parameterUsageSummaryNamed(t *testing.T, analysis *ParameterUsageAnalysis, name string) ParameterUsageCallableSummary {
	t.Helper()
	for _, summary := range analysis.Summaries() {
		if summary.Name == name {
			return summary
		}
	}
	t.Fatalf("missing parameter-usage summary for %s", name)
	return ParameterUsageCallableSummary{}
}

func parameterUsageParameterNamed(t *testing.T, summary ParameterUsageCallableSummary, name string) ParameterUsageParameterSummary {
	t.Helper()
	for _, parameter := range summary.Parameters {
		if parameter.Name == name {
			return parameter
		}
	}
	t.Fatalf("missing parameter %s in %#v", name, summary.Parameters)
	return ParameterUsageParameterSummary{}
}

func hasParameterShape(shapes []ParameterShapeDemand, expected ParameterShapeDemand) bool {
	for _, shape := range shapes {
		if shape == expected {
			return true
		}
	}
	return false
}

func hasParameterUsePlace(uses []ParameterUse, expected string) bool {
	for _, use := range uses {
		if use.Place.String() == expected {
			return true
		}
	}
	return false
}
