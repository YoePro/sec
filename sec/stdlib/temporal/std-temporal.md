# Sec Standard Library — `temporal`

- **Status:** Draft — foundational architecture and cross-package semantic contract; not a complete public-API specification
- **Created:** 2026-10-16
- **Last updated:** 2026-10-18
- **Document revision:** 0.2
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/temporal/std-temporal.md`
- **Repository path:** `sec/stdlib/temporal/std-temporal.md`
- **Parent specification:** `rules/library/stdlib.md` (to be synchronized; see § 2)
- **Primary core authority:** `rules/types/temporal.md`, `rules/types/units.md`
- **Implementation-governance target:** existing governance records where applicable; a dedicated `implementation-status-std-temporal.yaml` only when needed and reconciled with those records
- **Replaces:** None — initial foundation
- **Repository sources reviewed:** `main` content as exposed on 2026-10-16; exact HEAD commit identity not verified
- **Specification scope:** Architectural obligations and package ownership only. Child rulebooks must enumerate all of their public declarations exhaustively.

---

## § 1. Purpose, status and authority

**§ 1(1)** `temporal` provides advanced temporal value processing above Sec's compiler/core temporal types: calendars, civil-time rules, parsing, formatting, time-zone conversions, calendar-relative arithmetic, historical interpretation and reusable date rules.

**§ 1(2)** This is the **parent book** for `stdlib/temporal`. It fixes domain boundaries, shared invariants, inter-package expectations, test obligations and standards dependencies. It deliberately does **not** declare new public types or functions. An identifier mentioned as a *candidate* is not a published API, and no `.sec` stub may be generated from candidate names alone.

**§ 1(3)** A child book becomes an implementation specification only when it provides the complete Sec declarations of every public type, enum member, union variant, property, function, constructor and destructor belonging to that child. No unspecified members or pseudo-constructors are permitted.

**§ 1(4)** This book does not change the language semantics of `date`, `time`, `datetime`, `duration` or unit-bearing literals. `rules/types/temporal.md` and `rules/types/units.md` are authoritative for those fundamentals. Discrepancies discovered in existing implementation files are implementation/backlog work, not permission for the stdlib to redefine core.

**§ 1(5)** Unresolved design alternatives in this revision are explicitly marked **Deferred**; they are not silently chosen by implementers. This book may be refined without renumbering existing paragraph identifiers. Add new numbered paragraphs or amend text in place; never repurpose a number for an unrelated subject.

---

## § 2. Layering and package naming

**§ 2(1)** The compiler owns unit syntax, temporal identities and compiler intrinsics. Trusted `sec/core` owns the mandatory public core operations for those identities. `stdlib/temporal` owns advanced reusable algorithms, policies and data-driven facilities.

**§ 2(2)** The stdlib area is named **`temporal`**, not `time`. `time` already denotes a compiler-known core type, and `datetime` is too narrow for the proposed domain.

**§ 2(3)** `rules/library/stdlib.md` currently contains a legacy section headed `time`. That section must be synchronized to the selected `temporal` organization and the responsibility split here; it is **not** an instruction to introduce a second competing `time` library.

**§ 2(4)** The stdlib must not shadow, redeclare, or attach ordinary `impl` blocks to compiler-known core temporal types. Core extensions, where justified, require their own canonical core/language specification changes.

**§ 2(5)** OS- and hardware-specific primitives reside in `sec/platform`, the relevant `os` or hardware module, or compiler/core as appropriate. `temporal` consumes their stable abstractions; it does not directly implement syscalls, scheduler internals or real-time network protocols.

**§ 2(6)** Dependencies must be explicit and acyclic. Pure calendar algorithms must be usable without OS time, heap allocation or IANA data. Data-intensive functionality may have independently declared target requirements.

---

## § 3. Non-negotiable semantic principles

**§ 3(1)** Every operation must identify which **domain** it uses: fixed elapsed time, absolute calendar day, local civil datetime, zoned instant, calendar-relative period, historical civil date, partial date or recurrence.

**§ 3(2)** No timezone, offset, geographical location, locale, calendar or current clock reading is inferred from an otherwise context-free `date`, `time` or `datetime` value.

**§ 3(3)** A `datetime` contains a date and a time of day, **not** a timezone or place. The core `datetime.Now` operation reads UTC by its specified acquisition contract; that provenance does not add an embedded UTC tag to the `datetime` value itself.

**§ 3(4)** A failed or ambiguous conversion is never silently replaced with a guessed successful result. Convenience policies may exist, but their selection must be explicit and documented.

**§ 3(5)** Fixed elapsed time and calendar-relative movement are different operations. Months and years do not have universal fixed lengths. A local civil day is not guaranteed to equal 24 elapsed hours.

**§ 3(6)** Representable mathematical operations must not suffer intermediate overflow; unrepresentable results must not wrap, silently clamp, saturate or lose precision. Use ordinary Sec checked operations and explicit errors.

**§ 3(7)** Algorithms with no data dependency must remain deterministic. Operations that depend on external data (tzdb, historical calendars, holidays, locale, leap seconds, ephemerides) must make their source/version and failure modes inspectable.

**§ 3(8)** Expensive work, hidden allocation, implicit I/O and implicit clock reads are forbidden from apparently pure conversions and arithmetic operations. Costs and resource ownership are part of the public contract.

---

## § 4. Existing core contract and units

**§ 4(1)** This book relies on the following established compiler-known types, without defining replacements:

| Core type | Phase-1 representation | Meaning |
|---|---|---|
| `date` | signed `int32` epoch-day count from 1970-01-01 | One day on the canonical proleptic-Gregorian day axis |
| `time` | `uint64` nanoseconds since midnight | Time of day, without offset or zone |
| `datetime` | epoch day plus nanoseconds since midnight | Civil date and time of day, unzoned |
| `duration` | signed `int64` nanoseconds | Fixed elapsed time |

**§ 4(2)** These are nominal temporal types, not numeric aliases. Storage fields are private trusted-core detail; stdlib algorithms must use public core interfaces when those are specified and implemented.

**§ 4(3)** Sec's general unit grammar admits quantity literals including `5<s>` and `500<ms>`. The `duration` conversion follows `rules/types/temporal.md` § 5 and the MD-015 corrections: an appropriately typed **literal** can be contextually materialized where a `duration` is expected; an already-bound unit-bearing quantity requires an explicit `duration(...)` conversion.

**§ 4(4)** Longer fixed units, including the intended spellings `days` and `week`, use the **same** general unit resolution and dimension rules. The unit catalog must define their names, linear scales and availability; this book does not assert that those spellings are among the four compiler-core units `s`, `ms`, `us`, `ns` already specified by the core rulebook. No duration-only lexer/parser rule is allowed.

**§ 4(5)** The following are intended **unit-system usage examples**, contingent on the identifiers resolving to linear time units with the stated scales; they are not declarations of a temporal public API:

```sec
let delay: duration := 500<ms>
let weekSpan: duration := 1<week>
// A function with parameter `d: duration` accepts the literal 5<days>
// when the `days` unit is available and exact conversion is representable.
```

**§ 4(6)** A fixed `1<week>` means exactly seven fixed 24-hour durations under its unit declaration. It does not mean “move to the same weekday in a local calendar while preserving the wall-clock hour.” The two concepts must remain separate.

**§ 4(7)** The `duration` carrier has a finite domain (signed 64-bit nanoseconds). Extra-wide astronomical or scientific timescales, if needed, require separate explicitly specified representations, not an undocumented enlargement of `duration`.

**§ 4(8)** Core currently owns `datetime.Now`, `date.Today` and `time.Now` as separate UTC wall-clock reads under the target's `UTCWallClock` capability. They do not establish monotonic time, synchronization accuracy, an injected clock or same-instant behavior across separate reads.

---

## § 5. Fundamental domain distinctions

**§ 5(1)** **Absolute day:** a serial day identity; usable as a neutral bridge between deterministic calendar systems. `date` has this underlying role and its ordinary component interpretation is proleptic Gregorian. A calendar rendering must not alter the absolute day.

**§ 5(2)** **Calendar representation:** a named calendar system and year/month/day identity, with calendar-specific month identities and validation. Candidate concept: `CalendarDate`. It does not replace `date`.

**§ 5(3)** **Historical civil interpretation:** a calendar system **plus a jurisdiction, effective-date rules and a selected dataset**. A historical civil date can be missing or repeated even if the corresponding absolute-day sequence is continuous.

**§ 5(4)** **Local datetime:** date plus clock time, lacking an offset. It does not uniquely identify a point on a global timescale.

**§ 5(5)** **Instant:** a unique point on a clearly specified time axis. Candidate concept: `Instant`. Its precise epoch, scale, supported range and leap-second behavior are deferred to `timescale`/core design; do not assume that an integer Unix timestamp alone settles these questions.

**§ 5(6)** **Zoned representation:** an instant projected through a specified named timezone and calendar. Candidate concepts `TimeZone`, `ZoneOffset`, `ZonedDateTime` and `OffsetDateTime` must be assessed for duplication and responsibility before their API is fixed.

**§ 5(7)** **Calendar period:** a movement expressed through year/month/week/day calendar components, interpreted relative to a specified start, calendar and policy. Candidate concept: `CalendarPeriod`; it is not interchangeable with `duration`.

**§ 5(8)** **Partial/uncertain expression:** an incomplete, approximate, bounded or rule-based temporal description. It is not an invalid `date` and must not silently become an arbitrary complete date.

---

## § 6. Calendars and calendar conversion

**§ 6(1)** The calendar engine must distinguish a **calendar system** (mathematical/rule-based mapping) from a **historical adoption profile** (where and when a civil authority used rules).

**§ 6(2)** Calendar conversion must preserve the absolute day wherever source interpretation uniquely resolves. It must make calendar identity, applicable variant and any historical profile explicit.

**§ 6(3)** Initial design coverage shall include proleptic Gregorian, Julian, ISO-week and Hebrew calendar capabilities. The architecture must accommodate Islamic variants, Persian, Chinese and other lunar/lunisolar calendars without assuming 12 uniquely numbered months per year.

**§ 6(4)** Calendar month identity must support leap/inserted months; years may have different month counts, month naming conventions, eras, year starts or non-midnight day boundaries. No generic API may rely on `month in 1..12` across all calendars.

**§ 6(5)** A single culturally named calendar may have multiple legitimate computational variants. For example, tabular Islamic, Umm al-Qura and observation-based Islamic dates are not globally interchangeable. Variant, geographic applicability and confidence must be explicit when relevant.

**§ 6(6)** Calendar conversion of a date must remain conceptually separate from the **observance of a holiday**. A fast or feast may be observed on a different civil date from its named nominal calendar date.

**§ 6(7)** Religious day boundaries (sunset, dawn, or other rule) require a separate day-boundary policy. Computing a *local instant* of sunset may additionally require latitude/longitude, ephemeris assumptions and a timezone. A bare calendar date must not gain implicit geography.

**§ 6(8)** For high latitudes where the required solar event does not occur, or observational evidence is unavailable, the calculation must produce a documented non-success/indeterminate outcome unless an explicit fallback policy was requested.

**§ 6(9)** The canonical calendar engine must support arithmetic on a chosen calendar without allocating unnecessary intermediate strings and without round-tripping through formatted text.

---

## § 7. Historical civil calendars and date resolution

**§ 7(1)** Historical date interpretation may result in **Unique**, **Nonexistent** or **Ambiguous/Multiple** absolute-day candidates. The exact Sec result type and payloads belong to the historical-calendar child book; these words denote semantic cases, not yet published enum members.

**§ 7(2)** A historical profile needs at minimum a jurisdiction/profile identifier, affected intervals, mapping rules, dataset version and sources. Interpretation must not silently assume present-day Gregorian rules for historical records.

**§ 7(3)** Sweden is a required regression profile, including its irregular early-eighteenth-century sequence: the Swedish 30 February 1712, the calendar reform in February 1753, and the succession **17 February 1753 → 1 March 1753** for successive Swedish civil dates.

**§ 7(4)** There are two different queries: “the next valid Swedish civil date after 1753-02-17” and “add an exact `1<days>` to an instant.” They must not share undocumented semantics. A historical day-step is calendar/profile aware; an elapsed-time unit is not.

**§ 7(5)** A global day axis can remain continuous through skipped civil labels. Conversely, civil labels may repeat when a jurisdiction changes its date-line practice. Samoa's repeated 4 July 1892 and skipped 30 December 2011 are mandatory historical edge-case test families.

**§ 7(6)** Calendar profiles are not ordinary geographical timezones. Timezone offset transitions, date-line changes and civil calendar reforms can interact but remain separate explicit sources of interpretation.

**§ 7(7)** Historical implementations must avoid unwarranted certainty. Incomplete legislation, local adoption differences, historical disagreement or records with an unqualified old/new style date must be expressible as unresolved interpretation, not guessed as a unique `date`.

---

## § 8. Timezones and civil-time resolution

**§ 8(1)** Named timezone rules must be modeled independently of `datetime`. A zone is not merely a constant UTC offset; historical offset changes and daylight-saving transitions matter.

**§ 8(2)** Resolving a local `datetime` under a zone may yield **Unique**, **Ambiguous** (normally two candidates) or **Nonexistent**. A general result model must not incorrectly assume *at most* two candidates if source rules can produce another cardinality.

**§ 8(3)** A strict resolver exposes ambiguity explicitly. Convenience resolution requires a separately identified policy (reject, choose earlier/later where meaningful, shift into valid time, etc.); no universal default is imposed by this parent book.

**§ 8(4)** Converting a uniquely known instant to a named zone is conceptually distinct from resolving an unzoned local `datetime` in that zone.

**§ 8(5)** IANA tzdb is the primary named-zone dataset. The implementation must record the version and support system-provided and explicitly pinned data when the target allows it. A fixed-offset use case must not require loading tzdb.

**§ 8(6)** Historical interpretation depends on the dataset's actual coverage, precision and source conventions. Its inability to answer an older date precisely must be reported, not hidden behind an arbitrary modern offset.

---

## § 9. Arithmetic, comparison, overflow and periods

**§ 9(1)** Elapsed-time arithmetic on `duration` follows the core contract. `temporal` must not introduce competing unit-specific overloads or constructors.

**§ 9(2)** Calendar movement is defined relative to a starting calendar date, an identified calendar system, optionally a historical profile, and explicit policies for invalid target dates.

**§ 9(3)** A proposed `CalendarPeriod` can hold year, month, week and day components; its final representation and valid mixed-component rules remain deferred. Calendar period components must never be silently converted into a constant nanosecond value.

**§ 9(4)** Calendar arithmetic must define month-end behavior: rejection, clamping to last valid day, intentional last-day preservation and any roll-forward policy. These policies are distinct; an implementation must not conflate them.

**§ 9(5)** Month-end **intent** may have to persist across repeated operations: 31 January → 28 February → 31 March differs from independently taking 28 February and adding one month with a simple clamp. The system must be able to represent that recurring rule explicitly.

**§ 9(6)** Applying period components in different orders may produce different results. The API must state whether operations are ordered, normalized or invalid as a mixed operation, especially across leap months, skipped civil dates and calendar boundaries.

**§ 9(7)** Calendar comparison must identify what is compared: absolute days, calendar-coordinate tuples in a specified system, a range of possible days, or exact instants. Comparing partial expressions as if they were unique dates is invalid.

**§ 9(8)** Date/datetime arithmetic and rounding semantics must preserve representation bounds and signal failure without wraparound. A calendar child book must describe year-zero, BCE and negative-year conventions where supported.

---

## § 10. Partial, uncertain and historical textual dates

**§ 10(1)** An incomplete date is not inherently uncertain; an approximate date is not inherently incomplete. These properties must be representable independently.

**§ 10(2)** Required input patterns include:

| Source text | Known information | Missing or qualified information |
|---|---|---|
| `1852` | year | month, day |
| `Maj 1852` | month, year | day |
| `18 Maj` | day, month | year |
| `omkring 1852` | approximate year | exact year/date |
| `före 18 maj 1852` | upper date boundary | exact event date |
| `tredje söndagen efter Trefaldighet 1723` | ecclesiastical calendar rule and year | resolved date until calendar/tradition is supplied |

**§ 10(3)** A partial date must preserve the exact set of components supplied. Missing components are never filled with January, day 1, the current year or another default unless the caller explicitly requests such a projection.

**§ 10(4)** Do not infer that `18 Maj`, `Maj 1852` and `1852` necessarily denote the same day: they describe overlapping **possible** dates. The library should support compatibility, containment and possible-overlap queries where meaningful.

**§ 10(5)** A historical/archival input must be able to retain the original source spelling, language, transcription/provenance and interpretation version separately from any resolved date, without embedding archival metadata in every ordinary `date`.

**§ 10(6)** Ecclesiastical dating must specify a tradition and applicable calendar (for example, a historical Swedish Lutheran record). Feast-based and church-year-relative expressions must resolve through versioned calendar rules, not the host computer's locale or today's calendar practice.

**§ 10(7)** The possible-day set for a partial or uncertain expression may be a bounded range, discrete set, recurrence or unresolved constraint. Do not require that every expression be flattened to one continuous interval when that would lose meaning.

**§ 10(8)** Natural-language parsing should be opt-in and must distinguish syntactic recognition from verified historical interpretation. Ambiguous expressions should preserve candidate interpretations with diagnostics.

---

## § 11. Recurrence, holidays and business calendars

**§ 11(1)** `recurrence` computes dates/times according to calendar rules. It does not itself schedule threads, timers, processes or tasks.

**§ 11(2)** A recurrence rule must be explicit about its calendar system, start/anchor, timezone resolution where relevant, exclusions, count/until bounds, and behavior when a target calendar date does not exist.

**§ 11(3)** `business` provides reusable civil-date policies such as weekdays, business days, observed holidays, settlement dates, working-hours intervals and combinations of business calendars. It does not own invoice ledgers, taxes, interest rates or financial transaction models.

**§ 11(4)** Date-rule capabilities must cover at least: fixed day-of-month; day offset before/after month boundary; Nth/last weekday; Nth/last working day; explicit exceptions; following, preceding and modified-following business-day adjustment.

**§ 11(5)** The following **are distinct rules**, even if they coincide in some months:

- the 27th day of each month;
- three calendar days before the last date of the current month;
- three days before the *first day* of the following month;
- the third-to-last business day of the month.

**§ 11(6)** If the 27th falls on a non-working day, the adjustment policy must be explicitly supplied by the caller. The library must not assume that every invoice is paid on the preceding Friday or following Monday.

**§ 11(7)** Different business calendars may have different weekends, holidays, early closes and exceptional closures. Combining them must specify whether *all* or *any* calendars need to be open, rather than inferring a default.

**§ 11(8)** A reproducible rule evaluation can report the rule identity, input parameters, anchor, selected calendar and holiday-data version used; this metadata is particularly important for accounting/audit applications.

**§ 11(9)** Holidays and religious observances are calendar **rules**, sometimes dependent on region, tradition or observation. `holiday` is a candidate subpackage because date conversion and observance calculation are different responsibilities.

---

## § 12. Intervals, ranges and constrained dates

**§ 12(1)** Examine Sec's built-in range semantics before creating temporal-specific types. Do not replicate general range machinery without a meaningful domain-specific reason.

**§ 12(2)** Temporal intervals must declare whether their endpoints are **dates** or **instants**, and whether each boundary is open or closed. Half-open `[start, end)` intervals are a useful interoperable default candidate, not yet a public API decision.

**§ 12(3)** Candidate operations include containment, overlap, intersection, bounded union, gap, splitting at calendar boundaries and coalescing. Operations shall not conflate an interval of exact instants with a set of possible days from a partial date.

**§ 12(4)** An interval covering local dates across a daylight-saving change need not have an elapsed duration equal to a fixed 24-hour multiple of its calendar-day count.

---

## § 13. Parsing and formatting

**§ 13(1)** `parse` converts known syntactic representations to typed domain values using strict validation by default. Error reporting must preserve the offending position and a meaningful typed reason where applicable.

**§ 13(2)** A parser must not silently discard offset, timezone annotations, calendar identity, uncertainty markers or precision from a source expression to force it into `date` or `datetime`.

**§ 13(3)** `format` supplies high-level representation and locale integration above the core formatting mechanism; it must not duplicate the core formatter architecture.

**§ 13(4)** Locale data (month/weekday names, digit systems, regional format conventions and week rules) belongs canonically to `locale`. `temporal/format` may depend on `locale` through explicit interfaces and datasets.

**§ 13(5)** Shared parsing/formatting should support ISO 8601 where applicable and the relevant Internet timestamp profiles. HTTP-date and mail-date remain the respective domain packages' public formats even if implemented using temporal helpers.

**§ 13(6)** String-producing operations must disclose allocation and failure behavior. Streaming or caller-provided writer/buffer variants should be evaluated where they materially reduce allocation; no undocumented hidden I/O is permitted.

**§ 13(7)** The current `sec/core/datetime.sec` includes format labels for RFC 3339 and HTTP despite `datetime` lacking timezone/offset identity. This is a **core follow-up**: such formats must not invent UTC or an offset. Review the core API and decide whether an explicitly supplied context is required or those forms move to a suitable higher-level type.

---

## § 14. Precision, time scales and clocks

**§ 14(1)** **Resolution**, **accuracy**, **precision**, **uncertainty** and **clock synchronization state** are separate concepts. Nanosecond fields do not imply nanosecond accuracy.

**§ 14(2)** An absolute-time subsystem must distinguish UTC, TAI, GPS time, Unix/POSIX timestamps and monotonic elapsed-time clocks. Conversion requires documented epoch, timescale and leap-second/smear policies.

**§ 14(3)** A monotonic clock reading is not an ordinary date/time-of-day and cannot be converted to a civil `datetime` without an explicitly established correlation to a civil reference clock.

**§ 14(4)** `timescale` and `precision` are candidate subpackages for conversion algorithms, versioned leap-second data and uncertainty-bearing observations. The physical clock read remains owned by the selected core/OS/platform layer.

**§ 14(5)** NTP and PTP protocol framing, sessions and wire exchange belong to networking/protocol packages; temporal may own protocol-independent time representations and scale conversion used by them.

**§ 14(6)** Clock samples with uncertainty must not be totally ordered as though the timestamps alone proved the true event ordering. An implementation may offer definite-before, definite-after and unresolved comparisons according to its verified uncertainty model.

**§ 14(7)** Leap seconds, negative leap seconds if applicable, timestamp era interpretation, and smear behavior require explicitly versioned rules. No unlabelled translation between incompatible scales is allowed.

---

## § 15. Data sources, reproducibility and platform profiles

**§ 15(1)** Pure algorithms are independently deployable. IANA timezone data, locale data, religious/holiday tables, historical adoption maps and leap-second tables are optional resources with individually visible loading and deployment requirements.

**§ 15(2)** Versioned datasets should expose their origin, version or effective date, declared coverage and interpretation conventions. Pinning a version must produce reproducible results for the same input when the dataset itself is unchanged.

**§ 15(3)** The implementation must distinguish “no data”, “unsupported calendar/profile”, “ambiguous input”, “data outside coverage” and “invalid input”. These are not interchangeable errors.

**§ 15(4)** Embedded/bare-metal builds may supply only pure calendar algorithms and explicitly packaged reduced datasets. Unsupported capabilities must be rejected or reported; the library must not silently substitute the compiler host's or OS's current timezone.

**§ 15(5)** The same program must not change pure date calculations merely because an OS time-zone database is upgraded. Data-dependent operations must clearly disclose whether they use the current system database or pinned data.

---

## § 16. Initial package taxonomy and responsibility matrix

**§ 16(1)** The following is an **architectural map**, not an instruction to create every listed directory or source file now. A child directory receives its own canonical book once work on its actual API begins.

```text
sec/stdlib/temporal/
├── std-temporal.md
├── calendar/
│   ├── historical/            [candidate]
│   └── systems/               [candidate]
├── arithmetic/
├── parse/
├── format/
├── timezone/
├── interval/                 [candidate]
├── recurrence/               [candidate]
├── business/                 [candidate]
├── holiday/                  [candidate]
├── timescale/                [candidate]
├── precision/                [candidate]
└── astronomy/                [evaluate: stdlib or external]
```

**§ 16(2)** First four child books recommended for detailed design:

| Canonical child book | Owns | Does not own |
|---|---|---|
| `calendar/std-temporal-calendar.md` | Calendar systems, conversion, component identity, historical delegation contracts | Core `date` identity; geographical timezone database |
| `arithmetic/std-temporal-arithmetic.md` | Calendar periods, month policies, period arithmetic | Unit system, core `duration` arithmetic |
| `timezone/std-temporal-timezone.md` | Named zones, zone transitions, local-time resolution | OS syscall layer, NTP/PTP wire protocols |
| `parse/std-temporal-parse.md` | Strict parsing and rich date expressions | The core general unit grammar |

**§ 16(3)** `format/std-temporal-format.md` should be designed together with the core formatter agreement. The order above reflects prerequisite concepts, not a claim that format is unimportant.

**§ 16(4)** Further candidates receive one book per directory under the same path-derived convention, for example `business/std-temporal-business.md` and `calendar/historical/std-temporal-calendar-historical.md`.

**§ 16(5)** A package may be split or merged before its own API is fixed. Avoid namespace pollution and duplicate public types. Common shared types must have one canonical owner selected before any child book publishes them.

---

## § 17. Documentation and source contract for child books

**§ 17(1)** All public Sec declarations in child books must be exact and exhaustive. Every member or variant requires its own immediately preceding explanatory comment, especially public enums and unions. Type representations must be justified.

**§ 17(2)** Public constructors use `init(...)`, not a pseudo-method `New()`; destructors are `free()`. Types are not overloadable; functions may be overloaded where the language permits. No `null`/`nil` in the Sec public API.

**§ 17(3)** Source documentation must be consumable by the LSP: summary, `Description:`, `Parameters:`, `Returns:`, `Errors:`, `Ownership:` and `Standards:` where applicable. Document whether returned storage is owned or borrowed and whether work allocates, does I/O or uses external data.

**§ 17(4)** Canonical public-function documentation layout in child source:

```sec
/**
 * One-sentence summary in present tense.
 *
 * Description:
 *   Exact semantics, including calendar, timezone and precision context.
 *
 * Parameters:
 *   - parameterName: Domain, accepted values and ownership.
 *
 * Returns:
 *   Exact success meaning and allocation/ownership behavior.
 *
 * Errors:
 *   - ErrorType.Member: Precisely stated failure condition.
 *
 * Ownership:
 *   Borrowing, moves and retained resources where relevant.
 *
 * Standards:
 *   - Relevant standard, section and version where applicable.
 */
