# MD-109 — Complete correction: Core-owned string materialization

| Field | Value |
|---|---|
| **Status** | Implementation requested by owner 2026-10-25 — pending complete application |
| **Created** | 2026-10-25 |
| **Updated** | 2026-10-25 |
| **Revision** | 1.0 — complete self-contained replacement draft |
| **Sec version** | 0.1 |
| **Canonical destination** | `rules/corrections/md109-core-string-materialization-correction.md` |
| **Replaces** | All earlier *drafts of this MD-109 correction* (including revisions 0.1 and 0.2); does **not** replace an applied rulebook |
| **Decision authority** | Sec owner, 2026-10-25 |
| **Implementation target** | `https://github.com/YoePro/sec`, `main` |
| **Related decisions** | MD-050, MD-072, MD-091, MD-109; relevant core allocation and compiler-known-member rules |
| **Implementation status** | Phase 1 inventory and phase 2 frontend/static API implemented and verified 2026-10-25; phase 3 runtime conformance blocked by the existing P23/P18/P19 capability boundary; phases 4–7 pending |

> **MANDATORY EXECUTION RULE FOR CLAUDE/CODEX:** Perform **one numbered implementation phase at a time**, run its checks, inspect failures, fix them, and only then start the next phase. Do not execute a large batch of modifications followed by a single final test. Read the latest `main` HEAD and compare its sync identifier before touching repository files. Preserve existing numbered paragraphs and governance IDs. Do not declare MD-109 resolved before code, tests, documentation, and governance are synchronized.

---

## 1. Purpose and exact resolution of MD-109

**1.1.** MD-109 exists because standard-library code cannot legally invoke the privileged bytes-to-string materializer, yet string-producing standard-library operations must be able to turn validated UTF-8 bytes into a Sec `string`. The core allocation prohibition introduced an artificial boundary that left `str.FromByteArray`, `str.Builder.Finish`, and other dependent call sites without an implemented public route.

**1.2.** This correction defines that route as two **public static associated functions on built-in `string`**, implemented in trusted `core/string`:

```sec
static fn FromByteArray(data: ref byte[]) Result[string, StringError]
static fn FromRuneArray(data: ref rune[]) Result[string, StringError]
```

**1.3.** These functions create strings from source data. They do **not** operate on an existing `string` instance, so `static` is required. `static` does not affect input ownership.

**1.4.** Both arrays are borrowed through `ref`. The input is neither moved nor consumed, and there is **no `<-` marker** for a named argument passed to either function. The caller retains its array after success or error. Do not add a consuming overload under this correction.

**1.5.** Core is henceforth **allocation-averse, not allocation-forbidden**. Core should normally prefer no-allocation algorithms, but may allocate for fundamental built-in type semantics, representation-sensitive materialization, and necessary compiler/core boundaries when the API documents effects, ownership, failure, and target constraints. This change does not make core a general high-level utilities package.

**1.6.** The public materialization methods are distinct from the private operation that actually constructs a trusted string representation. `_stringFromUtf8` (compiler-known identity `CKF-STRING-FROM-UTF8`, or its actual current canonical spelling) remains inaccessible from ordinary stdlib code. **Never** relax that visibility to solve MD-109.

**1.7.** A `string` remains valid UTF-8 only. This correction does not authorize binary strings, lossy default decoding, zero-terminated decoding, Latin-1 reinterpretation, or new Unicode normalization behavior.

---

## 2. Normative public API and error type

**2.1.** The canonical surface in `sec/core/string.sec` is:

```sec
impl string {
    static fn FromByteArray(data: ref byte[]) Result[string, StringError]
    static fn FromRuneArray(data: ref rune[]) Result[string, StringError]
}
```

This block specifies public signatures. **The actual source declarations must have working bodies**, not bodyless placeholders or `panic`/`TODO` fallbacks.

**2.2.** A call is written using the type name, never a receiving instance:

