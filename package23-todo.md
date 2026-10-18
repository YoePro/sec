# SEC-MLIR Package 23 — implementation TODO

Package: `SEC-MLIR-P23` — String Values, UTF-8 and Core Materialization  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package23.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package23.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_string_core_sync_package23.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_string_package23.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v19.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v19.md`
- `rules/mlir/packages/sec-mlir-dialect_package23_decision-index.md`
- `rules/corrections/applied/md050-md071-md072-string-utf8-positions-materialization-correction-20261017.md`

This is an implementation plan, not a completion report and not a new string
API design. It carries the settled source contract through Semantic IR and the
one Sec MLIR lowering track. It must not revive the removed core byte-decoding
API or a historical backend.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree contained 143 changed entries when this package was audited.
Those intended predecessor changes, including the Package 20–22 plans, must be
captured before P23 acceptance can be reproduced.

| Area | Current implementation | P23 consequence |
|---|---|---|
| Source string rules | `string` is valid UTF-8; public text positions use runes; byte-specific APIs use byte indexes. | Treat MD-050, MD-071 and MD-072 as settled authority, then verify each lower layer preserves them. |
| Frontend concat | Sema records immutable `StringConcatPlan` segments, constant folding, runtime materialization, allocation context and `StringError`. | Reuse this plan; do not re-typecheck or re-evaluate source expressions in IR/lowering. |
| Semantic IR | Fully constant concatenation becomes `const.string`; runtime concatenation is deliberately not handled. | P23 must add the remaining typed string values, operations and fallible materialization CFG. |
| Core materializer | Core-only `_stringFromUtf8` accepts borrowed contiguous byte input and returns `Result[string, StringError]`. | Keep it private and typed; it is not public `string.FromByteArray`. |
| Array conversions | Compiler-known surface has `ToByteArray`, `ToRuneArray` and `FromRuneArray` result types. | Finish only with P18/P19 ownership, arrays and collection-error prerequisites. |
| Schema/track | P23 targets dialect schema 19 after schema 18; no executable string lowering path exists yet. | Do not claim schema 19 or restore any legacy/native parallel track before predecessors are executable. |

Primary implementation anchors:

```text
internal/sema/string_materialization.go
internal/sema/semantic_facts.go
internal/sema/string_from_utf8_intrinsic.go
internal/sema/compiler_known_members.go
internal/sema/compiler_known_synthetic.go
internal/ir/semantic/string_concat.go
internal/lowering/secmlir/emitter.go
sec/core/string.sec
sec/core/error.sec
governance/types.yaml
governance/lexer.yaml
```

## Authority and dependency constraints

1. **MD-050 is settled:** every `string` value is valid UTF-8. Invalid byte
   sequences can only fail at a decoding/materialization boundary; they are not
   silently admitted as strings.
2. **MD-071 and MD-072 are settled:** public `IndexOf`, `LastIndexOf`,
   `Slice`, `GetRune` and `RuneAt` use rune positions. `GetByte`, `ByteAt` and
   `CharAt` use byte indexes. P23 implements this contract; it must not list it
   as an unresolved decision.
3. **MD-074 is settled:** global `len(...)` returns `uint`. `string.Len` is a
   scalar-value count and `SizeOf` is its UTF-8 byte count; no signed `len`
   narrowing may be introduced.
4. **MD-091 supersedes stale API wording:** public byte decoding is stdlib
   `str.FromByteArray`/`str.FromByteArrayLossy`. The private
   `_stringFromUtf8` (`CKF-STRING-FROM-UTF8`) is known only to core — owner
   decision 2026-10-18, never to stdlib — and core may allocate only in
   `ToString()` (it is used by core byte formatters). `byte[].ToString()` is
   not a decoder. How stdlib (`str.FromByteArray`, `Builder.Finish`, io, url)
   materializes validated bytes is open as MD-109; until then those paths
   panic or fail to compile (Known hole).
5. **MD-073 remains open:** scalar-to-`char` behavior above byte range is not
   decided. `ToCharArray`, char conversion expansion and related lowering stay
   blocked or explicitly unsupported.
6. **Existing package dependencies remain feature-specific:** P18/P19 own
   array/collection representation and ownership; P22 owns named-contract
   interoperability; schema 18 must be executable before schema 19.
7. **One track only:** accepted work is Sema → Semantic IR → Sec dialect MLIR
   → standard MLIR → LLVM dialect → LLVM IR → `llc` object (same LLVM build) → clang link
   (2026-10-18). Historical lowering prose
   does not authorize a second backend.

## Completion boundary

P23 is complete only when valid UTF-8 string values, scalar/byte measures,
comparison/search/slice plans, traversal and one-allocation runtime
concatenation/interpolation have immutable Sema facts, verified Semantic IR,
schema-19 operations and an executable single-track lowering. Every public
position/error unit must remain the settled rune-or-byte unit, source operands
must evaluate once left-to-right, and failures must use established `Result`
and error identities.

P23 does not decide MD-073 or MD-109, define grapheme/locale/normalization
semantics, expose core byte decoding, choose a physical string descriptor/ABI,
introduce a general runtime, or implement broad collection conversion before
its prerequisites.

## A. Establish baseline and synchronize authority

- [ ] P23-01 — Record the exact execution snapshot, untracked inventory,
  P18–P22 dependency matrix, toolchain versions and `SEC_MLIR_BIN` setting.
- [ ] P23-02 — Run and record focused Go, vet, parser/Sema, Semantic IR,
  dialect/lowering and native-track controls; distinguish pre-existing failures
  from P23 failures.
- [ ] P23-03 — Reconcile the package, sync, Semantic IR, schema-19 and
  lowering-19 inputs with current lexer, string, error, collection and
  governance rules without renumbering canonical paragraphs.
- [ ] P23-04 — Update stale P23/source registry wording so MD-050/071/072 and
  MD-074 are recorded as resolved, MD-091's API removal is honored, and only
  MD-073/MD-109 remain explicit string-design dependencies.
- [ ] P23-05 — Update the existing relevant governance entries and
  `language-rulebook-status.md` with accurate partial/evidence status; do not
  create duplicate implementation IDs or promote frontend-only facts to a
  lowering claim.

## B. Credit the existing frontend foundation

- [x] P23-06 — Represent source concatenation/interpolation as a detached
  `StringConcatPlan`, preserving ordered segments, fully folded text, runtime
  materialization, allocation context and canonical `StringError` handling.

  Current anchors: `internal/sema/string_materialization.go`,
  `internal/sema/semantic_facts.go` and
  `internal/sema/string_materialization_test.go`.

- [x] P23-07 — Constant-fold fully static string concatenation into Semantic
  IR `const.string` without an allocation path.

  Current anchor: `internal/ir/semantic/string_concat.go`. Runtime concat is
  intentionally still rejected here and is not P23 completion.

- [x] P23-08 — Type-gate the private core `_stringFromUtf8` materializer to a
  borrowed contiguous byte sequence and `Result[string, StringError]`, without
  consuming, adopting or mutating the source bytes.

  Current anchor: `internal/sema/string_from_utf8_intrinsic.go`.

- [x] P23-09 — Preserve the existing compiler-known result surfaces for
  `ToByteArray`, `ToRuneArray` and `FromRuneArray`, and the core synthetic
  identity for `_stringFromUtf8`.

  These are frontend identities only; array conversion runtime lowering remains
  P18/P19-dependent.

## C. Complete immutable string semantic facts

- [ ] P23-10 — Define detached, read-only facts for a string value's valid
  UTF-8 status, scalar length, byte size, literal/static lifetime, comparison
  semantics, allocation context and fallibility. Reuse canonical TypeIDs and
  core error identities; never retain mutable AST/typechecker state.
- [ ] P23-11 — Define `StringLengthPlan` with explicit scalar-count versus
  UTF-8-byte-count result and `uint` width semantics. Audit compiler-known
  documentation and diagnostics that still say encoded-byte length for `len`.
- [ ] P23-12 — Define ordered `StringOperationPlan` variants for equality,
  lexicographic comparison, starts/ends/contains, rune-position search,
  byte-position access, checked public slicing, private byte slicing, split,
  trim and traversal. Each plan must name input/output/error units.
- [ ] P23-13 — Establish source evaluation-once, left-to-right rules for every
  multi-operand string operation and interpolation segment; preserve borrowed
  backing lifetime and do not infer an ABI/descriptor layout.
- [ ] P23-14 — Encode byte-to-rune translation only where a public rune-based
  API requires it. Private `__StringSliceUnchecked` may use bytes only behind
  validated bounds, valid UTF-8 boundary and backing-lifetime facts.
- [ ] P23-15 — Model equality as logical content equality and ordering as
  Unicode-scalar lexicographic order for valid UTF-8. Do not add locale,
  normalization, grapheme or platform-library behavior.
- [ ] P23-16 — Give runtime concatenation/interpolation one shared plan:
  operands evaluate once left-to-right, all required text conversion failures
  happen before allocation, exactly one result materialization is selected, and
  allocation failure follows the established result/error route.
- [ ] P23-17 — Create explicit capability boundaries for arrays, rune/char
  conversion, target/noalloc materialization, FFI views and unsupported string
  operations. Reject a path structurally rather than falling back to host
  strings or hidden allocation.

## D. Semantic IR values, operations and control flow

- [ ] P23-18 — Add typed Semantic IR records for static/dynamic string values,
  scalar/byte measures, checked position and slice plans, searches, traversal,
  string operation results and one-materialization concatenation. Do not use a
  generic opaque string-call operation.
- [ ] P23-19 — Extend `const.string` and string construction validation to
  admit only valid UTF-8 payloads and preserve static immutable lifetime;
  reject invalid encoded constants deterministically.
- [ ] P23-20 — Build string length/size, equality/order and simple predicate
  operations from resolved facts, preserving `uint` result types and scalar
  versus byte units across operation boundaries.
- [ ] P23-21 — Build checked public indexing/search/slice CFG with explicit
  bounds, UTF-8 boundary/rune translation and canonical error construction.
  The success branch alone may expose the resulting string/rune/byte value.
- [ ] P23-22 — Build split, trim and traversal from explicit immutable plans;
  preserve empty/embedded-NUL/non-BMP values, callback/evaluation order and
  iterator/ownership prerequisites. Keep unsupported iterator forms rejected.
- [ ] P23-23 — Build runtime concat/interpolation CFG from `StringConcatPlan`:
  compute required lengths with overflow checks, defer exactly one allocation
  until all pre-allocation failures are resolved, write segments in source
  order, validate the final valid-UTF-8 invariant and return canonical failure
  results.
- [ ] P23-24 — Model `_stringFromUtf8` only as private borrowed-byte decoding;
  retain byte ownership on both success/failure. Route public decode syntax to
  its stdlib surface once MD-109 is resolved, rather than adding
  `string.FromByteArray`.

## E. Schema 19, emission and verification

- [ ] P23-25 — Install schema 19 only after schema 18 is executable and P23
  Semantic IR facts are verified. Add the smallest typed Sec string operation
  family necessary for facts above; reuse ordinary integers, Result and core
  errors rather than adding a string runtime type.
- [ ] P23-26 — Emit schema-19 operations exclusively from verified semantic
  plans, retaining compiler metadata for measure unit, source order, static
  lifetime, allocation context, position kind and plan identity—not as a new
  user-visible payload.
- [ ] P23-27 — Add `--sec-verify-strings` checks for valid constant UTF-8,
  typed values, scalar/byte unit compatibility, `uint` length, operand
  dominance/evaluation order, bounds and UTF-8 boundary checks, private-op
  confinement, error identity and single-materialization structure.
- [ ] P23-28 — Add deterministic parse/print and negative fixtures for invalid
  UTF-8 constants, signed `len`, unit mixing, unchecked public slice/index,
  missing bounds/translation, early allocation, reordered interpolation,
  public core decoding and invalid array/char capability claims.

## F. Lower through the single native track

- [ ] P23-29 — Add `--sec-lower-strings` for static literals, scalar/byte
  length, equality/order and supported basic operations through the Sec →
  standard MLIR → LLVM dialect path. Preserve valid UTF-8 and source semantics;
  do not lower to host string helpers as a semantic substitute.
- [ ] P23-30 — Lower rune-based search/index/slice using explicit scalar/byte
  conversion logic and checked CFG. Lower byte APIs by byte semantics only;
  do not conflate descriptor size, code units, scalars or graphemes.
- [ ] P23-31 — Lower one-materialization concat/interpolation with explicit
  overflow checks, target/noalloc capability, one allocation after prior
  fallible conversions, ordered segment writes and canonical allocation/error
  handling. Do not fuse away observable evaluation or failure order.
- [ ] P23-32 — Lower private `_stringFromUtf8` only where its borrowed input,
  strict validation and `Result[string, StringError]` guarantees can be
  represented. Keep stdlib decode/lossy-decode lowering behind MD-109.
- [ ] P23-33 — Lower `ToByteArray`, `ToRuneArray` and `FromRuneArray` only
  once P18/P19 array allocation, copying, ownership, `CollectionError` and
  rune iteration support are executable. Keep `ToCharArray` unavailable under
  MD-073.
- [ ] P23-34 — Add target capability checks for UTF-8 scan, allocation,
  overflow, noalloc and FFI boundary paths; produce structured unsupported
  lowering instead of choosing an ABI or runtime implicitly.

## G. Required test matrix

- [ ] P23-35 — Test valid UTF-8 literals and dynamic construction including
  ASCII, multibyte BMP, non-BMP, combining marks and embedded NUL; reject
  malformed byte sequences only at their decoding boundary (§§1, 19, 24).
- [ ] P23-36 — Test `len` scalar counts and `SizeOf` UTF-8 bytes with
  ASCII/multibyte/non-BMP values, `uint` typing, overflow paths and no
  signed/host-width truncation (§§3, 11, 20).
- [ ] P23-37 — Test equality/order, starts/ends/contains, rune-position
  searches and byte-specific access. Include boundary positions, empty strings,
  embedded NUL and differences between byte and rune offsets (§§2, 12, 15).
- [ ] P23-38 — Test public checked slicing/indexing and private byte slicing:
  negative/out-of-range positions, mid-sequence byte boundaries, exact error
  identity, valid results and borrowed backing lifetime (§§2, 14, 21).
- [ ] P23-39 — Test runtime concat/interpolation for constant folding,
  left-to-right exactly-once evaluation, fallible segment conversion, overflow,
  allocation failure, one materialization and valid UTF-8 final output
  (§§16, 23, 31).
- [ ] P23-40 — Test private `_stringFromUtf8`, public stdlib-only decoding
  boundary, byte/rune array conversions where P18/P19 permit them, and explicit
  rejection for removed core `FromByteArray`, `byte[].ToString()` decoding and
  MD-073 char paths (§§4–5, 24, 33).
- [ ] P23-41 — Add Semantic IR/MLIR verifier negatives, deterministic
  parse/print, target 32/64/noalloc controls and differential native-track
  tests; run focused Go/vet/dialect/lowering/lit suites plus full repository
  checks and record exact commands/results (§§27–34).

## H. Acceptance, governance and handoff

- [ ] P23-42 — Reconcile source, Semantic IR, schema and lowering acceptance
  gates separately. Frontend concat facts and static folds do not by themselves
  satisfy runtime string lowering completion.
- [ ] P23-43 — Synchronize governance, language-rulebook status, decision
  references, compiler-known registries and implementation evidence. Ensure
  MD-050/071/072/074 and MD-091 are reflected consistently.
- [ ] P23-44 — Produce the required implementation report with snapshot,
  prerequisite matrix, implemented operation families, explicit unsupported
  boundaries, schema/lowering status, test evidence, deviations and next
  package boundary.

## Recommended execution order

1. A: capture the snapshot and reconcile the settled decisions and stale API
   wording before changing string implementation surface.
2. C: extract complete immutable string plans and make units/lifetimes/
   fallibility explicit; preserve the existing concat plan as the starting
   point.
3. D: add Semantic IR one operation family at a time, starting with static
   values, length/size and comparisons, then checked positions and concat.
4. E/F: only after executable schema-18 predecessors, install schema 19 and
   lower through the one native track; widen to arrays/stdlib decoding only as
   their dependencies close.
5. G/H: retain structured unsupported boundaries for MD-073, MD-109 and every
   unavailable P18/P19 capability; promote status only with end-to-end proof.
