# Sec Standard Library — `temporal/calendar`

- **Status:** Draft API candidate — declarations only; not implemented, not compiler-verified
- **Created:** 2026-10-16
- **Last updated:** 2026-11-15
- **Document revision:** 0.3
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/temporal/calendar/std-temporal-calendar.md`
- **Repository path:** `sec/stdlib/temporal/calendar/std-temporal-calendar.md`
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`
- **Core authorities:** `rules/types/temporal.md`, `rules/types/units.md`
- **Supersedes candidate:** `temporal/calendar` v0.2 (cumulative, not a main-branch commit)
- **Repository baseline:** `YoePro/sec` `main`, `ee75e5d036d6611519a1b34d570bb6778002acfd`

---

## § 1. Scope and authority

**§ 1(1)** This package owns pure proleptic-calendar coordinates, stable month
identities, validation, absolute-day conversion and partial-date compatibility.
It does not own historic local adoption, locale, timezone, clock acquisition,
uncertain interpretation, ecclesiastical expressions or date parsing.

**§ 1(2)** All public declarations in the three `.sec` files below are
provisional. The code blocks in § 2 are the exhaustive source declaration
contract, including their temporary stub bodies. There are no implied
members, constructors, properties or variants beyond the code shown.

**§ 1(3)** Ordinary functions have bodies in Sec. A stub returns the owning
`CalendarError.NotImplemented` rather than a misleading successful value.
`mut fn` is not used for concrete function declarations.

**§ 1(4)** Any version change must update this book and all affected `.sec`
files together. A calendar-system ID must not silently change conversion
semantics when the implementation is supplied.

## § 2. Canonical API and source declarations

### § 2.1 — `calendar/types.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal/calendar
 * File: calendar/types.sec
 * Purpose: Calendar identity, stable month identity and input coordinates.
 * Status: Draft candidate API v0.2; NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.2
 * Updated: 2026-11-15
 * Rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * References: rules/types/temporal.md; rules/types/units.md;
 *             UTS #35, calendar month codes (conceptual reference).
 */

/**
 * Identifies a deterministic proleptic calendar conversion algorithm.
 *
 * Description:
 *   Does not express national adoption history, religious observance, a
 *   timezone, a location, or the language used for month names. The enum is
 *   a phase-1 catalog, not the full eventual calendar registry.
 * Ownership: Copyable value; no allocation or I/O.
 */
enum CalendarSystem {
    // Proleptic Gregorian calendar, independent of local reform history.
    Gregorian,
    // Proleptic Julian calendar, independent of local reform history.
    Julian,
    // Deterministic Hebrew lunisolar calendar (variant to be specified).
    Hebrew,
    // Arithmetic tabular Islamic calendar, never observational/Umm al-Qura.
    IslamicTabular,
}

/**
 * Identifies a named month consistently across ordinary and leap years.
 *
 * Description:
 *   Number identifies the calendar's stable month code, NOT its ordinal
 *   position within a particular year. IsLeap marks an inserted month.
 *   The pair is interpreted only with an explicit CalendarSystem.
 *   For example, the Hebrew Adar I can use Number=5, IsLeap=true;
 *   Adar II uses Number=6, IsLeap=false (UTS #35 month-code model).
 *   These are calendar coordinates, NOT proof of calendar validity.
 * Ownership: Plain copyable value; no allocation.
 */
type CalendarMonth struct {
    // Stable calendar-specific month-code number; not year-local ordinal.
    Number: int,
    // True for an intercalary instance of the named month.
    IsLeap: bool,
}

/**
 * Contains calendar coordinates that must be validated before interpretation.
 *
 * Description:
 *   Year uses the selected system's own era/year-numbering convention;
 *   do not assume a year zero across all calendars. CalendarFields is an
 *   unvalidated coordinate record, not a guaranteed existing calendar day.
 *   It neither owns nor implies a CalendarSystem, historical profile or zone.
 * Ownership: Plain copyable value; no allocation.
 */