```sec
let text := try string.FromByteArray(bytes)
let other := try string.FromRuneArray(runes)
```

Apply the canonical Sec reference-argument call syntax for `ref` parameters; do not invent extra borrowing syntax or make a named argument consuming. Calls through `someString.FromByteArray(...)` / `someString.FromRuneArray(...)` must be diagnosed as invalid static-member usage.

**2.3.** Both return the **existing** `StringError`. Its compiler-known public declaration is retained exactly as already owned by the canonical core rulebook (including members unrelated to these functions):

```sec
type StringError union error {
    Allocation(AllocationError),
    Format(FormatError),
    InvalidUtf8(uint),
    InvalidCodePoint,
    OutOfBounds,
}
```

The implementation must not create `Utf8Error`, `DecodeError`, another `StringError`, or a new allocation-error family.

**2.4.** Normal failure variants for each function:

| Function | Normal externally reachable failure |
|---|---|
| `FromByteArray` | `StringError.InvalidUtf8(uint)`; `StringError.Allocation(AllocationError)` |
| `FromRuneArray` | `StringError.Allocation(AllocationError)` |

**2.5.** Every legal Sec `rune` represents a valid Unicode scalar. Safe input to `FromRuneArray` cannot produce `InvalidCodePoint`: this pre-existing variant is **not** removed from `StringError` simply because this function cannot produce it in safe code. A forged invalid rune inside broken privileged code is a violated representation invariant, not an ordinary public decoding error.

**2.6.** Storage-size arithmetic, allocation effects, rollback/cleanup, and availability in a selected target's allocation context follow existing canonical allocation rules. Do not add a hidden default heap allocator to make the functions succeed everywhere.

---

## 3. Complete required source/LSP comment — `FromByteArray`

**3.1.** Place the following structured comment immediately **before** the real public declaration in `sec/core/string.sec`. The body that follows must implement sections 4–5.

```sec
/**
 * Creates an immutable string from strictly validated UTF-8 bytes.
 *
 * Description:
 *   Reads the entire represented byte array and rejects malformed UTF-8.
 *   No replacement characters, truncation, encoding fallback, normalization,
 *   terminator handling, or case conversion are performed. This is a static
 *   associated function and has no string receiver.
 *
 * Parameters:
 *   - data: Borrowed, read-only byte array containing the complete UTF-8
 *     sequence to decode. Every represented byte, including 0x00, is data.
 *
 * Returns:
 *   An independently owned, immutable string containing exactly the Unicode
 *   scalar sequence represented by the input bytes. An empty array returns
 *   an empty string.
 *
 * Errors:
 *   - StringError.InvalidUtf8(offset): The first malformed or truncated UTF-8
 *     sequence; offset is the zero-based start byte index of that sequence.
 *   - StringError.Allocation(cause): String storage could not be materialized.
 *
 * Ownership:
 *   Borrows data for this call only. Does not consume, modify, adopt, or
 *   retain a mutable alias to the caller's byte array. The caller retains the
 *   array. Result validity never depends on the array's subsequent lifetime.
 *
 * Allocation:
 *   May allocate result storage through the permitted allocation context.
 *   Failure produces StringError.Allocation; no partial string is returned.
 *
 * Standards:
 *   - RFC 3629, UTF-8, sections 3 and 4.
 *   - The Unicode Standard, UTF-8 encoding form.
 */
static fn FromByteArray(data: ref byte[]) Result[string, StringError] {
    // Actual implementation is required here; no placeholder in final source.
}
```

**3.2.** Names in the comment must match the actual parameter. If the implementation changes an internal local identifier, do not change this public contract silently.

---

## 4. `FromByteArray`: strict UTF-8 contract

**4.1.** Examine exactly the number of bytes represented by `data`. A NUL byte (`0x00`) is U+0000 and is **not** a terminator. A final NUL is not stripped. A leading UTF-8 BOM, when represented by U+FEFF, is preserved as text rather than implicitly discarded.

