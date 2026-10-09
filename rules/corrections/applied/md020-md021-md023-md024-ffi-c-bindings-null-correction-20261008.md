# MD-020, MD-021, MD-023 and MD-024 — C FFI Qualification, Bindings, ABI and Null Sentinel

- **Status:** Applied 2026-10-09 to `rules/platform/ffi.md`, `rules/platform/abi.md`, `rules/platform/target_profiles.md`, `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/foundations/operators.md`, `rules/memory/raw_pointers.md`, `rules/memory/unsafe.md`, `rules/tooling/formatter.md`, `rules/projects/modules.md`, `rules/declarations/functions.md`, `rules/declarations/struct.md`, and `rules/compiler/linking.md`; compiler implementation and the MD-021 binding-environment format remain tracked in governance
- **Created:** 2026-10-08
- **Last updated:** 2026-10-08
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Proposed canonical path:** `rules/corrections/pending/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md`
- **Decision IDs:** `MD-020`, `MD-021`, `MD-023`, `MD-024`
- **Amends:** `rules/platform/ffi.md`, `rules/platform/abi.md`, `rules/platform/target_profiles.md`, `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/memory/raw_pointers.md`, `rules/memory/unsafe.md`, `rules/tooling/formatter.md`, `rules/projects/modules.md`, project/package build-metadata rules where applicable
- **Related correction:** `MD-012/MD-022` approved 2026-10-08 (its use of `C::` must be migrated to `c::`)
- **Replaces:** No existing numbered paragraphs. Apply as additions/amendments without renumbering published rulebook references.

## 1. Scope and intent

1.1. C interoperation exists to make existing C-compatible native libraries and operating-system APIs practical to use from Sec on all supported targets, including Windows. The compiler validates resolved ABI facts instead of requiring programmers to reconstruct them manually.

1.2. This correction standardizes the C namespace, its lexical spelling, selected target-provided C binding types, FFI-legal scalar representations, and the strictly unsafe C null sentinel.

1.3. The source spelling `extern "C"` continues to denote the C calling convention. It is distinct from the type qualification `c::` and **must not** be renamed to `extern "c"`.

1.4. Ordinary external declarations and `unsafe extern` retain their existing different meanings. No rule here makes every ordinary foreign call require an `unsafe` block; `null` syntax requires `unsafe` for its own narrower reason.

## 2. MD-020 — One C qualification: `c::`

2.1. The only canonical C-qualification prefix is **lowercase `c::`**. Capitalized `C::` is removed from normative Sec 0.1 syntax, including compiler-known fundamental C types and C-only type/adapter constructs. This is a deliberate language-level spelling decision, not merely a formatter preference.

```sec
let value: c::int := 42
let length: c::stddef::size_t := 42

extern "C" fn GetVersion() c::int
extern "C" fn RegisterCallback(callback: c::fn(c::int) void) void

let callback := c::callback(OnValue)
```

2.2. The fundamental compiler-known C family includes `c::char`, `c::schar`, `c::uchar`, `c::short`, `c::ushort`, `c::int`, `c::uint`, `c::long`, `c::ulong`, `c::long_long`, `c::ulong_long`, `c::float`, `c::double`, `c::long_double`, and `c::bool`. Their physical representation is selected by the active C ABI model.

2.3. The existing C-specific constructions use the same prefix: `c::fn(...)`, `c::flex[T]` and `c::callback(expression)`.

2.4. `::` is one lexical token recognized with longest-match precedence over a single `:`; `:=` and `:<-` remain separate, correctly recognized tokens. Qualification contains **no whitespace** inside or adjacent to `::`.

```sec
let a: c::int := 1                 // Valid.
let b: c::stddef::size_t := 2     // Valid.
let c: C::int := 3                 // Invalid: old prefix.
let d: c :: int := 4              // Invalid: separated qualification.
let e: c: :int := 5               // Invalid: split punctuation.
```

2.5. The formatter always emits `c::name` and `c::namespace::name` without spaces for valid parsed syntax. It must not silently rewrite arbitrary separated `:` tokens into `::` or change invalid source semantics. Existing optional language-correction behavior, if enabled and unambiguous, is independently governed by the formatter rulebook.

2.6. `c::` is a type/foreign-construct qualifier, not ordinary value-member access; an ordinary value binding named `c` does not alter foreign type lookup.

2.7. Additional foreign-language bindings (e.g. Rust) may be designed later, but this correction introduces neither `rust::` nor an `extern "Rust"` ABI contract.

### 2.8. Grammar amendment

Replace the previous uppercase/lowercase split in `ForeignTypeReference` with the following exact production, subject to the already defined `TypeReference` and expression productions:

```text
ForeignTypeReference
    ::= "c" "::" Identifier { "::" Identifier }
      | "c" "::" "fn" "(" [ ForeignParameterTypeList ] ")" TypeReference
      | "c" "::" "flex" "[" TypeReference "]"

ForeignCallbackExpression
    ::= "c" "::" "callback" "(" Expression ")"
```

