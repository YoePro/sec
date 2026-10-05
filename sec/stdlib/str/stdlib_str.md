# Sec Standard Library — `str`

- **Status:** Draft — proposed normative standard-library design
- **Created:** 2026-10-05
- **Last updated:** 2026-10-05
- **Document revision:** 0.1
- **Sec language version:** 0.1
- **Canonical path:** `stdlib/str/stdlib_str.md`
- **Repository path:** `sec/stdlib/str/stdlib_str.md`
- **Implementation-status fragment:** `implementation-status-std-str.yaml`
- **Repository core/string rechecked:** 2026-10-05
- **Repository core/rune rechecked:** 2026-10-05
- **External design references rechecked:** 2026-10-05

---

# 1. Purpose and authority

This book defines the intended architecture and public API of Sec's `str`
standard-library package.

The package is the **allocating string-utility layer above the compiler/core
string and rune facilities**. It is not a second implementation of the Sec
string type, and it must not become a location for functionality that belongs
deeper in the language, compiler core, `core/string`, `core/rune`, generic
iterator infrastructure, or another foundational package.

The package is imported as:

```sec
import "str"
```

and is referenced as `str` in source code.

This revision is deliberately explicit about the boundary between layers.
That boundary is more important than matching any one existing language's
string library.

## 1.1 Core allocation rule

The governing rule is:

> **Core APIs do not dynamically allocate as part of their normal observable
> operation. `ToString()` is the sole general exception.**

`ToString()` remains available at the deepest appropriate level because it is
a universal conversion contract. Keeping it there avoids language-wide
special cases for string conversion.

This exception must not be used as a loophole to hide unrelated allocation
behind core APIs.

For example, a core function named `SplitToArray()` is allocating because its
observable result requires dynamic array storage. Calling `ToString()` or an
allocator internally does not make that function non-allocating.

## 1.2 Deepest-owner rule

An API belongs at the deepest layer that can correctly own its semantics.

Therefore:

- compiler-known properties and operations remain compiler-owned;
- allocation-free primitive string operations remain in `core/string`;
- allocation-free rune classification and rune-to-rune conversion remain in
  `core/rune`;
- generic lazy iterator transforms belong in the generic iterator/core layer;
- Unicode tables and primitive Unicode facts must not be duplicated in
  `stdlib/str`;
- `stdlib/str` composes those facilities when a result must be materialized or
  new text must be produced.

A stdlib function must not be created merely because it is convenient to put
code there.

## 1.3 `Len` is not a `str` API

`Len` is compiler/core functionality.

Neither `core/string` nor `stdlib/str` owns a library declaration whose purpose
is to implement `.Len`.

This rule applies equally to new stdlib-owned types. `str.Builder` must not add
an ordinary library member named `Len` as a substitute for compiler-owned
length semantics. Where a builder-specific measurement is useful, this book
uses explicit names such as `ByteCount`.

If the compiler later defines that compiler-owned `Len` applies to `Builder`,
that remains a compiler decision, not an API declaration in this package.

## 1.4 What belongs in `core/string`

Typical allocation-free operations belong in `core/string`, including, when
their semantics are accepted by the core specification:

- byte/rune access;
- comparison;
- prefix/suffix tests;
- exact substring search;
- byte/rune boundary queries;
- slices/views into existing string storage;
- trim operations that return a slice/view;
- lazy split iteration;
- split-once when the result can be represented without dynamic allocation;
- lazy line iteration;
- iteration over runes.

`stdlib/str` must not duplicate these merely to offer a differently named
version.

## 1.5 What belongs in `core/rune`

Rune-level classification and one-rune-to-one-rune transforms remain deep:

```text
Is*
ToLower
ToUpper
Utf8Length
...
```

`stdlib/str` may use them but must not carry a second copy of Unicode
classification logic.

Full-string transformations can require allocation and can expand one rune
into several runes. Those operations belong in `stdlib/str`.

## 1.6 What `stdlib/str` owns

`stdlib/str` owns operations whose useful public result inherently requires one
or more of:

1. a newly materialized dynamic collection;
2. newly constructed string storage;
3. a mutable builder that owns dynamic storage;
4. expansion/contraction of text that cannot be represented as a view of the
   input;
5. a runtime decoding/transforming step whose result is new text.

The package may still return an unchanged input string on a proven no-op if
Sec's string ownership model permits that safely. An allocating-layer API is
not required to perform a pointless allocation merely to prove that it belongs
in stdlib.

## 1.7 Public API style

Public package functions use Sec public API naming:

```text
CamelCase
```

Initialisms retain their canonical uppercase spelling where applicable.

Package functions are preferred for transformations of built-in `string`:

```sec
str.ToUpper(value)
str.Split(value, ",")
str.ReplaceAll(value, "old", "new")
```

`stdlib/str` must not pretend that these functions are compiler-owned methods
of the built-in `string` type.

Types actually owned by this package, such as `Builder`, use normal methods.

---

# 2. Documentation contract for source and LSP

Public declarations in `str` must use structured documentation comments that
can be consumed by the Sec LSP for hover and generated documentation.

The canonical public-function comment shape is:

```sec
/**
 * One-sentence summary in present tense.
 *
 * Description:
 *   Additional semantic detail when the summary is not sufficient.
 *
 * Parameters:
 *   - name: Meaning, accepted domain, indexing unit, and ownership where relevant.
 *
 * Returns:
 *   Exact success meaning. State whether returned storage is newly allocated,
 *   materialized, borrowed, reused, or derived from the input where relevant.
 *
 * Errors:
 *   - ErrorType.Member: Exact condition that produces the error.
 *
 * Ownership:
 *   State borrows, moves, retained resources, and post-call ownership.
 *
 * Unicode:
 *   State whether behavior is byte-based, rune-based, Unicode-property-based,
 *   normalization-based, or locale-sensitive/locale-independent.
 *
 * Allocation:
 *   State what storage can be allocated and whether a no-op may return/reuse
 *   existing string storage.
 *
 * Standards:
 *   - Unicode Standard / UAX / other standard when directly applicable.
 */
```

Rules:

1. The first line must be suitable as compact LSP hover text.
2. `Parameters:` names every parameter exactly once.
3. `Returns:` is mandatory for every non-`void` function and every fallible
   function.
4. `Errors:` is mandatory for `Result`-returning APIs and enumerates all errors
   directly visible at that abstraction level.
5. `Ownership:` is mandatory when a value is borrowed, moved, retained, owns
   storage, or when a builder is consumed.
6. `Unicode:` is mandatory for text-semantic transforms whose meaning depends
   on runes, Unicode properties, case data, or normalization.
7. `Allocation:` is mandatory for all public functions in this package except
   trivially non-allocating methods on already-owned stdlib types.
8. A byte offset must always be documented as a **byte offset**. A rune index
   must always be documented as a **rune index**. The word "index" alone is not
   sufficient when either interpretation is possible.
9. `Standards:` is mandatory where a Unicode annex or other external standard
   directly defines the behavior.
10. TODO text is forbidden in public API comments in a conforming
    implementation.
11. Comments must describe the API implemented in the current revision, not
    planned behavior.
12. Comments documenting members or enum/error alternatives appear immediately
    before the declaration they document.

---

# 3. Package architecture

`str` is a foundational standard-library package above core.

Its dependency direction must remain shallow:

```text
compiler-known primitives
        ↓
core/rune
core/string
core/iterator and collection primitives
canonical allocation/error primitives
        ↓
stdlib/str
        ↓
fmt / io / encoding / net / application libraries
```

`str` must not depend on `fmt`, `io`, `net`, HTTP, JSON, regex, locale-aware
formatting, or application-specific text packages merely to implement basic
text manipulation.

A future dedicated Unicode package may provide deeper Unicode algorithms or
tables. If that happens, `str` should consume those primitives rather than
duplicate them.

## 3.1 Text model

Sec strings contain valid UTF-8.

Consequences for this package:

- byte operations must not create invalid UTF-8;
- rune-producing callbacks must return valid Sec `rune` values;
- positions that cut string storage must be checked for UTF-8 rune boundaries;
- operations described as rune-based operate on Unicode scalar values, not
  bytes and not grapheme clusters;
- grapheme-cluster display operations are not silently approximated using rune
  count.

## 3.2 Locale policy

Unless a function explicitly accepts locale information, string transforms in
this package are locale-independent.

Default Unicode casing and case folding are not the same as culture-specific
casing.

Locale-sensitive text rules belong in a future locale/i18n facility.

## 3.3 Allocation policy

The package should be allocation-aware without forcing every caller to reason
about implementation details.

Rules:

1. Dynamic allocation failures are propagated using the canonical lower-level
   allocation/collection error type already owned by the deeper runtime/core
   layer.