**4.2.** Accept only canonical UTF-8 encoding of Unicode scalar values: 1-, 2-, 3-, and 4-byte forms, including supplementary-plane scalars. The following must be rejected:

1. continuation bytes used as initial bytes;
2. invalid leading bytes (`C0`, `C1`, `F5`–`FF`, etc.);
3. missing or invalid continuation bytes;
4. overlong encodings;
5. UTF-8 encodings of surrogate code points (`U+D800`–`U+DFFF`);
6. encodings above `U+10FFFF`;
7. incomplete sequences at the end of input.

**4.3.** Report `Err(StringError.InvalidUtf8(offset))` at the **zero-based starting byte** of the first malformed or truncated sequence in the input. For a byte that is invalid immediately, that byte's own position is the offset. Do not return an index measured in runes or the position of the first *later* failed continuation byte if the malformed sequence began earlier.

**4.4.** An invalid input produces no string and causes no caller-visible mutation. Do not panic, replace bytes with U+FFFD, skip bytes, coerce to Latin-1, or return a partial valid prefix.

**4.5.** The source bytes may be modified or the source array freed **after** a successful call without changing or invalidating the returned string. Source storage and result storage cannot share a mutable alias. Use an independent allocation or another proven independent immutable-ownership mechanism that preserves this observable guarantee.

**4.6.** On an allocation failure, return the existing `StringError.Allocation(cause)`, clean up any intermediates exactly once, and preserve the borrowed source array unchanged. Check size calculations for overflow rather than allowing unchecked wraparound or undersized allocation.

---

## 5. `FromByteArray`: implementation boundary and repair

**5.1.** Read the actual current `sec/core/string.sec` before modifying it. The public repository version previously inspected contains a largely complete strict UTF-8 scan ending in:

```sec
return value.ToString()
```

That last statement is **not an acceptable trusted-core materialization step under the MD-091 type-summary decision**. Reuse valid scanning/validation logic, but replace this final operation with real trusted-core creation of immutable string storage.

**5.2.** Find the *actual current* declaration/lowering for `_stringFromUtf8` / `CKF-STRING-FROM-UTF8` in the compiler-known registry and trusted-core loader. Use its genuine signature; the correction intentionally does **not** invent an intrinsic call expression. If that mechanism does not yet support producing independently owned strings from validated borrowed bytes, implement the missing compiler/core backing contract using its existing authority boundary **before** exposing the public function as working.

**5.3.** Only loader-proven, trusted core may invoke the private operation. Ordinary `stdlib/str`, `stdlib/io`, `stdlib/net/url`, and user source must not gain access by changing import names, package names, paths, or identifier prefixes.

**5.4.** The safest initial implementation order is **validate first; materialize second**. Avoid allocating a result before detecting malformed input when the existing scanning path supports this. An implementation may later fuse passes for efficiency only if it preserves the same error priority, offsets, ownership, and failure guarantees.

**5.5.** The final method must be `static`, have `data: ref byte[]`, and use `data` in its public documentation. Do not accidentally leave a second public old signature with `value: ref byte[]` or the earlier non-static member in source/compiler registry. A *private* local variable name is not part of the API.

---

## 6. Complete required source/LSP comment — `FromRuneArray`

**6.1.** Place the following comment immediately before the complete implementation in `sec/core/string.sec`.

