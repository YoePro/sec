# Sec Standard Library — temporal/calendar/systems

- **Status:** Reserved for future API design; no public declarations
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 0.1
- **Sec version:** 0.1
- **Canonical path:** `stdlib/temporal/calendar/systems/std-temporal-calendar-systems.md`
- **Parent:** `stdlib/temporal/std-temporal.md`
- **Replaces:** None

## § 1. Purpose

**§ 1(1)** Pending algorithm variants and registry.

## § 2. Current contract

**§ 2(1)** No calendar should assume exactly 12 months or identical numbering across systems.

**§ 2(2)** There are presently **no public Sec declarations** in this package. No `.sec` file is generated here until the relevant type semantics and error contracts are defined.


## § 3. Calendar year and month identity

**§ 3(1)** Calendar month identifiers are stable within a calendar system and are independent of their ordinal position in a particular year.

**§ 3(2)** Gregorian and Julian calendars contain twelve months in every year. Leap years change month length, not month count.

**§ 3(3)** The fixed Hebrew calendar uses a 19-year cycle with leap years 3, 6, 8, 11, 14, 17 and 19. Leap years contain thirteen months.

**§ 3(4)** Hebrew month ordering begins with Tishri (M01). Adar I (M05L) precedes Adar II (M06) in leap years.
Ordinary years contain Adar (M06) without M05L.

**§ 3(5)** Islamic tabular calendars contain twelve months. Leap years affect the length of the twelfth month, not the number of months.

**§ 3(6)** This revision limits Hebrew AM and Islamic AH year numbers to positive integers. Gregorian and Julian year numbering is
astronomical, including year zero.

**§ 3(7)** The precise Islamic tabular leap-year variant and epoch are not yet selected.

## § 4. Month identity and ordinal conversion

**§ 4(1)** CalendarMonth.Number represents a stable month identity, not its ordinal position in a particular year.

**§ 4(2)** MonthAt returns the stable identity of a month at a one-based ordinal position.

**§ 4(3)** MonthOrdinal returns the one-based position of a stable month identity in a selected calendar year.

**§ 4(4)** Hebrew leap years insert M05L at position 6.
M06 through M12 consequently occupy positions 7–13.

**§ 4(5)** A month identity not present in the selected year produces CalendarError.InvalidMonth.

**§ 4(6)** For every valid year and ordinal:

MonthOrdinal(year, MonthAt(year, ordinal, system), system) equals ordinal, after successful Result unwrapping.

**§ 4(7)** These operations are deterministic, do not allocate, and require no external calendar database.


## § 5. Calendar month lengths

**§ 5(1)** Month lengths are calculated according to the selected calendar system and year.

**§ 5(2)** Gregorian leap years are divisible by four, except century years not divisible by 400.

**§ 5(3)** Julian leap years are divisible by four.

**§ 5(4)** Hebrew month identities remain stable regardless of leap-year month insertion.

**§ 5(5)** Hebrew Heshvan contains 30 days in years of length 355 or 385, otherwise 29 days.

**§ 5(6)** Hebrew Kislev contains 29 days in years of length 353 or 383, otherwise 30 days.

**§ 5(7)** Hebrew Adar I contains 30 days and is present only during leap years. Ordinary Adar and Adar II contain 29 days.

**§ 5(8)** Islamic tabular calendars alternate 30- and 29-day months. The twelfth month gains one additional day in leap years.

**§ 5(9)** The Islamic tabular leap-year variant remains unresolved. Month 12 must not assume a particular variant before that decision is made.

**§ 5(10)** All calculations must use exact integer arithmetic and must not silently overflow.
