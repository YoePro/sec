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
	Requirement    AtomicRequirement
	Implementation AtomicImplementation
	Property       string
	Observed       AtomicProperty
}

// Error identifies the exact implementation, operation and execution contract
// whose guarantee is missing or incompatible, without prescribing source key
// spellings or an Unknown-capability policy for Sema (MD-095).
// Rules: rules/concurrency/atomics.md — §§38–39,48(1–2).
func (e AtomicContextViolation) Error() string {
	if e.Property == "nonblocking" {
		if e.Observed == AtomicPropertyUnknown {
			return fmt.Sprintf("atomic %s implementation %q has no proof of nonblocking operation in context %q", e.Requirement.Operation, e.Implementation.Identity, e.Requirement.ContextID)
		}
		return fmt.Sprintf("atomic %s implementation %q may block in context %q requiring nonblocking operation", e.Requirement.Operation, e.Implementation.Identity, e.Requirement.ContextID)
	}
	if e.Observed == AtomicPropertyUnknown {
		return fmt.Sprintf("atomic %s implementation %q has no proof of interrupt safety in context %q", e.Requirement.Operation, e.Implementation.Identity, e.Requirement.ContextID)
	}
	return fmt.Sprintf("atomic %s implementation %q is not interrupt-safe in context %q", e.Requirement.Operation, e.Implementation.Identity, e.Requirement.ContextID)
}

// validateContext rejects a purportedly supported implementation unless it
// proves every required guarantee. ISR-safe atomicity requires both interrupt
// safety and nonblocking execution, even when NoBlocking is not separately set.
// Native/intrinsic mechanisms and lock freedom supply no implicit exemption;
// interrupt masking is not inferred from a name or assumed cross-core safe.
// All failed guarantees are retained, without a diagnostic-priority decision.
// This validates already-resolved plan requirements only; it neither produces
// source requirement keys nor makes missing capabilities positive (MD-095).
// Rules: rules/concurrency/atomics.md — §§37(1–5),38(3–7),39(1–5).
func (c AtomicCapability) validateContext() error {
	var violations []error
	if (c.Requirement.NoBlocking || c.Requirement.InterruptSafe) && c.Implementation.Blocking != AtomicPropertyNo {
		violations = append(violations, AtomicContextViolation{Requirement: c.Requirement, Implementation: c.Implementation, Property: "nonblocking", Observed: c.Implementation.Blocking})
	}
	if c.Requirement.InterruptSafe && c.Implementation.InterruptSafe != AtomicPropertyYes {
		violations = append(violations, AtomicContextViolation{Requirement: c.Requirement, Implementation: c.Implementation, Property: "interrupt-safe", Observed: c.Implementation.InterruptSafe})
	}
	return errors.Join(violations...)
}