The parser distinguishes the fundamental names from target-defined binding names during semantic resolution. Do not interpret the new production as permission to use any arbitrarily spelled C type without a real binding.

## 3. MD-021 — Ownership and selection of C bindings

3.1. The **compiler** owns the semantic meanings of the fundamental C ABI type family, the foreign-specific constructs, the ABI representation/legality checks and lowering under the active `CompilationPlan`.

3.2. **Platform bindings**, normally provided from `sec/platform`, provide target-specific C declarations such as `c::stddef::size_t`, `c::time::time_t`, `c::stdarg::va_list` and (when available) `c::posix::socklen_t`. These names are not hardcoded as universal fundamental scalar types.

3.3. The selected `CompilationPlan` resolves one compatible platform C binding environment for the target and makes its standard C binding types available under `c::` **without a source-level import**. The set of available bindings remains target-specific. The compiler must report unavailable/unresolved names; it must not substitute host definitions.

3.4. Third-party C libraries are represented by **ordinary Sec binding packages/modules**, imported through the normal module system. Such packages may carry native link dependencies using the existing project/package build-metadata mechanism. They need not modify the compiler when they add a new foreign function or type declaration.

```sec
import "bindings/sqlite"

// The imported module exposes its declared Sec binding surface.
// Its native library dependency is resolved by project/package metadata.
```

3.5. `bindings/sqlite` above illustrates a regular logical import path; it does **not** reserve that path or prescribe a package registry. Third-party packages use whichever import roots are permitted by the canonical module/package rules.

3.6. No second shadow language for C header files, C typedef syntax, C preprocessor directives, or automatic parsing of C headers is introduced. An optional external binding generator may emit normal Sec declarations.

3.7. The **exact source/metadata serialization and selection mechanism for target-provided binding declarations** belongs to the target/platform/build rules. This decision locks responsibility and lookup behavior; it does not silently invent TOML fields, proprietary annotations, or a binding database format. This is a documentation/implementation specification still to be completed before the MD-021 registry entry can be marked fully synchronized.

3.8. Missing, incompatible, ambiguous, stale or untrusted binding metadata must fail closed during compilation rather than falling back to the compiler host's C ABI.

## 4. MD-023 — ABI legality for Sec scalar and enum types

4.1. The existing legal categories — verified C scalars, ABI-stable Sec fixed-width scalars, `RawPtr[T]`, permitted call-bounded references and explicitly C-compatible aggregate/callable types — remain in effect. Source-level type identity is **never inferred** from identical representation.

4.2. Sec `int` and `uint` are legal in a foreign signature **only where the selected ABI model proves that the resolved Sec type's size, sign, alignment, argument/return classification and other required properties match the actual foreign ABI contract in that exact position**. Merely having the same size on one host does not grant portable compatibility.

```sec
// Valid only when the chosen target ABI proves the declared representation.
extern "C" fn ReadNativeCount() uint
```

4.3. The 128- and 256-bit Sec integers (`int128`, `uint128`, `int256`, `uint256`) may be used at FFI boundaries **only when the type is supported by that Sec compiler/target and the selected foreign ABI explicitly supports the concrete representation and call position**. LLVM integer support alone is not evidence of an interoperable foreign ABI.

4.4. A normal closed Sec enum is **not automatically FFI-legal** merely because it has an integer-backed representation. To represent a C enum with C ABI and its open-value behavior, declare it as an `extern "C"` enum, following the established FFI rulebook.

```sec
extern "C" type Color enum {
    Red = 1
    Green = 2
    Blue = 3
}
```

4.5. On unsupported or unverified targets the compiler must reject a foreign signature before lowering; it must not guess representation, silently truncate, or reinterpret a closed Sec enum as an open C enum.

4.6. Existing MD-022 literal-shaping and explicit-conversion rules remain intact, with **all** `C::...` spellings in that earlier correction migrated to `c::...`.

## 5. MD-024 — `null` is unsafe FFI-only syntax

5.1. Sec has **no general-purpose null value** and no nullable ordinary references. `Option[T]` and its `Some`/`None` variants express optional values in normal Sec code. `null` is solely a foreign/raw-pointer sentinel needed to interoperate with C-compatible APIs.

5.2. `null` is recognized as a reserved lexical spelling so that a user binding cannot shadow the FFI sentinel. Its use as an expression or test is **semantically permitted only in an explicit `unsafe` context** and where the operand or target is a supported foreign/raw-pointer type.

5.3. Exact and symmetric null-test operators are `is null` and `is not null`. Both produce `bool`. Both may appear in **any boolean-expression position**, not only `if` and `while`, provided the expression is inside `unsafe`.

```sec
unsafe {
    let missing: bool := pointer is null
    let present: bool := pointer is not null

    if pointer is null {
        return None
    }

    if pointer is not null {
        UseForeignPointer(pointer)
    }
}
```

