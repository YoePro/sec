# FFI / ABI Prerequisite Synchronization

- **Status:** Review artifact
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Baseline:** repository `main` commit `0f92cf4`
- **Scope:** prerequisites for rewriting `rules/platform/ffi.txt`
- **Out of scope:** writing `rules/platform/abi.md`

## 1. Purpose

This synchronization checks the normative rules that the FFI rulebook depends on.

It does not define the ABI rulebook and it does not rewrite unrelated rulebooks.

The goal is to ensure that the FFI rewrite can rely on one coherent set of language, ownership, reference, layout, callable, and platform semantics.

## 2. Baseline note

The repository baseline for this review is:

```text
0f92cf4
A LOT of updated rulebooks, mainly in declatations, with all the cascading corrections.
```

The locally prepared `lambda-functions.md` v2 package is newer than this repository baseline and is treated as an intended prerequisite even though it is not yet synchronized into `main`.

## 3. Prerequisites already semantically suitable

### 3.1 Platform model

`rules/platform/platform_model.md` already establishes the correct ownership boundary:

- FFI owns source-level foreign-interface semantics.
- ABI owns physical calling-convention semantics.
- one resolved `ABIModel` is selected through the active `CompilationPlan`;
- later stages consume that model instead of guessing ABI from OS or architecture names.

No prerequisite correction is required.

### 3.2 Layout

`rules/memory/layout.md` already separates storage layout from callable ABI.

It correctly establishes that:

- layout does not decide whether a type may cross FFI;
- explicit layout does not automatically imply FFI compatibility;
- generic layout is resolved per concrete instantiation;
- raw-pointer layout does not imply ownership, lifetime, validity, or non-nullness;
- ABI parameter/return classification belongs to the ABI rules.

No semantic prerequisite correction is required.

Stale file-name references or implementation-status material may be cleaned independently, but they do not block the FFI rewrite.

### 3.3 Ownership

`rules/memory/ownership.md` already requires explicit ownership contracts across FFI.

It identifies the required questions:

- borrowed or owned;
- ownership transfer on call;
- transfer only on success;
- return ownership;
- release responsibility;
- foreign retention;
- lifetime and thread restrictions.

No prerequisite correction is required.

The FFI rewrite must provide or deliberately delimit the source-level mechanism that supplies this information.

### 3.4 Reference model

`rules/memory/reference_model.md` already defines:

- safe references as non-null;
- `RawPtr[T]` as raw and potentially null/invalid;
- call-bounded foreign use;
- explicit foreign retention;
- retention-driven lifetime/pinning obligations;
- conservative treatment of unknown foreign retention;
- raw-pointer-to-reference conversion as unsafe.

No semantic prerequisite correction is required.

The exact FFI source syntax for retention remains an FFI design question.

### 3.5 Error handling

The current error-handling rules already establish ordinary `Result`/`try` semantics and the functions-v2 call-transfer rule.

Foreign error conventions should therefore remain raw foreign semantics until a wrapper explicitly translates them into a Sec `Result`.

No prerequisite correction is required.

## 4. Required prerequisite corrections

Four targeted corrections are required before the FFI rewrite.

### 4.1 Function/callable type model in `types.md`

The current type overview still presents only:

```sec
fn(int) bool
```

The functions/lambda v2 model requires callable types to preserve:

```sec
fn(int) int
mut fn() int
-> fn() Resource

fn(ref Buffer) void
fn(ref mut Buffer) void
fn(-> Buffer) void
fn(...int) int
```

This matters directly to callbacks and ABI/FFI callable validation.

Correction:
`types-callable-model-correction.md`

### 4.2 `@link_name` in the closed attribute set

The current FFI rulebook uses and reports implementation support for:

```sec
@link_name("open")
extern "C" fn c_open(...) ...
```

However, the canonical attribute rulebook defines a closed attribute set and does not currently register `@link_name`.

The implementation and FFI rule therefore conflict with the closed-set governance rule.

Correction:
`attributes-link-name-correction.md`

### 4.3 `RawPtr[T]` null value domain

`reference_model.md` correctly states that `RawPtr[T]` may be null.

The older `raw_pointers.txt` instead says a raw pointer may represent null only at an FFI boundary or inside an extern wrapper.

Those are different claims.

The corrected distinction is:

- null is part of the possible `RawPtr[T]` value domain;
- safe references remain non-null;
- source syntax for constructing a null raw pointer is separately restricted;
- FFI wrappers normalize nullable foreign results into explicit safe Sec types where appropriate.

Correction:
`raw-pointers-null-domain-correction.md`

### 4.4 Ambiguous `...` in `unsafe extern` example

`unsafe.md` currently contains:

```sec
unsafe extern "system" fn rawSysCall(...) int64
```

After functions v2, `...` has concrete native typed-variadic meaning only in a declaration such as:

```sec
values: ...T
```

Foreign varargs are deliberately separate and will be defined by FFI/ABI.

The unsafe rulebook must therefore not use bare `...` in a code example where it can look like accepted Sec syntax.

Correction:
`unsafe-extern-ellipsis-correction.md`

## 5. Deliberately deferred to the FFI rewrite

The prerequisite sync does not decide these FFI-owned source semantics:

1. whether safe `ref T` / `ref mut T` may appear directly in an extern signature, or whether raw foreign signatures use `RawPtr[T]` and wrappers perform the conversion;
2. exact source syntax for foreign pointer retention;
3. exact ownership-transfer annotations/contracts for pointee/resource ownership;
4. foreign effect declarations and trusted foreign effect claims;
5. foreign exception/unwind classification;
6. foreign/C variadic declaration and call syntax;
7. closure callback/trampoline/context rules;
8. null-literal source contexts and whether an "extern wrapper" is syntactically recognized or merely a semantic/documentation pattern;
9. library/object dependency metadata;
10. exact foreign-compatible layout source syntax.

These are real FFI design questions and should be addressed while rewriting `ffi.txt`.

## 6. Important FFI invariants already fixed by prerequisites

The future FFI rulebook must preserve these already-decided invariants:

- `RawPtr[T]` carries no pointee ownership by itself.
- Consuming a `RawPtr[T]` value does not imply ownership of its pointee.
- `unsafe` does not disable ownership, borrowing, effects, cleanup, or target validation.
- native Sec typed variadics are not C varargs.
- native closure representation is not automatically a foreign callback representation.
- unresolved generic templates do not cross ABI; only concrete monomorphized instances may acquire a concrete ABI.
- storage layout and call ABI remain separate.
- the active `CompilationPlan.ABIModel` is authoritative for target ABI validation.

## 7. Result

After applying the four targeted corrections, the prerequisite layer is coherent enough to begin the `ffi.txt -> ffi.md` v2 design/rewrite.

The ABI rulebook remains intentionally out of scope for this thread.
