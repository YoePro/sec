# Sec Standard Library — `fmt`

**Version:** 1  
**Date:** 2026-10-30  
**Status:** Foundation specification — implementation-oriented, evolving  
**Sec language version:** 0.1  
**Canonical path:** `sec/stdlib/fmt/std-fmt.md`  
**Module:** `fmt`  
**Implementation tracking:** MD-118, `governance/stdlib.yaml`  
**Repository reference:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (2026-10-29)  
**Previous material:** Supersedes the directory-only root book and our prior working drafts; does not change numbered paragraphs in Sec's canonical language rulebooks.  
**Revision policy:** Keep existing section identifiers stable. Add subsections or append sections when extending the book. Record changes in § 19.

> **Reading convention.** **Established** identifies an accepted project decision or a verified existing contract. **Version 1 proposal** specifies the first root API intended for review and implementation; it becomes binding when accepted. **Reserved** identifies planned capabilities that are **not** yet public declarations. This book does not claim that an empty `.sec` file implements its planned API.

---

## § 1. Purpose

**§ 1(1).** The `fmt` standard-library module provides type-safe, familiar text printing and composite formatting, then extends the same formatting model to memory buffers, Writers, scanning and richer presentation. The objective is a best-in-class standard library: comprehensive, efficient, predictable and pleasant to use without third-party packages for common formatting tasks.

**§ 1(2).** The first implementation milestone is deliberately narrow: **`fmt.Print`, `fmt.Println` and `fmt.Printf`**, together with the root types and internal infrastructure needed to implement them correctly. This is not a decision to restrict the eventual package to those three functions.

**§ 1(3).** The design combines useful characteristics of Go, Rust, C/C++, Zig, Odin and .NET without adopting their unrelated language or runtime semantics.

**§ 1(4).** Nonnegotiable quality requirements:

1. A single canonical formatter, not a separate implementation for every destination.
2. Ordinary Sec type checking, ownership, failure and effects; no hidden boxing or `any` conversion.
3. Explicit failures, including partial external writes and allocation errors.
4. Streaming without mandatory creation of a complete intermediate string.
5. Correct UTF-8 and unambiguous byte counts versus scalar or display widths.
6. Documented allocation behavior on every public operation.
7. Hosted and embedded profiles treated honestly: no imaginary stdout on targets that lack it.
8. A complete and discoverable public API; richer facilities in dedicated sub-books.

## § 2. Layer boundaries — established

**§ 2(1).** `core/format.sec` owns canonical value-format semantics. It currently declares `FormatAlignment`, `FormatSign` and `FormatSpec`, with the latter containing `Fill`, `Alignment`, `Sign`, `Alternate`, `ZeroPad`, `Width`, `Precision` and `Presentation`. Those declarations retain their existing identities and **are not redeclared here**. The code does not yet contain a complete general streaming formatting entry point.

**§ 2(2).** Core has privileged access to compiler-internal functions. Core **may** contain allocating operations when that privilege is essential; such operations are marked/documented as allocating and should remain few. No stdlib module gains compiler-only privileges through import.

**§ 2(3).** `string.FromByteArray(data: ref byte[]) Result[string, StringError]` belongs in core and uses `_stringFromUtf8(data)`. This is the accepted materialization boundary; `fmt` must neither duplicate it nor call its private intrinsic directly.

**§ 2(4).** `str.Builder` owns growable UTF-8 construction and storage. `fmt` may use it as a destination. The obsolete `fmt/string_builder.sec` is to be removed, not generalized or exposed as `fmt.StringBuilder`.

**§ 2(5).** `io` owns Reader, Writer, standard-stream and device/transport semantics. In particular:

```sec
interface Writer {
    mut fn Write(data: ref byte[]) Result[uint, IOError]
}
```

A successful `Write` can transfer fewer bytes than requested. A formatting adapter must handle that correctly.

**§ 2(6).** `byte.ToString(fmt: byte.ByteStringFormat)` already offers `Decimal`, `Hexadecimal`, `Octal` and `Binary`. The future `fmt/dump` provides offset, grouping, columns and ASCII presentation for **sequences**; it must not duplicate the single-byte conversion or allocate a string per byte when an efficient direct formatting path exists.