2. `str` must not invent a second general-purpose allocation error.
3. File-specific semantic errors may wrap the canonical lower-level error in a
   `union error`.
4. Functions should calculate a required output size before allocating when
   this is reasonably possible.
5. Size arithmetic must be overflow-checked.
6. A function must not partially publish a result after allocation failure.
7. Builder mutation that fails to grow must preserve a valid builder value.
8. Newly produced strings must always be valid UTF-8.

The signatures in this revision use `CollectionError` as the current
lower-level allocation-growth error where appropriate. If the canonical core
allocator/error book later renames that type, `str` must use the canonical
type rather than preserve a local duplicate.

## 3.4 Lazy-first rule

If an operation can naturally be expressed as a non-allocating iterator/view
and is fundamental enough to belong in core, the lazy primitive is defined
there first.

`stdlib/str` then materializes it.

Example:

```text
core/string:
    value.Split(separator) -> StringSplitIterator

stdlib/str:
    str.Split(value, separator) -> Result[string[], CollectionError]
```

The stdlib function is not the owner of split scanning semantics. It is the
owner of the materialized result.

This rule also guides line splitting, fields/word-like splitting, and generic
iterator transformations.

---

# 4. Design influences

This package deliberately combines ideas from several standard libraries
rather than cloning one.

## 4.1 Go

Useful influences:

- clear separation between immutable strings and `strings.Builder`;
- `Split`, `SplitN`, `SplitAfter`, `Fields`, and predicate-based field
  splitting;
- `Join`, `Map`, `Repeat`, `Replace`, `ReplaceAll`;
- simple APIs that state exact separator semantics.

Reference:

- https://pkg.go.dev/strings

## 4.2 Odin

Useful influences:

- explicit recognition that many text transforms allocate;
- broad string utility coverage;
- builder support;
- naming-style transforms such as snake/camel/Pascal/delimiter case;
- allocator-awareness.

Reference:

- https://pkg.odin-lang.org/core/strings/

## 4.3 Rust

Useful influences:

- allocation-free `str` views and iterators as the lower-level model;
- allocating `String` transformations separated from borrowed `str`;
- byte-boundary correctness for UTF-8 string modification;
- explicit distinction between Unicode scalar values and bytes.

References:

- https://doc.rust-lang.org/std/primitive.str.html
- https://doc.rust-lang.org/std/string/struct.String.html

## 4.4 Zig

Useful influences:

- allocation should be visible in API design;
- growable buffers are explicit owned data structures;
- allocation failure is part of normal API semantics.

Reference:

- https://ziglang.org/documentation/master/

Sec does **not** have to copy Zig's allocator-passing syntax. The design
influence is explicit ownership and fallibility.

## 4.5 C++

Useful influences:

- rich structural editing operations;
- insertion, erasure, replacement, appending, and capacity-aware building;
- strong separation between string storage and `string_view`.

Reference:

- https://en.cppreference.com/cpp/string/basic_string

Sec should not copy C++'s very large overload surface merely for compatibility.

## 4.6 C#

Useful influences:

- a rich immutable `String` API;
- a dedicated mutable `StringBuilder`;
- padding and normalization;
- clear distinction between repeated mutable construction and immutable string
  results.

References:

- https://learn.microsoft.com/dotnet/api/system.string
- https://learn.microsoft.com/dotnet/api/system.text.stringbuilder

Sec must not copy UTF-16 indexing semantics.

## 4.7 F#

Useful influences:

- functional `map`, `mapi`, `filter`, and `collect`;
- explicit string concatenation over sequences;
- `collect` as a one-input-element-to-many-output-elements transform.

Reference:

- https://fsharp.github.io/fsharp-core-docs/reference/fsharp-core-stringmodule.html

## 4.8 Vale

Useful influences:

- functions and method-style calls can share one conceptual operation;
- string utilities can remain ordinary library functions rather than becoming
  privileged compiler methods;
- ownership semantics should remain visible.

Reference:

- https://vale.dev/guide/functions

## 4.9 V

Useful influences include a dedicated string builder and a broad practical
string utility surface. Sec should retain its own ownership and Unicode rules
rather than copy V signatures.

Reference:

- https://modules.vlang.io/strings.html

## 4.10 42

42 remains a design comparison point for a rich language/library boundary.
No Sec API in this revision is copied solely from 42; the package must still
obey Sec's allocation and deepest-owner rules.

---

# 5. Canonical file manifest

Revision 0.1 uses the following package layout:

```text
stdlib/str/
├── stdlib_str.md
├── builder.sec
├── case.sec
├── collect.sec
├── edit.sec
├── escape.sec
├── join.sec
├── replace.sec
├── split.sec
├── transform.sec
├── whitespace.sec
├── naming.sec
└── normalize.sec
```

`edit.sec`, `naming.sec`, and `normalize.sec` are required additions to the
initial directory layout.

All `.sec` files use:

```sec
module str
```

The file split is semantic. It is not permission to duplicate private
infrastructure in each file. Shared private helpers may be moved to an internal
file later if implementation size justifies it, but a new public file is not
created merely to hide a helper.

---

# 6. Required source-file headers

Every source file must carry a header describing file identity, purpose,
allocation role, Unicode role where applicable, and the governing rulebook.

The exact line wrapping may change, but the information must not be omitted.

## 6.1 `builder.sec`

```sec
module str

/*
 * Sec Standard Library - str - Mutable UTF-8 string construction
 *
 * File:        stdlib/str/builder.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Owned mutable UTF-8 construction buffer used to efficiently materialize
 *   strings from repeated append operations.
 *
 * Allocation:
 *   Owns growable dynamic storage. Growth is fallible.
 *
 * Unicode:
 *   Appended strings are already valid UTF-8. Appended runes are encoded as
 *   valid UTF-8 before being committed to the buffer.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.2 `case.sec`

```sec
module str

/*
 * Sec Standard Library - str - Allocating Unicode case conversion
 *
 * File:        stdlib/str/case.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Locale-independent full-string Unicode case conversion and case folding.
 *
 * Allocation:
 *   Produces materialized string results and may expand one input rune into
 *   multiple output runes.
 *
 * Unicode:
 *   Uses canonical repository Unicode data. Must not duplicate or fork the
 *   lower-level Unicode/rune property tables.
 *
 * Standards:
 *   - The Unicode Standard - Default Case Conversion
 *   - Unicode CaseFolding.txt
 *   - Unicode SpecialCasing.txt
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.3 `collect.sec`

```sec
module str

/*
 * Sec Standard Library - str - String and rune/byte materialization
 *
 * File:        stdlib/str/collect.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Materializes strings into owning dynamic byte/rune arrays and materializes
 *   rune sequences into new strings.
 *
 * Allocation:
 *   All public operations in this file materialize owning storage.
 *
 * Unicode:
 *   Rune materialization follows Sec's valid UTF-8 string iteration semantics.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.4 `edit.sec`

```sec
module str

/*
 * Sec Standard Library - str - Position-based immutable string editing
 *
 * File:        stdlib/str/edit.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Creates new strings by inserting, removing, or replacing explicit byte
 *   ranges in existing strings.
 *
 * Allocation:
 *   Successful edits that change content materialize new string storage.
 *
 * Unicode:
 *   Edit boundaries are byte offsets and must lie on valid UTF-8 rune
 *   boundaries.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.5 `escape.sec`

```sec
module str

/*
 * Sec Standard Library - str - Sec string-literal escaping and unescaping
 *
 * File:        stdlib/str/escape.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Produces and parses escaped text using the canonical Sec string-literal
 *   escape rules.
 *
 * Allocation:
 *   Escaping and unescaping materialize new strings.
 *
 * Unicode:
 *   Unicode escapes must decode only to valid Unicode scalar values.
 *
 * Standards:
 *   - Sec lexical/string-literal rulebook
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.6 `join.sec`

```sec
module str

/*
 * Sec Standard Library - str - Sequence joining and concatenation
 *
 * File:        stdlib/str/join.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Materializes a sequence of strings as one string, optionally inserting a
 *   separator between elements.
 *
 * Allocation:
 *   Produces one materialized output string.
 *
 * Unicode:
 *   Copies valid UTF-8 string data without changing rune contents.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.7 `replace.sec`

```sec
module str

/*
 * Sec Standard Library - str - Search-based immutable string replacement
 *
 * File:        stdlib/str/replace.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Creates new strings by locating exact substring occurrences and replacing
 *   selected occurrences with replacement text.
 *
 * Allocation:
 *   Content-changing replacements materialize new string storage.
 *
 * Unicode:
 *   Search is exact and byte-stable over valid UTF-8 strings. Empty search
 *   values match only at rune boundaries.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.8 `split.sec`

```sec
module str

/*
 * Sec Standard Library - str - Materialized string splitting
 *
 * File:        stdlib/str/split.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Materializes lazy/core string splitting and field iteration into owning
 *   dynamic collections.
 *
 * Allocation:
 *   Allocates the result collection. Substring values may remain views of the
 *   source string when Sec string ownership permits this safely.
 *
 * Unicode:
 *   Empty-separator splitting and predicate-based fields operate at rune
 *   boundaries.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.9 `transform.sec`

```sec
module str

/*
 * Sec Standard Library - str - Allocating rune-to-string transformations
 *
 * File:        stdlib/str/transform.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Materializes rune mapping, filtering, expansion, repetition, and reversal
 *   as new strings.
 *
 * Allocation:
 *   Public functions produce new string storage unless the documented no-op
 *   fast path safely reuses the original string.
 *
 * Unicode:
 *   Transform callbacks operate on Unicode scalar values. Reverse reverses
 *   runes, not grapheme clusters.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.10 `whitespace.sec`

```sec
module str

/*
 * Sec Standard Library - str - Allocating whitespace and layout transforms
 *
 * File:        stdlib/str/whitespace.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Produces new strings by collapsing whitespace, normalizing whitespace,
 *   indenting/dedenting lines, and padding text.
 *
 * Allocation:
 *   Content-changing operations materialize new string storage.
 *
 * Unicode:
 *   General whitespace recognition uses the canonical rune/core Unicode
 *   whitespace property. Indentation rules are separately specified.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.11 `naming.sec`

```sec
module str

/*
 * Sec Standard Library - str - Identifier-style naming conversions
 *
 * File:        stdlib/str/naming.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Converts human/programmatic word sequences between common identifier and
 *   delimiter naming styles.
 *
 * Allocation:
 *   Produces new materialized strings.
 *
 * Unicode:
 *   Word classification uses canonical rune/core Unicode classification and
 *   locale-independent case conversion.
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

## 6.12 `normalize.sec`

```sec
module str

/*
 * Sec Standard Library - str - Unicode normalization materialization
 *
 * File:        stdlib/str/normalize.sec
 * Module:      str
 * Revision:    1
 * Updated:     2026-10-05
 * Author:      Jonas Engström
 *
 * Purpose:
 *   Materializes Unicode NFC, NFD, NFKC, and NFKD normalized strings.
 *
 * Allocation:
 *   Normalization produces a materialized string when normalization changes
 *   representation; a safe no-op may reuse the original string.
 *
 * Unicode:
 *   Uses the repository's canonical Unicode normalization data and algorithm.
 *
 * Standards:
 *   - Unicode Standard Annex #15 - Unicode Normalization Forms
 *     https://www.unicode.org/reports/tr15/
 *
 * Rulebook:
 *   stdlib/str/stdlib_str.md
 */