```

**§ 17(5)** Every future `.sec` file must carry a source header identifying: canonical path, module, purpose, revision/update date, governing child book, standards and dataset/registry revision where applicable. A file's claims must be limited to actual implemented semantics.

**§ 17(6)** A future child book must fully define fallible constructor syntax and error unions using actual Sec syntax. Concept names in this parent book are not substitutes for signatures. Never generate a fake compilation stub from an architectural list.

---

## § 18. Error and diagnostic design

**§ 18(1)** Fallible operations use Sec's ordinary typed error model; no hidden exceptions, sentinel dates or unchecked “best effort” approximations.

**§ 18(2)** Useful semantic error categories to cover in child books include: invalid calendar component; nonexistent civil date; multiple candidate dates; unsupported conversion; range overflow; unresolved historical interpretation; unavailable dataset; out-of-coverage data; ambiguous local time; nonexistent local time; missing solar event; and lossy/inexact timestamp conversion.

**§ 18(3)** The exact public error names, types, payloads and complete variant sets are **Deferred** to owning child books. A child book must never leave its public error family as an open-ended prose list.

**§ 18(3a)** A child module's error type must not reuse the name of a compiler-known core error type. The calendar-relative arithmetic module's public error type is `TemporalArithmeticError` (`arithmetic/std-temporal-arithmetic.md` § 3(3)); core `ArithmeticError` keeps its own identity (MD-083, 2026-10-18).

**§ 18(4)** Compiler and LSP diagnostics should explain the wrong domain or missing context and the valid resolution (e.g. elapsed duration vs calendar period, required timezone, required calendar variant). Reject misleading suggestions that would silently alter semantics.

---

## § 19. Cross-package conformance tests

**§ 19(1)** Each child book must define exhaustive unit tests, property tests, round trips, interoperability samples, arithmetic overflow tests and negative tests where applicable. Existing compiler-core regressions are separate from stdlib behavior tests, but both must remain synchronized.

**§ 19(2)** The following constitute required **cross-book acceptance scenarios**; exact test syntax and fixture serialization are specified later:

| ID | Scenario | Expected essential behavior |
|---|---|---|
| T01 | `500<ms>` in expected-`duration` call position | Exact contextual conversion under core rules |
| T02 | Bound `500<ms>` quantity passed to `duration` argument | Requires written conversion unless binding already has type `duration` |
| T03 | `1<week>` and `5<days>` | Ordinary linear time-unit resolution; no duration-specific grammar |
| T04 | Start local `2026-03-28 12:00` Stockholm; add one calendar day | Local `2026-03-29 12:00`; elapsed difference 23 h if resolved in zone |
| T05 | Same start; add elapsed 24 h | Local `2026-03-29 13:00` after zone conversion |
| T06 | Swedish civil day after `1753-02-17` | `1753-03-01`, using Swedish historical profile |
| T07 | Swedish `1712-02-30` | Supported by correct historical profile; rejected by proleptic Gregorian |
| T08 | Samoa `1892-07-04` local civil label | Multiple historical occurrences under applicable profile |
| T09 | Samoa `2011-12-30` | Nonexistent civil date under applicable transition rules |
| T10 | Hebrew 9 Av 5786 | Gregorian `2026-07-23`; local sunset beginning requires place/context |
| T11 | `1852`, `Maj 1852`, `18 Maj` | Partial components preserved; compatible possible dates but not falsely identical |
| T12 | Church-year phrase for a Swedish historical record | Requires tradition, year and applicable historical calendar |
| T13 | Month-end-preserving monthly rule from 31 January | End-of-month intent survives February and into March |
| T14 | Monthly 27th falling on a weekend | No silent weekday adjustment without caller policy |
| T15 | Three calendar days before end of February 2026 | `2026-02-25` under stated interpretation |
| T16 | Local time repeated during DST fall-back | Explicit ambiguous candidates; no implicit choice |
| T17 | Local time missing during DST spring-forward | Explicit nonexistent result; no implicit shift |
| T18 | Clock sample ordering within overlapping uncertainty | No unsupported assertion of real-world order |
| T19 | Pin timezone/holiday data and repeat same evaluation | Reproducible result and version identity |
| T20 | Unsupported target without tzdb or wall clock | Pure operations remain available; unsupported data/time features fail explicitly |

**§ 19(3)** Additional mandatory families include: BCE/astronomical year conventions; leap months; leap years; negative epoch days; civil calendars with non-January year starts; multi-zone transitions; uncommon historical offsets; nanosecond boundaries; leap-second conversion; local sunset absent at high latitude; and finite/unknown temporal bounds.

**§ 19(4)** Verify independently computed reference results rather than reusing the same implementation algorithm as the sole test oracle. Data-backed fixtures must capture source, data version and license conditions.

---

## § 20. Implementation governance, rollout and synchronization

**§ 20(1)** This document has status **Draft architecture**. It does not assert that any new `temporal` package APIs are implemented, compiled, tested or supported on a target.

**§ 20(2)** Governance must distinguish specification complete, public API fixed, files scaffolded, implementation in progress, tests present, conformance achieved, platform support and known bugs. Reuse the current governance schema and existing IDs rather than introducing duplicate entries.

**§ 20(3)** Reconcile these observed dependencies before claiming stdlib conformance:

1. `rules/library/stdlib.md` still discusses a `time` stdlib group rather than the chosen `temporal` structure.
2. `rules/types/temporal.md` explicitly defers calendar arithmetic, parser/formatter and zone APIs; this parent book must not claim to have filled in core signatures.
3. The reviewed core date/time implementation contains unfinished or placeholder members; current source existence is not proof of executable functionality.
4. The core datetime formatter's offset-requiring formats need a separate semantic review (§ 13(7)).
5. The intended `days` / `week` unit spellings must be reconciled with the actual unit catalog while retaining the generic unit-resolution contract (§ 4(4)).

**§ 20(4)** Recommended design order: (A) calendar/day-axis and period boundaries; (B) core `Instant`/timescale feasibility; (C) calendar and arithmetic child books; (D) timezone resolution; (E) parsing/formatting; (F) historical/partial dates; (G) recurrence, holidays/business rules, high-precision extensions.

**§ 20(5)** A scaffold tool may create a directory only after its child book has defined the canonical path and intended files. It must preserve existing implementations, report discrepancies and not fabricate `.sec` files or APIs from candidate names.

---

## § 21. Explicitly deferred design questions

**§ 21(1)** This revision does not settle:

- `Instant` epoch, underlying width, UTC/TAI relationship and whether it is a core or stdlib-owned type;
- exact public representations for `CalendarSystem`, `CalendarDate`, `CalendarPeriod`, `HistoricalCalendar`, partial/uncertain dates and resolution results;
- calendar variants and first-release coverage;
- how to encode leap-month identity and calendar-specific eras/year numbering;
- policy shape for period-component order, invalid target dates and month-end persistence;
- the exact historical-profile data distribution strategy and provenance schema;
- standard clock/time-source interfaces and whether uncertainty-bearing samples belong in stdlib;
- exact locale and formatting-pattern integration;
- which astronomy/liturgical-computation components belong in stdlib rather than external packages;
- final boundaries of `interval`, `recurrence`, `business` and `holiday` packages.

**§ 21(2)** These are future child-book or core design tasks, **not requests to stop drafting this foundation**. A later decision must update the owning numbered paragraph, affected child books and governance together.

---

## § 22. Standards, datasets and references

This list identifies applicable sources and design references, **not** a blanket claim of full implementation or complete conformance.

### § 22.1. Normative Sec dependencies

- Sec temporal core contract: https://github.com/YoePro/sec/blob/main/rules/types/temporal.md
- Sec units and quantity literals: https://github.com/YoePro/sec/blob/main/rules/types/units.md
- Sec stdlib layering: https://github.com/YoePro/sec/blob/main/rules/library/stdlib.md
- MD-015 applied correction: https://github.com/YoePro/sec/blob/main/rules/corrections/applied/md015-time-unit-duration-conversion-correction-20261013.md
- Current source review: `sec/core/date.sec`, `time.sec`, `datetime.sec`, `duration.sec` in the same repository.

### § 22.2. External standards and reference data

- ISO 8601-1 and ISO 8601-2 — date, time and extended expressions: https://www.iso.org/iso-8601-date-and-time-format.html
- RFC 3339 — Internet timestamps: https://www.rfc-editor.org/rfc/rfc3339
- RFC 9557 — Internet timestamps with additional information: https://www.rfc-editor.org/rfc/rfc9557
- RFC 9636 — TZif time zone information format: https://www.rfc-editor.org/rfc/rfc9636
- RFC 5545 — iCalendar / recurrence: https://www.rfc-editor.org/rfc/rfc5545
- RFC 5905 — Network Time Protocol v4 (network-protocol ownership stays outside temporal): https://www.rfc-editor.org/rfc/rfc5905
- IANA Time Zone Database: https://www.iana.org/time-zones
- Unicode CLDR: https://cldr.unicode.org/
- Unicode locale/calendar data and calendar algorithms: https://unicode-org.github.io/icu/userguide/datetime/calendar/
- BIPM time scales / TAI and UTC: https://www.bipm.org/en/time-metrology
- IEEE 1588 Precision Time Protocol (protocol-layer reference): https://standards.ieee.org/ieee/1588/6825/

### § 22.3. Design comparison material (non-normative)

- Noda Time concepts: https://nodatime.org/
- Java `java.time`: https://docs.oracle.com/en/java/javase/25/docs/api/java.base/java/time/package-summary.html
- TC39 Temporal concepts: https://tc39.es/proposal-temporal/
- Rust Jiff: https://docs.rs/jiff/

---

## § 23. Revision discipline

**§ 23(1)** This is revision **0.1**, establishing the initial root specification and the shared requirements for future child books.

**§ 23(2)** Revisions may expand requirements as the APIs mature. Preserve numbered-paragraph identities as stable cross-reference anchors; never silently change a fundamental assumption inherited from the canonical compiler/core rulebooks.

**§ 23(3)** Before a child book is considered ready for implementation, review and reconcile its interactions with the other relevant packages; test the named boundary examples; and update implementation-governance records without duplicates.

---

**Document revision:** 0.1  
**Last updated:** 2026-10-16  
**Status:** Draft foundational architecture — no new public API published by this revision.