**§ 2(7).** `unicode`, `temporal`, core numeric types and other domain modules own their relevant facts and type semantics. `fmt` combines their formatted representations; it does not define a second Unicode database, calendar system or numeric conversion engine.

## § 3. Heterogeneous arguments — established MD-117

**§ 3(1).** Sec's general `[...Types]` type-pack facility and `...ref Types` parameter mode are the basis for fmt's heterogeneous arguments; no special `Printf` call syntax is required.

**§ 3(2).** Printing observes rather than consumes input values. Its first public API uses shared borrowed argument packs:

```sec
fn Example[...Types](values: ...ref Types) void {
}
```

The example merely illustrates an accepted parameter shape; it is not an additional fmt export.

**§ 3(3).** Arguments are evaluated exactly once, in source order, and their borrow/cleanup obligations follow `rules/declarations/functions.md` § 38A. The pack itself has no required heap allocation and does not escape the call.

**§ 3(4).** Each concrete specialization must have a valid way to produce the requested representation. Membership in a heterogeneous pack **does not** imply a universal structural `FormatArg` interface. If a type cannot be formatted, give a normal specialization diagnostic identifying the argument and its type.

**§ 3(5).** For Version 1 the guaranteed common path is the existing applicable `ToString() Result[string, StringError]`. It may allocate. The faster streaming path is a core-format extension and shall replace temporary materialization where available. Neither a nonallocating guarantee nor a new compiler-known format-call rule is invented here.

## § 4. Root layout and responsibilities

All root `.sec` files belong to `module fmt`. Files are implementation boundaries, not separate imports.

| File | Owner and intended exports | Maturity |
|---|---|---|
| `fmt.sec` | Module entry, existing `Println(string)` migration, shared internal dispatch | Present; migrate |
| `error.sec` | The root formatting/output error type | Version 1 proposal |
| `print.sec` | `Print`, `Println`, `Printf`, `Printfln` | First milestone |
| `stderr.sec` | `EPrint`, `EPrintln`, `EPrintf`, `EPrintfln` | Reserved next family |
| `writer.sec` | `FPrint`, `FPrintln`, `FPrintf`, `FPrintfln` | Reserved next family; internal Writer adapter needed first |
| `string.sec` | `SPrint`, `SPrintln`, `SPrintf`, `SPrintfln` | Reserved next family |
| `buffer.sec` | `Appendf`, `FormatTo`, `FormatToN` | Reserved |
| `measure.sec` | `FormattedSize` | Reserved |
| `scan.sec` | Root-facing `Scanf`, `Fscanf`, `Sscanf`; further `Scan*` candidates | Reserved; owned in detail by `scan/` |
| `options.sec` | Shared future output options, only when an exact need is identified | Reserved, no declarations |
| `compiled.sec` | Reuse of verified format descriptions, not a new format language | Reserved, no declarations |
| `string_builder.sec` | Old independent 512-byte stdout buffer | **Obsolete: remove** |

The six special-purpose areas stay separate:

| Sub-book | Focus |
|---|---|
| `scan/std-fmt-scan.md` | Typed format-controlled input, parsing and limits |
| `pretty/std-fmt-pretty.md` | Structured/debug output, depth and traversal controls |
| `table/std-fmt-table.md` | Cells, columns, rows and table rendering |
| `layout/std-fmt-layout.md` | Width, wrapping, alignment and display columns |
| `dump/std-fmt-dump.md` | Hex dumps and structured binary presentation |
| `style/std-fmt-style.md` | Styles, colors, optional ANSI presentation |

**§ 4(1).** These sub-books do not automatically export types from the `fmt` root. New public declarations must be complete and documented before being introduced.

**§ 4(2).** Root scanning may delegate to a parser under `fmt/scan`, but dependencies must not form a root↔submodule cycle. Exact module interfaces are specified when `scan` is implemented.

## § 5. First public error contract (`error.sec`) — Version 1 proposal

Core already defines the **complete** existing `StringError`, `FormatError` and `IOError` identities. Do not redeclare them in `fmt`.

```sec
/**
 * Reports a failure during formatted output to an external destination.
 *
 * Description:
 *   Preserves the original text/format failure or I/O failure without
 *   changing the semantics of the underlying error.
 *
 * Ownership:
 *   The error value owns its payload according to normal Sec union rules.
 */
type WriteFormatError union error {
    // Text conversion, invalid format or materialization failure.
    Text(StringError),

    // Failure reported by the selected output stream or Writer.
    IO(IOError),
}
```

