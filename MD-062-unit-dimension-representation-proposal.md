# MD-062 — `UnitDimension` runtime representation in Semantic IR, Sec MLIR and LLVM lowering

- **Status:** Design proposal — **not owner-approved; not normative; not integrated**
- **Created:** 2026-10-28
- **Document revision:** 0.1 (proposal for review)
- **Sec language version:** 0.1
- **Repository baseline:** `main` at `7be8777` plus the uncommitted MD-062 steps 1–5
- **Decision being addressed:** `missing-decisions.yaml` MD-062, remaining item (a) "DECISION NEEDED"
- **Intended destination after approval:** `rules/corrections/` (as an MD-062 supplement), then `applied/` only after integration
- **Primary owners:** `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`
- **Additional owners:** `rules/types/units.md` ("Semantic IR and lowering"), `rules/library/core-library.md` § 27, `rules/compiler/incremental_compilation.md`, the `frontend.units-v2` and lowering governance entries
- **Depends on:** MD-062 §§ 7(5)–7(13), 12(7)–12(8) (decided); MD-116 / v11.41.3 (latest installed early admission)
- **Separately open:** MD-062 item (b), `decimal<V>` `Reference`, which waits on unit quantity representation; MD-062 item (c), canonical `ModuleIdentity` in serialized axis identities (this proposal depends on it, see § 8)

> **Review boundary.** Every spelling below is proposed and new: `unit-dimension.*`, `!sec.unit_dimension`, `#sec.unit_dimension`, `sec.unit_dimension.*`, and the descriptor layout. The repository folds `UnitDimension` only at compile time today. Any value that escapes folding fails closed.

---

## § 1. Problem

**§ 1(1).** MD-062 makes `UnitDimension` an ordinary copyable value. It can be stored in locals, passed to and returned from functions, and kept in aggregates. Under § 7(12), runtime equality must be correct when the operands are parameters or fields. Under § 12(8), Sec MLIR and target lowering must support the value and both equality predicates once it escapes compile-time evaluation.

**§ 1(2).** Steps 1–5 implement every source-level rule. Equality, `AxisCount` and `IsDimensionless` fold to constants when both operands are direct `<U>.Dimension` reads. No version of Semantic IR or the dialect defines a type or operation for a `UnitDimension` value, so the following fail with an unsupported-feature diagnostic:

```sec
fn SameDimension(left: UnitDimension, right: UnitDimension) bool {
    return left == right
}

let d := <m>.Dimension
let n := d.AxisCount
```

**§ 1(3).** The representation must satisfy all of the following MD-062 constraints:

- (a) exact canonical-mapping equality: no raw hash, and no pointer identity without guaranteed interning (§ 7(12));
- (b) no heap allocation, no mutable global registry and no per-quantity runtime tag (§§ 7(5), 7(12), 12(8));
- (c) equivalence across modules and separate compilation, independent of placement, compile order or import alias (§ 7(13));
- (d) no promised FFI or ABI layout (§ 7(5)).

## § 2. Options considered

| Option | Runtime value | Equality | (c) across separate compilation | Cost |
| --- | --- | --- | --- | --- |
| **A. Plan-wide ordinal** | `i32` index into a dimension table interned over the `CompilationPlan` | integer compare | **Fragile.** Two separately compiled artifacts each number their own dimensions, and their ordinals collide unless a global interning step exists. That step amounts to a link-time registry. | Smallest and fastest |
| **B. Exponent vector** | fixed `[N x i16]` over the plan's axis universe | elementwise compare | **Fragile.** N and the axis order depend on which axes the plan contains, so artifacts disagree on layout. | Grows with every domain axis |
| **C. Content-addressed static descriptor** | `ptr` to an immutable constant holding the canonical encoding | pointer-equal fast path, then exact byte compare | **Robust.** Equality depends only on content, so artifacts agree without any coordination. | One pointer per value; equality is O(len) in the worst case |

**§ 2(1).** A is the "canonical collision-free ID" route in § 7(12). It is correct only while one compilation sees the whole program. Package 25 § 37 and monomorphization.md § 15.6 already admit separately compiled artifacts. A would therefore need a follow-up decision on artifact-level renumbering at link time, and that is the registry MD-062 excludes.

**§ 2(2).** B fixes a layout that every module boundary would have to share, and that layout changes whenever a project adds a domain axis.

**§ 2(3).** C is the "statically materialized immutable descriptors" route in § 7(12). A pointer is compared only as a sound shortcut: equal addresses imply equal immutable content. Inequality is never decided by address.