type CalendarFields struct {
    // Calendar-specific year number, subject to the selected system's rules.
    Year: int,
    // Stable month identity; ordinal position is obtained separately.
    Month: CalendarMonth,
    // Day number in the month, subject to calendar-specific validation.
    Day: int,
}

/**
 * Describes recoverable calendar API failures.
 *
 * Description:
 *   The NotImplemented variant is temporary for the stub stage and does
 *   not imply a supported conversion. Other variants specify stable error
 *   categories; precise source positions belong to parse errors, not here.
 */
enum CalendarError error {
    // An API declaration exists, but no implementation is provided yet.
    NotImplemented,
    // The requested year is invalid under the selected calendar's era rules.
    InvalidYear,
    // The month code or intercalary marker is invalid for the calendar/year.
    InvalidMonth,
    // The day is invalid for the chosen calendar/year/month.
    InvalidDay,
    // The represented absolute day/coordinates exceed the supported range.
    OutOfRange,
    // The algorithm or its required data are unavailable on this target.
    UnsupportedSystem,
}
```

### § 2.2 — `calendar/conversion.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal/calendar
 * File: calendar/conversion.sec
 * Purpose: Pure conversion and stable-month/ordinal queries.
 * Status: Draft candidate API v0.2; NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.2
 * Updated: 2026-11-15
 * Rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * References: rules/types/temporal.md; rules/types/units.md;
 *             UTS #35, calendar month codes (conceptual reference).
 */

/**
 * Projects a core absolute day into the selected proleptic calendar.
 *
 * Parameters:
 *   - value: Valid core date; no timezone is inferred.
 *   - system: Algorithm and era convention selected by the caller.
 * Returns: Calendar coordinates for the same absolute day.
 * Errors: NotImplemented, OutOfRange, UnsupportedSystem.
 * Ownership: Pure value operation; no heap or I/O.
 */
