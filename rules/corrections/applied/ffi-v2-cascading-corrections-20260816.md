# FFI v2 Cascading Corrections

- **Status:** Applied 2026-08-16
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Source:** `rules/platform/ffi.md` revision 2.0
- **Repository baseline:** `0f92cf4`

This file records only cross-rulebook corrections required by FFI v2.

It does not replace the target rulebooks.

The previously prepared prerequisite corrections remain applicable:

- callable type model in `types.md`;
- `@link_name` registration in attributes;
- `RawPtr[T]` null-domain wording;
- removal of ambiguous bare `...` from generic unsafe extern examples.

## 1. Grammar

Add foreign declaration/type grammar for:

```text
extern "C" type Name struct { ... }
extern "C" type Name struct
extern "C" type Name union { ... }
extern "C" type Name enum { ... }

C::name
c::namespace::name

C::fn(...) ReturnType
C::callback(...)

C bitfield declarator: bit[N]
C flexible array member: C::flex[T]

C varargs marker: final bare ...
```

`extern` function declarations remain bodyless in Sec 0.1.

General Sec-to-C exported function definitions are not added.

## 2. Struct rules

Add:

> A struct declared `extern "C"` uses the active C ABI's struct layout and is not implicitly Defaultable merely because its fields are Defaultable.

Bodyless `extern "C" type Name struct` is an incomplete foreign struct.

Ordinary Sec structs remain non-FFI-compatible unless they have an explicit foreign representation.

## 3. Union rules

Clarify that ordinary Sec unions remain tagged.

`extern "C" type Name union` is a separate foreign representation:

- untagged;
- overlapping storage;
- no compiler-tracked active member;
- active C ABI layout;
- direct member access is semantically foreign-unsafe.

## 4. Enum rules

Add `extern "C"` enum semantics:

- representation selected by active C ABI;
- open over representable raw values;
- undeclared values remain possible;
- exhaustive interpretation requires unknown handling.

This does not change ordinary closed Sec enum semantics.

## 5. Raw-pointer rules

Synchronize with FFI v2:

- `RawPtr[T]` value domain may contain the foreign zero address;
- `null` is not an ordinary Sec value;
- source use of `null` is restricted to unsafe foreign/raw-pointer context;
- `is null` is the canonical test;
- equality with `null` is invalid;
- moving a `RawPtr[T]` never transfers pointee ownership.

## 6. Borrowing/reference rules

Add the FFI call-bounded foreign borrow specialization:

```text
extern parameter ref T
    non-null shared borrow for the call
    foreign must not retain

extern parameter ref mut T
    non-null exclusive mutable borrow for the call
    foreign must not retain
```

Raw foreign pointer returns and stored foreign pointer fields do not use `ref`/`ref mut`.

## 7. Functions/callables

Add `C::fn(...) R` as a foreign C ABI function-pointer type distinct from native callable types.

Add `C::callback(callable)` as the explicit Sec-to-C callback adapter for Sec 0.1.

Accepted callback source:

- environment-free;
- reusable;
- exact foreign-facing signature.

Rejected:

- capturing closure;
- `mut fn`;
- `-> fn`.

General named C symbol export remains outside Sec 0.1.

## 8. Variadics

Keep native Sec variadics:

```sec
values: ...T
```

separate from C varargs:

```sec
unsafe extern "C" fn Foreign(
    fixed: C::int,
    ...
) C::int
```

C varargs:

- require `unsafe extern`;
- use C default argument promotions;
- do not accept `ref`/`ref mut` directly;
- do not support Sec spread;
- use target binding `va_list`, not a Sec native pack.

## 9. Layout

FFI v2 depends on the general layout rules for:

- normal C ABI struct/union layout;
- explicit packing/alignment overrides;
- misaligned access restrictions;
- layout queries.

Add the FFI-specific declarators:

```sec
field: C::uint bit[N]
field: C::flex[T]
```

C bitfield placement is ABI-owned and does not reuse Sec register bit-order semantics.

Flexible-array members are final-field-only and contain no Sec descriptor.

## 10. Attributes/effects

Extern declarations may use existing effect-guarantee attributes.

For extern declarations, the guarantees are trusted foreign contracts and their trust provenance must be preserved.

`@link_name` changes only the foreign symbol name.

No per-function `@library` attribute is introduced by FFI v2.

## 11. Error handling

Raw foreign status/null/error conventions do not become `Result` or `Option` automatically.

`try` applies only when the declared Sec expression uses ordinary Sec fallible semantics.

Wrappers explicitly normalize foreign failure into Sec domain types.

No foreign unwind or non-local transfer may cross active Sec frames in Sec 0.1.

## 12. Platform/build rules

Native libraries, objects, frameworks, import libraries, and search paths are package/build metadata resolved into the active `CompilationPlan`.

FFI declarations carry foreign symbol and ABI semantics, not repeated per-function linker dependency annotations.

## 13. Semantic IR

Preserve until ABI-aware lowering:

```text
calling convention
unsafe-extern contract
foreign symbol
C:: / c:: type identity
foreign struct/union/enum/incomplete kind
bitfield metadata
flexible-array metadata
C::fn signature
C::callback adaptation
C varargs marker
trusted effect provenance
foreign dependency references
```

Do not collapse foreign data/callable semantics into generic raw machine types before legality and ABI classification are complete.