```

---

# 7. Shared public API and error conventions

## 7.1 No package-level `Len`

No source file in this package declares:

```sec
fn Len(...)
property Len ...
```

for the purpose of implementing Sec length semantics.

## 7.2 No hidden formatting dependency

`Builder.Append(123)` is **not** added merely for convenience.

Numeric, temporal, and general value formatting belongs to the formatting
system (`core/format` and `stdlib/fmt` according to their own specifications).

`str.Builder` should instead be usable as a destination/sink by that formatting
system when the lower-level format contract is ready.

This avoids creating a second formatting system inside `str`.

## 7.3 Error shape

File-specific errors should be closed when the set of semantic failure reasons
is closed.

Payload-free families use:

```sec
enum SomeError error {
    ...
}
```

Payload-bearing families use:

```sec
type SomeError union error {
    ...
}
```

Allocation failures are not translated into vague text errors.

## 7.4 Transactional result construction

Unless explicitly documented otherwise, a failed operation:

- does not publish a partial result;
- does not mutate input strings;
- leaves a borrowed/mutable `Builder` valid;
- preserves ownership of consumed arguments if Sec's call/move rules require
  transactional rollback on a failed operation.

## 7.5 Overflow

Every size multiplication/addition used to calculate result capacity must be
checked.

Examples include:

- `Repeat`;
- `Join`;
- padding;
- naming transforms;
- replacement where replacement text is larger than matched text.

Overflow is not allowed to wrap into a smaller allocation.

The exact lower-level overflow/allocation error type should follow the
canonical collection/allocation specification rather than inventing a
`str`-specific general overflow family.

---

# 8. `builder.sec`

## 8.1 Focus

`builder.sec` owns one thing: efficient incremental construction of valid UTF-8
strings in owned mutable storage.

It is the preferred internal implementation tool for stdlib transforms whose
output size cannot be determined cheaply in one pre-scan.

It does not own:

- formatting of arbitrary values;
- I/O;
- parsing;
- search;
- split semantics;
- Unicode classification tables.

## 8.2 Type shape

Recommended public type:

```sec
@noCopy
type Builder struct {
    // private storage
}
```

The builder owns its dynamic storage and therefore has:

```sec
fn free()
```

The representation remains private.

## 8.3 Constructors

The empty/default constructor need not be written if Sec provides the normal
implicit empty `init`.

The useful explicit constructor is a capacity hint:

```sec
init(initialCapacity: uint) Result[Builder, CollectionError]
```

The exact fallible-constructor spelling is subject to the canonical Sec
constructor syntax.

`initialCapacity` is a byte-capacity hint, because UTF-8 storage is byte-based.

It does not change the logical content.

## 8.4 Public inspection

The builder may expose:

```sec
property ByteCount: uint
property Capacity: uint
property IsEmpty: bool
```

`ByteCount` is the number of UTF-8 bytes currently committed.

`Capacity` is the number of bytes currently available without another growth
allocation.

These names are intentionally not `Len`.

`Capacity` is observable performance information, not part of compiler length
semantics.

## 8.5 Growth

```sec
fn Reserve(additionalBytes: uint) Result[void, CollectionError]
```

Semantics:

- ensures room for at least `additionalBytes` beyond current content;
- preserves current content;
- may allocate;
- on failure, leaves the builder valid and current content unchanged;
- a request already satisfied by current capacity succeeds without allocating.

The API does not expose a `SetCapacity` operation in revision 0.1.

## 8.6 Append operations

Required overloads:

```sec
fn Append(value: string) Result[void, CollectionError]
fn Append(value: rune) Result[void, CollectionError]
```

String append copies the source bytes into builder-owned storage.

Rune append encodes the rune as UTF-8 and commits the complete encoding
atomically. A partial UTF-8 sequence must never be left in the builder.

A byte append is intentionally not part of the initial public API because
arbitrary bytes can violate the builder invariant that its contents are valid
UTF-8.

## 8.7 Line append

Required convenience operations:

```sec
fn AppendLine() Result[void, CollectionError]
fn AppendLine(value: string) Result[void, CollectionError]
```

`AppendLine()` appends exactly U+000A LINE FEED (`"\n"`).

`AppendLine(value)` is equivalent in semantics to appending `value` and then
U+000A, but must avoid unnecessary intermediate strings.

The function does not use platform-native CRLF automatically. Text files and
protocols that require CRLF own that policy at their layer.

## 8.8 Repeated append

Recommended:

```sec
fn AppendRepeat(value: string, count: uint) Result[void, CollectionError]
fn AppendRepeat(value: rune, count: uint) Result[void, CollectionError]
```

This avoids materializing a temporary `str.Repeat(...)` result merely to append
it to a builder.

The implementation must overflow-check the required growth before mutation.

## 8.9 Clear

```sec
fn Clear()
```

`Clear`:

- resets logical content to empty;
- keeps owned capacity for reuse;
- does not allocate;
- leaves the builder ready for further appends.

No `ResetAndFree` synonym is required; `free()` owns destruction.

## 8.10 Materialization

Two distinct operations are useful:

```sec
fn ToString() Result[string, CollectionError]
fn Finish(->self) Result[string, CollectionError]
```

`ToString()`:

- does not consume the builder;
- returns an immutable string representing current content;
- therefore must behave as though it copies/snapshots builder content;
- later builder mutation must not mutate the returned string.

`Finish(->self)`:

- consumes the builder;
- transfers or materializes its contents as an immutable string;
- may reuse backing allocation when runtime representation permits;
- must not promise a zero-copy implementation as observable semantics;
- after successful consumption, the original builder no longer owns storage.

If Sec's final consuming-receiver syntax differs, the declaration spelling
changes but ownership semantics do not.

## 8.11 Explicit exclusions

Revision 0.1 does not add:

```text
Append(int)
Append(float)
Append(bool)
AppendFormat
Printf
WriteTo(io.Writer)
ReadFrom(io.Reader)
```

Formatting belongs to `fmt`/core formatting. I/O belongs to `io`.

## 8.12 Implementation strategy

Preferred implementation:

1. private growable byte storage;
2. append strings by byte copy;
3. append runes by direct UTF-8 encoding;
4. geometric growth within canonical allocator rules;
5. no UTF-8 revalidation after operations that preserve the invariant;
6. use `Reserve` internally before committing multi-byte additions.

## 8.13 Required tests

At minimum:

- empty builder;
- capacity hint zero and nonzero;
- ASCII append;
- 2-, 3-, and 4-byte rune append;
- repeated append;
- `AppendLine`;
- `Clear` retains usability;
- `ToString` snapshot remains unchanged after later append;
- `Finish` ownership behavior;
- allocation failure during growth leaves builder valid;
- overflow request is rejected;
- no operation exposes invalid UTF-8.

---

# 9. `case.sec`

## 9.1 Focus

`case.sec` owns full-string, locale-independent Unicode case transforms whose
results are newly materialized strings.

It builds on deeper rune/Unicode facts.

It must not duplicate `core/rune.ToUpper`, `core/rune.ToLower`, `IsUpper*`,
`IsLower*`, or Unicode category tables.

## 9.2 Required public surface

```sec
fn ToLower(value: string) Result[string, CollectionError]
fn ToUpper(value: string) Result[string, CollectionError]
fn ToTitle(value: string) Result[string, CollectionError]
fn CaseFold(value: string) Result[string, CollectionError]
```

## 9.3 `ToLower`

`ToLower` performs default Unicode full-string lowercase conversion.

Important distinction:

```text
rune.ToLower() -> rune
str.ToLower()  -> string
```

The string operation may expand one input rune into multiple output runes if
required by canonical Unicode full case mapping.

It is locale-independent.

## 9.4 `ToUpper`

`ToUpper` performs default Unicode full-string uppercase conversion.

It may expand output and therefore cannot be specified merely as:

```sec
for r in value {
    builder.Append(r.ToUpper())
}
```

when Unicode full mappings require more than one output rune.

The lower layer must provide canonical mapping data; `str` materializes the
result.

## 9.5 `ToTitle`

`ToTitle` performs Unicode titlecasing over text using the canonical word
boundary/titlecase algorithm selected by Sec.

Revision 0.1 recommendation:

- word boundaries follow Unicode default word-boundary semantics where required;
- first cased rune of each applicable word receives title mapping;
- following cased runes receive lowercase mapping according to default Unicode
  titlecasing semantics;
- locale-specific titlecasing is excluded.

This function must not use ASCII-only "split on spaces and uppercase the first
byte" behavior.

## 9.6 `CaseFold`

`CaseFold` performs full, locale-independent Unicode case folding suitable for
caseless text comparison pipelines.

It is **not** the same operation as `ToLower`.

It does not silently normalize Unicode normalization form unless the Unicode
case-folding standard itself requires a mapping. Callers needing normalized
caseless text compose normalization explicitly.

Example conceptual pipeline:

```text
NormalizeNFC(CaseFold(value))
```

when the application requires both folding and canonical normalization.

## 9.7 No comparison functions here

Functions such as:

```text
EqualsIgnoreCase
CompareIgnoreCase
StartsWithIgnoreCase
```

are not added to this file merely because casing exists here.

They can be implemented allocation-free using streaming fold logic and
therefore should first be considered for a deeper comparison/search layer.

## 9.8 Implementation strategy

1. Iterate valid input runes.
2. Query canonical lower-level Unicode case data.
3. Use a builder when mappings can expand.
4. Fast-path ASCII where useful without changing semantics.
5. Do not make result depend on process locale.
6. Never carry a second stale copy of Unicode case tables in this file.

## 9.9 Required tests

Include:

- ASCII lower/upper;
- already-cased no-op inputs;
- non-ASCII one-to-one mappings;
- full mappings that expand;
- titlecasing across punctuation and whitespace boundaries;
- case folding where folding differs from lowercase;
- mixed scripts;
- empty input;
- allocation failure;
- canonical Unicode conformance vectors for the repository Unicode version.

---

# 10. `collect.sec`

## 10.1 Focus

`collect.sec` owns explicit materialization between immutable strings and owning
dynamic rune/byte collections, plus construction of strings from rune
sequences.

It does not own UTF-8 validity rules themselves; those are deeper.

## 10.2 Required public surface

Recommended:

```sec
fn ToRuneArray(value: string) Result[rune[], CollectionError]
fn ToByteArray(value: string) Result[byte[], CollectionError]