```sec
/**
 * Creates an immutable UTF-8 string from Unicode scalar values.
 *
 * Description:
 *   Encodes every input rune in order using canonical UTF-8. The operation
 *   does not normalize, change case, add a terminator, or inspect a receiver.
 *   Every rune validly constructible in safe Sec is a Unicode scalar value.
 *
 * Parameters:
 *   - data: Borrowed, read-only array of valid Sec rune values.
 *
 * Returns:
 *   An independently owned immutable string representing all input runes in
 *   order. An empty rune array returns an empty string.
 *
 * Errors:
 *   - StringError.Allocation(cause): String storage could not be materialized.
 *
 * Ownership:
 *   Borrows data without consuming or mutating it. The caller retains the
 *   array, and the returned string remains valid after the array is changed,
 *   moved, or destroyed according to normal ownership rules.
 *
 * Allocation:
 *   May allocate result storage through the permitted allocation context.
 *
 * Standards:
 *   - The Unicode Standard, Unicode scalar values and UTF-8 encoding form.
 */
static fn FromRuneArray(data: ref rune[]) Result[string, StringError] {
    // Actual implementation is required here; no placeholder in final source.
}
```

**6.2.** The example above is a documentation template: the final function must contain working code rather than this placeholder comment.

---

## 7. `FromRuneArray`: implementation contract

**7.1.** Encode runes in their input order as shortest-form UTF-8. Do not truncate to 8 bits, reinterpret rune memory as UTF-8 bytes, insert a terminator, normalize text, or special-case scripts.

**7.2.** UTF-8 output length is the checked sum of each rune's encoded length. Use the existing canonical `rune.Utf8Length`/encoder facilities where their implementation is correct; do not derive a new incompatible UTF-8 encoding rule. Reject storage-size overflow through the canonical allocation/error contract.

**7.3.** The function may use a temporary owned byte buffer followed by the trusted-core string materializer, or a direct trusted-core allocation/encoding path. Either is acceptable if it does not depend on ordinary stdlib and preserves error cleanup and ownership. Avoid an unnecessary *second* allocation when a supported safe core mechanism can produce the string in one pass.

**7.4.** The current observed core body is a stub that returns `Ok("")` irrespective of input. Remove it. The empty-result optimization is valid **only when the input is actually empty**.

**7.5.** The public safe-code path has no meaningful `InvalidCodePoint` error because the language disallows invalid `rune` values. Do not reintroduce surrogate runes as ordinary values or add an always-false `IsSurrogate` prerequisite to this function.

**7.6.** Borrowed inputs remain unmodified and usable after success or failure. The returned string must be independent of the mutable source array, and any error path must release temporary encoding storage and report `StringError.Allocation` where allocation failed.

---

## 8. Separate `ToString()` semantics from materialization

**8.1.** `string.FromByteArray(data)` is the explicit **strict UTF-8 decoder/materializer**. `byte[].ToString()` is **not** its implementation substitute.

**8.2.** MD-091 records the later decision that `byte[].ToString()` is a **type summary**, not a UTF-8 decoder. Some existing copies of `core-library.md` §5.2 and `compiler_known_members.md` still describe byte-sequence `ToString()` as a strict UTF-8 conversion. These texts conflict. Resolve them **explicitly**, preserving the MD-091 type-summary decision and adding the public core `string.FromByteArray()` route. Do not silently preserve contradictory statements as if both are normative.

**8.3.** Keep universal `ToString()` semantics and all other compiler-known type-summary behavior intact. Do not use MD-109 as a reason to redesign array stringification generally. Existing `rune[]`/`char[]` conversion rules must be reconciled with the *actual* later applied decisions, not guessed from stale text.

**8.4.** Source files/comments/tests that rely on `byte[].ToString()` as a decoder must be migrated to `string.FromByteArray(...)`. If a verified, later, explicit user decision supersedes MD-091, stop the conflicting sub-edit, report the exact authoritative decision, and follow it; do not pick a convenient behavior silently.

---

## 9. Source and rulebook changes — complete scope

**9.1. `sec/core/string.sec`.** Add the two working `static fn` implementations and the LSP comments from §§3 and 6, with `data: ref ...`. Preserve sound UTF-8 validation, fix the materialization path, remove the empty `FromRuneArray` stub, and remove obsolete TODOs claiming that core allocation is categorically forbidden.