**§ 5(1).** The variants above are the **complete proposed public type**, not placeholders. `Text` includes `StringError.Format(FormatError)` and `StringError.Allocation(AllocationError)` where appropriate; `IO` preserves the existing I/O error, including `NoProgress`.

**§ 5(2).** A successful print returns `Ok(numberOfUtf8BytesWritten)`. `Err` does **not** claim rollback or zero bytes written. Bytes already accepted by an external Writer may remain visible.

**§ 5(3).** Only introduce additional fmt-owned error types when another destination or operation genuinely requires a distinct contract. They are **not** part of the first public root API merely because `error.sec` exists.

## § 6. First public API (`print.sec`) — Version 1 proposal

The following are **complete proposed public signatures**. They are declaration excerpts, not bodyless Sec function definitions: real `.sec` source includes function bodies. The four operations form one coherent stdout family. **Implementation starts with the first three**, followed by `Printfln` using the same mechanism.

```sec
/**
 * Writes default representations of all arguments to stdout.
 *
 * Description:
 *   Concatenates argument representations in order, without inserting
 *   separators or a final newline. An empty call writes nothing.
 *
 * Parameters:
 *   - values: Shared borrows of zero or more independently typed values.
 *
 * Returns:
 *   Number of UTF-8 bytes successfully written, or zero when empty.
 *
 * Errors:
 *   - WriteFormatError.Text: A value could not be represented as text.
 *   - WriteFormatError.IO: The output stream failed or made no progress.
 *
 * Ownership:
 *   Does not copy or consume the original values merely for printing.
 *
 * Allocation:
 *   May allocate when a concrete value needs its allocating ToString().
 *   Does not require a complete intermediate string for all arguments.
 */
fn Print[...Types](values: ...ref Types) Result[uint, WriteFormatError]

/**
 * Writes default argument representations separated by spaces and then LF.
 *
 * Description:
 *   Inserts exactly one U+0020 SPACE between adjacent argument values.
 *   Appends one U+000A LINE FEED, including for an empty call.
 *
 * Parameters:
 *   - values: Shared borrows of zero or more independently typed values.
 *
 * Returns:
 *   Complete UTF-8 byte count including LF on success.
 *
 * Errors:
 *   - WriteFormatError.Text: Text representation failed.
 *   - WriteFormatError.IO: Stream failure or no progress.
 *
 * Ownership:
 *   Does not consume or mutate input values.
 *
 * Allocation:
 *   May allocate for particular ToString() implementations.
 */
fn Println[...Types](values: ...ref Types) Result[uint, WriteFormatError]

/**
 * Writes values according to an explicit composite format to stdout.
 *
 * Description:
 *   The first parameter is always a format, never ordinary printed data.
 *   Version 1 supports the composite baseline defined in § 8.
 *
 * Parameters:
 *   - format: UTF-8 composite format, not a C printf percent format.
 *   - values: Shared borrows of the referenced typed arguments.
 *
 * Returns:
 *   Complete UTF-8 byte count on success; no implicit newline.
 *
 * Errors:
 *   - WriteFormatError.Text: Invalid format, unsupported presentation,
 *     or failed conversion/materialization.
 *   - WriteFormatError.IO: Output failure or no progress.
 *
 * Ownership:
 *   Values are evaluated once and borrowed for the call.
 *
 * Allocation:
 *   May allocate for type-specific fallback materialization, not for a
 *   mandatory whole-result string.
 */
fn Printf[...Types](format: string, values: ...ref Types) Result[uint, WriteFormatError]

/**
 * Writes an explicit composite format followed by one LF.
 *
 * Parameters:
 *   - format: Composite format interpreted as in Printf.
 *   - values: Shared borrows of the referenced typed arguments.
 *
 * Returns:
 *   Complete UTF-8 byte count including LF on success.
 *
 * Errors:
 *   - WriteFormatError.Text: Formatting/text failure.
 *   - WriteFormatError.IO: Stream failure or no progress.
 *
 * Ownership:
 *   Does not consume or mutate source values.
 *
 * Allocation:
 *   Same conditions as Printf.
 */
fn Printfln[...Types](format: string, values: ...ref Types) Result[uint, WriteFormatError]
```

