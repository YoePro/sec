package sema

import "sec/internal/ast"

// interruptPanicRoot retains the source ISR context independently of the
// effective guarantee's explicit/implied provenance. Writing @noPanic too must
// not erase the interrupt context. An @isr entry takes precedence over a helper.
// This does not activate unsupported attributes or certify other ISR domains.
// Rules: rules/errors/panic.md — §22(1)–(6); rules/platform/interrupts.md — §§24–25,43.
func interruptPanicRoot(attributes []*ast.Attribute) (*ast.Attribute, string) {
	for _, name := range []string{"isr", "interruptSafe"} {
		for _, attribute := range attributes {
			if attribute != nil && attribute.Name != nil && attribute.Name.Value == name {
				if name == "isr" {
					return attribute, "ISR root:"
				}
				return attribute, "Interrupt-safe callable:"
			}
		}
	}
	return nil, ""
}

// interruptPanicHelp gives repairs compatible with the implied ISR guarantee:
// removing an explicit @noPanic or entering unsafe cannot waive this contract.
// Recoverable failures must be handled locally in ISR-safe code.
// Rules: rules/errors/panic.md — §§22,28(2),(5);
// rules/platform/interrupts.md — §§24–29,43.
func interruptPanicHelp(kind EffectKind, attribute string) string {
	prefix := "@" + attribute + " requires noPanic on every reachable synchronous and defer path; unsafe does not waive this requirement. "
	switch kind {
	case EffectMayPanicArithmetic:
		return prefix + "Prove the operand ranges, or use try with local handlers for ArithmeticError; the handlers must also be ISR-safe."
	case EffectMayPanicBounds:
		return prefix + "Prove the index is in range, or use try with local handlers for IndexError; the handlers must also be ISR-safe."
	case EffectMayPanicContract:
		return prefix + "Prove the constrained conversion, or use try with local handlers for its error; the handlers must also be ISR-safe."
	case EffectMayPanicAssertion:
		return prefix + "Prove the assertion condition on every reaching path, or handle the failing case locally without panic."
	case EffectMayPanicExplicit, EffectMayPanicUnreachable:
		return prefix + "Remove the reachable panic path, prove it unreachable, or move this work outside ISR execution."
	case EffectMayPanicForeign:
		return prefix + "Foreign abort behavior is unknown. Supply a documented trusted @noPanic foreign contract, or move the call outside ISR execution; other ISR/FFI requirements still apply."
	case EffectMayPanicInvalidReference:
		return prefix + "This Arena-backed reference is validated at run time and may panic as stale. Keep the Arena live and unreset across the use, or move the use outside ISR execution."
	case EffectMayPanicUnknownCallee:
		return prefix + "This call's target is unknown. Use a closed, compiler-resolved target set whose members satisfy noPanic, or move the call outside ISR execution."
	}
	return prefix + "Remove the panic-capable operation or move it outside ISR execution."
}
