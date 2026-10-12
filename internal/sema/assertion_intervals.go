package sema

import "math/big"

// productOrQuotientInterval evaluates the four interval corners with arbitrary
// precision. Integer quotients truncate toward zero; intervals containing zero
// divisors give no proof. This proves the assertion's condition only: checked
// arithmetic retains its independent overflow/division panic effects.
// Rules: rules/errors/panic.md — §§15.6,15.8;
// rules/foundations/operators.md — "Integer division";
// rules/errors/runtime_checks.md — arithmetic checks.
func productOrQuotientInterval(operator string, leftMin, leftMax, rightMin, rightMax *big.Int) (*big.Int, *big.Int, bool) {
	if leftMin.Cmp(leftMax) > 0 || rightMin.Cmp(rightMax) > 0 {
		return nil, nil, false
	}
	if operator == "/" && rightMin.Sign() <= 0 && rightMax.Sign() >= 0 {
		return nil, nil, false
	}
	var minimum, maximum *big.Int
	for _, left := range []*big.Int{leftMin, leftMax} {
		for _, right := range []*big.Int{rightMin, rightMax} {
			value := new(big.Int)
			if operator == "*" {
				value.Mul(left, right)
			} else {
				value.Quo(left, right)
			}
			if minimum == nil || value.Cmp(minimum) < 0 {
				minimum = new(big.Int).Set(value)
			}
			if maximum == nil || value.Cmp(maximum) > 0 {
				maximum = new(big.Int).Set(value)
			}
		}
	}
	return minimum, maximum, true
}