**§ 6(1).** The `f` suffix is retained for familiarity. It identifies an **explicit format description**; `Print`/`Println` never interpret user-supplied text as a format program.

**§ 6(2).** `Print`, `Println`, `Printf` and `Printfln` are separate operations with no hidden content-based mode selection. Format errors are reported through `Result` rather than printed as mysterious substitutions such as `%!…`.

**§ 6(3).** There is no implied `Flush` operation or terminal style in any of these functions. A newline is LF, not automatic CRLF.

## § 7. Exact plain-output behavior — Version 1 proposal

**§ 7(1).** `Print()` produces no bytes and returns `Ok(0)`.

**§ 7(2).** `Println()` produces the single byte `0A` (LF) and returns `Ok(1)`.

**§ 7(3).** `Print("A", "B")` produces `AB` with byte count `2`. `Println("A", "B")` produces `A B\n` with byte count `4`.

**§ 7(4).** Plain `string` values are printed without added quotes or escaping. A `rune` presents its scalar as text under the canonical rune representation. A `byte` is **not** implicitly converted into an ASCII or UTF-8 character; its default textual representation follows core.

**§ 7(5).** Formatter output contains exactly the selected text; no implicit trimming, normalization, color codes, locale conversion, timestamps or terminal-width adjustments.

**§ 7(6).** Unsupported types fail at concrete generic specialization when no applicable formatting operation exists. This is a compile-time type error, not a runtime silent fallback or implicit dump of private fields.

**§ 7(7).** Byte counts refer to **UTF-8 bytes actually written**, not `string.Len` (Unicode scalars). For a complete output, return bytes including separators and LF. The output sink may chunk writes without changing the text.

## § 8. Initial composite format for `Printf` — Version 1 proposal

**§ 8(1).** The proposed canonical format language is a composite field language, **not** the `%d/%s` grammar of C. The familiar `Printf` name does not require C's unsafe variadic conversion model. **Core owns this format language and its parser/validator; `fmt` is a client of that core contract.** Version 1 specifies the smallest semantic subset required by `Printf`, not an independent `fmt` parser. The notation below changes no Sec source-language syntax.

**§ 8(2).** The initial *implementable subset* is:

```text
Format       := (Literal | EscapedOpen | EscapedClose | AutoField | IndexField)*
EscapedOpen  := "{{"        // emits one literal "{"
EscapedClose := "}}"        // emits one literal "}"
AutoField    := "{}"        // next argument, from index 0
IndexField   := "{" Digits "}"   // explicit zero-based argument index
Digits       := ASCII decimal digits (one or more)
Literal      := ordinary format text excluding unescaped "{" or "}"
```

**§ 8(3).** Auto fields use arguments in order. Indexed fields may reuse an argument but never reevaluate its source expression. **Do not mix auto and indexed fields** in one format. All arguments must be referenced at least once; extra arguments, missing arguments and out-of-bounds indexes are format errors. (This strictness is proposed for early bug detection; record feedback before locking it.)

**§ 8(4).** A lone `{` or `}` is invalid, while `{{` and `}}` are literal braces. `""` is a valid format if the pack is empty. Repetition, escaped braces and literal Unicode text must preserve exact code-point content without normalization.

**§ 8(5).** Reserved for a later core-format increment: `{0:08X}`, alignment, fill, sign, width, precision and type-specific presentation. Those must reuse `core.FormatSpec` rather than defining a second parser or incompatible interpretation in `fmt`. Until that contract is complete, a field with `:` must report an unsupported/invalid format according to the existing `FormatError` contract; it must **not** be silently treated as ordinary text.

**§ 8(6).** `Printf` must validate the complete **dynamic format's structure and argument indexes before beginning external writes**, using the canonical core format-validation path. The core contract may validate with a pass that does not materialize the full output. An invalid format alone must therefore leave the destination untouched. A later value-conversion or I/O failure can leave partial output, as § 10 defines.

**§ 8(7).** Formats statically known to the compiler should eventually receive compile-time diagnostics through an accepted general validation mechanism. MD-117 already handles heterogeneity; this book does **not** create special name-based compiler behavior for `fmt.Printf`. Runtime validation is mandatory, even if compiler diagnostics have not yet been implemented.