fn FromRunes(values: Iterator[rune]) Result[string, CollectionError]
fn FromRunes(values: ref rune[]) Result[string, CollectionError]
```

The exact generic iterator parameter syntax must follow the canonical iterator
rulebook.

The array overload is retained because it permits pre-sizing without requiring
the caller to construct an iterator adapter explicitly.

## 10.3 `ToRuneArray`

Materializes one Unicode scalar value per array element.

The operation:

- allocates owning dynamic array storage;
- iterates according to canonical Sec UTF-8 string semantics;
- does not expose UTF-8 encoding bytes.

This is the natural destination for the currently allocating
`string.ToRuneArray()` behavior when core becomes allocation-free.

## 10.4 `ToByteArray`

Returns an owning copy of the string's UTF-8 byte representation.

It is not a view.

A zero-copy byte view, if Sec exposes one, belongs in core because no
materialization is necessary.

## 10.5 `FromRunes`

Materializes a new UTF-8 string by encoding each input rune.

The function:

- accepts only valid Sec `rune` values;
- performs checked output-size growth;
- never creates invalid UTF-8;
- uses builder/direct pre-sizing as appropriate.

## 10.6 Byte-to-string conversion is not owned here by default

A general:

```text
FromBytes(byte[])
```

is deliberately not specified in revision 0.1.

Reason:

- validating UTF-8 is a fundamental string/runtime concern;
- the universal `ToString()` exception can provide the canonical conversion
  contract where appropriate;
- `str` should not become the owner of UTF-8 validity.

If a future encoding package exposes explicit UTF-8 decode operations, that
package may also own byte-to-string decoding APIs.

## 10.7 Required tests

- empty string/arrays;
- ASCII;
- all UTF-8 sequence lengths;
- rune-array round trip;
- byte-array exact UTF-8 contents;
- large sequence growth;
- allocation failure;
- no accidental sharing for APIs documented as owning copies.

---

# 11. `edit.sec`

## 11.1 Focus

`edit.sec` owns **position-based immutable editing**.

This is deliberately separate from `replace.sec`.

```text
edit.sec:
    "change this known range"

replace.sec:
    "find occurrences of this text and change them"
```

## 11.2 Position unit

Revision 0.1 uses **UTF-8 byte offsets** for structural edit boundaries.

Reasons:

1. core string slicing/search already naturally exposes byte offsets;
2. exact search results can feed directly into edits;
3. UTF-8 storage is byte-addressed;
4. the implementation can reject boundaries inside a multi-byte rune.

Every public parameter name should make the unit visible, e.g.:

```text
byteIndex
startByte
endByte
```

The range is half-open:

```text
[startByte, endByte)
```

## 11.3 Error type

Recommended:

```sec
type EditError union error {
    /**
     * A byte position or range lies outside the source string.
     */
    OutOfBounds(uint),

    /**
     * The requested range begins after it ends.
     */
    StartAfterEnd,

    /**
     * A byte position lies inside a UTF-8 encoded rune.
     */
    NotRuneBoundary(uint),

    /**
     * Result storage could not be allocated.
     */
    Allocation(CollectionError),
}
```

Exact payload syntax may change to match the canonical error-union rules.

## 11.4 Required public surface

```sec
fn Insert(
    value: string,
    byteIndex: uint,
    insertion: string
) Result[string, EditError]

fn RemoveRange(
    value: string,
    startByte: uint,
    endByte: uint
) Result[string, EditError]

