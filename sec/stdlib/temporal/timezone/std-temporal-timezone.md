# Sec Standard Library — temporal/timezone

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/timezone/std-temporal-timezone.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Pinned IANA named timezones and local civil-time resolution.

**§ 1(2)** `datetime` stays unzoned. Named zones are not equal to constant offsets. Resolution reports gap, overlap or unique matching offsets.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `timezone/timezone.sec`

```sec
module timezone

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/timezone/timezone.sec
 * Module: timezone
 * Purpose: Explicit named zone identification and local time resolution.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/timezone/std-temporal-timezone.md
 * Standards / dependencies: IANA tzdb; RFC 9636
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

// Refers to a specific IANA time zone in a specifically versioned dataset.
type TimeZoneId struct {
    // IANA identifier, e.g. Europe/Stockholm.
    Name: string,
    // An explicit IANA tzdb version; no implicit host version is assumed.
    DatabaseVersion: string,
}

// A signed number of seconds east of UTC, subject to validation at use.
type ZoneOffset struct {
    // Signed offset seconds; not itself an IANA time zone.
    SecondsEastOfUTC: int32,
}

// Outcome of mapping a zone-local datetime into offset candidates.
type LocalTimeResolution union {
    // One valid zone offset maps the local datetime to an absolute candidate.
    Unique(ZoneOffset),
    // Multiple valid offsets map the same local datetime to distinct candidates.
    Ambiguous(ZoneOffset[]),
    // No offset maps this local datetime to a valid instant in the named zone.
    Nonexistent,
}

// Errors loading and consulting a versioned zone database.
enum TimeZoneError error {
    // Resolver implementation has not been provided yet.
    NotImplemented,
    // Zone name was not found in the selected data version.
    UnknownZone,
    // The required pinned zone database is unavailable.
    DatasetUnavailable,
    // The required historical or future range is not covered by this dataset.
    OutsideCoverage,
    // The offset cannot be represented or violates supported invariants.
    InvalidOffset,
}

/**
 * Resolves a zone-local datetime into explicit possible UTC offsets.
 * Description:
 *   Never selects earlier/later silently and does not change datetime's
 *   timezone-free definition. Exact Instant representation is deferred.
 * Parameters:
 *   - value: Local wall-clock date and time.
 *   - zone: IANA identifier plus explicitly pinned rules revision.
 * Returns: Unique, Ambiguous or Nonexistent offset resolution.
 * Errors: TimeZoneError.NotImplemented, UnknownZone, DatasetUnavailable,
 *         OutsideCoverage.
 * Ownership: Zone identity is a value; no implicit OS state or network read.
 * Standards: IANA tzdb; RFC 9636 for TZif when applicable.
 */
fn ResolveLocal(value: datetime, zone: TimeZoneId) Result[LocalTimeResolution, TimeZoneError] {
    return Err(TimeZoneError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

## § 4. Open design and compatibility

**§ 4(1)** Exact `Instant` type, leap-second semantics, TZif loading, and greater-than-two candidates require later review. The first representation may be revised before API lock.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** Europe/Stockholm: 2026-10-25 02:30 ambiguous.

**§ 5(2)** Europe/Stockholm: 2026-03-29 02:30 nonexistent.

**§ 5(3)** Fixed offsets must not require tzdb.

**§ 5(4)** Missing tzdb version is an error.