**9.2. `rules/library/core-library.md`.** Amend its core allocation principle to *prefer allocation-free behavior* rather than forbid allocations other than `ToString()`. Replace §5.2's current non-static associated declarations with the exact `static fn` signatures. State borrowed input, independent result, strict Unicode handling, and actual errors. Reconcile `byte[].ToString()` text with MD-091. **Do not renumber §5.2, §15.1, or unrelated normative paragraphs.**

**9.3. `rules/compiler/compiler_known_members.md`.** Make both associated functions compiler-known **static associated** members; ensure no receiver, correct parameter types, correct return type, and stable registry IDs. Align AST/Sema lookup, overload resolution, Semantic IR, target lowering, diagnostics, LSP hover/completion, and source declaration identity. Fix the type-summary/decoder conflict without altering unrelated universal conversion semantics. Do not duplicate compiler-known identities.

**9.4. `rules/declarations/functions.md`, ownership/reference rulebooks, and applicable compiler docs.** Audit for text that treats associated functions as implicitly instance-bound, or treats borrowed `ref` as an owning argument. Correct only genuine contradictions; preserve the established call-site `<-` rule for consuming **named** values. These two methods do **not** consume their inputs.

**9.5. `sec/stdlib/str/collect.sec` and `sec/stdlib/str/stdlib_str.md`.** Remove the MD-109 `panic`/TODO fallback for strict `FromByteArray`. Do not retain a competing strict decoder under `str.FromByteArray` unless a later explicit decision calls for a documented forwarding compatibility alias. Use the core function from ordinary stdlib code. A separate deliberately **lossy** API, if canonically specified, remains a distinct named stdlib operation; it may transform bytes and then call the strict core route.

**9.6. `str.Builder` and other producers.** Update actual callers that currently fail due to the unavailable bytes-to-string materialization path. The MD-109 entry mentions `Builder.Finish`, `stdlib/io` directory-entry names, and `stdlib/net/url` percent decoding. For each site, preserve the site's own error type and semantic contract while routing UTF-8 materialization through public core. Do not assume all callers can directly return `StringError` without a correctly defined error mapping.

**9.7. Compiler-known array `ToString()`.** Ensure implementation and tests reflect the *later canonical* MD-091 type-summary decision for bytes. A public `FromByteArray()` must not be hidden inside another unrelated `ToString()` overload or exposed as a new array instance method to avoid the trusted-core boundary.

**9.8. Documentation and manual.** Update `string` type documentation, method index, LSP hover and relevant examples. Show both functions as static and document borrowed ownership and strict UTF-8 versus array stringification. Do not write fake constructors such as `static fn New()`.

**9.9. Governance and correction history.** Preserve original MD-091/MD-072 histories while recording this later superseding decision precisely. Update only affected `implementation-status-<book>.yaml` fragments following the existing `governance/` schema; reconcile against the registry to avoid duplicate IDs. Mark MD-109 resolved/remove its `missing-decisions.yaml` entry **only after implementation, rulebooks and tests are consistent**, following the repository's actual decision-tracking procedure.

---

## 10. Mandatory implementation and verification order

**10.1. Phase 1 — Inventory; no edits.** Read `main` HEAD and sync ID, then inspect all files named in §9, the applied MD-091 correction, actual private materializer/CKF contract, StringError implementation, relevant compiler registry and current tests. Produce a list of source-level conflicts. **Check:** report exact file paths and concrete signatures; no source changes yet.

**10.2. Phase 2 — Static-associated metadata.** Change the canonical declarations and compiler-known member classification for `FromByteArray` and `FromRuneArray` together so their APIs agree, but make no unrelated runtime changes. **Test:** parser, compiler/core bootstrap registration, static qualified-call resolution, borrowed-reference arguments, completion/hover, and rejected instance calls. Fix failures before Phase 3.