fn ReplaceRange(
    value: string,
    startByte: uint,
    endByte: uint,
    replacement: string
) Result[string, EditError]
```

A rune overload for `Insert` may be added:

```sec
fn Insert(
    value: string,
    byteIndex: uint,
    insertion: rune
) Result[string, EditError]
```

It is convenience, not a separate semantic operation.

## 11.5 `Insert`

Rules:

- `byteIndex` may equal the string byte length;
- the index must be a rune boundary;
- insertion before byte 0 prepends;
- insertion at end appends;
- inserting an empty string is a semantic no-op;
- no-op may return/reuse the original string safely;
- changed output is valid UTF-8.

## 11.6 `RemoveRange`

Rules:

- `startByte <= endByte`;
- both boundaries may equal end of string as appropriate;
- both must be rune boundaries;
- `[x, x)` removes nothing;
- `[0, ByteLen)` produces empty string;
- the output concatenates the prefix before `startByte` and suffix after
  `endByte`.

No separate `Remove(start, count)` API is required in revision 0.1. The
half-open range form avoids byte-count/end ambiguity and aligns with slicing.

## 11.7 `ReplaceRange`

Equivalent in result semantics to removing the selected range and inserting
`replacement` at the same position, but implemented in one allocation/growth
plan.

It must not materialize an unnecessary intermediate string.

## 11.8 No mutable string editing

These functions do not mutate `value`.

Repeated edits should use a higher-level algorithm or `Builder` rather than
building a chain of temporary immutable strings.

## 11.9 Required tests

- insertion at start/end/middle;
- range removal at start/end/full;
- zero-length range;
- replacement with shorter/equal/longer text;
- multi-byte rune boundaries;
- rejection of byte offset inside 2/3/4-byte rune;
- out-of-bounds;
- start-after-end;
- allocation failure;
- source unchanged on failure.

---

# 12. `escape.sec`

## 12.1 Focus

`escape.sec` owns runtime conversion to and from the canonical **Sec
string-literal escape representation**.

It does not own JSON, HTML, URL, shell, SQL, regex, XML, CSV, or protocol
escaping.

Those are different grammars and belong to their respective packages.

## 12.2 Required public surface

Recommended:

```sec
fn EscapeLiteral(value: string) Result[string, CollectionError]

fn Quote(value: string) Result[string, CollectionError]

fn UnescapeLiteral(value: string)
    Result[string, UnescapeError]

fn Unquote(value: string)
    Result[string, UnescapeError]
```

## 12.3 Source of truth

The Sec lexical/string-literal rulebook is authoritative for:

- recognized short escapes;
- hexadecimal/Unicode escape syntax;
- quote escaping;
- backslash escaping;
- whether named/octal escapes exist;
- raw-string behavior.

This stdlib file must **not invent an escape grammar** that differs from the
compiler parser.

If compiler literal syntax changes, this file and its tests must change in the
same revision.

## 12.4 `EscapeLiteral`

Returns escaped literal **content without surrounding quote delimiters**.

It must:

- escape characters required for a normal Sec string literal;
- preserve printable content where the language rules permit it;
- emit canonical, deterministic escape spelling when multiple spellings could
  represent the same rune.

## 12.5 `Quote`

Returns one complete ordinary Sec string literal representation:

```text
"escaped content"
```

`Quote(value)` is conceptually `EscapeLiteral(value)` plus delimiters, but must
not require an intermediate allocation.

## 12.6 `UnescapeError`

Recommended shape:

```sec
type UnescapeError union error {
    /**
     * An escape begins but is incomplete.
     */
    Truncated(uint),

    /**
     * The escape introducer is not legal in Sec string literals.
     */
    InvalidEscape(uint),

    /**
     * A hexadecimal digit required by an escape is invalid.
     */
    InvalidHexDigit(uint),

    /**
     * A Unicode escape denotes no valid Unicode scalar value.
     */
    InvalidUnicodeScalar(uint),

    /**
     * Quoted input is missing the required opening or closing delimiter.
     */
    InvalidQuotes,

    /**
     * Result storage could not be allocated.
     */
    Allocation(CollectionError),
}
```

Offsets are byte offsets into the escaped input.

## 12.7 `UnescapeLiteral`

Parses escaped content that does **not** include surrounding quotes.

It returns newly materialized valid UTF-8 text.

## 12.8 `Unquote`

Requires the canonical ordinary string-literal delimiters, verifies them, and
unescapes the contained text.

Raw/multiline literal syntaxes should not be silently accepted unless this
function's contract is explicitly expanded in a later revision.

## 12.9 Required tests

Tests must be derived from the Sec lexer/parser literal tests, including:

- every legal short escape;
- quote/backslash escapes;
- every Unicode escape form;
- lowest/highest valid scalar values;
- surrogate/non-scalar rejection;
- truncated escapes;
- invalid digits;
- invalid escape introducers;
- empty quoted string;
- compiler/runtime round-trip:
  `parse(Quote(value)) == value` for generated valid test strings.

---

# 13. `join.sec`

## 13.1 Focus

`join.sec` owns materialization of multiple existing strings into one string.

It does not own generic collection iteration, formatting, or line-ending
policy.

## 13.2 Required public surface

```sec
fn Join(
    values: Iterator[string],
    separator: string
) Result[string, CollectionError]

fn Concat(
    values: Iterator[string]
) Result[string, CollectionError]
```

Where useful for performance and ergonomics, array overloads are recommended:

```sec
fn Join(
    values: ref string[],
    separator: string
) Result[string, CollectionError]

fn Concat(
    values: ref string[]
) Result[string, CollectionError]
```

## 13.3 `Join`

Rules:

- no separator before the first element;
- no separator after the last element;
- empty input produces `""`;
- one input produces that content with no separator;
- empty separator is valid and is equivalent in content to `Concat`;
- source string contents are copied/combined exactly;
- no Unicode normalization or case conversion occurs.

## 13.4 `Concat`

`Concat(values)` is semantically equivalent to `Join(values, "")`.

It remains public because concatenation is a common operation and because a
direct API communicates intent without constructing an empty separator
argument.

## 13.5 Capacity planning

For sized inputs such as arrays:

1. sum all source byte lengths;
2. add `(count - 1) * separator.ByteLen` where count > 1;
3. detect overflow;
4. allocate once where allocator/runtime behavior permits;
5. copy each segment.

For one-pass iterators where total size is unknown, use `Builder`.

Do not consume a one-pass iterator twice merely to precompute length.

## 13.6 Optional builder integration

A later revision may add:

```sec
fn AppendJoin(
    builder: ref mut Builder,
    values: Iterator[string],
    separator: string
) Result[void, CollectionError]
```

This is intentionally **not required** in revision 0.1 until the generic
iterator and mutable-reference API are locked.

## 13.7 Required tests

- zero/one/many elements;
- empty elements;
- empty separator;
- Unicode contents and separators;
- exact byte preservation;
- array pre-sizing;
- one-pass iterator;
- overflow;
- allocation failure.

---

# 14. `replace.sec`

## 14.1 Focus

`replace.sec` owns exact substring search-and-replace that produces new string
content.

Position-based edits belong in `edit.sec`.

Regex replacement belongs in a regex package.

## 14.2 Required public surface

```sec
fn ReplaceAll(
    value: string,
    search: string,
    replacement: string
) Result[string, CollectionError]

fn ReplaceN(
    value: string,
    search: string,
    replacement: string,
    count: uint
) Result[string, CollectionError]

fn ReplaceFirst(
    value: string,
    search: string,
    replacement: string
) Result[string, CollectionError]

fn ReplaceLast(
    value: string,
    search: string,
    replacement: string
) Result[string, CollectionError]
```

No sentinel such as `-1 means all` is used.

## 14.3 Match semantics

For non-empty `search`:

- matching is exact;
- matching is case-sensitive;
- matching is normalization-sensitive;
- matching is over the UTF-8 byte representation of valid Sec strings;
- matches do not overlap;
- after a match is replaced, scanning continues after the matched input.

These semantics are compatible with ordinary exact substring search.

## 14.4 Empty `search`

Empty search has defined behavior instead of being an undocumented corner
case.

Revision 0.1 recommendation:

- empty search matches at the beginning of the string, between adjacent runes,
  and at the end;
- it never matches inside a multi-byte UTF-8 encoding;
- therefore an input containing `N` runes has `N + 1` empty-search boundaries.

This mirrors the useful behavior found in mature string libraries while
remaining Unicode-safe.

`ReplaceFirst` uses the first boundary.

`ReplaceLast` uses the final boundary.

`ReplaceN` replaces at most the first `count` boundaries/occurrences.

## 14.5 `ReplaceN`

`count == 0` returns unchanged content.

The function replaces at most `count` non-overlapping occurrences.

It does not use a signed sentinel for unlimited replacement; `ReplaceAll` is
the explicit unlimited form.

## 14.6 No-op behavior

If no match exists, or replacement leaves bytes unchanged, the function may
safely return/reuse the original string representation if Sec ownership permits
that optimization.

The API does not promise distinct backing storage.

## 14.7 Implementation strategy

- use core exact search;
- pre-count occurrences when input can be scanned cheaply and pre-sizing
  materially helps;
- otherwise build in one streaming pass;
- empty-search logic advances by rune boundary, never one raw byte;
- overflow-check expanded result size.

## 14.8 Required tests

- no matches;
- one/many matches;
- adjacent matches;
- potentially overlapping patterns such as replacing `"aa"` in `"aaa"`;
- replacement shorter/equal/longer;
- empty replacement;
- empty search for ASCII and multi-byte UTF-8;
- count zero/one/beyond available matches;
- first/last;
- allocation failure;
- result remains valid UTF-8.

---

# 15. `split.sec`

## 15.1 Focus

`split.sec` is the **materializing counterpart** to allocation-free split and
field iteration.

Its primary output is a dynamic collection.

It must not become the authoritative owner of delimiter scanning when a
fundamental lazy iterator belongs in core.

## 15.2 Required public surface

Revision 0.1 target:

```sec
fn Split(
    value: string,
    separator: string
) Result[string[], CollectionError]

