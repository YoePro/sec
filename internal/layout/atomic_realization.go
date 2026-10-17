package layout

import "fmt"

// AtomicImplementationKind distinguishes the selected target mechanism from
// language eligibility. A compiler intrinsic or platform primitive does not
// imply one native instruction; emulation need not use a blocking mutex.
// Rules: rules/concurrency/atomics.md — §§37–38;
// rules/platform/platform_model.md — §9 "Architecture capability".
type AtomicImplementationKind string

const (
	AtomicNativeInstruction AtomicImplementationKind = "native-instruction"
	AtomicCompilerIntrinsic AtomicImplementationKind = "compiler-intrinsic"
	AtomicPlatformPrimitive AtomicImplementationKind = "platform-primitive"
	AtomicEmulated          AtomicImplementationKind = "emulated"
)

// AtomicProperty is an explicit target proof of a property or its absence.
// Zero is unknown: a missing guarantee is never affirmative evidence. These
// internal facts introduce no source-level lock-free query or new attribute.
// Rules: rules/concurrency/atomics.md — §§37–39,50 (future input areas);
// rules/platform/target_profiles.md — §10 and atomic capability separation.
type AtomicProperty string

const (
	AtomicPropertyUnknown AtomicProperty = ""
	AtomicPropertyYes     AtomicProperty = "yes"
	AtomicPropertyNo      AtomicProperty = "no"
)

// AtomicImplementation names the target-declared realization of one exact
// requirement. Identity is its compiler/platform implementation identity, not
// a source spelling or an instruction to silently select a runtime fallback.
// LockFree, Blocking and InterruptSafe are independent declared proof facts;
// none is inferred from Kind, target word width, profile name or compiler host.
// Rules: rules/concurrency/atomics.md — §§37(1–5),38(1–6),39;
// rules/platform/target_profiles.md — §10 and atomic capability separation.
type AtomicImplementation struct {
	Kind          AtomicImplementationKind
	Identity      string
	LockFree      AtomicProperty
	Blocking      AtomicProperty
	InterruptSafe AtomicProperty
}

// validate requires a declared implementation for affirmative capability,
// preserving unknown progress/context properties rather than inventing them.
// Denial or unknown availability cannot masquerade as a selected realization.
// Rules: rules/concurrency/atomics.md — §§37–39;
// rules/platform/platform_model.md — §9 "Architecture capability".
func (c AtomicCapability) validate() error {
	if err := c.Requirement.validate(); err != nil {
		return err
	}
	switch c.Availability {
	case AtomicSupported, AtomicUnsupported:
		if c.Provenance == "" {
			return fmt.Errorf("known atomic capability requires target producer provenance")
		}
	case AtomicUnknown:
	default:
		return fmt.Errorf("invalid atomic availability %q", c.Availability)
	}
	if c.Availability != AtomicSupported {
		if c.Implementation != (AtomicImplementation{}) {
			return fmt.Errorf("unavailable or unknown atomic capability cannot select an implementation")
		}
		return nil
	}
	switch c.Implementation.Kind {
	case AtomicNativeInstruction, AtomicCompilerIntrinsic, AtomicPlatformPrimitive, AtomicEmulated:
	default:
		return fmt.Errorf("supported atomic capability requires a declared implementation kind, got %q", c.Implementation.Kind)
	}
	if c.Implementation.Identity == "" {
		return fmt.Errorf("supported atomic capability requires an implementation identity")
	}
	for _, property := range []AtomicProperty{c.Implementation.LockFree, c.Implementation.Blocking, c.Implementation.InterruptSafe} {
		switch property {
		case AtomicPropertyUnknown, AtomicPropertyYes, AtomicPropertyNo:
		default:
			return fmt.Errorf("invalid atomic implementation property %q", property)
		}
	}
	return c.validateContext()
}