**10.3. Phase 3 — Bytes materialization.** Implement strict `FromByteArray` validation and verified trusted-core creation. Remove the bad `data.ToString()` path. **Test:** the entire §11.1 byte corpus and forced allocation failure; no stubs. Fix before Phase 4.

**10.4. Phase 4 — Runes materialization.** Implement `FromRuneArray` UTF-8 encoding and trusted-core creation. **Test:** §11.2, exact byte sequences, ownership, and forced allocation failure. Fix before Phase 5.

**10.5. Phase 5 — Stdlib integration, one consumer at a time.** Update `collect.sec`, then `Builder.Finish`, then IO and URL conversion sites verified in the actual repository. For each consumer: change → build/test → inspect → correct → next. Do not migrate all consumers before testing any.

**10.6. Phase 6 — Rulebook reconciliation.** Amend each canonical rulebook in §9 individually. Check link references, stable paragraph numbers, declarations, status fragments, MD-091 conflicts, and LSP generated docs after each edit. Ensure type summaries are no longer confused with UTF-8 decoding.

**10.7. Phase 7 — Final regression and MD-109 closure.** Run targeted and applicable full suites. Check changed file status and tests, then update governance/MD-109 only if all exit conditions pass. Archive the applied correction according to repository convention; report commit/sync ID, tests run, and any remaining failures.

---

## 11. Conformance tests (mandatory)

### 11.1 `FromByteArray`

**11.1.1.** Accept empty input (`[] → ""`), plain ASCII, ASCII mixed with non-ASCII, UTF-8 scalars of 2/3/4 bytes, U+0000 at start/middle/end, and U+FEFF without automatic removal.

**11.1.2.** Reject each malformed family with `StringError.InvalidUtf8`: isolated continuation (`80`, `BF`), illegal lead (`C0`, `C1`, `F5`, `FF`), truncated 2/3/4-byte sequences, incorrect continuation, overlong encodings, surrogate encoding (`ED A0 80`), scalar above `U+10FFFF` (`F4 90 80 80`). Include a valid prefix preceding each invalid sequence and verify the **exact first-sequence byte offset**.

**11.1.3.** Force allocation failure and verify `StringError.Allocation(...)`, unchanged source, no partial output, no leak, and no double-free. Check arithmetic overflow on extreme represented sizes where the target/runtime permits simulation.

**11.1.4.** Modify/free/move the source array after success (in legal Sec order) and verify that the returned string remains valid and unchanged. Repeat for error paths; input remains accessible and unmodified in all cases.

**11.1.5.** Verify `string.FromByteArray(data)` resolves statically and does not require `<-`; `text.FromByteArray(data)` is rejected. Confirm byte-array `ToString()` retains its MD-091 type-summary semantics, not UTF-8 decoding.

### 11.2 `FromRuneArray`

**11.2.1.** Check empty array, ASCII, BMP rune, supplementary scalar, U+0000, mixed scripts, and multiple runes. Compare exact resulting UTF-8 bytes against standard encodings.

**11.2.2.** Confirm the output has the same scalar order and count, without normalization, case conversion, NUL termination, or buffer reinterpretation.

**11.2.3.** Round-trip a string through `ToRuneArray` and `FromRuneArray` using a borrowed rune array. The returned string must be independent of the mutable rune array.

**11.2.4.** Verify allocator-failure cleanup, overflow checking, no partial results, no accidental `InvalidCodePoint` branch for ordinary safe `rune` inputs, and no panic in valid normal cases.

**11.2.5.** Verify qualified static calls, wrong-type/instance-call diagnostics, LSP hover, borrowed caller preservation, and absence of consuming markers.

### 11.3 Integration

**11.3.1.** `str.Builder.Finish` produces the correct immutable string for ASCII, multi-byte Unicode, and empty content, with its own consuming-builder semantics preserved as documented.

**11.3.2.** URL percent-decoding materializes valid strict UTF-8, rejects invalid strict UTF-8 through its canonical URL-error mapping, and preserves percent-decoding grammar.

