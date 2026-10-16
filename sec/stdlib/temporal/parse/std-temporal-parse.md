# Sec Standard Library — temporal/parse

- **Status:** Draft API scaffold — incomplete implementation; signatures provisional
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/parse/std-temporal-parse.md`
- **Replaces:** None
- **Parent rulebook:** `stdlib/temporal/std-temporal.md`

## § 1. Purpose and scope

**§ 1(1)** Strict temporal parsing, including partial dates.

**§ 1(2)** ISO local datetime parsing never accepts or invents offsets. Partial parsing keeps the exact input components; language selection is explicit. Core unit syntax is never reparsed as Sec source by this API.

**§ 1(3)** All functions in this book currently fail explicitly with `NotImplemented`; no successful runtime functionality, compiler validation or conformance is asserted.

## § 2. Source-declaration authority

**§ 2(1)** The complete declarations below (including every public type, enum/union variant, public field, function and their stub bodies) define this provisional API. Comments immediately precede their declarations. The corresponding `.sec` source files must be updated whenever this book changes.

### § 2.1. `parse/parse.sec`

```sec
module parse

/*
 * Sec Standard Library — temporal API scaffold
 * Canonical path: stdlib/temporal/parse/parse.sec
 * Module: parse
 * Purpose: Strict parsing into core or calendar types without timezone guesses.
 * Status: Draft API, NOT IMPLEMENTED, not compiler-verified
 * Revision: 0.1
 * Updated: 2026-10-16
 * Governing rulebook: stdlib/temporal/parse/std-temporal-parse.md
 * Standards / dependencies: Sec std-temporal.md
 * NOTE: Stub results are explicit NotImplemented failures. No successful
 *       behavior is claimed by this file.
 */

import "temporal/calendar"

// Parsing failures; NotImplemented is temporary and not a valid date outcome.
enum ParseError error {
    // No parser implementation exists yet.
    NotImplemented,
    // The input is empty or contains invalid syntax for the selected grammar.
    InvalidSyntax,
    // Calendar components are invalid or mutually inconsistent.
    InvalidComponent,
    // The input includes a timezone/offset not admitted by this parser.
    UnexpectedOffset,
    // The calendar result cannot be represented by the destination type.
    OutOfRange,
    // Required locale/grammar support is not available.
    UnsupportedLanguage,
}

/**
 * Parses a Gregorian ISO 8601 full calendar date without a timezone.
 * Parameters:
 *   - source: Date input in the explicitly supported ISO 8601 profile.
 * Returns: Core proleptic-Gregorian date.
 * Errors: ParseError.NotImplemented, InvalidSyntax, InvalidComponent, OutOfRange.
 * Ownership: Source borrowed; no implicit locale or timezone.
 * Standards: ISO 8601-1 (selected full calendar date profile).
 */
fn ParseISODate(source: string) Result[date, ParseError] {
    return Err(ParseError.NotImplemented)
}

/**
 * Parses an ISO local datetime; offset/zone suffixes are not accepted.
 * Parameters:
 *   - source: Complete local date and clock-time representation.
 * Returns: Unzoned core datetime.
 * Errors: ParseError.NotImplemented, InvalidSyntax, InvalidComponent,
 *         UnexpectedOffset, OutOfRange.
 * Ownership: No clock read or timezone inference.
 * Standards: ISO 8601-1 (selected local datetime profile).
 */
fn ParseISOLocalDateTime(source: string) Result[datetime, ParseError] {
    return Err(ParseError.NotImplemented)
}

/**
 * Parses a partial date without filling missing components.
 * Description:
 *   Supports a selected language (e.g. "sv" for Maj 1852 and 18 Maj)
 *   once its grammar/data are available; no current-host-locale fallback.
 * Parameters:
 *   - source: Source expression, preserved separately by the caller if needed.
 *   - language: Explicit BCP 47 language tag.
 *   - system: Calendar for interpreting named months and components.
 * Returns: Exact known/missing fields only; no invented year/day.
 * Errors: ParseError.NotImplemented, InvalidSyntax, InvalidComponent,
 *         UnsupportedLanguage.
 * Ownership: May use explicit language data; no hidden clock access.
 */
fn ParsePartialDate(source: string, language: string, system: calendar.CalendarSystem) Result[calendar.PartialDate, ParseError] {
    return Err(ParseError.NotImplemented)
}
```

## § 3. Errors and ownership

**§ 3(1)** Success paths are unimplemented. For now every fallible callable returns its owning module's `NotImplemented` error, never `Ok` with a fabricated default.

**§ 3(2)** Inputs and outputs carry the ownership contracts declared in source comments. Dataset-backed operations must never implicitly fetch data or consult host settings.

## § 4. Open design and compatibility

**§ 4(1)** RFC 3339 parsing with offset must await exact `Instant`/offset contract. Liturgical, approximate and uncertain expressions need additional dedicated grammar and model.

**§ 4(2)** Public API is not locked. No dependent module may claim these signatures are finalized or implemented. Any revision retains paragraph identities where possible.

## § 5. Initial verification scenarios

**§ 5(1)** 2026-10-16 full date.

**§ 5(2)** Reject timezone suffix for local datetime.

**§ 5(3)** Maj 1852; 18 Maj; 1852.

**§ 5(4)** Do not use system locale as implicit default.

