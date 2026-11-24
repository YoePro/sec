package layout

import (
	"errors"
	"fmt"
)

// AtomicContextViolation preserves a failed target proof separately from
// source-language type/order validity. Unknown properties are insufficient
// evidence in a restricted context. Requirement and implementation are detached
// values; the error never changes the selected plan or selects a fallback.
// Rules: rules/concurrency/atomics.md — §§38(3–7),39(1–5),48;
// rules/platform/target_profiles.md — §10 and atomic capability separation.
type AtomicContextViolation struct {
	Key            AtomicCapabilityKey
	Implementation AtomicImplementation
	Property       string
	Observed       AtomicProperty
}

// Error identifies the exact implementation, operation and execution
// category whose guarantee is missing or incompatible.
// Rules: rules/concurrency/atomics.md — §§38–39, 37A(13), 48(1–2).
func (e AtomicContextViolation) Error() string {
	if e.Property == "nonblocking" {
		if e.Observed == AtomicPropertyUnknown {
			return fmt.Sprintf("atomic %s implementation %q has no proof of nonblocking operation in context %q", e.Key.Operation, e.Implementation.Identity, e.Key.Context)
		}
		return fmt.Sprintf("atomic %s implementation %q may block in context %q requiring nonblocking operation", e.Key.Operation, e.Implementation.Identity, e.Key.Context)
	}
	if e.Observed == AtomicPropertyUnknown {
		return fmt.Sprintf("atomic %s implementation %q has no proof of interrupt safety in context %q", e.Key.Operation, e.Implementation.Identity, e.Key.Context)
	}
	return fmt.Sprintf("atomic %s implementation %q is not interrupt-safe in context %q", e.Key.Operation, e.Implementation.Identity, e.Key.Context)
}

// validateContext rejects a purportedly supported implementation unless it
// proves every required guarantee. ISR-safe atomicity requires both interrupt
// safety and nonblocking execution, even when NoBlocking is not separately set.
// Native/intrinsic mechanisms and lock freedom supply no implicit exemption;
// interrupt masking is not inferred from a name or assumed cross-core safe.
// All failed guarantees are retained, without a diagnostic-priority decision.
// This validates already-resolved plan keys only; it never makes a missing
// capability positive (atomics.md § 37A(13)).
// Rules: rules/concurrency/atomics.md — §§37(1–5),38(3–7),39(1–5), 37A(13).
func (c AtomicCapability) validateContext() error {
	var violations []error
	if (c.Key.NoBlocking || c.Key.InterruptSafe) && c.Implementation.Blocking != AtomicPropertyNo {
		violations = append(violations, AtomicContextViolation{Key: c.Key, Implementation: c.Implementation, Property: "nonblocking", Observed: c.Implementation.Blocking})
	}
	if c.Key.InterruptSafe && c.Implementation.InterruptSafe != AtomicPropertyYes {
		violations = append(violations, AtomicContextViolation{Key: c.Key, Implementation: c.Implementation, Property: "interrupt-safe", Observed: c.Implementation.InterruptSafe})
	}
	return errors.Join(violations...)
}