fn SplitN(
    value: string,
    separator: string,
    maxParts: uint
) Result[string[], CollectionError]

fn SplitAfter(
    value: string,
    separator: string
) Result[string[], CollectionError]

fn SplitAfterN(
    value: string,
    separator: string,
    maxParts: uint
) Result[string[], CollectionError]

fn SplitLines(
    value: string
) Result[string[], CollectionError]

fn Fields(
    value: string
) Result[string[], CollectionError]

fn FieldsBy(
    value: string,
    separator: fn(rune) bool
) Result[string[], CollectionError]
```

Callback syntax is illustrative; canonical callable syntax governs the actual
source declaration.

## 15.3 Core counterpart requirement

Before `SplitAfter`, `SplitLines`, `Fields`, or `FieldsBy` are implemented as
independent scanning algorithms here, ask whether the corresponding lazy
operation belongs in `core/string`.

Preferred relationship:

```text
core:
    lazy iterator / view

stdlib/str:
    collect iterator into dynamic array
```

This prevents semantic divergence between lazy and allocating APIs.

## 15.4 `Split`

`str.Split` collects the core split iterator.

Important allocation rule:

- the dynamic array is owning allocated storage;
- each returned `string` element may be a safe slice/view of `value`;
- the function does not clone every substring merely to claim it allocates.

If Sec's ownership/lifetime model cannot safely place source-derived strings in
the returned array, the implementation must materialize element storage as
required by that model. The public documentation must state which ownership
rule applies.

## 15.5 Separator semantics

The stdlib materializer must preserve the core split semantics exactly.

It must not have a subtly different policy for:

- leading separators;
- trailing separators;
- adjacent separators;
- empty separator;
- empty source.

If core split semantics change, `str.Split` changes with them because core is
authoritative.

## 15.6 `SplitN`

`maxParts` means the maximum number of result elements.

Rules:

- `maxParts == 0` produces an empty result array;
- `maxParts == 1` produces one element containing the unsplit input according
  to the ownership/view rules;
- otherwise splitting stops after producing at most `maxParts`;
- the final part contains the unsplit remainder.

No negative sentinel is used.

## 15.7 `SplitAfter`

Like `Split`, but the separator is retained at the end of each produced part
where a separator was consumed.

Example conceptual result:

```text
SplitAfter("a,b,c", ",")
=> ["a,", "b,", "c"]
```

The separator must not be duplicated.

## 15.8 `SplitLines`

Line splitting needs canonical semantics, not `Split(value, "\n")` shorthand.

Revision 0.1 should recognize at least the line ending semantics already locked
by the core string/Unicode specification.

The core line iterator is authoritative.

The stdlib function materializes it.

Whether line terminators are retained should be fixed by the core `SplitLines`
contract. If both retaining and non-retaining forms are needed, they receive
distinct names rather than a boolean flag whose meaning is easy to forget.

## 15.9 `Fields`

`Fields` splits on one or more consecutive Unicode whitespace runes and omits
empty fields produced solely by leading, trailing, or repeated whitespace.

Whitespace classification comes from canonical `core/rune`.

This function is intentionally distinct from exact `Split(" ")`.

## 15.10 `FieldsBy`

The predicate returns true for runes that act as field separators.

Runs of separator runes delimit fields and do not produce empty field elements
at the edges or between consecutive separators.

This is the allocating counterpart of a lazy field iterator if/when core owns
that primitive.

## 15.11 Explicit exclusions

Not part of `str/split`:

```text
RegexSplit
CsvSplit
ShellSplit
PathSplit
UrlSplit
```

Each has grammar/domain semantics beyond ordinary strings.

## 15.12 Required tests

For every split family:

- empty source;
- empty separator where applicable;
- no separator;
- leading/trailing separator;
- repeated separators;
- multi-byte separator;
- multi-byte source;
- max part counts 0/1/exact/greater-than-available;
- retained separator behavior;
- Unicode whitespace fields;
- allocation failure while growing result array;
- exact agreement with lazy core iterator output.

---

# 16. `transform.sec`

## 16.1 Focus

`transform.sec` owns newly materialized strings produced by generic rune
transformations.

The functional influence is especially close to F#'s distinction between map,
filter, and collect.

Generic lazy iterator adapters are not owned here.

## 16.2 Required public surface

```sec
fn Map(
    value: string,
    mapping: fn(rune) rune
) Result[string, CollectionError]

fn MapIndexed(
    value: string,
    mapping: fn(uint, rune) rune
) Result[string, CollectionError]

fn Filter(
    value: string,
    keep: fn(rune) bool
) Result[string, CollectionError]

fn FilterIndexed(
    value: string,
    keep: fn(uint, rune) bool
) Result[string, CollectionError]

fn Collect(
    value: string,
    mapping: fn(rune) string
) Result[string, CollectionError]

fn CollectIndexed(
    value: string,
    mapping: fn(uint, rune) string
) Result[string, CollectionError]

fn Repeat(
    value: string,
    count: uint
) Result[string, CollectionError]

fn Reverse(
    value: string
) Result[string, CollectionError]
```

Indexes passed to `*Indexed` callbacks are **rune indexes**, not byte offsets.

## 16.3 `Map`

Exactly one output rune is produced for each input rune.

This is different from full Unicode case conversion when one input rune may map
to several output runes.

For expansion use `Collect`.

## 16.4 `MapIndexed`

The callback receives:

1. zero-based rune index;
2. input rune.

The index increments once per input rune independent of UTF-8 byte length.

## 16.5 `Filter`

Preserves the original order of runes for which `keep(rune)` is true.

No bytes from a multi-byte encoding are filtered independently.

## 16.6 `Collect`

`Collect` is the one-to-many transform:

```text
rune -> string
```

All returned strings are concatenated in input order.

This permits expansion such as:

```text
'a' -> "[a]"
```

without abusing `Map`.

The function must not depend on temporary-string concatenation in a loop;
builder-based collection is preferred.

## 16.7 `Repeat`

Rules:

- `count == 0` => `""`;
- `count == 1` may reuse the input safely;
- total output byte size is checked before allocation;
- output is exact repeated byte content because each input instance is already
  valid UTF-8.

## 16.8 `Reverse`

`Reverse` reverses the sequence of Unicode scalar values.

It does **not** reverse raw bytes.

It does **not** promise grapheme-cluster reversal.

Therefore combining-mark sequences and emoji grapheme clusters can appear
visually changed after rune reversal. A future grapheme-aware text package may
provide a distinct operation.

## 16.9 Explicit exclusions

No `ForEach`, `Exists`, `All`, or similar observing traversal is added merely
for symmetry. Such operations can be allocation-free and belong in generic
iterator/core facilities if Sec wants them.

## 16.10 Required tests

- empty input;
- ASCII;
- multi-byte runes;
- indexed callback indexes;
- filter all/none/some;
- collect empty/one/many-character results;
- repeat zero/one/many;
- repeat overflow;
- rune reversal with multi-byte UTF-8;
- combining marks documenting non-grapheme behavior;
- allocation failure.

---

# 17. `whitespace.sec`

## 17.1 Focus

`whitespace.sec` owns transformations that **change text layout** and therefore
produce new content.

Plain `Trim`, `TrimStart`, and `TrimEnd` do not belong here when core can return
a non-allocating slice/view.

## 17.2 Required public surface

```sec
fn CollapseWhitespace(
    value: string
) Result[string, CollectionError]

fn NormalizeWhitespace(
    value: string
) Result[string, CollectionError]

fn Indent(
    value: string,
    prefix: string
) Result[string, CollectionError]

fn Dedent(
    value: string
) Result[string, CollectionError]

fn PadLeft(
    value: string,
    targetRuneCount: uint,
    padding: rune
) Result[string, CollectionError]

fn PadRight(
    value: string,
    targetRuneCount: uint,
    padding: rune
) Result[string, CollectionError]

fn Center(
    value: string,
    targetRuneCount: uint,
    padding: rune
) Result[string, CollectionError]
```

Convenience overloads using U+0020 SPACE as padding may be added:

```sec
fn PadLeft(value: string, targetRuneCount: uint)
    Result[string, CollectionError]

fn PadRight(value: string, targetRuneCount: uint)
    Result[string, CollectionError]

fn Center(value: string, targetRuneCount: uint)
    Result[string, CollectionError]
