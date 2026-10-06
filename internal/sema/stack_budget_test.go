package sema

import (
	"math/big"
	"testing"
)

// TestStackBudgetContracts checks finite availability and defensive ownership,
// without interpreting absent contracts as zero or inventing domain identities.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "No-budget behavior".
func TestStackBudgetContracts(t *testing.T) {
	for _, test := range []struct {
		domain  string
		level   StackMeasurementLevel
		maximum *big.Int
	}{
		{"", StackMeasurementSemantic, big.NewInt(1)},
		{"thread", "", big.NewInt(1)},
		{"thread", "other", big.NewInt(1)},
		{"thread", StackMeasurementMachine, nil},
		{"thread", StackMeasurementSemantic, big.NewInt(-1)},
	} {
		budget, err := NewStackBudget(test.domain, test.level, test.maximum)
		if _, valid := budget.MaximumBytes(); err == nil || valid {
			t.Fatal("invalid budget became available", budget, err)
		}
	}
	for _, level := range []StackMeasurementLevel{StackMeasurementSemantic, StackMeasurementMachine} {
		input := new(big.Int).Lsh(big.NewInt(1), 128)
		budget, err := NewStackBudget("physical-stack", level, input)
		if err != nil {
			t.Fatal(err)
		}
		copy := budget
		input.SetInt64(1)
		maximum, valid := budget.MaximumBytes()
		if !valid || maximum.BitLen() != 129 || budget.Domain() != "physical-stack" || budget.MeasurementLevel() != level {
			t.Fatal("budget identity or count changed", budget)
		}
		maximum.SetInt64(2)
		fresh, valid := copy.MaximumBytes()
		if !valid || fresh.BitLen() != 129 {
			t.Fatal("mutable output aliases budget")
		}
		equivalent, err := NewStackBudget(copy.Domain(), copy.MeasurementLevel(), fresh)
		if err != nil || equivalent != copy {
			t.Fatal("unstable value identity", err)
		}
	}
	zero, err := NewStackBudget("physical-stack", StackMeasurementSemantic, big.NewInt(0))
	if maximum, valid := zero.MaximumBytes(); err != nil || !valid || maximum.Sign() != 0 {
		t.Fatal("explicit zero budget lost", err)
	}
	if maximum, valid := (StackBudget{}).MaximumBytes(); valid || maximum != nil {
		t.Fatal("absence became zero-byte budget")
	}
}

// TestStackBudgetComparison distinguishes sufficiency, exact excess, failed
// upper-bound proofs and nonfinite demand at precisely matching scopes.
// Rules: rules/analysis/stack_analysis.md — "Budget validation" and "UpperBound".
func TestStackBudgetComparison(t *testing.T) {
	for _, level := range []StackMeasurementLevel{StackMeasurementSemantic, StackMeasurementMachine} {
		budget, err := NewStackBudget("thread-a", level, big.NewInt(100))
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			kind  StackBoundKind
			bytes int64
			want  StackBudgetResult
		}{
			{StackBoundExact, 0, StackBudgetSatisfied},
			{StackBoundExact, 99, StackBudgetSatisfied},
			{StackBoundExact, 100, StackBudgetSatisfied},
			{StackBoundExact, 101, StackBudgetExactExcess},
			{StackBoundUpperBound, 99, StackBudgetSatisfied},
			{StackBoundUpperBound, 100, StackBudgetSatisfied},
			{StackBoundUpperBound, 101, StackBudgetUpperUnproven},
			{StackBoundUnknown, 0, StackBudgetUnknown},
			{StackBoundUnbounded, 0, StackBudgetUnbounded},
		} {
			bound := UnknownStackBound()
			switch test.kind {
			case StackBoundExact:
				bound, err = NewExactStackBound(big.NewInt(test.bytes))
			case StackBoundUpperBound:
				bound, err = NewUpperStackBound(big.NewInt(test.bytes))
			case StackBoundUnbounded:
				bound = UnboundedStackBound()
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := budget.Compare("thread-a", level, bound)
			if err != nil || got != test.want {
				t.Fatalf("%s: got %s, %v; want %s", bound, got, err, test.want)
			}
		}
		for _, mismatch := range []struct {
			domain string
			level  StackMeasurementLevel
		}{
			{"thread-b", level}, {"", level}, {"thread-a", ""}, {"thread-a", "other"},
		} {
			if result, err := budget.Compare(mismatch.domain, mismatch.level, UnknownStackBound()); err == nil || result != "" {
				t.Fatal("mismatched scope accepted", result, err)
			}
		}
	}
	if result, err := (StackBudget{}).Compare("", "", UnknownStackBound()); err == nil || result != "" {
		t.Fatal("absent budget activated", result, err)
	}
	zero, _ := NewStackBudget("thread-a", StackMeasurementSemantic, big.NewInt(0))
	bound, _ := NewExactStackBound(big.NewInt(0))
	if result, err := zero.Compare("thread-a", StackMeasurementSemantic, bound); err != nil || result != StackBudgetSatisfied {
		t.Fatal("exact zero cannot satisfy zero availability", result, err)
	}
}

// TestStackBudgetMachineAuthority uses independent summary evidence and large
// byte counts; a semantic estimate never satisfies or rejects a machine budget.
// Rules: rules/analysis/stack_analysis.md — "Machine-level revalidation",
// "Semantic and machine frame authority", and "CompilationPlan dependence".
func TestStackBudgetMachineAuthority(t *testing.T) {
	budget, _ := NewStackBudget("thread-a", StackMeasurementMachine, big.NewInt(4500))
	semantic, _ := NewUpperStackBound(big.NewInt(4608))
	machine, _ := NewExactStackBound(big.NewInt(4384))
	var store StackSummaryStore
	if err := store.RecordSemantic(SemanticStackSummary{Callable: "entry", CompilationPlanID: "plan", TransitiveMaximum: semantic}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMachine(MachineStackSummary{Callable: "entry", CompilationPlanID: "plan", TransitiveMaximum: machine}); err != nil {
		t.Fatal(err)
	}
	summary, _ := store.Machine("entry", "plan")
	if result, err := budget.Compare("thread-a", StackMeasurementMachine, summary.TransitiveMaximum); err != nil || result != StackBudgetSatisfied {
		t.Fatal("final machine evidence not authoritative", result, err)
	}
	if result, err := budget.Compare("thread-a", StackMeasurementSemantic, semantic); err == nil || result != "" {
		t.Fatal("cross-level estimate compared", result, err)
	}
	semantic, _ = NewUpperStackBound(big.NewInt(4000))
	machine, _ = NewExactStackBound(big.NewInt(4600))
	if result, err := budget.Compare("thread-a", StackMeasurementMachine, machine); err != nil || result != StackBudgetExactExcess {
		t.Fatal("final machine excess hidden", result, err)
	}
	if result, err := budget.Compare("thread-a", StackMeasurementSemantic, semantic); err == nil || result != "" {
		t.Fatal("semantic success substituted", result, err)
	}
	large := new(big.Int).Lsh(big.NewInt(1), 128)
	budget, _ = NewStackBudget("thread-a", StackMeasurementMachine, large)
	for _, delta := range []int64{-1, 0, 1} {
		count := new(big.Int).Add(large, big.NewInt(delta))
		bound, _ := NewExactStackBound(count)
		want := StackBudgetSatisfied
		if delta > 0 {
			want = StackBudgetExactExcess
		}
		if result, err := budget.Compare("thread-a", StackMeasurementMachine, bound); err != nil || result != want {
			t.Fatal("large comparison overflow", result, err)
		}
	}
}
