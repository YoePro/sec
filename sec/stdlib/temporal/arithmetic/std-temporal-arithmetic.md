# Sec Standard Library — temporal/arithmetic

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-18
- **Document revision:** 0.2
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/arithmetic/std-temporal-arithmetic.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Calendar period components and explicit calendar arithmetic.

**§ 1(2)** Period steps are calendar-relative; `duration` stays in compiler/core and unit semantics are unchanged. Month-end policy is explicit; component order is explicit.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `arithmetic/period.sec`

```sec
module arithmetic

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/arithmetic/period.sec
 * Module: arithmetic
 * Purpose: Calendar-relative period values and explicit month policies.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/arithmetic/std-temporal-arithmetic.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

// Calendar-relative components are NOT Sec durations and are not fixed seconds.
type CalendarPeriod struct {
    // Calendar years; may have different day counts.
    Years: int,
    // Calendar months; includes any leap-month semantics of the calendar.
    Months: int,
    // Calendar week steps, not a fixed elapsed duration at local times.
    Weeks: int,
    // Calendar day steps, not an elapsed duration.
    Days: int,
}

// Policy for a target day missing from the target month.
enum MonthEndPolicy {
    // Reject the date transformation instead of modifying the target.
    Reject,
    // Use the last valid date in the target month.
    Clamp,
    // Advance excess day numbers into the following month.
    RollForward,
}

// Explicit evaluation order for mixed calendar-period components.
enum PeriodOrder {
    // Apply years, then months, then weeks, then days.
    YearsMonthsWeeksDays,
    // Apply days, then weeks, then months, then years.
    DaysWeeksMonthsYears,
}

// Failures in calendar-relative computations.
enum TemporalArithmeticError error {
    // No implementation exists yet.
    NotImplemented,
    // A target calendar date is not valid under the requested policy.
    InvalidTarget,
    // A result cannot be represented as a core date.
    OutOfRange,
    // An operation cannot be performed in the selected calendar variant.
    UnsupportedSystem,
}
```

### § 2.2. `arithmetic/apply.sec`

```sec
module arithmetic

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/arithmetic/apply.sec
 * Module: arithmetic
 * Purpose: Explicit calendar-period arithmetic.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/arithmetic/std-temporal-arithmetic.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

import "temporal/calendar"

/**
 * Adds months in a specified calendar using an explicit missing-day policy.
 * Description: Stateless; preserving end-of-month INTENT across repeated
 *   recurrences is a separate recurrence-rule concern.
 * Parameters:
 *   - value: Absolute day used as the starting calendar date.
 *   - months: Signed count of calendar month transitions.
 *   - system: Explicit calendar algorithm, not historical civil profile.
 *   - policy: Month-end resolution policy.
 * Returns: Absolute day reached by applying the calendar operation.
 * Errors: TemporalArithmeticError.NotImplemented, InvalidTarget, OutOfRange,
 *         UnsupportedSystem.
 * Ownership: No allocation or I/O.
 */
fn AddMonths(value: date, months: int, system: calendar.CalendarSystem, policy: MonthEndPolicy) Result[date, TemporalArithmeticError] {
    return Err(TemporalArithmeticError.NotImplemented)
}

/**
 * Applies a calendar-relative period using an explicitly chosen order.
 * Description: Weeks/days are calendar steps, not duration nanoseconds.
 * Parameters:
 *   - value: Starting absolute day.
 *   - period: Signed calendar components.
 *   - system: Calendar algorithm.
 *   - policy: Policy for invalid target dates after month/year changes.
 *   - order: Order of evaluating the components.
 * Returns: Resulting absolute day.
 * Errors: TemporalArithmeticError.NotImplemented, InvalidTarget, OutOfRange,
 *         UnsupportedSystem.
 * Ownership: Pure, no I/O or allocation.
 */
fn AddPeriod(value: date, period: CalendarPeriod, system: calendar.CalendarSystem, policy: MonthEndPolicy, order: PeriodOrder) Result[date, TemporalArithmeticError] {
    return Err(TemporalArithmeticError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

**§ 3(3)** `TemporalArithmeticError` (§ 2.1, `arithmetic/period.sec`) is the canonical public error type of `temporal/arithmetic` in Sec 0.1 (MD-083, 2026-10-18). The module must not declare another type named `ArithmeticError`: the compiler-known core `ArithmeticError` keeps its own identity and purpose and is never redeclared, shadowed by a competing public type, or repurposed for temporal-library failures. This decision fixes the name only; the current variants are exactly those declared in § 2.1, and the definition may be extended later through ordinary revision of this book and its source declarations.

## § 4. Open design and compatibility

**§ 4(1)** The final period representation and component-order policy remain reviewable. `PreserveEndOfMonth` must be recurrence-anchor semantics and is deliberately NOT equated with one-shot Clamp.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** 31 Jan + 1 month with Reject/Clamp/RollForward.

**§ 5(2)** DST: 1 calendar day is not 1<days> elapsed.

**§ 5(3)** Leap-month period application.

**§ 5(4)** Overflow into core date bounds.

