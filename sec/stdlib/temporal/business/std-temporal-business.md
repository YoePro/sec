# Sec Standard Library — temporal/business

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/business/std-temporal-business.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Monthly date rules, pinned business calendars, due-date adjustment.

**§ 1(2)** Monthly date rules distinguish N days before the last day from N days before next month. Business-day adjustment is always explicit and financial ledger/payment semantics remain outside temporal.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `business/rules.sec`

```sec
module business

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/business/rules.sec
 * Module: business
 * Purpose: Monthly civil-date rules and business-day conventions.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/business/std-temporal-business.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

// A monthly target date described relative to the chosen Gregorian month.
type MonthlyDateRule union {
    // Select day N of the month; invalid months must be an explicit error.
    DayOfMonth(int),
    // Move N calendar days before the month's final civil day.
    DaysBeforeMonthEnd(int),
    // Move N calendar days before the following month's first civil day.
    DaysBeforeNextMonth(int),
}

// Adjustments for a target date that is not a business day.
enum BusinessDayAdjustment {
    // Keep the computed civil date without checking bank/working-day rules.
    None,
    // Use the next business day.
    Following,
    // Use the preceding business day.
    Preceding,
    // Use next business day unless that changes month; then use preceding.
    ModifiedFollowing,
    // Use preceding business day unless that changes month; then use following.
    ModifiedPreceding,
}

// Refers to explicitly versioned working-day rules; does not own the dataset.
type BusinessCalendar struct {
    // Stable ruleset identifier including business jurisdiction where relevant.
    Identifier: string,
    // Immutable version or effective revision of the business-day data.
    Revision: string,
}

// Failures in monthly rules and business-day adjustment.
enum BusinessError error {
    // Implementation is not available yet.
    NotImplemented,
    // Monthly rule selects a non-existing civil day.
    InvalidDateRule,
    // Ruleset ID is unknown.
    UnknownCalendar,
    // The requested ruleset revision is unavailable.
    DatasetUnavailable,
    // Data does not cover the requested date.
    OutsideCoverage,
    // Day or month exceeds representable bounds.
    OutOfRange,
}
```

### § 2.2. `business/calculate.sec`

```sec
module business

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/business/calculate.sec
 * Module: business
 * Purpose: Invoice-date and business-day date computations.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/business/std-temporal-business.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

/**
 * Computes a monthly civil target date without business-day adjustment.
 * Description: Uses proleptic Gregorian month/year for this first API draft.
 * Parameters:
 *   - year: Gregorian year.
 *   - month: Gregorian month number 1..12.
 *   - rule: Explicit day-of-month or boundary-relative rule.
 * Returns: Date in that month (or boundary-relative result where documented).
 * Errors: BusinessError.NotImplemented, InvalidDateRule, OutOfRange.
 * Ownership: Pure arithmetic, no external data or I/O.
 */
fn ResolveMonthlyDate(year: int, month: int, rule: MonthlyDateRule) Result[date, BusinessError] {
    return Err(BusinessError.NotImplemented)
}

/**
 * Checks a civil date against an explicit pinned working-day calendar.
 * Parameters:
 *   - value: Gregorian civil date.
 *   - rules: Borrowed working-day ruleset reference.
 * Returns: True only if the selected calendar classifies the date as working.
 * Errors: BusinessError.NotImplemented, UnknownCalendar,
 *         DatasetUnavailable, OutsideCoverage.
 * Ownership: Borrows rules descriptor; no hidden current-date query.
 */
fn IsBusinessDay(value: date, rules: ref BusinessCalendar) Result[bool, BusinessError] {
    return Err(BusinessError.NotImplemented)
}

/**
 * Adjusts a civil date to a business day using explicit convention.
 * Parameters:
 *   - value: Unadjusted Gregorian civil date.
 *   - rules: Borrowed, versioned working-day ruleset.
 *   - policy: None, Following, Preceding or modified alternative.
 * Returns: Explicitly adjusted civil date.
 * Errors: BusinessError.NotImplemented, UnknownCalendar,
 *         DatasetUnavailable, OutsideCoverage, OutOfRange.
 * Ownership: No transfer of calendar ownership.
 */
fn AdjustBusinessDay(value: date, rules: ref BusinessCalendar, policy: BusinessDayAdjustment) Result[date, BusinessError] {
    return Err(BusinessError.NotImplemented)
}

/**
 * Resolves a monthly due date and then applies a business-day convention.
 * Parameters:
 *   - year: Target Gregorian year.
 *   - month: Target Gregorian month 1..12.
 *   - rule: Monthly date calculation, e.g. DayOfMonth(27).
 *   - rules: Borrowed pinned working-day calendar.
 *   - policy: Explicit adjustment convention.
 * Returns: Adjusted due date; never silently picks Friday or Monday.
 * Errors: BusinessError.NotImplemented, InvalidDateRule,
 *         UnknownCalendar, DatasetUnavailable, OutsideCoverage, OutOfRange.
 * Ownership: No transfer of ruleset ownership.
 */
fn DueDate(year: int, month: int, rule: MonthlyDateRule, rules: ref BusinessCalendar, policy: BusinessDayAdjustment) Result[date, BusinessError] {
    return Err(BusinessError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

## § 4. Open design and compatibility

**§ 4(1)** First draft applies proleptic Gregorian calendar to the monthly rules. Future financial-calendar variants may add calendar input without changing the correctness of the simple case.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** 27th of each month when date is Saturday.

**§ 5(2)** 3 days before month end vs before next-month start.

**§ 5(3)** ModifiedFollowing must stay in month.

**§ 5(4)** Pinned bank holiday dataset version.