fn FromDate(value: date, system: CalendarSystem) Result[CalendarFields, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Resolves a full set of proleptic calendar coordinates to a core day.
 *
 * Parameters:
 *   - fields: Unvalidated coordinates, including stable month identity.
 *   - system: Calendar conversion rules; not historical civil adoption.
 * Returns: Matching core date, when the selected representation is valid.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *         OutOfRange, UnsupportedSystem.
 * Ownership: Pure value operation; no heap or I/O.
 */
fn ToDate(fields: CalendarFields, system: CalendarSystem) Result[date, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Checks that full calendar coordinates represent a valid day.
 *
 * Parameters:
 *   - fields: Calendar coordinates to validate.
 *   - system: Calendar rules used to interpret the fields.
 * Returns: Ok() if the coordinates identify an existing calendar day.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *         OutOfRange, UnsupportedSystem.
 * Ownership: Pure operation; no allocation.
 */
fn ValidateFields(fields: CalendarFields, system: CalendarSystem) Result[void, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Returns the count of months in the selected calendar year.
 *
 * Parameters:
 *   - year: Calendar-native year number.
 *   - system: Selected calendar system.
 * Returns: Positive month count including inserted months.
 * Errors: NotImplemented, InvalidYear, OutOfRange, UnsupportedSystem.
 * Ownership: Pure operation.
 */
fn MonthsInYear(year: int, system: CalendarSystem) Result[int, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Returns the count of valid day numbers in the given month.
 *
 * Parameters:
 *   - year: Calendar-native year number.
 *   - month: Stable month identity, not an ordinal.
 *   - system: Calendar rules.
 * Returns: Number of calendar days in that month.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, OutOfRange,
 *         UnsupportedSystem.
 * Ownership: Pure operation.
 */
fn DaysInMonth(year: int, month: CalendarMonth, system: CalendarSystem) Result[int, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Maps a one-based month position to its stable month identity for a year.
 *
 * Description:
 *   A 13-month year must expose all months, including the inserted month.
 *   The calendar-specific start of the year must be standardized in the
 *   calendar systems child book before this function is implemented.
 * Parameters:
 *   - year: Calendar-native year number.
 *   - ordinal: One-based position within that year's canonical month order.
 *   - system: Explicit calendar rules and month order.
 * Returns: Stable month identifier.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, OutOfRange,
 *         UnsupportedSystem.
 * Ownership: Pure operation.
 */
fn MonthAt(year: int, ordinal: int, system: CalendarSystem) Result[CalendarMonth, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Returns a stable month identifier's one-based position in a chosen year.
 *
 * Parameters:
 *   - year: Calendar-native year number.
 *   - month: Stable month identifier, not an ordinal.
 *   - system: Explicit calendar rules and month order.
 * Returns: One-based ordinal, inverse of MonthAt for a valid month.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, OutOfRange,
 *         UnsupportedSystem.
 * Ownership: Pure operation.
 */
fn MonthOrdinal(year: int, month: CalendarMonth, system: CalendarSystem) Result[int, CalendarError] {
    return Err(CalendarError.NotImplemented)
}
```

### § 2.3 — `calendar/partial.sec`

```sec
module calendar

/*
 * Sec Standard Library — temporal/calendar
 * File: calendar/partial.sec
 * Purpose: Partial calendar coordinates and date compatibility queries.
 * Status: Draft candidate API v0.3; NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.3
 * Updated: 2026-11-15
 * Rulebook: stdlib/temporal/calendar/std-temporal-calendar.md
 * References: rules/types/temporal.md; Library of Congress EDTF;
 *             GEDCOM 7 (comparison and scope references, not full conformance).
 */

/**
 * Stores exactly the calendar coordinates known from an incomplete date.
 *
 * Description:
 *   None denotes a component not supplied, NOT a default value, an
 *   approximate component, or an uncertain interpretation. The year is
 *   expressed using the chosen calendar's year numbering. Month identity
 *   is stable and independent of its ordinal within a given year.
 *   CalendarSystem is supplied separately to the operations below.
 *   Values are unvalidated until ValidatePartial succeeds.
 * Ownership: Copyable value; no allocation.
 * Standards: EDTF reduced-precision dates are related, but a missing year
 *   with known day/month is a Sec-specific extension, not an EDTF claim.
 */
type PartialDate struct {
    // Known calendar year; None if it was not supplied.
    Year: Option[int],
    // Known stable calendar month; None if it was not supplied.
    Month: Option[CalendarMonth],
    // Known day-of-month; None if it was not supplied.
    Day: Option[int],
}

/**
 * Validates that the supplied components could identify at least one date.
 *
 * Description:
 *   Components are tested together under the explicit calendar system.
 *   A missing component remains absent. For example, 29 February without
 *   a year may be valid, but 29 February 1900 is not Gregorian-valid.
 *   Year-only, month/year, month/day and wholly unspecified values are
 *   representable. A wholly unspecified value means no known components,
 *   not a known calendar date.
 *   Success means that at least one date in the supported core date range
 *   matches, not that one unique day has been identified.
 * Parameters:
 *   - value: Unvalidated partial coordinates; borrowed/copyable by value.
 *   - system: Calendar algorithm and its year/month conventions.
 * Returns: Ok() when the components admit at least one representable date.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *   OutOfRange, UnsupportedSystem.
 * Ownership: Pure, no heap allocation or I/O.
 */
fn ValidatePartial(value: PartialDate, system: CalendarSystem) Result[void, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Tests whether a known core date satisfies every present component.
 *
 * Description:
 *   The candidate is converted into calendar coordinates under system.
 *   Any missing component is a wildcard, not a default. The partial value
 *   must first satisfy the same validity requirements as ValidatePartial;
 *   an invalid partial input is an error, not simply a false match.
 * Parameters:
 *   - value: Partial date to test.
 *   - candidate: One fully known core absolute day.
 *   - system: Explicit proleptic calendar; no historical adoption rules.
 * Returns: true exactly when all supplied components match candidate.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *   OutOfRange, UnsupportedSystem.
 * Ownership: Pure, no allocation or I/O.
 */
fn MatchesDate(value: PartialDate, candidate: date, system: CalendarSystem) Result[bool, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Tests whether two partial descriptions admit at least one common day.
 *
 * Description:
 *   The result describes date compatibility only, NOT proof that two
 *   historical events are the same event. The comparison is symmetric and
 *   is limited to dates representable by core date and the selected system.
 *   Missing components are constraints left open, never filled with the
 *   host's current date. Invalid partial inputs produce an error.
 *   This is not a generic comparison of approximate/uncertain dates.
 * Parameters:
 *   - left: First partial description.
 *   - right: Second partial description.
 *   - system: One explicit calendar system used for both descriptions.
 * Returns: true if a representable core date satisfies both descriptions.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *   OutOfRange, UnsupportedSystem.
 * Ownership: Pure; no allocation or I/O.
 */
fn MayDescribeSameDate(left: PartialDate, right: PartialDate, system: CalendarSystem) Result[bool, CalendarError] {
    return Err(CalendarError.NotImplemented)
}

/**
 * Converts a fully specified partial date to its unique core absolute day.
 *
 * Description:
 *   Returns None if any of Year, Month or Day is absent. Never fills in
 *   the current year, day 1 or another invented default. All known
 *   coordinates must be valid, even when the result is None.
 *   Historical civil labels must instead use temporal/calendar/historical.
 * Parameters:
 *   - value: Partial calendar coordinates.
 *   - system: Explicit proleptic calendar algorithm.
 * Returns: Some(date) when complete and valid; otherwise None for an
 *   incomplete but valid partial description.
 * Errors: NotImplemented, InvalidYear, InvalidMonth, InvalidDay,
 *   OutOfRange, UnsupportedSystem.
 * Ownership: Pure; no allocation or I/O.
 */
fn ToDateIfComplete(value: PartialDate, system: CalendarSystem) Result[Option[date], CalendarError] {
    return Err(CalendarError.NotImplemented)
}
```

## § 3. Stable calendar identity and year-local position

**§ 3(1)** `CalendarMonth.Number` is a stable calendar-specific month-code
number, **not** the month ordinal in that year. `IsLeap` distinguishes an
intercalary instance. Both fields are interpreted only with an explicit
`CalendarSystem`.

**§ 3(2)** `MonthAt(year, ordinal, system)` and
`MonthOrdinal(year, month, system)` are inverse for valid months. Ordinal
values are one-based. An invalid code or ordinal returns `InvalidMonth`.
The exact Hebrew civil/scriptural ordering and tabular-Islamic algorithm/epoch
are **not yet locked**; the systems child book must specify them before
conversion implementation is permitted.

**§ 3(3)** `CalendarFields` holds full coordinates without asserting validity.
All conversion operations use the core `date` identity as the neutral absolute
calendar day, not as local time or an instant.

## § 4. Partial-date contract

**§ 4(1)** `PartialDate` stores exactly three independent optional components.
None means **not supplied**. Missing precision and uncertain/approximate
claims are separate concepts. This type does not encode "about 1852", "before
1852", or a source's transcription/provenance.

**§ 4(2)** A partial value is not automatically a single day or one continuous
range. `18 May` may identify many dates; `May 1852` identifies a subset of one
year. Components without a year remain potentially valid if they occur in at
least one supported year. In particular, 29 February is potentially valid in
Gregorian without a specified year.

**§ 4(3)** `ValidatePartial` checks existence of at least one date in the
representable core-date domain. `MatchesDate` checks each present component
against one fully known day. `MayDescribeSameDate` expresses existence of at
least one day satisfying **both** descriptions. It does **not** establish that
recorded events are identical.

**§ 4(4)** `ToDateIfComplete` returns `Ok(None)` only for a **valid but
incomplete** partial value; when all components are present and valid it
returns `Ok(Some(date))`. Invalid fields must produce a typed error. Stub
implementations return `Err(CalendarError.NotImplemented)` until implemented.

**§ 4(5)** All these operations interpret one explicit `CalendarSystem`.
Cross-system comparisons must first have an independently resolved absolute
day, or use a separately defined future cross-system API. Historical skipped,
repeated and indeterminate civil dates are not silently resolved here.

**§ 4(6)** A fully unspecified `PartialDate` (all fields None) is permitted as
an unknown description; it does not mean the same thing as a known core `date`.
As long as a selected supported calendar has at least one representable date,
validation succeeds and it matches any candidate day in range.

## § 5. Validation and error semantics

**§ 5(1)** `InvalidYear`, `InvalidMonth` and `InvalidDay` signal impossible or
invalid supplied components; `OutOfRange` addresses core representation limits;
`UnsupportedSystem` reports an unavailable algorithm or variant.
The provisional `NotImplemented` variant is strictly scaffold-only.

**§ 5(2)** No malformed partial value is converted to a successful false
match. No operation silently substitutes today, the first of the month,
January, or a host-specific calendar.

**§ 5(3)** All package operations are pure and do not consult system locale,
clock, timezone, filesystem, network or historical datasets. No heap allocation
is needed by the contracted success paths in this revision.

## § 6. Required conformance scenarios once implemented

| ID | Input and system | Required semantic outcome |
|---|---|---|
| CAL-001 | 1970-01-01 Gregorian -> `date` -> Gregorian | Round-trip identity |
| CAL-002 | Gregorian 2024-02-29 | Valid full date |
| CAL-003 | Gregorian 1900-02-29 | `InvalidDay` |
| CAL-004 | Gregorian 2000-02-29 | Valid full date |
| CAL-005 | Hebrew leap-year intercalary month | Valid stable month identity, distinct ordinal |
| CAL-006 | Gregorian partial year-only `1852` | Valid; matches 1852 dates, not 1853 |
| CAL-007 | Gregorian partial `May 1852` | Valid; matches any May 1852 day |
| CAL-008 | Gregorian partial `18 May` | Valid; matches May 18 in multiple years |
| CAL-009 | `1852`, `May 1852`, `18 May` | Every pair may describe same day |
| CAL-010 | `May 1852` vs `June 1852` | No common date |
| CAL-011 | `29 Feb` with no year | Valid, since leap years exist |
| CAL-012 | `29 Feb 1900` Gregorian | `InvalidDay` |
| CAL-013 | `31 Feb` with no year | `InvalidDay` |
| CAL-014 | Complete valid partial | `ToDateIfComplete` returns `Some(date)` |
| CAL-015 | Incomplete valid partial | `ToDateIfComplete` returns `None` |
| CAL-016 | Invalid partial compared to valid date | Error, not `Ok(false)` |
| CAL-017 | All components absent | Valid unknown description; compatible with dates |
| CAL-018 | Gregorian year zero/negative years | Follow final calendar-system year convention |
| CAL-019 | Partial with a leap month valid only in relevant years | Validate against year-aware identities |
| CAL-020 | Repeated/skipped historical civil labels | Routed to `calendar/historical`, not this API |

## § 7. Standards, dependencies and deferred design

**§ 7(1)** Relevant specifications and comparisons:

- Sec `rules/types/temporal.md` and `rules/types/units.md` (normative for core).
- Unicode UTS #35, Date Field Symbol Table / month codes:
  https://www.unicode.org/reports/tr35/tr35-dates.html
- Library of Congress EDTF (reduced precision, uncertainty, open intervals):
  https://www.loc.gov/standards/datetime/
- FamilySearch GEDCOM 7 standard (partial and qualified genealogical dates):
  https://gedcom.io/specifications/FamilySearchGEDCOMv7.html

**§ 7(2)** EDTF supports a reduced-precision year or year/month. It is a
reference for some use cases, **not** a claim that Sec `PartialDate` is an
EDTF-conformant serialization or that EDTF defines a standalone day/month with
no year. GEDCOM's syntax also does not by itself define this Sec in-memory API.
Parsing and serialization remain in `temporal/parse` / `temporal/format`.

**§ 7(3)** Deferred: calendar year/era contracts; Hebrew month ordering;
Islamic tabular variants and epochs; systems with observational uncertainty;
partial-date enumeration over bounded ranges; uncertain dates; liturgical
expressions; historical-adoption rules. Each must have its own child contract
before implementation, without changing the meaning of a published identifier.

**§ 7(4)** This candidate has been checked for internal consistency and
source/book synchronization, but **has not been compiled with Sec**.
