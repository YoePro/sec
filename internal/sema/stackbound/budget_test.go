package stackbound

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
		level   MeasurementLevel
		maximum *big.Int
	}{
		{"", Semantic, big.NewInt(1)},
		{"thread", "", big.NewInt(1)},
		{"thread", "other", big.NewInt(1)},
		{"thread", Machine, nil},
		{"thread", Semantic, big.NewInt(-1)},
	} {
		budget, err := NewBudget(test.domain, test.level, test.maximum)
		if _, valid := budget.MaximumBytes(); err == nil || valid {
			t.Fatal("invalid budget became available", budget, err)
		}
	}
	for _, level := range []MeasurementLevel{Semantic, Machine} {
		input := new(big.Int).Lsh(big.NewInt(1), 128)
		budget, err := NewBudget("physical-stack", level, input)
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
		equivalent, err := NewBudget(copy.Domain(), copy.MeasurementLevel(), fresh)
		if err != nil || equivalent != copy {
			t.Fatal("unstable value identity", err)
		}
	}
	zero, err := NewBudget("physical-stack", Semantic, big.NewInt(0))
	if maximum, valid := zero.MaximumBytes(); err != nil || !valid || maximum.Sign() != 0 {
		t.Fatal("explicit zero budget lost", err)
	}
	if maximum, valid := (Budget{}).MaximumBytes(); valid || maximum != nil {
		t.Fatal("absence became zero-byte budget")
	}
}

// TestStackBudgetComparison distinguishes sufficiency, exact excess, failed
// upper-bound proofs and nonfinite demand at precisely matching scopes.
// Rules: rules/analysis/stack_analysis.md — "Budget validation" and "UpperBound".
func TestStackBudgetComparison(t *testing.T) {
	for _, level := range []MeasurementLevel{Semantic, Machine} {
		budget, err := NewBudget("thread-a", level, big.NewInt(100))
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			kind  Kind
			bytes int64
			want  BudgetResult
		}{
			{Exact, 0, Satisfied},
			{Exact, 99, Satisfied},
			{Exact, 100, Satisfied},
			{Exact, 101, ExactExcess},
			{UpperBound, 99, Satisfied},
			{UpperBound, 100, Satisfied},
			{UpperBound, 101, UpperUnproven},
			{Unknown, 0, BudgetUnknown},
			{Unbounded, 0, BudgetUnbounded},
		} {
			bound := UnknownBound()
			switch test.kind {
			case Exact:
				bound, err = NewExact(big.NewInt(test.bytes))
			case UpperBound:
				bound, err = NewUpper(big.NewInt(test.bytes))
			case Unbounded:
				bound = UnboundedBound()
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
			level  MeasurementLevel
		}{
			{"thread-b", level}, {"", level}, {"thread-a", ""}, {"thread-a", "other"},
		} {
			if result, err := budget.Compare(mismatch.domain, mismatch.level, UnknownBound()); err == nil || result != "" {
				t.Fatal("mismatched scope accepted", result, err)
			}
		}
	}
	if result, err := (Budget{}).Compare("", "", UnknownBound()); err == nil || result != "" {
		t.Fatal("absent budget activated", result, err)
	}
	zero, _ := NewBudget("thread-a", Semantic, big.NewInt(0))
	bound, _ := NewExact(big.NewInt(0))
	if result, err := zero.Compare("thread-a", Semantic, bound); err != nil || result != Satisfied {
		t.Fatal("exact zero cannot satisfy zero availability", result, err)
	}
}
