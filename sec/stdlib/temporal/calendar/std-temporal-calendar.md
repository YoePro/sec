# Sec Standard Library — temporal/calendar

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/calendar/std-temporal-calendar.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Calendar systems, coordinate conversion and partial dates.

**§ 1(2)** Proleptic algorithms are distinct from jurisdictional historical adoption. `CalendarMonth` permits leap months. `PartialDate` stores known/missing components without inventing missing information.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `calendar/types.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/calendar/types.sec
 * Module: calendar
 * Purpose: Named calendar systems and calendar-coordinate values.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

// Identifies the calendar algorithm used for converting a serial day.
// The supported set is intentionally preliminary and may grow.
enum CalendarSystem {
    // Proleptic Gregorian calendar, independent of historical adoption.
    Gregorian,
    // Proleptic Julian calendar, independent of historical adoption.
    Julian,
    // Algorithmic Hebrew lunisolar calendar; exact era rules require a child revision.
    Hebrew,
    // Tabular Islamic calendar; never denotes observational or Umm al-Qura rules.
    IslamicTabular,
}

// Identifies a month in calendars that may contain intercalary months.
type CalendarMonth struct {
    // Ordinal month identifier in the selected calendar's own system.
    Number: int,
    // True when this is a calendar-specific inserted/leap month.
    IsLeap: bool,
}

// Holds full calendar coordinates WITHOUT asserting their validity.
// Call ValidateFields or ToDate before relying on the fields as a date.
type CalendarFields struct {
    // Calendar-specific year number, not assumed to use the Gregorian era.
    Year: int,
    // Calendar-specific month identity, including possible leap-month status.
    Month: CalendarMonth,
    // Day number within the represented calendar month.
    Day: int,
}

// Calendar operation failures; NotImplemented is temporary scaffolding.
enum CalendarError error {
    // The operation has no implementation yet; NOT a successful calendar result.
    NotImplemented,
    // The month identifier is invalid in the selected calendar/year.
    InvalidMonth,
    // The day number is invalid in the selected calendar/month.
    InvalidDay,
    // A requested year or serial day exceeds supported representational limits.
    OutOfRange,
    // The selected algorithm or variant is not available on this target.
    UnsupportedSystem,
}
```

### § 2.2. `calendar/conversion.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/calendar/conversion.sec
 * Module: calendar
 * Purpose: Stateless conversions between core date and calendar coordinates.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

/**
 * Projects an absolute core date onto the selected calendar system.
 *
 * Description:
 *   Does not use timezones, historical adoption profiles or locale.
 * Parameters:
 *   - value: Unzoned absolute core day.
 *   - system: Explicit calendar system.
 * Returns:
 *   Calendar-specific year, month and day; no owning heap allocation required.
 * Errors:
 *   - CalendarError.NotImplemented: Scaffold only.
 *   - CalendarError.OutOfRange: Calendar coordinates cannot be represented.
 * Ownership: Pure value conversion, no I/O.
 * Standards: Calendar-specific algorithms; see child rulebook.
 */
fn FromDate(value: date, system: CalendarSystem) Result[CalendarFields, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Converts validated calendar coordinates into a core absolute day.
 * Description: Never assumes historical local civil adoption or timezone.
 * Parameters:
 *   - fields: Calendar coordinate tuple.
 *   - system: Explicit calendar system.
 * Returns: Core absolute day matching the selected calendar system.
 * Errors:
 *   - CalendarError.NotImplemented: Scaffold only.
 *   - CalendarError.InvalidMonth: Illegal calendar-month identity.
 *   - CalendarError.InvalidDay: Illegal day within the month.
 *   - CalendarError.OutOfRange: Result not representable by core date.
 * Ownership: Pure value conversion; no I/O or string allocation.
 */
fn ToDate(fields: CalendarFields, system: CalendarSystem) Result[date, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Validates a full coordinate tuple against the selected calendar.
 * Parameters:
 *   - fields: Calendar coordinate tuple.
 *   - system: Explicit calendar system.
 * Returns: Ok(void) only when all components are meaningful together.
 * Errors: CalendarError.NotImplemented, InvalidMonth, InvalidDay, OutOfRange.
 * Ownership: Pure operation.
 */
fn ValidateFields(fields: CalendarFields, system: CalendarSystem) Result[void, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Returns the number of months in the chosen year.
 * Parameters:
 *   - year: Year in the selected calendar's numbering system.
 *   - system: Calendar system.
 * Returns: Count including inserted months, when present.
 * Errors: CalendarError.NotImplemented, OutOfRange, UnsupportedSystem.
 * Ownership: Pure operation.
 */
fn MonthsInYear(year: int, system: CalendarSystem) Result[int, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Returns valid days in a specific calendar month of a given year.
 * Parameters:
 *   - year: Calendar-specific year.
 *   - month: Month with explicit leap-month flag.
 *   - system: Calendar system.
 * Returns: Day count.
 * Errors: CalendarError.NotImplemented, InvalidMonth, OutOfRange.
 * Ownership: Pure operation.
 */
fn DaysInMonth(year: int, month: CalendarMonth, system: CalendarSystem) Result[int, CalendarError] {
    return Err(CalendarError.NotImplemented)
}
```

### § 2.3. `calendar/partial.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/calendar/partial.sec
 * Module: calendar
 * Purpose: Partial components without invented missing values.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

// Partial calendar coordinates. None denotes a truly absent component.
// A partial date is NOT necessarily approximate or historically uncertain.
type PartialDate struct {
    // Known year or None when the original expression omitted a year.
    Year: Option[int],
    // Known calendar-month identity or None when it was not specified.
    Month: Option[CalendarMonth],
    // Known day-of-month or None when the expression did not specify one.
    Day: Option[int],
}

/**
 * Checks whether present components can coexist in the chosen calendar.
 * Description:
 *   Does NOT insert missing components or resolve uncertainty. Missing
 *   year means e.g. 29 February may remain conditionally possible.
 * Parameters:
 *   - value: Exact known/missing component representation.
 *   - system: Calendar system for constraints.
 * Returns: Ok(void) for a possible partial date, not a unique full date.
 * Errors: CalendarError.NotImplemented, InvalidMonth, InvalidDay, OutOfRange.
 * Ownership: Pure, no allocation.
 */
fn ValidatePartial(value: PartialDate, system: CalendarSystem) Result[void, CalendarError] {
    return Err(CalendarError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

## § 4. Open design and compatibility

**§ 4(1)** Gregorian, Julian, Hebrew and tabular Islamic names are a first candidate set, not a claim that algorithms exist. ISO-week is not forced into Y/M/D coordinates. Variant/era semantics and normalization remain to be refined.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** 1970-01-01 round-trip Gregorian.

**§ 5(2)** 29 February and invalid day.

**§ 5(3)** Hebrew leap month.

**§ 5(4)** 1852 and Maj 1852 may remain partial.