```

## 17.3 `CollapseWhitespace`

Every non-empty run of Unicode whitespace is replaced with one U+0020 SPACE.

Leading and trailing whitespace runs become one space each if present.

Examples:

```text
"a   b"   -> "a b"
"  a  "   -> " a "
```

Whitespace membership uses canonical `rune.IsWhitespace`.

## 17.4 `NormalizeWhitespace`

`NormalizeWhitespace` performs:

1. Unicode whitespace collapse;
2. removal of the resulting leading/trailing whitespace.

Examples:

```text
"  a   b  " -> "a b"
"\tA\nB\r"  -> "A B"
```

This name does **not** mean Unicode normalization. NFC/NFD/NFKC/NFKD belong in
`normalize.sec`.

## 17.5 `Indent`

Prepends `prefix` to each logical line.

Revision 0.1 rules:

- the first line is indented;
- every line following a recognized line break is indented;
- an empty input produces `prefix` only if the canonical line model defines
  one empty logical line; otherwise empty input stays empty.

This final empty-input detail must be kept identical to the core line iterator
semantics. The implementation must not invent a second line model.

The original line separators are preserved exactly.

## 17.6 `Dedent`

`Dedent` removes the greatest common indentation from non-empty lines.

Revision 0.1 indentation characters are:

```text
U+0020 SPACE
U+0009 CHARACTER TABULATION
```

General Unicode whitespace is **not** indentation for this operation.

Rules:

1. inspect non-empty logical lines;
2. determine a common leading indentation prefix in terms of exact spaces/tabs;
3. remove that exact common prefix from each non-empty line;
4. preserve line separators and remaining content.

Tabs are not silently expanded to an arbitrary number of columns.

This keeps `Dedent` deterministic without a tab-width setting.

## 17.7 Padding

Padding target is measured in **runes**, not bytes and not terminal display
columns.

If current rune count is already at least target, the operation returns
unchanged content.

`PadLeft` adds padding before content.

`PadRight` adds padding after content.

`Center` distributes padding as evenly as possible. When an odd number of
padding runes is required, revision 0.1 places the extra rune on the right.

Terminal display-width padding is a different problem and belongs in a
display/terminal/text-width facility.

## 17.8 Required tests

- collapse with all Unicode whitespace classes supported by core;
- distinction between collapse and normalize;
- no-op text;
- indent single/multi-line;
- preserve LF/other canonical line separators;
- dedent spaces;
- dedent tabs;
- mixed spaces/tabs exact-prefix behavior;
- blank lines;
- pad ASCII and multi-byte runes;
- center odd/even padding;
- target smaller/equal/larger;
- allocation failure.

---

# 18. `naming.sec`

## 18.1 Focus

`naming.sec` owns common **word-sequence to identifier/delimiter naming-style**
conversion.

It is intentionally separate from `case.sec`.

Case conversion answers:

```text
"What case should these runes have?"
```

Naming conversion additionally answers:

```text
"Where are the word boundaries and separators?"
```

## 18.2 Required public surface

```sec
fn ToCamelCase(value: string)
    Result[string, CollectionError]

fn ToPascalCase(value: string)
    Result[string, CollectionError]

fn ToSnakeCase(value: string)
    Result[string, CollectionError]

fn ToScreamingSnakeCase(value: string)
    Result[string, CollectionError]

fn ToKebabCase(value: string)
    Result[string, CollectionError]

fn ToScreamingKebabCase(value: string)
    Result[string, CollectionError]

fn ToDelimiterCase(
    value: string,
    delimiter: rune,
    uppercase: bool
) Result[string, CollectionError]
```

`ToCamelCase` means lower camel case.

`ToPascalCase` means upper camel case.

## 18.3 Word segmentation

Revision 0.1 should use deterministic identifier-oriented segmentation rather
than locale-sensitive natural-language word segmentation.

Recommended boundary rules:

1. Unicode whitespace separates words.
2. `_`, `-`, and ordinary punctuation used as delimiters separate words.
3. a transition from lowercase letter to uppercase letter starts a new word;
4. in an uppercase run followed by a lowercase letter, the final uppercase
   rune begins the next word, preserving common acronym behavior;
5. digit runs remain intact;
6. a letter-to-digit or digit-to-letter transition may start a boundary when
   doing so preserves stable identifier conversion; this exact policy must be
   locked by tests before implementation;
7. separators are not emitted except for delimiter-style outputs.

Target example:

```text
HTTPRequest   -> HTTP + Request
helloWorld    -> hello + World
hello-world   -> hello + world
hello_world   -> hello + world
```

This section contains one deliberate implementation-lock item: the exact
letter/digit transition rule must be chosen consistently and then frozen in
tests. It must not vary by target naming style.

## 18.4 `ToCamelCase`

- first word begins in lowercase form;
- subsequent words begin in title/uppercase form;
- remaining casing uses the canonical naming conversion policy;
- separators are removed.

Example intent:

```text
"hello world" -> "helloWorld"
```

## 18.5 `ToPascalCase`

Each output word begins in title/uppercase form, separators removed.

```text
"hello world" -> "HelloWorld"
```

## 18.6 `ToSnakeCase`

Words are joined by `_` and converted using locale-independent lowercase
mapping.

```text
"HelloWorld" -> "hello_world"
```

## 18.7 `ToScreamingSnakeCase`

Words are joined by `_` and converted using locale-independent uppercase
mapping.

```text
"HelloWorld" -> "HELLO_WORLD"
```

## 18.8 `ToKebabCase`

Words are joined by `-` and converted using locale-independent lowercase
mapping.

## 18.9 `ToScreamingKebabCase`

Words are joined by `-` and converted using locale-independent uppercase
mapping.

## 18.10 `ToDelimiterCase`

Generic delimiter form used by the named wrappers where appropriate.

`uppercase == false` uses the lowercase naming policy.

`uppercase == true` uses the uppercase naming policy.

The delimiter itself is inserted only between output words.

## 18.11 Unicode relationship

Naming segmentation uses lower-level rune classification.

Full output casing may use `str` case primitives when mappings can expand.

The implementation must not maintain private ASCII-only letter tables and call
the result Unicode-capable.

## 18.12 Non-goals

This file does not validate that output is a legal Sec identifier.

It does not escape keywords.

It does not transliterate scripts.

It does not remove diacritics.

Those are separate concerns.

## 18.13 Required tests

- snake/kebab/camel/Pascal inputs;
- repeated delimiters;
- leading/trailing delimiters;
- acronym runs (`HTTPRequest`);
- lower-to-upper transitions;
- Unicode letters;
- digits;
- mixed scripts;
- punctuation;
- empty input;
- already-normalized target style;
- allocation failure;
- idempotence tests where appropriate, e.g.
  `ToSnakeCase(ToSnakeCase(x)) == ToSnakeCase(x)`.

---

# 19. `normalize.sec`

## 19.1 Focus

`normalize.sec` owns materialization of the four Unicode normalization forms.

It does not own the Unicode character database itself.

## 19.2 Normalization form type

Recommended:

```sec
enum NormalizationForm {
    /**
     * Canonical decomposition followed by canonical composition.
     */
    NFC

    /**
     * Canonical decomposition.
     */
    NFD

    /**
     * Compatibility decomposition followed by canonical composition.
     */
    NFKC

    /**
     * Compatibility decomposition.
     */
    NFKD
}
```

Representation rationale:

- this is a closed set defined by Unicode normalization;
- no payload is required;
- therefore an enum is appropriate.

## 19.3 Required public surface

```sec
fn Normalize(
    value: string,
    form: NormalizationForm
) Result[string, CollectionError]

fn NormalizeNFC(value: string)
    Result[string, CollectionError]

fn NormalizeNFD(value: string)
    Result[string, CollectionError]

fn NormalizeNFKC(value: string)
    Result[string, CollectionError]

fn NormalizeNFKD(value: string)
    Result[string, CollectionError]
```

The four named functions are convenience APIs over `Normalize`.

## 19.4 Standards

Behavior follows Unicode Standard Annex #15 for the repository's canonical
Unicode data version:

- https://www.unicode.org/reports/tr15/

The Unicode data version used by implementation and tests must be recorded in
the source/generated-data provenance. This book must not silently upgrade the
repository Unicode version by itself.

## 19.5 No `IsNormalized` here by default

`IsNormalized(value, form)` can be an observing/non-materializing operation.

Therefore it should first be considered for the deeper Unicode/core layer
rather than automatically added to `stdlib/str` for symmetry.

`str` owns normalization **materialization**.

## 19.6 Implementation strategy

Preferred architecture:

```text
lower-level Unicode decomposition/composition data/iterator
        ↓
str.Normalize
        ↓
