package layout

import (
	"fmt"
	"strings"
)

// DefaultLogicalEpochBits is the canonical logical validity-epoch width on
// every target, independent of pointer width.
//
// Rules:
//   - rules/memory/reference_model.md — § 33 "Default logical epoch width", § 34
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — § 24
const DefaultLogicalEpochBits uint16 = 64

// ArenaEpochExhaustion is the language outcome for exhausted Arena Reset.
// No non-panic alternative is valid, including on runtime-free profiles.
// Rules: rules/memory/arena.md §39(1),(4),(7); MD-140 §§2–3.
type ArenaEpochExhaustion string

const ArenaEpochPanic ArenaEpochExhaustion = "panic"

// ArenaEpochShortWidthSafety identifies a conforming shorter-width strategy.
// Checked advancement preserves stale-reference safety by panicking before
// wrap or commit. This is a policy obligation, not evidence that an executable
// check exists, that exhaustion is impossible or that metadata can be omitted.
// Rules: rules/memory/reference_model.md §34; rules/memory/arena.md §39(5)–(7).
type ArenaEpochShortWidthSafety string

const ArenaEpochCheckedAdvance ArenaEpochShortWidthSafety = "checked-advance"

// ResolvedArenaEpochPolicy projects the Arena constraints from the one
// reference policy. LogicalEpochBits is independent of pointer width and
// applies in full even when several machine words are needed. Representation
// producers must preserve checked advancement, any reserved values and no
// stale revival; this record chooses no physical counter, initial value,
// maximum value, panic endpoint or proof-elimination form.
// Rules: rules/memory/arena.md §§38(8),39(4)–(9);
// rules/memory/reference_model.md §§33–34,37,40; MD-140 §§3,7.
type ResolvedArenaEpochPolicy struct {
	LogicalEpochBits        uint16
	Exhaustion              ArenaEpochExhaustion
	ShortWidthSafety        ArenaEpochShortWidthSafety
	ShortWidthJustification string
}

// ResolvedReferencePolicy is the reference subset of a CompilationPlan. The
// zero value is the canonical default: a 64-bit logical epoch on 32- and
// 64-bit targets alike. A shorter width is selected only at compile time and
// only together with the proof or exhaustion strategy that keeps stale
// references safe; runtime code never chooses the width, and the width is
// never part of a source reference type.
type ResolvedReferencePolicy struct {
	LogicalEpochBits uint16
	// ShortWidthProof names the compile-time proof/retirement/exhaustion
	// strategy that permits LogicalEpochBits below the default.
	ShortWidthProof string
	// ArenaExhaustion selects the Arena-specific language outcome. Empty
	// means the canonical panic outcome; a target trap is its physical
	// realization, never an alternative outcome here (MD-140).
	ArenaExhaustion ArenaEpochExhaustion
	// ArenaShortWidthSafety qualifies ShortWidthProof for Arena use. A
	// generic slot/domain retirement proof alone cannot justify Arena Reset.
	ArenaShortWidthSafety ArenaEpochShortWidthSafety
}

// EpochBits returns the effective logical epoch width.
// Rules: rules/memory/reference_model.md §§33–34.
func (p ResolvedReferencePolicy) EpochBits() uint16 {
	if p.LogicalEpochBits == 0 {
		return DefaultLogicalEpochBits
	}
	return p.LogicalEpochBits
}

// Validate rejects a shorter-than-default width without a compile-time proof
// and widths that are not supported machine-independent byte multiples.
// This general reference-policy check does not turn non-Arena retirement
// evidence into Arena safety evidence; ResolveArenaEpochPolicy checks that
// boundary before a complete scalar plan can be accepted.
// Rules: rules/memory/reference_model.md §§33–34,38–40;
// rules/memory/arena.md §39(4),(7),(9); MD-140 §§3,7.
func (p ResolvedReferencePolicy) Validate() error {
	bits := p.EpochBits()
	switch bits {
	case 8, 16, 32, 64:
	default:
		return fmt.Errorf("logical epoch width must be 8, 16, 32 or 64 bits, got %d", bits)
	}
	if bits < DefaultLogicalEpochBits && strings.TrimSpace(p.ShortWidthProof) == "" {
		return fmt.Errorf("a %d-bit logical epoch requires a compile-time proof or exhaustion strategy", bits)
	}
	if p.ArenaExhaustion != "" && p.ArenaExhaustion != ArenaEpochPanic {
		return fmt.Errorf("Arena epoch exhaustion must panic; policy %q is not permitted", p.ArenaExhaustion)
	}
	if p.ArenaShortWidthSafety != "" && p.ArenaShortWidthSafety != ArenaEpochCheckedAdvance {
		return fmt.Errorf("unsupported Arena short-width safety justification %q", p.ArenaShortWidthSafety)
	}
	return nil
}

// ResolveArenaEpochPolicy resolves only the Arena-specific constraints under
// the canonical logical width. Short-width selection requires both explicit
// checked-advance safety and nonempty producer justification; the general
// reference model's retirement proof is not silently reused for Arena Reset.
// Neither the default 64 bits nor this short-width selection eliminates any
// dynamic check: that needs separate domain/plan-qualified analysis proof.
// Rules: rules/memory/arena.md §§39(4)–(9); rules/platform/target_profiles.md §13;
// rules/memory/reference_model.md §§33–34,40; MD-140 §§3,5,7.
func (p ResolvedReferencePolicy) ResolveArenaEpochPolicy() (ResolvedArenaEpochPolicy, error) {
	if err := p.Validate(); err != nil {
		return ResolvedArenaEpochPolicy{}, err
	}
	if p.EpochBits() < DefaultLogicalEpochBits && p.ArenaShortWidthSafety != ArenaEpochCheckedAdvance {
		return ResolvedArenaEpochPolicy{}, fmt.Errorf("a %d-bit Arena epoch requires an explicit checked-advance safety justification consistent with panic exhaustion", p.EpochBits())
	}
	return ResolvedArenaEpochPolicy{
		LogicalEpochBits: p.EpochBits(), Exhaustion: ArenaEpochPanic,
		ShortWidthSafety: p.ArenaShortWidthSafety, ShortWidthJustification: p.ShortWidthProof,
	}, nil
}