**11.3.3.** OS/IO name materialization works without stdlib direct access to the private string intrinsic and without relaxing filesystem error handling.

**11.3.4.** Compile test for trusted-core-only helper visibility: user or stdlib attempts to invoke `_stringFromUtf8` are rejected even when spelling a package or module named `core`.

**11.3.5.** Run existing ToString/array, allocation, compiler-known-member, ownership, and UTF-8 regression suites. No unrelated output or formatting changes are allowed.

---

## 12. Completion criteria and prohibitions

**12.1.** The correction is complete only when **both** static public functions are fully implemented and callable, return `Result[string, StringError]`, borrow their arrays, uphold UTF-8/string ownership invariants, and pass all required tests.

**12.2.** Remove the exact MD-109 panic and any equivalent “materialization undecided” stubs. No `return Ok("")` implementation is acceptable for non-empty rune input.

**12.3.** Keep private materialization restricted to loader-proven trusted core; stdlib uses the public static functions only. No new unrestricted bytes-to-string cast, unsafe representation shortcut, or implicit conversion is introduced.

**12.4.** `byte[].ToString()` is not silently redefined as a decoder. Reconcile contradicted canonical texts under the later MD-091 decision before treating the documentation as complete.

**12.5.** Do not make changes to `core/rune` Unicode properties, normalization, unrelated `str` transforms, ownership grammar, or other MD-* decisions as collateral to MD-109.

**12.6.** Archive only after application and verification. A pending correction stays pending; a completed correction records the actual application date and evidence. Preserve stable section/paragraph references when revising canonical rules.

---

## 13. Source and standards audit trail

**13.1.** Repository locations for the agent to read at its checked-out HEAD:

- `sec/core/string.sec`
- `sec/stdlib/str/collect.sec`
- `sec/stdlib/str/stdlib_str.md`
- `rules/library/core-library.md` (§§1, 5.2, 15.1)
- `rules/compiler/compiler_known_members.md` (associated functions, byte-sequence `ToString`, string materialization, registry and tests)
- `rules/declarations/functions.md` and ownership/reference specifications
- the applied MD-091 correction and MD-072 documentation
- `missing-decisions.yaml` (MD-109)
- `governance/` and per-book implementation-status fragments
- actual compiler/loader declaration of `_stringFromUtf8` / `CKF-STRING-FROM-UTF8`.

**13.2.** Public upstream entry points:

- Core source: <https://raw.githubusercontent.com/YoePro/sec/main/sec/core/string.sec>
- Core rulebook: <https://raw.githubusercontent.com/YoePro/sec/main/rules/library/core-library.md>
- Compiler-known members: <https://raw.githubusercontent.com/YoePro/sec/main/rules/compiler/compiler_known_members.md>
- MD-109 governance: <https://raw.githubusercontent.com/YoePro/sec/main/missing-decisions.yaml>
- RFC 3629: <https://www.rfc-editor.org/rfc/rfc3629>
- Unicode Standard: <https://www.unicode.org/versions/Unicode18.0.0/>

**13.3.** Audit limitation: the referenced web versions were inspected for contract shape, but this document does **not** claim that a specific live private intrinsic signature or every stdlib caller has been inspected at the exact implementation commit. The implementing agent **must** verify them against its current `main` HEAD. Where current source has moved or an intervening applied correction changes the exact mechanics, preserve the semantic decisions in this document and update only the affected source paths/implementation detail, reporting the change.

---

## 14. Required agent completion report

**14.1.** On completion, the agent must report, in a compact reproducible record:

1. checked-out `main` HEAD/sync identifier and any changes since prior sync;
2. changed source and canonical-rulebook paths;
3. confirmation that both static signatures exactly match section 2 and that no old public duplicate remains;
4. the **actual** lower-level string materialization mechanism used and why it respects the trusted-core boundary;
5. tests executed (commands and pass/fail totals) after **each phase**;
6. MD-091 byte-array `ToString()` reconciliation and its regression test;
7. status of MD-109 and governance fragments;
8. any remaining implementation blockers, with precise evidence rather than an unsupported claim of completion.

