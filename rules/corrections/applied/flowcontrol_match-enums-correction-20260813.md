# Match correction — closed and open enum domains

**Status:** Applied
**Applied:** 2026-08-13
**Language version:** Sec 0.1
**Created:** 2026-08-13
**Last updated:** 2026-08-13
**Target:** `rules/control-flow/flowcontrol_match.txt`
**Source of truth:** `rules/declarations/enums.md`

## Required domain correction

The current general statement that enum exhaustiveness is based on the complete representable
underlying integer domain is no longer correct for ordinary enums.

### Ordinary enums

An ordinary enum is closed.

Its complete semantic match domain is the set of unique numeric value classes declared by its
members.

A `match` is exhaustive when all such value classes are covered by unguarded patterns or an
unguarded catch-all is present.

Numeric aliases share one coverage class.

### Bit-backed enums

A `bit[N]` enum is open over all `2^N` representable bit patterns.

Declared members may cover only part of that domain.

A `match` over a bit-backed enum is exhaustive only when:

```text
all representable bit patterns are covered by unguarded patterns
OR
an unguarded catch-all is present
```

Canonical hardware-facing example:

```sec
match mode {
    AlertPinMode.ALERT_DISABLED => HandleDisabled()
    AlertPinMode.COMPARATOR => HandleComparator()
    AlertPinMode.INTERRUPT => HandleInterrupt()
    _ => HandleUnknownMode()
}
```

Tooling should make clear that the fallback handles reserved, undocumented, future, or other
undeclared hardware encodings.