## § 3. Recommendation

**§ 3(1).** Adopt **C**, keep compile-time folding as the primary path, and use the descriptor only for values that actually escape. Most programs observe dimensions through `<U>.Dimension` comparisons that fold today, so the descriptor cost applies only to genuinely dynamic uses.

## § 4. Canonical axis identity and encoding

**§ 4(1).** **Axis identity.**

- The compiler-known axes `length`, `mass`, `time`, `electric_current`, `thermodynamic_temperature`, `amount_of_substance`, `luminous_intensity` and `information` keep their bare names. These names are reserved: a bare identity never contains `::`.
- Every user or domain axis is `ModuleIdentity::axis`. The `::` separator therefore keeps the two classes apart.
- Sema already stores exactly these keys (`unitAxisKey`). § 8 covers the remaining requirement that `ModuleIdentity` be the canonical, plan-independent module identity.

**§ 4(2).** **Canonical text.**

- A dimension is encoded as its non-zero entries, sorted by identity under byte-wise UTF-8 order.
- Each entry is written `identity^exponent`. The exponent is a decimal integer with a leading `-` only when negative, never `0`, and its magnitude is at most 32767 (Sema's existing `maxUnitDimensionExponent`).
- Entries are joined by `,` with no spaces. The dimensionless value is the empty string.
- Example: `length^2,mass^1,time^-2`.

Identities are identifiers or module paths and never contain `^` or `,`, so the encoding is injective. Two dimensions are equal exactly when their canonical texts are byte-equal, which is § 7(7).

**§ 4(3).** The canonical text is a private compiler encoding. It is not a source spelling, it is not what `ToString` returns (§ 7(4)), and it carries no ABI promise.

## § 5. Semantic IR (proposed)

**§ 5(1).** **Type kind.** Add the type kind `unit-dimension`, an intrinsic nominal `core::UnitDimension`, with these properties:

- copy-trivial and of immediate ownership;
- no default value: there is no `default` operation and no zero-initialization;
- no destructor.

**§ 5(2).** **Operations.** All are pure and have no failure edge.

| Operation | Operands | Result | Attributes |
| --- | --- | --- | --- |
| `unit-dimension.constant` | — | `UnitDimension` | `dimension="<canonical text>"` and the existing unit-metadata provenance `<U>.Dimension` |
| `unit-dimension.compare` | `UnitDimension`, `UnitDimension` | `bool` | `predicate=eq\|ne` |
| `unit-dimension.axis-count` | `UnitDimension` | `uint` | — |
| `unit-dimension.is-dimensionless` | `UnitDimension` | `bool` | — |

**§ 5(3).** **Verifier.**

- The `dimension` attribute must be canonical under § 4(2): sorted, no duplicates, no zero exponents, every exponent within range.
- Operand and result types must match the table above.
- A provenance annotation, when present, must name `Dimension`.

**§ 5(4).** Folding stays permitted under § 7(12): the step-5 constant folding remains a valid lowering of these operations on constant operands.

## § 6. Sec MLIR (proposed v11.41.4 early admission)

**§ 6(1).** **Version admission.** This is installed after v11.41.3, on the same terms as earlier early admissions: schema 12 inherits it unchanged, schemas 1–10 reject it, and the default emitted schema does not change.

**§ 6(2).** **Type and attribute.**

- Type: `!sec.unit_dimension`.
- Attribute: `#sec.unit_dimension<"length^1,time^-1">`. The verifier rejects non-canonical text.

**§ 6(3).** **Operations.** All are `Pure` and have folders for constant operands.

```mlir
%d  = "sec.unit_dimension.constant"() <{dimension = #sec.unit_dimension<"length^1">}> : () -> !sec.unit_dimension
%eq = "sec.unit_dimension.compare"(%a, %b) <{predicate = "eq"}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1
%n  = "sec.unit_dimension.axis_count"(%a) : (!sec.unit_dimension) -> !sec.uint
%z  = "sec.unit_dimension.is_dimensionless"(%a) : (!sec.unit_dimension) -> i1
```

`compare` mirrors `sec.ref.compare`: one operation with an `eq`/`ne` predicate rather than two operations.

**§ 6(4).** **Signatures.** `!sec.unit_dimension` is admitted wherever ordinary copy-trivial values are admitted: function parameters and results, struct fields, union payloads and fixed arrays. It is not admitted as a raw-pointer pointee, in `c::` signatures or in FFI.

## § 7. LLVM lowering (proposed)

**§ 7(1).** **Value.** A value is an opaque `ptr` to a read-only descriptor constant:

```llvm
@__sec_udim.<h> = linkonce_odr hidden unnamed_addr constant { i32, i32, [N x i8] }
                  { i32 <axisCount>, i32 <N>, [N x i8] c"<canonical text>" }, comdat
```

- `<h>` is a hex prefix of SHA-256 over the canonical text, used **only** as a symbol name so that duplicates may coalesce.
- If two descriptors share a symbol name but have different content, the build must fail at emission. Correctness never depends on the hash, because equality reads the content.
- On targets without COMDAT support, the descriptor uses `private` linkage, and duplicates stay harmless.

**§ 7(2).** **Equality.** Equality is decided as follows:

1. If the pointers are equal, the result is `eq`.
2. Otherwise, compare `axisCount`, then the length `N`.
3. Then compare the bytes with an inline loop.

No libc `memcmp` is used, so firmware and freestanding profiles need no runtime library. `ne` is the logical negation of `eq`.

**§ 7(3).** **Properties.** `axis_count` loads field 0 and zero-extends it to the plan's `uint`. `is_dimensionless` is `axisCount == 0`.

**§ 7(4).** **Guarantees.** This lowering uses no heap, no mutable globals, no initialization order and no registry, and descriptors live in read-only data.

**§ 7(5).** **Debug info.** Optionally, debug info may present the canonical text. That is a presentation only and not an ABI.

## § 8. Separate compilation and module identity

**§ 8(1).** Descriptors are content-addressed, so artifacts compiled separately produce identical bytes for the same dimension and compare equal by content. This holds even when the linker keeps duplicate copies. Compile order and import aliases do not appear in the encoding.

**§ 8(2).** This guarantee is only as strong as `ModuleIdentity` inside axis identities. Today Sema uses the module path of the declaring file. MD-062 item (c) must fix that string as the canonical, plan-independent `ModuleIdentity` and carry it through module surfaces, incremental facts and serialization (§ 7(9), § 12(6)). This proposal **requires** item (c) before claiming cross-artifact equality, but not before admitting same-artifact runtime values.

## § 9. Non-goals

- No dimension arithmetic, constructors, iteration, reflection or special `ToString` (§ 7(4)).
- No FFI or ABI layout guarantee.
- No change to source syntax, Sema typing, or the compile-time folding of step 5.
- No representation for unit quantities or `decimal<V>`. That is MD-062 item (b).

## § 10. Decisions requested

| ID | Question | Recommendation |
| --- | --- | --- |
| **D1** | Runtime representation: A, B or C | **C**, the content-addressed static descriptor |
| **D2** | Axis identity: bare compiler-known axes plus `ModuleIdentity::axis` | **Yes**, matching current Sema keys |
| **D3** | Canonical text encoding as in § 4(2) | **Yes** |
| **D4** | Semantic IR and dialect spellings in §§ 5–6, a single `compare` with `eq`/`ne`, admitted as early admission v11.41.4 | **Yes** |
| **D5** | LLVM descriptor layout, COMDAT coalescing and inline byte-compare equality (§ 7) | **Yes** |
| **D6** | Admit same-artifact runtime values now and gate the cross-artifact claim on item (c) | **Yes** |

## § 11. Required tests after approval

- **Semantic IR:**
  - round trip of all four operations;
  - verifier rejection of non-canonical text (unsorted, duplicate axis, zero exponent, exponent out of range), wrong operand types, and a default of `UnitDimension`.
- **Sec MLIR:**
  - C++ registration, parser and printer round trip;
  - attribute verifier;
  - folders;
  - schema gate: 1–10 reject, 11 and 12 accept;
  - the Go emitter and the lowering-support manifest agree on spellings.
- **LLVM:**
  - parameters, locals, struct fields and returns;
  - `==`/`!=` on equal content behind different addresses (two descriptors forced apart);
  - `AxisCount` and `IsDimensionless` on dimensionless values, on single axes and on negative exponents;
  - descriptors in read-only data;
  - no heap allocation;
  - freestanding build without libc.
- **Separate compilation (after item (c)):** two artifacts materialize the same dimension and compare equal at runtime. Same-spelled axes from different modules compare unequal.
- **Negative:** FFI or `c::` signature use is rejected, and an unsupported target fails closed with `[unsupported-lowering]`.