## 15. Application checkpoint — 2026-10-25 (partial, not archived)

**15.1. Sync.** Local `main`, `origin/main` and a fresh upstream main lookup
agree on `9eb7c77e4e3153b61d29610581e33e07e57e2fa0`. Existing uncommitted P15/P16/P17
work is preserved. The supplied draft moved from
`rules/md109-core-string-materialization-complete.md` to its canonical pending
correction path. This is not an applied archive.

**15.2. Actual-source differences.** At this HEAD core had no public
`FromByteArray`. `FromRuneArray` was non-static but already encoded via the
canonical rune-sequence `ToString`; it was not an empty-result stub. That
encoding route is preserved. Byte-sequence `ToString` remains a type summary.

**15.3. Phase 2 completed.** Both source signatures now exactly match §2,
with one declaration each and the supplied structured comments. Source,
compiler-known IDs, static lookup, element/arity/ownership checks and LSP
completion/hover agree. Instance calls and consuming markers are rejected.
The byte body calls the actual private `_stringFromUtf8(data)` contract
(CKF-STRING-FROM-UTF8): borrowed contiguous bytes, strict validation and
`Result[string, StringError]`. The helper remains loader-proven-core-only;
user modules named `core` or `str` cannot call it. This states the frontend
contract, not an executable materializer.

**15.4. Phase 3 stop.** P23 §§35–37 require allocation-aware string operations
and P18/P19 prerequisites. They prohibit selecting a hidden `malloc`/heap or
physical descriptor to bypass missing representation/allocation infrastructure.
Both new public byte/rune fixtures pass CLI `check`; both `emit-mlir` attempts
reject with `[unsupported-lowering]` naming P23 and leave no output artifact.
Thus the UTF-8 corpus, exact rune output, forced allocation failure and
independent-result storage/cleanup tests cannot execute yet. This checkpoint
neither waives those gates nor marks phases 3/4 complete. Per §10's order,
consumer migration and canonical rulebook reconciliation (phases 5/6) remain
pending. MD-109 remains present/open; no new decision ID duplicates it.

**15.5. Verification.**

- `go test ./internal/parser ./internal/sema -count=1`: both complete suites
  passed (77.918s / 13.522s).
- The focused `TestStringMaterialization*` / registry / core / ToString / LSP
  matrix passed in `internal/sema`, `cmd/compiler`, `cmd/lsp`.
- `go test ./internal/ir/... ./internal/lowering/... -count=1`: five packages
  passed with `SEC_MLIR_BIN` set to the current native tools.
- `go vet ./internal/sema ./internal/ir/... ./internal/lowering/... ./cmd/compiler ./cmd/lsp`: passed.
- Two existing IO integration tests remain red on Linux binding and error-channel
  migration; constructor lookup now succeeds. No repository-wide green result
  or IO/URL/Builder completion is claimed.
- Governance check: 0 problems in 45 fragments; core fragment canonical,
  no type hazards; whitespace check passed. Exact commands and boundaries are
  retained in `compiler.compiler-known-surfaces` in `governance/core.yaml`.

**15.6. Changed owners.** Source: `sec/core/string.sec`; compiler-known static
registry/calls/synthetic metadata and the focused
`internal/sema/string_materialization_calls.go`; a stable-ID-keyed unsupported
branch in Semantic IR `buildCall`; focused Sema/CLI/LSP tests and fixtures.
`missing-decisions.yaml`, `governance/core.yaml`, `language-rulebook-status.md`
and the shared `mlir/compiler-packages-handoff.md` record partial application.
Canonical core/registry/stdlib texts and old applied MD-091/MD-072 history are
not falsely marked reconciled; phase 6 work still requires completion.