5.4. The right operand `null` in these productions is a sentinel, not a general value. The unary `!` operator can still negate a boolean expression, but `is not null` is a **canonical supported spelling** and must not be rejected.

5.5. `== null`, `!= null`, and implicit comparisons or conversions involving `None` remain invalid. A null test performs no dereference; `unsafe` is required to prevent foreign null semantics spreading into ordinary Sec code.

5.6. Construction of a foreign null pointer from `null` requires an explicit type context as well as `unsafe`. A stand-alone `null` has no inferred Sec type.

```sec
unsafe {
    let pointer: RawPtr[byte] := null  // Valid typed foreign sentinel.
    let unknown := null                // Invalid: no target pointer type.
}

let pointer: RawPtr[byte] := null      // Invalid: outside unsafe.
let optional: Option[byte] := null    // Invalid: use None instead.
```

5.7. Ordinary Sec wrappers validate foreign nullable results inside their unsafe boundary and expose `Option[T]` or an appropriate `Result[T, E]` to callers. The use of raw pointers in a function signature does not by itself force every external function call to be unsafe; the existing `extern` versus `unsafe extern` rule still governs calls.

### 5.8. Grammar amendment

```text
ForeignNullSentinel
    ::= "null"

ForeignNullTest
    ::= Expression "is" "null"
      | Expression "is" "not" "null"
```

`ForeignNullTest` is a boolean expression form, with precedence following the existing `is`/`is not` family. The compiler must bind the left operand once and evaluate it once. This production grants no general-purpose null operand or null expression type.

## 6. Rulebook synchronization and source migration

6.1. Update `rules/platform/ffi.md` consistently, including examples, `c::` type names, C function pointers, callbacks, flexible members, C varargs, semantics, parser requirements and Sema requirements. Do not leave mixed `C::` and `c::` public syntax.

6.2. Update `rules/foundations/grammar.md` and `rules/foundations/lexical_structure.md` with the canonical `::` token and both null tests. Preserve existing `:<-`, `:=`, and ordinary `:` semantics.

6.3. Update `rules/memory/raw_pointers.md` and `rules/memory/unsafe.md` to make unsafe-only **source syntax** for null tests explicit. Do not accidentally imply that a raw pointer test dereferences memory, validates pointer lifetime, or proves general address safety.

6.4. Update the relevant formatter rule to emit the canonical C qualifier spelling and never inject spaces inside or around `::`.

6.5. Update the selected platform/ABI/project/module rulebooks with the responsibility split in § 3. Keep unresolved binding-serialization metadata explicitly outstanding until written in its owning rulebook; do not declare implementation complete prematurely.

6.6. The earlier `MD-012/MD-022` correction is still normative as to semantics, but every example/type use of `C::` in it must be migrated to `c::`. Update compiler-known member declarations, hover surfaces, generated docs, tests and diagnostics consistently.

## 7. Required conformance tests

7.1. Lexer: tokenizes `c::int` and `c::stddef::size_t` correctly; preserves `:<-`, `:=`, `:` longest match; rejects malformed/whitespace-separated foreign qualification as syntax rather than silently altering tokens.

7.2. Formatter: valid FFI types and constructs always print without spaces around `::`; formatting is idempotent. Formatter does not auto-rewrite `C::` into `c::` unless a separately enabled, explicitly authorized language-migration mode defines that rewrite.

7.3. Sema: fundamental `c::` types resolve through the selected ABI, while hierarchical target binding names resolve through the selected environment; no host fallback. A local value identifier named `c` cannot change foreign type resolution.

7.4. ABI: tests cover both matching and mismatching positions for `int`/`uint`, accepted/unsupported 128-/256-bit types, and C enums versus normal Sec enums. Include Windows and non-Windows ABI profiles wherever available.

7.5. Null: positive tests for `is null` and `is not null` in `if`, `while`, assignment and general boolean-expression contexts **inside unsafe**; negative tests for both outside `unsafe`, for `== null`/`!= null`, for untyped `null`, for `Option[T] := null`, and for nonpointer operands.

7.6. Wrappers: verify that a foreign nullable result can be normalized to `Option[T]` without letting null syntax escape the unsafe implementation.

7.7. Regression: continue testing ordinary `extern "C"` calls without unconditional unsafe requirements and `unsafe extern "C"` with required explicit unsafe context.

## 8. Governance handling

8.1. MD-020, MD-023 and MD-024 are normatively decided. Mark their missing-decision entries resolved **only after** the owning rulebooks and governance fragments are synchronized.

8.2. MD-021's architecture, import behavior and namespace-availability semantics are normatively decided; track its concrete binding-environment format/selection schema as the remaining specification work until the target/platform/build rules define it. Do not invent implementation metadata to claim closure.

8.3. MD-019 (general explicit alignment/packing/offset representation) is separate and remains open. This correction must not preempt it.