**§ 8(8).** Invalid syntax or argument addressing uses `StringError.Format(FormatError.InvalidFormat)`; malformed presentation uses the appropriate existing `FormatError` variant once the presentation syntax is introduced. This library does not invent new `FormatError` variants to handle the baseline.

**§ 8(9).** Illustrative calls, assuming their concrete argument types provide `ToString()`:

```sec
let bytesWritten := try fmt.Printf("Hello {}, you have {} messages", name, count)
let second := try fmt.Printf("{1} / {0} / {1}", left, right)
let braces := try fmt.Printf("{{{0}}}", value)
let lineBytes := try fmt.Println("Ready:", count)
```

The expression `{0:08X}` is **future functionality**, not an implementation claim for the initial milestone.

## § 9. Formatting participation and storage

**§ 9(1).** Version 1's baseline participation rule uses the applicable `ToString() Result[string, StringError]`, including built-in types and explicitly supported user types. No arbitrary structural reflection and no private-field access are introduced. If only a materializing `ToString` exists, its allocation and failure behavior remains observable.

**§ 9(2).** The larger design shall use one core-owned formatting engine/sink contract for direct streaming. The exact future sink signature is **not invented** here; `core/format.sec` currently has no completed interface. `fmt` will adapt that engine to Writers and byte buffers when available.

**§ 9(3).** As an interim implementation technique, an individual argument may be converted with `ToString()` and its UTF-8 bytes sent to the output sink, then the temporary destroyed before proceeding. This is an implementation fallback, not a second format engine or a zero-allocation guarantee.

**§ 9(4).** The `str.Builder` remains the shared owning text-buffer facility. Do not expose the historical fixed-size `fmt.StringBuilder`, and do not add `AppendFormat` to `str.Builder` merely to make fmt convenient; future `fmt.Appendf` owns that operation.

**§ 9(5).** Output transport works with bytes. String length semantics matter: `string.Len` counts Unicode scalars; `string.SizeOf` counts UTF-8 bytes. Never call `io.Write` with `.Len` as though it were byte length.

## § 10. Writer and stream behavior — first implementation dependency

**§ 10(1).** Stdout and stderr are target capabilities. The standard library must not use a hidden host fallback on freestanding/embedded targets. When a target cannot provide the stream, use its established capability restriction or return an appropriate `IOError.Unsupported` through `WriteFormatError.IO` when the call remains available.

**§ 10(2).** Every output adapter shall correctly handle a successful short write by continuing with unwritten bytes. A permanent lack of progress is reported as `IOError.NoProgress`; the exact retry policy follows the common `io.WriteAll` contract, not a second fmt-specific I/O algorithm.

**§ 10(3).** A Writer that reports more bytes than offered violates its contract. Do not silently add that number to the success count; follow the common `io` contract-violation policy rather than fabricating successful output.

**§ 10(4).** If an error occurs after earlier successful chunks, preserve the underlying error. No rollback of already accepted external bytes is promised. The `Result[uint, …]` success count is available only on full success.

**§ 10(5).** The existing Linux-specific `io.Println(value: string) void` forwards a raw syscall, ignores its status and uses `value.Len` for the write length. It is **not** adequate for the new fmt contract: it neither reports errors nor guarantees correct multibyte UTF-8 output. Use or introduce a properly specified stdout Writer/adapter; do not copy the old behavior.

**§ 10(6).** The current `fmt/fmt.sec` contains only that legacy `Println(value: string) void` forwarding call. Its migration to the new `Println[...Types](...) Result[uint, WriteFormatError]` requires changing call sites to handle results explicitly. Do not keep a conflicting overload solely to discard errors.

## § 11. Later root output APIs — reserved names and signatures

The following are **planned root names** for the same formatter semantics, with full signature shapes given to keep our file responsibilities coherent. They are **not yet approved public exports**; exact error/result types for the non-Writer destinations remain to be specified before implementation. They are presented as a **catalogue**, not incomplete Sec declarations.

