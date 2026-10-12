package sema

import (
	"math/big"
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAssertionExtendedProofs verifies multiplication/division, integer
// truncation, zero-divisor rejection, mixed symbolic/constant chains, binding
// independence, conservative alias/call handling, and mutation at branch joins.
// Assertion proof must never erase effects from evaluating the condition.
// Rules: rules/errors/panic.md — §§15.6,15.8;
// rules/foundations/operators.md — integer multiplication/division/comparisons;
// rules/errors/runtime_checks.md — arithmetic checks.
func TestAssertionExtendedProofs(t *testing.T) {
	const file = "../../testdata/sema/assertion_extended_proofs_valid.sec"
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
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	want := map[string]bool{
		"positive product": true, "negative product": true, "two negatives": true,
		"positive quotient": true, "negative quotient": true, "negative divisor": true,
		"two negative quotients": true, "possible zero divisor": false,
		"refined product": true, "refined quotient": true,
		"constant bridge": true, "mirrored constant bridge": true,
		"named constant chain": true, "insufficient constant chain": false,
		"unrelated assignment": true, "unrelated branch assignment": true,
		"relevant assignment": false, "relevant branch assignment": false,
		"member mutation": false, "effectful rhs": false,
		"standalone alias call": false,
	}
	for _, fact := range a.resolvedAssertions {
		expected, exists := want[fact.Message]
		if !exists || fact.Proven != expected {
			t.Errorf("%q proven=%t, want %t (registered=%t)", fact.Message, fact.Proven, expected, exists)
		}
		delete(want, fact.Message)
	}
	if len(want) != 0 {
		t.Fatal("missing assertion facts", want)
	}
	g := a.CallGraph()
	summary := g.EffectSummary(callGraphNodeIDByName(t, g, "Products"))
	if !summary.MayPanic {
		t.Fatal("assertion proof erased checked arithmetic effects")
	}
	for _, effect := range summary.DirectEffects {
		if effect.Kind == EffectMayPanicAssertion {
			t.Fatal("proven assertion retained failure effect", effect)
		}
	}
}

// TestProductOrQuotientIntervalsAgainstEnumeration checks sign changes and
// truncation independently by enumerating every small integer operand pair.
// Rules: rules/foundations/operators.md — integer multiplication and division.
func TestProductOrQuotientIntervalsAgainstEnumeration(t *testing.T) {
	for _, operator := range []string{"*", "/"} {
		for leftMin := int64(-3); leftMin <= 3; leftMin++ {
			for leftMax := leftMin; leftMax <= 3; leftMax++ {
				for rightMin := int64(-3); rightMin <= 3; rightMin++ {
					for rightMax := rightMin; rightMax <= 3; rightMax++ {
						minimum, maximum, known := productOrQuotientInterval(operator, big.NewInt(leftMin), big.NewInt(leftMax), big.NewInt(rightMin), big.NewInt(rightMax))
						if operator == "/" && rightMin <= 0 && rightMax >= 0 {
							if known {
								t.Fatal("zero divisor interval admitted", rightMin, rightMax)
							}
							continue
						}
						if !known {
							t.Fatal("missing interval")
						}
						for left := leftMin; left <= leftMax; left++ {
							for right := rightMin; right <= rightMax; right++ {
								value := left * right
								if operator == "/" {
									value = left / right
								}
								if minimum.Cmp(big.NewInt(value)) > 0 || maximum.Cmp(big.NewInt(value)) < 0 {
									t.Fatal("unsound interval", operator, leftMin, leftMax, rightMin, rightMax, value, minimum, maximum)
								}
							}
						}
					}
				}
			}
		}
	}
}
