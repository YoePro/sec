# Sec Standard Library — temporal/calendar

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/calendar/historical/std-temporal-calendar-historical.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Historical civil-calendar adoption and skipped/repeated date labels.

**§ 1(2)** Historical interpretation requires a pinned jurisdiction profile, independent from IANA timezone information. Distinguish one, several, zero and indeterminate candidates.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `calendar/historical/historical.sec`

```sec
module historical

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/calendar/historical/historical.sec
 * Module: historical
 * Purpose: Jurisdiction-specific historical civil label resolution.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/calendar/historical/std-temporal-calendar-historical.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

import "temporal/calendar"

// Identifies a pinned historical rule profile; it does not load or own data.
type HistoricalProfile struct {
    // Profile ID such as a registered Swedish civil adoption profile.
    Identifier: string,
    // Exact version of the adopted historical-rule dataset.
    Revision: string,
}

// Result of interpreting a local historical date label.
// Multiple includes all distinct absolute-day candidates; never silently picks one.
type HistoricalResolution union {
    // Exactly one matching absolute day exists.
    Unique(date),
    // More than one matching absolute day exists under the selected profile.
    Multiple(date[]),
    // The named civil date did not occur under the selected rules.
    Nonexistent,
    // Available historical evidence does not justify one determined result.
    Indeterminate,
}

// Failures to retrieve or apply a historical rule profile.
enum HistoricalError error {
    // No implementation exists for the requested operation yet.
    NotImplemented,
    // The identifier has no registered rules.
    UnknownProfile,
    // The requested pinned dataset is unavailable.
    DatasetUnavailable,
    // The dataset does not establish rules for the supplied period.
    OutsideCoverage,
    // The civil-label fields are malformed for the profile's general syntax.
    InvalidLabel,
}

/**
 * Resolves a historical civil date label to zero, one or several days.
 * Description:
 *   Unlike a proleptic-calendar conversion, respects skipped/repeated
 *   historical civil labels (e.g. Swedish 1753 reform).
 * Parameters:
 *   - fields: Civil year/month/day as historically recorded.
 *   - profile: Borrowed pinned jurisdiction-specific rule profile.
 * Returns: Explicit uniqueness, repetition, absence or indeterminate outcome.
 * Errors: HistoricalError.NotImplemented, UnknownProfile,
 *         DatasetUnavailable, OutsideCoverage, InvalidLabel.
 * Ownership: No profile ownership transfer; Multiple may allocate.
 */
fn ResolveCivilDate(fields: calendar.CalendarFields, profile: ref HistoricalProfile) Result[HistoricalResolution, HistoricalError] {
    return Err(HistoricalError.NotImplemented)
}

/**
 * Gets the label of the next physical day under a historical civil profile.
 * Description:
 *   For Swedish 1753-02-17 this is 1753-03-01. Input is an absolute
 *   day so repeated/skipped civil labels remain representable.
 * Parameters:
 *   - value: Known absolute day immediately before the requested next day.
 *   - profile: Borrowed, pinned jurisdiction profile.
 * Returns: Civil label in the profile's calendar for the following day.
 * Errors: HistoricalError.NotImplemented, UnknownProfile,
 *         DatasetUnavailable, OutsideCoverage.
 * Ownership: Pure once the named profile data are provided.
 */
fn NextCivilDate(value: date, profile: ref HistoricalProfile) Result[calendar.CalendarFields, HistoricalError] {
    return Err(HistoricalError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

## § 4. Open design and compatibility

**§ 4(1)** Profiles, data schema and geopolitical coverage are NOT committed by this draft. Source provenance is mandatory when real data become available.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** Sweden: 1712-02-30.

**§ 5(2)** Sweden: 1753-02-17 followed by 1753-03-01.

**§ 5(3)** Samoa: duplicate 1892-07-04.

**§ 5(4)** Samoa: absent 2011-12-30.