| File | Names | Target / expected behavior |
|---|---|---|
| `stderr.sec` | `EPrint`, `EPrintln`, `EPrintf`, `EPrintfln` | stderr; otherwise matches matching Print family |
| `writer.sec` | `FPrint`, `FPrintln`, `FPrintf`, `FPrintfln` | user-supplied `io.Writer`; generic `W: io.Writer` path |
| `string.sec` | `SPrint`, `SPrintln`, `SPrintf`, `SPrintfln` | valid UTF-8 `string` result; may allocate; return `StringError` |
| `buffer.sec` | `Appendf` | incrementally extend existing `str.Builder` |
| `buffer.sec` | `FormatTo`, `FormatToN` | exact and deliberately bounded formatting into caller storage |
| `measure.sec` | `FormattedSize` | exact size in UTF-8 bytes for stable formatter |
| `scan.sec` | `Scanf`, `Fscanf`, `Sscanf` | format-directed typed scanning from stdin, Reader, string |

**§ 11(1).** `Scan`, `Scanln`, `Fscan`, `Fscanln`, `Sscan`, `Sscanln` remain candidates, not promised exports. The `scan` sub-book will decide whether ordinary token scanning belongs beside its formatted variants.

**§ 11(2).** `options.sec` has no public type in Version 1. `compiled.sec` has no public type in Version 1. We will add complete declarations when those contracts have actually been determined rather than filling source files with fabricated APIs.

**§ 11(3).** Future result types for capacity-limited formatting, size overflow and scanning must not be invented by analogy alone. Their complete variants/fields, ownership and error behavior will be specified when those APIs enter implementation.

## § 12. Sub-book integration and advanced capabilities

**§ 12(1).** `fmt/pretty` will describe human-readable and debug-formatted structures with explicit visibility, recursion, cycle, element-count and output-size limits.

**§ 12(2).** `fmt/table` and `fmt/layout` will cover multilingual column alignment, multiline cells, wrapping, width negotiation and large/streaming datasets. Terminal display width is distinct from UTF-8 byte count, Unicode scalar count and grapheme count. The relevant Unicode library must provide reliable support before claiming display-correctness.

**§ 12(3).** `fmt/dump` will provide structured views of bytes and offsets. It reuses native byte hex semantics rather than repeatedly calling an allocating `byte.ToString()` for every byte.

**§ 12(4).** `fmt/style` will provide explicit, opt-in styles and terminal adaptation. Plain Print* never injects ANSI sequences, guesses whether it is attached to a terminal or performs cursor control.

**§ 12(5).** `fmt/scan` will provide strict typed scanning with bounds, clear EOF/mismatch distinctions, no unsafe C-style destination pointers and no silent partial update of typed results. Its root API is reserved until the sub-book settles the full contract.

## § 13. Root documentation contract

**§ 13(1).** Every public `.sec` declaration must have an LSP-readable `/** ... */` comment immediately before the declaration. Include its concise purpose, `Description` where needed, `Parameters`, `Returns`, `Errors`, `Ownership`, `Allocation` and `Standards`/`Rulebook` references where relevant. Enum/union variant comments appear **before** each member.

**§ 13(2).** Each source file must have a source header identifying canonical path, module, purpose, version/date, allocation and normative reference. An existing `module fmt` line is a scaffold, not a completed file.

**§ 13(3).** Do not use pseudo-constructors (`static fn New`) or omit public enum variants. Real constructors use Sec's approved `init(...)` forms. No new constructor is introduced by the first root print API.

**§ 13(4).** Changes to existing numbered Sec rulebooks must preserve paragraph references. This document does not reopen MD-117 or change ownership, lexing, generics or compiler permissions.

## § 14. Required tests for `Print`, `Println`, `Printf`

The table describes **implementation acceptance tests**, not tests already passing.

| Area | Test requirement |
|---|---|
| Heterogeneous calls | Empty pack, one value, `(string, int, bool)`, user-defined `ToString`, move-only borrowed value |
| Plain text | `Print()` empty; adjacent values with no separator; `Println()` LF; one space between arguments; double existing trailing LF not trimmed |
| Format fields | `{}`, multiple `{}`, `{0}` and `{1}`, repeated index without reevaluation, escaped braces, empty format |
| Invalid format | Lone brace, index out of range, mixed auto/indexed, too few and extra arguments; no output on structural error |
| Conversion failures | `ToString()` failure preserved as `WriteFormatError.Text`; no accidental consumption |
| UTF-8 | Multibyte strings and runes, byte counts based on encoded length, no truncation by scalar count |
| Destination | Short write, `Ok(0)` progress exhaustion, I/O error after partial output, invalid returned write count |
| Effects and ownership | Left-to-right exactly once, borrowed arguments, correct destruction of temporaries, no pack escape |
| Portability | Target without stdout, valid alternative Writer path once exported |
| Tooling | Invalid format locations, concrete-type errors, complete signature/help documentation |