Builder / pre-sized materialization
```

`normalize.sec` must not carry an independently maintained copy of canonical
combining classes or composition tables if those data already exist deeper in
the repository.

## 19.7 No-op optimization

If the lower-level normalization check/iterator proves the input is already in
the requested form, the implementation may safely return/reuse the original
string according to Sec ownership semantics.

The API does not promise distinct storage.

## 19.8 Required tests

Use the Unicode Normalization Test suite for the repository Unicode version.

Also include:

- empty string;
- ASCII;
- canonical equivalents;
- decomposed/composed accents;
- compatibility characters;
- combining-mark order;
- idempotence for every form;
- cross-form Unicode normalization invariants;
- allocation failure.

---

# 20. Migration consequences for current `core/string`

The current repository was rechecked on 2026-10-05.

Several existing functions are evidence of useful behavior but conflict with
the newly locked rule that core is allocation-free except `ToString()`.

They should not remain public allocating core APIs merely because they already
exist.

## 20.1 `SplitToArray`

Current behavior materializes a dynamic `string[]`.

Destination:

```text
core/string.Split(...)      -> lazy iterator remains in core
stdlib/str.Split(...)       -> materialized dynamic array
```

`string.SplitToArray()` should therefore be removed from the final core public
surface once `str.Split` replaces it.

## 20.2 `ToRuneArray`

Current behavior allocates `rune[]`.

Destination:

```text
stdlib/str/collect.sec
str.ToRuneArray(value)
```

The rune iterator remains in core.

## 20.3 `FromRuneArray`

Constructing new UTF-8 string storage from runes is allocating.

Destination:

```text
stdlib/str/collect.sec
str.FromRunes(...)
```

Core retains rune validity and UTF-8 encoding primitives needed by the
implementation.

## 20.4 `ToLower` and `ToUpper`

Current string versions build new strings.

Destination:

```text
stdlib/str/case.sec
str.ToLower(value)
str.ToUpper(value)
```

`rune.ToLower()` and `rune.ToUpper()` remain in core because they return rune
values without dynamic allocation.

The stdlib versions must additionally implement full Unicode string mappings
rather than merely concatenate one-rune mappings.

## 20.5 `Trim`, `TrimStart`, `TrimEnd`

Current implementations return slices/views of existing string storage.

They remain appropriate for core if the final ownership/lifetime model confirms
that these operations do not materialize new storage.

No allocating `str.Trim` duplicate is added.

## 20.6 `Split`

The lazy core split iterator remains appropriate in core.

`str.Split` is its materializer.

## 20.7 `FromByteArray`

This API requires a separate core audit.

UTF-8 validation belongs deep, but constructing an owning string from arbitrary
bytes can allocate.

The final core contract should prefer one of:

1. a non-allocating validation/view operation plus an allowed `ToString()`
   materialization step; or
2. the canonical `byte[]`/byte-view `ToString()` conversion using the universal
   `ToString()` allocation exception.

The important rule is that an ordinary core helper named `FromByteArray` must
not remain allocating merely by internally calling `ToString()` and thereby
circumvent the allocation rule.

---

# 21. Cross-file ownership of operations

The following table is normative guidance for avoiding duplicate APIs.

| Operation | Owner |
|---|---|
| compiler-known `Len` | compiler |
| rune `Is*` | core/rune |
| rune `ToLower` / `ToUpper` | core/rune |
| string byte/rune access | core/string |
| string slice/view | core/string |
| exact search | core/string |
| compare | core/string |
| prefix/suffix tests | core/string |
| non-allocating trim view | core/string |
| lazy split | core/string |
| generic lazy iterator map/filter | core iterator layer |
| universal `ToString()` | deepest owning type/core contract; allocation exception |
| mutable string construction | str/builder |
| full-string case conversion | str/case |
| rune/byte array materialization | str/collect |
| known-range immutable editing | str/edit |
| Sec literal escaping | str/escape |
| join/concat | str/join |
| exact search-and-replace materialization | str/replace |
| split result array materialization | str/split |
| map/filter/collect result string | str/transform |
| collapse/pad/indent/dedent | str/whitespace |
| identifier-style naming conversion | str/naming |
| NFC/NFD/NFKC/NFKD materialization | str/normalize |
| arbitrary value formatting | core/format + stdlib/fmt |
| regex | regex package |
| URL escaping | net/url |
| JSON escaping | JSON package |
| HTML escaping | HTML package |
| locale-sensitive casing/collation | locale/i18n package |
| terminal display-width operations | terminal/text-width facility |

---

# 22. APIs intentionally not added

A rich standard library does not mean every conceivable helper belongs in
`str`.

The following are intentionally excluded from this package revision.

## 22.1 Parsing numbers

Not owned here:

```text
ParseInt
ParseUint
ParseFloat
ParseDecimal
```

Numeric types/packages own numeric parsing.

## 22.2 Formatting arbitrary values

Not owned here:

```text
FormatInt
FormatFloat
AppendFormat
Printf
```

Formatting belongs to the formatting system.

## 22.3 Encoding-specific decode/encode families

Not automatically owned here:

```text
UTF8.Decode
UTF16.Decode
Base64
Hex
```

Encoding packages own those semantics.

## 22.4 Domain escaping

Not owned here:

```text
EscapeHTML
EscapeJSON
EscapeURL
EscapeSQL
EscapeShell
EscapeRegex
```

## 22.5 Locale-aware text

Not owned here:

```text
ToUpper(locale)
CompareCulture(...)
Collate(...)
```

## 22.6 Regex and globbing

Pattern languages are separate packages.

## 22.7 Display-cell width

Rune count is not terminal display width.

Operations involving East Asian Width, grapheme clusters, ANSI sequences, or
terminal cells belong elsewhere.

---

# 23. Suggested implementation order

To reduce circular dependencies and dogfood the package progressively:

1. **`builder.sec`**
   - establishes efficient result construction.
2. **`collect.sec`**
   - moves obvious allocating conversions out of core.
3. **`join.sec`**
   - simple builder/pre-size consumer.
4. **`split.sec`**
   - materializes existing core split iterators.
5. **`replace.sec`**
   - exercises search + builder.
6. **`edit.sec`**
   - exercises boundary validation and exact range composition.
7. **`transform.sec`**
   - exercises rune iteration and callbacks.
8. **`whitespace.sec`**
   - exercises rune classification and line iteration.
9. **`case.sec`**
   - only after full Unicode mapping primitives/data path is ready.
10. **`normalize.sec`**
    - after normalization data/iterator infrastructure is decided.
11. **`naming.sec`**
    - after case behavior and word-segmentation rules are stable.
12. **`escape.sec`**
    - synchronize directly with the canonical Sec literal grammar and lexer
      tests.

`escape.sec` may be implemented earlier if the literal grammar is already
fully locked; its position here reflects the need to synchronize with compiler
syntax, not low importance.

---

# 24. Conformance requirements

An implementation conforms to revision 0.1 only if:

1. no stdlib file redefines compiler-owned `Len`;
2. no allocating core/string API is kept merely for compatibility without an
   explicit exception in the core rulebook;
3. `str` does not duplicate core rune classification tables;
4. returned strings are always valid UTF-8;
5. byte/rune indexing units are explicitly documented;
6. dynamic allocation failure is propagated using canonical error contracts;
7. source inputs are not mutated;
8. builder failure leaves builder state valid;
9. Unicode case/normalization tests use the repository's declared Unicode data
   version;
10. `EscapeLiteral`/`UnescapeLiteral` agree with compiler literal grammar;
11. materialized split output agrees with the corresponding core lazy iterator;
12. public declarations contain the structured documentation required by
    section 2;
13. public API additions not specified by this book require a revision of this
    book rather than silently appearing in source.

---

# 25. Review points before revision 0.2

Most architecture in this revision follows directly from the core-allocation
decision. The following details should be verified/locked while implementation
begins:

1. exact canonical lower-level allocation error type used by stdlib
   materializers;
2. exact fallible `init(...)` spelling for `Builder`;
3. final consuming-receiver spelling for `Builder.Finish`;
4. exact core line-iterator semantics used by `SplitLines` and `Indent`;
5. whether `Builder.Capacity` should remain public performance information;
6. exact letter/digit boundary rule in `naming.sec`;
7. exact Sec string-literal escape grammar consumed by `escape.sec`;
8. whether full Unicode titlecasing requires a deeper reusable word-boundary
   iterator before `str.ToTitle` is implemented.

These are implementation/specification lock points, not reasons to weaken the
package architecture.

---

# 26. Summary

The package is intentionally rich, but the richness is layered.

The design principle is:

```text
compiler/core:
    know what a string and rune are
    inspect them
    classify them
    search them
    slice them
    iterate them
    do not allocate
    except the universal ToString() contract

stdlib/str:
    materialize
    construct
    transform
    edit
    join
    replace
    split into owning collections
    normalize
    case-convert
```

That division gives Sec both a capable low-level string model and a comfortable
high-level standard library without creating two competing definitions of
string semantics.
