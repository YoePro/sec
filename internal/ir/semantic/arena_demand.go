package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/sema"
)

// ArenaDemandFacts is one Arena domain's compile-time byte demand (P19
// §§ 79–81, 130): Kind "constant" with an upper-bound Bytes, or "unknown";
// Expression is the symbolic derivation; CapacityBytes the fixed backing
// when known. It is metadata, never a runtime value or allocator choice.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–81, 130.
type ArenaDemandFacts struct {
	Domain        uint32
	Kind          string
	Bytes         *big.Int
	Expression    string
	CapacityBytes *big.Int
}

// arenaDemandFactsOf detaches Sema's summaries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — § 130.
func arenaDemandFactsOf(summaries []sema.ArenaDemandSummary) []ArenaDemandFacts {
	var facts []ArenaDemandFacts
	for _, summary := range summaries {
		fact := ArenaDemandFacts{Domain: uint32(summary.Domain), Kind: summary.Kind, Expression: summary.Expression}
		if summary.Bytes != nil {
			fact.Bytes = new(big.Int).Set(summary.Bytes)
		}
		if summary.CapacityBytes != nil {
			fact.CapacityBytes = new(big.Int).Set(summary.CapacityBytes)
		}
		facts = append(facts, fact)
	}
	return facts
}

// verifyArenaDemand checks the summary shape: a domain, a known kind, bytes
// exactly for constant demand, an expression, and no duplicate domain.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79, 130.
func verifyArenaDemand(fn *Function) error {
	seen := map[uint32]bool{}
	for _, fact := range fn.ArenaDemand {
		if fact.Domain == 0 || seen[fact.Domain] || fact.Expression == "" ||
			(fact.Kind != "constant" && fact.Kind != "unknown") || (fact.Kind == "constant") != (fact.Bytes != nil) ||
			fact.Bytes != nil && fact.Bytes.Sign() < 0 {
			return fmt.Errorf("function %s has an invalid Arena demand summary for domain %d", fn.ID, fact.Domain)
		}
		seen[fact.Domain] = true
	}
	return nil
}