**§ 14(1).** Explicitly test that the implementation does not accidentally call the old syscall-based `io.Println` as the final output contract.

**§ 14(2).** Ensure the initial implementation does **not** claim complete support for `FormatSpec` advanced fields until those contracts and tests are implemented.

## § 15. Implementation sequence — small verified steps

Implement and test each step before moving to the next. Follow existing `governance/stdlib.yaml` ownership instead of creating duplicate progress IDs.

1. **Error and stream foundation:** Add the complete `WriteFormatError` definition in `error.sec`; verify correct `io.Writer` reuse and that stdout has an error-aware adapter with UTF-8 byte counts. Keep legacy migration separate.
2. **Plain value formatter:** Implement the smallest type-checked, borrowed `ToString()` fallback. Verify it works for strings, integers, booleans and a user-defined type. Do not invent an erased `FormatArg`.
3. **`Print`:** Implement ordered concatenation and byte counts; tests for empty/mixed values, Unicode and failed writes.
4. **`Println`:** Reuse the same path with exactly one space between arguments and a final LF; tests for empty and nested/newline content.
5. **`Printf`:** Complete the minimal canonical **core** validation path for § 8's fields and escaped braces; let `fmt` use that validation and stream the results. Verify structural invalidity causes no prior write, with no second parser in `fmt`.
6. **`Printfln`:** Reuse `Printf` and append a single LF; verify the resulting byte count and failure behavior.
7. **Broaden destination adapters:** Expose `FPrint*`, `EPrint*`, `SPrint*` using the **same** formatter and agreed error contracts; do not repeat format parsing in each file.
8. **Core formatter integration:** Move built-in value rendering toward the canonical `core/format` streaming contract when that contract becomes complete; keep results byte-identical to the baseline.
9. **Broaden remaining root API:** Specify `Appendf`, `FormatTo`, `FormatToN`, `FormattedSize` and scanning in complete detail and only then publish them.
10. **Cleanup:** Remove the obsolete `fmt/string_builder.sec`, migrate the old `fmt.Println(string) void` and remove any unreferenced direct-syscall print implementation once adapters replace it.

**§ 15(1).** A compiler test passing for a generic type pack is not evidence that the full `fmt` implementation exists. Mark separate status for type-pack support, formatter logic, Writer integration and actual call-level tests.

## § 16. Known dependencies and discrepancies

**§ 16(1).** `rules/declarations/functions.md` § 38A (MD-117) now defines the heterogeneous generic pack syntax, although its parser/Sema/IR and lowering support is not necessarily fully implemented. This is a **dependency**, not another language-design problem for `fmt`.

**§ 16(2).** `core/format.sec` has only the initial specification data structures. It does not yet expose the canonical composite-format validation or general formatter-to-sink implementation interface. These are core prerequisites for the full `Printf` contract; `fmt` must not fill the gap with a competing permanent parser. The short-term `ToString` value-format path is legitimate, but performance claims must stay truthful.

**§ 16(3).** `str.Builder` is nearly complete but its book and current source have some constructor/`Finish` inconsistencies. Resolve those in `str`/core when moving to allocating output, not through an alternate fmt Builder.

**§ 16(4).** The existing core library book still contains older wording about byte decoding in places, while the accepted `string.FromByteArray` core implementation exists. The new core privilege decision governs the boundary; synchronize the old text in its owning book separately.

**§ 16(5).** The new root files pushed to `main` on 2026-10-29 contain only `module fmt`. `std-fmt.md` on that commit contains only a directory tree. This Version 1 book replaces that skeleton; neither the empty files nor this book establishes implemented status.

## § 17. Questions consciously left open

Only these are **future design work**, not excuses to change accepted MD-117 rules:

| ID | Topic | Why it is not guessed in Version 1 |
|---|---|---|
| FMT-V1-01 | Full core streaming formatter/sink signature | No complete canonical core declaration exists yet |
| FMT-V1-02 | Advanced composite-format specifiers and presentation mapping | Must be consistent with core `FormatSpec`, numeric `ToString` and byte formatting |
| FMT-V1-03 | Compile-time literal format validation integration | Requires a normal, traceable validation contract, not an ad hoc name check |
| FMT-V1-04 | Detailed stdout/stderr target-capability interface | Must follow `io` and target profile contracts |
| FMT-V1-05 | Detailed scanning results, limits, mutation and errors | Owned by `fmt/scan` book |
| FMT-V1-06 | Exact buffer, measure and compiled format APIs | Need independent capacity and error contracts |

**§ 17(1).** These identifiers are internal cross-references in this book, **not** new governance or missing-decision IDs. MD-118 is the existing root fmt design owner.

**§ 17(2).** The API names, byte-count conventions and baseline brace behavior in §§ 5–8 are a concrete Version 1 **proposal** for implementation. They must be reviewed before being treated as a newly adopted language- or stdlib-wide normative decision.

## § 18. References

### § 18.1 Sec source and rulebooks

- [`rules/library/stdlib.md`](https://github.com/YoePro/sec/blob/main/rules/library/stdlib.md) — stdlib boundary and MD-118 ownership.
- [`rules/declarations/functions.md` § 38A](https://github.com/YoePro/sec/blob/main/rules/declarations/functions.md) — heterogeneous packs, borrow/transfer rules.
- [`rules/declarations/generics.md` § 16A](https://github.com/YoePro/sec/blob/main/rules/declarations/generics.md) — pack inference and specialization.
- [`rules/library/core-library.md` § 15](https://github.com/YoePro/sec/blob/main/rules/library/core-library.md) — `ToString`, `StringError`, `FormatError`.
- [`sec/core/format.sec`](https://github.com/YoePro/sec/blob/main/sec/core/format.sec) — existing core format data types.
- [`sec/core/string.sec`](https://github.com/YoePro/sec/blob/main/sec/core/string.sec) — `string.FromByteArray` / private core materialization.
- [`sec/core/byte.sec`](https://github.com/YoePro/sec/blob/main/sec/core/byte.sec) — native byte display and hexadecimal.
- [`sec/core/error.sec`](https://github.com/YoePro/sec/blob/main/sec/core/error.sec) — canonical text, formatting and I/O errors.
- [`sec/stdlib/str/stdlib_str.md` § 8](https://github.com/YoePro/sec/blob/main/sec/stdlib/str/stdlib_str.md) — `str.Builder` ownership.
- [`sec/stdlib/io/std-io.md` §§ 6–8](https://github.com/YoePro/sec/blob/main/sec/stdlib/io/std-io.md) — Reader, Writer and short-write behavior.
- [`rules/memory/allocation.md`](https://github.com/YoePro/sec/blob/main/rules/memory/allocation.md) — permitted allocation contexts and effects.
- [`missing-decisions.yaml`](https://github.com/YoePro/sec/blob/main/missing-decisions.yaml) — MD-118 and related status.

### § 18.2 External design references

- Go `fmt`: https://pkg.go.dev/fmt
- Rust `std::fmt`: https://doc.rust-lang.org/std/fmt/
- C++ formatting: https://en.cppreference.com/w/cpp/utility/format
- ISO C `printf`: https://en.cppreference.com/w/c/io/fprintf
- Zig `std.fmt`: https://github.com/ziglang/zig/blob/master/lib/std/fmt.zig
- Odin `core:fmt`: https://pkg.odin-lang.org/core/fmt/
- .NET composite formatting: https://learn.microsoft.com/en-us/dotnet/standard/base-types/composite-formatting
- UTF-8 / RFC 3629: https://www.rfc-editor.org/rfc/rfc3629
- Unicode UAX #29: https://unicode.org/reports/tr29/
- Unicode UAX #11: https://unicode.org/reports/tr11/

## § 19. Document history

| Version | Date | Change |
|---|---|---|
| **1** | **2026-10-30** | Initial standalone implementation-oriented root book, with `Print`, `Println`, `Printf` first; complete first error type, canonical core-format baseline, root catalogue, integration boundaries, tests and implementation order. |

---

**End of Version 1.** This is the starting point for successive revisions. Public API decisions and implemented status are tracked separately; extending the book does not silently change earlier accepted semantics.
