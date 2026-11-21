# MD-049 — Decimal range scale: inference, coefficient capacity, and exact boundaries

- **Document class:** Normative rulebook correction / implementation contract
- **Status:** All five owner-reviewed design and integration points locked; normative correction ready for rulebook/governance integration. Not yet merged, applied, or implemented by this document.
- **Created:** 2026-11-21
- **Last updated:** 2026-11-21
- **Document revision:** 1.2
- **Sec version:** 0.1 design
- **Decision:** MD-049 (`decimal` and `decimal128` named types with range-constrained fixed scale)
- **Repository baseline:** `YoePro/sec`, `main`, commit `00e3e2664604c25f2c2ea87b6b24c80147fbde12` (verified 2026-11-21)
- **Normative owners:** `rules/types/default_values.md`; `rules/types/contracts.md`; `rules/types/types.md`; `rules/foundations/grammar.md`; `rules/foundations/lexical_structure.md`; `rules/foundations/operators.md`; `rules/library/core-library.md`; `rules/compiler/compiler_known_members.md`; `rules/compiler/semantic_ir.md`; `rules/tooling/diagnostics.md`; `rules/tooling/formatter.md`; active MLIR schema and compatibility rulebooks
- **Implementation/governance:** Existing `frontend.default-values` and type-contract/type-core integrations in `governance/types.yaml`; frontend CTE, immutable decimal facts, Semantic IR/MLIR, formatter, LSP and diagnostic registry; `missing-decisions.yaml` MD-049. A corrections-file move to `rules/corrections/applied/` follows the integration gate in § 18, not implementation completion.
- **Preserve existing decisions:** Decimal numeric representation `coefficient × 10^(-scale)`; signed 64/128-bit coefficient; signed 32-bit scale including negative values; mathematical equality; exact, source-sensitive literal scale; MD-066/067/068 arithmetic and division rules except the expressly locked zero-multiplication amendment (§ 12.8); range bound CTE requirements; and the no-silent-precision-loss policy.

## § 1. Scope and precedence

**§ 1(1).** This correction defines the `scale N` contract on decimal-family named types, fallback scale inference from `range`, the way finite coefficient width limits an inferred scale, and the resulting default-value resolution. It does **not** change the variable-scale nature of plain unconstrained `decimal`/`decimal128`, binary floats, or units. It makes exactly one general decimal-arithmetic adjustment: the scale of an exact zero product (§ 12.8), superseding `operators.md` § 27.1(5) only for multiplication.

**§ 1(2).** Preserve existing canonical paragraph IDs when merging this text. Add stable subordinate paragraphs or new subsections; do not renumber existing references.

**§ 1(3).** Fixed-scale resolution follows this order: (a) a fixed scale inherited from a named base type, whether that base scale was explicit or inferred; (b) a locally declared explicit `scale N`, provided no inherited fixed scale conflicts; (c) a `range`-derived scale from all available resolved bounds, when neither an inherited nor a local explicit scale exists; (d) variable-scale decimal semantics when no scale source exists. A fixed inherited or explicitly declared scale is never silently replaced or reduced. § 10 gives the complete inheritance and multiple-range rules.

## § 2. Fixed-scale named decimal type contract

**§ 2(1).** `scale N` is a contextual named-type contract available for the `decimal` and `decimal128` families. `N` is a compile-time integer in the full signed `int32` scale domain. It constrains the *type's representable values and representation*, not just its default initialization. It is not a second decimal primitive or a unit of measure.

```sec
type Price decimal scale 2
type Rate decimal128 scale 6 range 0..1
```

**§ 2(2).** Every materialized value of a fixed-scale named type has `Scale == N`, with mathematical value `coefficient × 10^(-N)` and a coefficient in the exact signed width of the carrier (64 bits for `decimal`, 128 bits for `decimal128`). `N` may be negative; the step is still `10^(-N)`.

**§ 2(3).** An ordinary input is accepted only when it can be *exactly* represented at the type's fixed scale, has a coefficient within the carrier domain, and satisfies all other type contracts. A source value needing rounding is not silently accepted. Explicit rounding APIs retain their existing contracts. Numeric equality and ordering remain mathematical, not representation-based.

**§ 2(4).** A zero default in a type with fixed scale `N` is represented as coefficient `0`, scale `N` (not scale 0). Other explicit and inferred defaults are also materialized with the type's fixed scale, regardless of their original literal spelling.

## § 3. Inferring a preferred scale from range boundaries

**§ 3(1).** For a named `decimal` or `decimal128` type with `range` but neither a locally explicit nor an inherited fixed `scale`, first resolve each bound in the existing compile-time constant-evaluation model. Use the resulting *decimal values' represented scales*, not the count of digit characters in raw source syntax. The normal decimal literal rule preserves written trailing zeros when representable. Constant expressions, imports, and references therefore use their resolved values' scales.

**§ 3(2).** The preferred inferred scale `P` is the greatest resolved scale across every available bound of the named type's full, source-ordered conjunction of inherited and locally declared decimal `range` contracts. No bound is selected merely because its interval is the numerically narrowest. Omitted bounds contribute no scale. For example, `range -1.00..<0.000` has `P = 3`. Do not infer a universal financial precision of 2 and do not use `float` nextafter.

**§ 3(3).** If a range bound cannot be evaluated, is incompatible with the underlying decimal family, or cannot be represented under the *existing unconstrained decimal constant rules*, use the existing declaration/CTE error. Do not fabricate a bound, coefficient, scale, or fallback precision.

**§ 3(4).** A compile-time integer bound accepted by the existing decimal-compatible `range` rules is interpreted with decimal scale **0** when used for inference; e.g. `range 0..100` prefers scale 0, and `range 0.00..100` prefers scale 2. Numeric conversion into the decimal carrier is still checked and never invents trailing fractional zeros. A computed decimal value with an actual nonzero scale takes that observed scale, not the integer spelling of its numerator or rational denominator.

## § 4. Coefficient capacity has priority over fractional resolution

**§ 4(1).** A preferred inferred scale is not a promise that the carrier can represent the interval's full required whole-number magnitude at that scale. For inferred (never explicit) scale, reduce `P` as far as needed to preserve coefficient capacity for the interval's numerical extent. The whole-number magnitude takes precedence over the number of fractional places written at the other endpoint. This rule applies equally to negative and positive bounds and to `decimal`/`decimal128`.

**§ 4(2).** Let `B` be the exact, resolved numerical interval boundaries, including their sign. Let `Cmin = -2^(W-1)` and `Cmax = 2^(W-1)-1`, where `W` is 64 or 128. The inferred scale `S` is the **greatest** signed-`int32` integer `S <= P` satisfying the coefficient-capacity envelope for every bound `b`:

```text
if b > 0: ceil(b × 10^S) <= Cmax
if b < 0: ceil((-b) × 10^S) <= -Cmin
if b = 0: the bound imposes no coefficient-capacity limit
```

All comparisons are mathematical/exact; the ceiling is used **only for the capacity check**, not to change any bound or construct a program value. The signed negative asymmetry is intentional. This rule may select `S = 0` or a negative scale; it is not artificially clamped to two or zero decimal places.

**§ 4(3).** A conforming compiler may implement § 4(2) with digit-count/exponent comparisons or bounded exact integer operations. It must not materialize huge `10^N` numbers, enumerate values in the range, loop billions of scale candidates, rely on host binary floats, or construct a very long decimal string merely to resolve a scale. Signed-`int32` scale underflow and size limits are checked.

**§ 4(4).** The original interval boundaries **remain exact numerical boundaries**, independently of inferred `S`. They need not themselves be members of the fixed-scale value grid. An included bound between grid points is still an included mathematical limit; the type's actual members are exactly the grid points satisfying the original bounds. There is no silent rounding, truncation, widening or rewriting of the `range` contract.

**§ 4(5).** If no such scale is possible within the signed-`int32` domain, or if the resolved grid and range have no common representable values, the declaration is invalid with a precise diagnostic. The compiler never silently widens the coefficient type or changes `decimal` into `decimal128`. An explicit or inherited fixed `scale N` is authoritative and is not automatically lowered by this rule. Under owner-locked point 1, a declared finite range bound beyond the fixed-scale carrier's numerical capacity makes that declaration invalid at compile time; an in-capacity boundary that lies between grid points remains an exact numerical bound rather than requiring rounding.

## § 5. Range default after scale resolution

**§ 5(1).** After fixing `S`, default selection considers only exact grid values `c × 10^(-S)` with `Cmin <= c <= Cmax` that satisfy all declared contracts. Test numeric zero first; otherwise select the *unique* member nearest to zero. An equal-distance tie requires an explicit default; an empty intersection is invalid.

**§ 5(2).** For exclusive upper bound zero, the nearest negative grid value, when otherwise in range, is `-1 × 10^(-S)`. The compiler must use exact coefficient/scale arithmetic, not a tiny expanded rational or a binary-floating predecessor.

**§ 5(3).** A calculated/default value must carry the inferred or explicit fixed scale `S`, even when an equivalent ordinary decimal can be represented at a different scale. Original literal scales are unchanged outside this fixed-scale named-type conversion.

## § 6. Examples

**§ 6(1).** Basic inference:

```sec
type Negative decimal range -1.00..<0.000
// Inferred scale: 3
// Values: -1.000, -0.999, ..., -0.001
// Default: -0.001
// Count: 1000
```

**§ 6(2).** Whole-number magnitude reduces fractional resolution:

```sec
type Wide decimal range 0.001..1000000000000000000
// Preferred scale from bounds: 3
// Selected scale: 0 (64-bit coefficient cannot hold 10^18 × 10^1)
// The lower bound is still exactly 0.001.
// Actual members are integral grid values from 1 through 10^18.
// Default: 1, represented with scale 0.
```

**§ 6(3).** The same case with an explicit fixed scale is *not* silently changed:

```sec
type WideExplicit decimal scale 3 range 0.001..1000000000000000000
// Compile-time error: the upper bound does not fit decimal at scale 3.
// No implicit scale reduction or rounding.
```

**§ 6(4).** For a range admitting much larger whole-number magnitudes, the inferred scale may be negative if the underlying decimal bound is itself valid at a negative scale. For `decimal128`, the same algorithm uses its signed 128-bit coefficient domain, typically preserving more fractional resolution.

**§ 6(5).** Inclusive bounds do not force the compiler to choose a boundary's original scale for the result. The *type's* resolved `S` is authoritative. Source textual scales influence `P` only and do not override coefficient capacity.

## § 7. Diagnostics and tooling

**§ 7(1).** A `range`-derived scale with no explicit `scale` emits a low-severity **information** diagnostic, not a warning or error. It reports the preferred and selected scale and invites explicit source spelling. It remains configurable by usual diagnostics presentation policy; this information alone does not fail a build.

**§ 7(2).** If coefficient capacity reduces the inferred scale, the information message must also state why and identify the limiting boundary/type width. It should mention when a boundary is not itself representable on the resulting grid while exact range comparisons remain unchanged.

**§ 7(3).** The Sec LSP should offer a semantics-preserving quick fix to add the resolved `scale S` contract, preserving the original `range` and its literals. The formatter must preserve semantically meaningful decimal trailing zeros in boundary literals. Hover may display `Scale`, step, original range, capacity reason and resolved default.

**§ 7(4).** Suggested information diagnostic:

```text
info: inferred decimal scale 0 (preferred scale 3)
The bound 1000000000000000000 requires the full integer coefficient
capacity of decimal. The exact range remains 0.001..1000000000000000000;
values of this type use integer steps. Consider declaring `scale 0` explicitly.
```

## § 8. Implementation and conformance

**§ 8(1).** Integrate `scale N` into the canonical named-type contract grammar, parser, AST, Semantic IR and type metadata while preserving the writable `Scale` property on ordinary `decimal`/`decimal128` and suppressing that setter only for fixed-scale named receivers (§ 12.1). Named types and inherited contracts must not discard the resolved fixed-scale requirement. Reject duplicate/conflicting explicit scale contracts, non-decimal use and out-of-range scale constants.

**§ 8(2).** The type checker resolves bounds exactly, derives `P`, applies § 4 capacity reduction, freezes `S` in the type contract, tests that the range has representable grid values, then selects its default under § 5. Downstream lowering, LSP and formatter consume the same fact rather than re-inferring it.

**§ 8(3).** Conformance cases include: trailing-zero distinctions; bounds from constants/imports/CTE arithmetic; `decimal` versus `decimal128`; 64-bit negative minimum versus positive maximum; integer-part-driven reductions 3→2→1→0→negative; exact interval comparisons with bounds between grid points; exclusive zero and nonzero bounds; empty grid/range intersection; zero with fixed nonzero scale; explicit-scale precedence; `scale` invalid on non-decimals; compound contracts; diagnostic and quick-fix fidelity; no giant-power or binary-float evaluation; formatting idempotence; cold/incremental/LSP agreement.

**§ 8(4).** Update existing `governance/types.yaml` `frontend.default-values` and relevant type-contract/compiler ownership records. Reconcile MD-049 in `missing-decisions.yaml` after integrating the normative rule; do not mark implementation complete merely because a normative correction is accepted. Remove the historical assumption that endpoint spelling never has an effect: in this **specific** fallback policy the compiled decimal bound's represented scale legitimately contributes to the inferred fixed-scale contract. This is not an authorization to use spelling as a universal precision policy.

## § 9. Explicitly unchanged rules

**§ 9(1).** Ordinary `decimal`/`decimal128` without fixed-scale contracts still use their existing variable-scale rules. Arithmetic (`+`, `-`, `*`, `/`, `%`), `Rescale`, `RoundTo`, equality, ordering, conversion failures, and signed scales retain their existing normative owners and behavior **except** for the expressly locked fixed-scale named-type obligations (§ 12) and exact-zero multiplication scale (§ 12.8). This correction does not adopt IEEE decimal interchange formats or Java `BigDecimal` wholesale.

**§ 9(2).** No new decision is required to preserve exact range comparisons or coefficient capacity. If integration exposes a concrete incompatibility with already accepted type-bound rules, report the conflict explicitly rather than silently introducing rounding or redefining bound validity.


## § 10. Inheritance, multiple ranges and absent bounds (owner-locked point 2)

**§ 10(1) — Effective scale across named ancestry.** For `type Child Parent`, if `Parent` has a fixed scale `S` (whether explicitly declared or previously inferred), `Child` has the same fixed scale `S` before considering its own local contracts. That requirement is transitive across all named-type generations and remains true after generic specialization. A child cannot widen or loosen the parent's set of admissible values or representations. Nominal type identity is unchanged by inheritance of the scale.

**§ 10(2) — Local `scale` on a fixed-scale base.** A child may repeat the inherited `scale S` as an identical assertion; it has no new semantic effect and is not a request to re-infer. A different local `scale T`, where `T != S`, is a compile-time declaration error even when the two scales happen to represent a particular numeric value equally. Repeated local `scale` declarations in the *same* declaration are invalid under § 8(1), including identical repetitions; this is distinct from restating the same inherited requirement. No conversion, rounding, or rebasing of an inherited type occurs implicitly.

**§ 10(3) — Child without inherited fixed scale.** When the base type retains ordinary variable-scale `decimal`/`decimal128` semantics, a locally explicit `scale N` fixes the child's scale. If no such local declaration exists, and local or inherited `range` contracts contain evaluable, scale-bearing decimal bounds, resolve one scale according to §§ 3–4. The child's resulting fixed-scale contract does not retroactively change the parent. If no scale-bearing bound exists anywhere in the effective contract set, the child remains variable-scale. A parent with an effective decimal `range` normally has already resolved its own scale; the child then follows § 10(1), not a second inference pass.

**§ 10(4) — Logical conjunction.** Multiple `range` contracts are permitted by the existing `TypeContract` repetition and § 17(2) of `rules/foundations/grammar.md`. All inherited and local ranges remain independent, source-provenanced contracts evaluated as one logical AND; their effective *numeric* domain is the intersection of those ranges and the applicable decimal carrier domain. An additional range can narrow that domain but cannot widen it, even when its written endpoints are broader than the parent's. Do not replace the source-ordered list of contracts with a single synthetic range for diagnostic, `ContractError.DeclarationIndex`, or incremental-identity purposes.

**§ 10(5) — Scale inputs versus numeric domain.** Where inference is permitted, *all* available finite and resolved decimal range bounds contribute their represented `Scale` to the preferred scale `P`, including bounds that are numerically redundant because another range is narrower. This intentionally gives range-bound representation an effect on fallback scale inference; numerical equality alone does not make their `Scale` information interchangeable. The fixed-scale value set must satisfy **all** ranges after `S` is resolved. §§ 3–4 continue to govern coefficient-capacity reduction: every explicitly written finite bound remains subject to its established exact-bound and capacity-validation contract. A boundary outside the representable fixed-scale carrier domain does not become valid merely because a second interval makes it redundant; in inferred-scale cases, coefficient-capacity reduction occurs before the final range/grid validity check.

**§ 10(6) — No scale re-inference for a fixed-scale child.** A new child's `range` bound containing more decimal places than inherited `S` does not alter `S`; the bound is compared exactly and is not rounded. The resulting permitted values are the inherited grid points satisfying the new bounds. If none remain, the declaration is invalid; a more precise bound is not itself an error merely because it falls between grid points (unless it violates the separately locked representability/domain rule of § 4(5)). A derived type's implicit default is independently resolved from the child's full effective value set; do not blindly reuse the parent's default if it violates a new range.

**§ 10(7) — One-sided ranges.** `RangeContract ::= "range" [Expression] RangeOperator [Expression]` admits an omitted bound. An absent lower or upper bound contributes no numeric endpoint, no inferred-scale candidate and no invented infinite/maximum-scale value. The existing scalar carrier domain still bounds actually representable values. With only one present, scale-bearing bound, infer from that bound when there is no fixed inherited or explicit scale. Its exact inclusive/exclusive semantics are preserved. A declaration with no scale-bearing bounds and no explicit/inherited scale keeps variable-scale semantics; `range ..` is not itself a request to introduce a fixed scale.

**§ 10(8) — Empty intersections and defaults.** The compiler must reject an empty mathematical intersection of contracts (or an intersection with no valid fixed-scale grid points) under the existing contract-consistency rules. A valid derived type's implicit default is computed using the resolved child's entire contract conjunction, following § 5. An explicit default of the child takes precedence but must itself be exactly representable at the inherited/declared fixed scale and satisfy every effective contract. Neither a parent's existing implicit default nor a child's differently spelled range endpoint overrides these rules.

**§ 10(9) — Information diagnostic and tooling.** Scale inference is reported with the already approved informational diagnostic only at a declaration that actually infers a scale. A child inheriting a fixed scale emits no second inference notice merely because it adds a `range`. If a quick fix inserts `scale S` for a declaration with inferred scale, the inserted clause must preserve the original effective scale, all range boundaries, ancestry constraints and default; it must never silently rewrite inherited types or remove meaningful trailing zeros. A conflicting explicit/inherited scale is an error, not an informational diagnostic.

## § 11. Point-2 conformance vectors (normative expected outcomes)

**§ 11(1).** The following examples fix the semantics of inheritance and multiple ranges. `Scale` denotes the resolved *type-level* scale, not the scale of an unconverted literal in its source context. Every expected default is a mathematical value materialized with the shown scale.

| Source | Effective scale | Effective domain / default | Expected validity |
|---|---:|---|---|
| `type Amount decimal scale 2; type Price Amount range 1..10` | 2 | Values `1.00`–`10.00` in `0.01` steps; default `1.00` | Valid; inherited scale |
| `type Amount decimal scale 2; type Bad Amount scale 3` | inherited 2 | Cannot change parent contract | Compile-time error |
| `type Amount decimal scale 2; type Good Amount scale 2` | 2 | Same scale, new nominal identity | Valid; identical restatement |
| `type Root decimal; type Child Root range -1.00..<0.000` | 3 | `-1.000`–`-0.001`; default `-0.001` | Valid; child's inference does not alter Root |
| `type Base decimal range -1.00..<0.00; type Child Base range -1.000..<0.000` | 2 | `-1.00`–`-0.01`; default `-0.01` | Valid; more precise bounds do not re-infer |
| `type Multi decimal range -100.00..100.00 range -10.000..10.000` | 3 | `-10.000`–`10.000`; default `0.000` | Valid; conjunction; maximum bound scale |
| `type Neg decimal range ..<0.000` | 3 | All representable grid points below zero; default `-0.001` | Valid; omitted lower bound gives no scale |
| `type Pos decimal range 0.00..` | 2 | Nonnegative representable grid points; default `0.00` | Valid; omitted upper bound gives no scale |
| `type Parent decimal scale 2 range 0..10; type Empty Parent range 11..12` | 2 | Empty intersection | Compile-time error |
| `type P decimal scale 2; type Q P range 0.001..<0.020` | 2 | Only `0.01`; default `0.01` | Valid; exact bounds between grid points |

**§ 11(2).** Implementations must test each case with `decimal128` where the scalar-width distinction is relevant, with multi-level derivation and with imported compile-time decimal bounds. Tests must assert the type-level scale, actual default `(coefficient,scale)`, accepted/rejected runtime conversions, `ContractError` provenance for inherited versus local failure, formatter preservation and cold/incremental/LSP equivalence. Negative cases must assert declaration rejection; informational findings must not be used as substitutes for errors.

**§ 11(3).** No test in this section is evidence that the current frontend already implements the required behavior. Existing integer-range intersection logic and general grammar provide precedent, but decimal range intersections, fixed-scale nominal invariants and downstream propagation remain explicit implementation work under MD-049.


## § 12. Fixed-scale invariants, arithmetic, core members, and zero products (owner-locked point 3 and point-4 amendment)

**§ 12(1) — Immutable value invariant.** Every reachable value of a nominal fixed-scale decimal type `T` has exactly the resolved scale `S`, an in-width signed coefficient, and a mathematical value satisfying **every** inherited and local contract. No safe creation, constant, assignment, arithmetic, property, compiler-known member, FFI, or lowering path may construct a `T` without this proof or its corresponding checked runtime path. The nominal identity of `T` is not its carrier identity; there is no implicit `decimal` ↔ `T` conversion. This remains true in generics and all module boundaries. Unsafe operations do not retroactively make an invalid value safe.

**§ 12(2) — Read-only `.Scale` on fixed-scale named types.** The inherited `Scale` getter exists and returns the exact type-level `S`. The inherited `try set requestedScale PrecisionError` is **not** available on `T` or a derived named type with fixed scale. An attempted `try t.Scale = 3` or `t.Scale = 3` is a compile-time non-writable-member error, including when the requested value equals `S`. The compiler must not use an implicit carrier conversion to make that setter available. The existing fallible, transactional setter on **plain** `decimal` and `decimal128` is unchanged. `core-library.md` § 10.1 and `compiler_known_members.md` §§ 10, 14 need explicit synchronization. The `Scale` getter remains distinct from the lowercase contextual contract spelling `scale` (§ 14).

**§ 12(3) — Named arithmetic result type.** When Sec's existing numeric-operator compatibility rules permit an arithmetic operation on two values of the same nominal constrained decimal type `T`, its **result is `T`**, not a silently erased carrier. This includes `+`, `-`, `*`, `/`, `%`, unary minus where valid, and corresponding compound assignments. Literals may shape to `T` only under the existing literal-shaping and exact-fit rules. No new conversion between unrelated nominal types is introduced. Other operator signatures, including mixed or unit-bearing operations, continue to follow their canonical owner; this clause does not automatically make them legal.

**§ 12(4) — Result check order.** For a result that must be `T`, (1) evaluate operands exactly once in existing order; (2) apply the operation's mathematical semantics and intrinsic error policy; (3) determine whether that result can be represented **exactly** with `T`'s required scale `S` and coefficient width; (4) check all inherited and local value contracts in canonical source order; (5) publish the new `T` value only after all checks succeed. An inexact result is never rounded to satisfy `T`. In particular, the ordinary division rounding permission of `operators.md` § 27.2 does **not** allow a `T / T` operation to approximate its mathematical quotient merely to produce `T`; any inexact quotient is a checked failure at this nominal boundary. Unconstrained primitive decimal division remains unchanged. Exact mathematical results may be rescaled *without rounding* into `T`'s fixed representation.

**§ 12(5) — Failure and transactional mutation.** Existing `try` and ordinary checked-operation rules apply (§ 15). A failed compound assignment leaves the original stored `T` unchanged, whether failure is intrinsic, scale-related or contractual; no partial coefficient/scale update is observable. Valid same-`T` copy/move assignments do not recheck an already proven invariant solely because the storage location changes. Static proof may eliminate runtime checks but must not alter the defined error class or value.

**§ 12(6) — Core value-producing methods.** An inherited core/known method with a declared `decimal`/`decimal128` return type does **not** silently substitute the constrained nominal type `T` when that substitution could bypass a fixed-scale or value contract. The specifically owner-locked `Normalize()`, `Rescale(scale)` and `RoundTo(scale[, mode])` methods, when exposed on a fixed-scale named receiver, retain their underlying-carrier output and their existing `PrecisionError` channel; they do **not** mutate or directly create a `T`. In particular `Rescale`/`RoundTo` retain `Result[decimal, PrecisionError]` (or `Result[decimal128, PrecisionError]`) and `Normalize()` returns the corresponding ordinary carrier. To obtain `T` again requires an existing checked `T(result)` conversion. Compiler-known `T`-substitution rules in `compiler_known_members.md` § 10(5) must be narrowed for these named fixed-scale receivers, not left contradictory.

**§ 12(7) — Other inherited methods.** Keep their original core signatures and applicable nominal-result substitution **except** for the explicit carrier-return exceptions in § 12(6). Read-only observations (`IsZero`, `Sign`, `ToString`, etc.) cannot invalidate `T`. For any other core method legitimately inherited on `T` whose ordinary "receiver type" result resolves to `T` (including `Floor`, `Ceiling`, `Truncate`, `Round`, `Abs`, `Min`, `Max`, `Clamp`), the compiler must perform the same exact-grid and full-contract validation as for any other operation constructing a `T`, using the existing contract/`try`/panic rules on failure; it may not emit an unchecked nominal result or reinterpret `T` as an unconstrained carrier. A method that cannot satisfy its declared effect/error contract for that check is **not** safely inheritable until the owning rulebook specifies a valid checked path; do not invent a new silent return type. The exact member availability and result types must agree across resolution, hover/completion, interfaces, Semantic IR, and runtime. This subsection changes neither primitive decimal methods nor user-defined members on `T`.

**§ 12(8) — Exact zero multiplication, including loops.** Amend `rules/foundations/operators.md` **§ 27.1(5)** without renumbering it. If a multiplication of *ordinary variable-scale decimal values* has an exactly zero mathematical product, the result coefficient is `0` and the preferred/result scale is `max(left.Scale, right.Scale)`, not `left.Scale + right.Scale`. The rule is symmetric and is applied at every multiplication, including constants, CTE, runtime, repeated loop iterations, and zero inputs with negative scales; no iterative-history or optimizer-dependent exception is allowed. It intentionally changes observable `.Scale` for zero products. An exact zero product targeting a **fixed-scale nominal** `T` instead takes the mandatory `T` scale `S`. Other exact zero operations retain existing § 27.1(5) behavior (including int32 clamping of their own preferred scale); nonzero multiplication retains its existing preferred-scale sum. Neither rounding nor general trailing-zero normalization is introduced.

```sec
// Expected primitive decimal coefficient/scale pairs:
// 0.00m  * 0.000m => (0, 3)
// 0.000m * 0.00m => (0, 3)
// 0.000m * 0.000m => (0, 3)
// 0.00m  * 1.234m => (0, 3)
// 1.20m  * 2.0m   => (2400, 3)
```

**§ 12(9) — No unrestricted repeated-growth effect.** A loop multiplying a zero decimal repeatedly by a constant of any scale cannot increase the zero result's scale beyond the greatest of the two input scales encountered so far. For a fixed-scale `T`, every successful iteration yields scale `S`. Independent nonzero products keep the MD-066/067 exact scaling behavior; this exception does not introduce automatic precision limiting for them.

## § 13. Decimal scale-preservation contract across frontend, CTE and IR (owner-locked point 4)

**§ 13(1) — Logical fact.** An exactly known decimal constant is conceptually a pair `(Coefficient: arbitrary-precision signed integer, Scale: signed int32 after valid decimal representation resolution)` plus carrier width (`decimal`/`decimal128`), nominal type identity if any, source provenance and the exact source lexeme where available. A transient literal parser may hold a wider scale until the existing int32 fit check. The mathematical `big.Rat` value may be cached **in addition** but cannot replace the scale-bearing fact. Equal rationals may carry different `Scale`. `Lexeme` is provenance, never the authoritative representation for a *computed* value.

**§ 13(2) — Literal and expression operations.** Preserve lexical trailing-zero scale when the literal is representable under existing literal rules. In CTE expressions apply the same exact arithmetic scale policy, zero-product exception (§ 12.8), and carrier capacity as the corresponding runtime operator. This includes arithmetic with integer operands at scale zero, signed scales, unary signs, literals, const bindings, compile-time functions, conversion, import and generic specialization. A computed constant with the mathematical value `2.4` and represented scale `3` is **not** indistinguishable from a computed constant with represented scale `1`.

**§ 13(3) — Known frontend loss boundary.** Current `internal/sema/types.go` `DefaultConstant` stores `Exact *big.Rat` and `DecimalBits` but not a distinct representation scale; `RangeContract.ExactMin/ExactMax` likewise store exact rationals plus lexemes. `internal/sema/contract_cte.go` `semanticCompileTimeExact` returns `(big.Rat, lexeme)`, and `contract_cte_operators.go` may synthesize decimal text through `constant.DecimalText`. These are **implementation gaps** for MD-049. Add an immutable scale-bearing result on the semantic constant path, retaining mathematical `Exact` only as a supplementary comparison representation. Both bounds must retain their own exact decimal pair and original source/provenance independently. Do not parse formatted decimal output to infer it. Existing `DecimalValue` in Sema and `semantic.DecimalConstant` in IR can be reused if their invariants are extended correctly; a new duplicate public decimal type is unnecessary.

**§ 13(4) — Bound resolution and type identity.** A `range` bound is first evaluated in its normal `SemanticCompileTimeRequiredContext`; its resolved decimal representation is retained even when the value originated in another module, a chain of immutable bindings, a function result or a generic specialization. Store separate `(exact mathematical value, represented decimal scale, source location)` for each present bound, including numerically redundant bounds. The `range` proof remains exact and scale-independent for numerical comparisons, but preferred scale inference is scale-sensitive. Freeze the final type-level scale once per resolved nominal declaration/instantiation; expose that read-only fact to every consumer with origin classified as `explicit`, `inherited`, or `inferred` and, for inferred results, preferred scale `P` and limiting coefficient-bound reason.

**§ 13(5) — Defaults and materialization.** Default resolution produces a decimal **coefficient/scale** pair matching the final type-level `S` and never reparses `DefaultResolution.Value.Lexeme` as authority. In current `internal/ir/semantic/defaults.go` `buildResolvedDefault` calls `parseDecimalFor(resolution.Value.Lexeme, ...)`; replace that authority path with a typed semantic default fact. Preserve exact `0` at `S` and exact negative-nearest defaults without generating enormous decimal expansions. Explicit source defaults are accepted only if the final nominal representation and all contracts validate them; a wrong-scale but numerically exact literal is rescaled exactly as allowed, not rejected just for different written zeros.

**§ 13(6) — IR and MLIR fidelity.** `internal/ir/semantic/model.go` already models `DecimalConstant{Coefficient,Scale,Lexeme}` and `internal/lowering/secmlir/emitter.go` already emits coefficient and signed-i32 scale in `sec.const.decimal`; these are useful existing foundations, **not proof that contract-derived scale is wired through**. Semantic IR builds exclusively from immutable Sema-resolved coefficient, scale, nominal contract and proof facts. MLIR emission and verification must preserve the full representation and checked contract obligations through every supported lowering boundary. An optimizer must never alter an observable `.Scale` or silently turn a nominal value into a plain carrier. If a selected schema or lowering package lacks the required representation/guard, reject that compilation path explicitly and retain partial status; do not invent an unverified operation or silently omit the check.

**§ 13(7) — Caches, formatter, LSP.** Decimal representation scale and fixed-type contract policy are semantic dependencies for incremental compilation, serialized caches, module interfaces, generic instantiations and LSP; edits of trailing zeros or a bound expression must invalidate dependent inferred scales even when the rational number is unchanged. A cold and valid incremental build must agree. Ordinary formatting preserves all numeric literal spellings, including significant decimal zeros; it must not 'normalize' `1.200m` to `1.2m`. Hover and quick fixes read the same resolved type-level facts. Format, parse, compile and print/parse round trips preserve the represented scale where applicable.

## § 14. Source grammar and contextual `scale` name (owner-locked point 5)

**§ 14(1) — Exact grammar amendment.** Add `ScaleContract` to `rules/foundations/grammar.md` § 17 without changing the `TypeDeclaration` or `NamedTypeDeclaration` numbering. The implementation-facing production is:

```text
TypeContract
    ::= RangeContract
      | ScaleContract
      | MembershipContract
      | MultipleOfContract
      | LengthContract
      | RegexContract
      | MarkerContract

ScaleContract
    ::= Contextual("scale") Expression
```

**§ 14(2) — Context, not reservation.** `scale` is an ordinary identifier lexically and may be a variable, field, function or parameter name outside the **head of a scale contract**. Recognize it contextually only at a named-type contract boundary; it does not become a hard keyword or globally reserved contract spelling. This is an **explicit exception** to `lexical_structure.md` § 7.3's normally reserved contract words. Add a dedicated subsection/caveat there, not `scale` to the globally reserved list. It is case-sensitive: `Scale` continues to name the instance property and is not the contract word. Do not change unit metadata fields also named `Scale`.

```sec
let scale := 3

type Rate decimal scale 2
// The binding 'scale' remains a legal identifier.
```

**§ 14(3) — Argument domain.** `scale` accepts exactly one `Expression`, evaluated under `SemanticCompileTimeRequiredContext`, whose computed result is an integral value in `[int32.Min, int32.Max]`. Signed/negative expressions and immutable compile-time constants are valid. Floating or decimal values that are merely mathematically integral are **not** implicitly accepted as the scale argument; use the existing explicit integer conversion if defined and valid. A runtime-dependent, nonintegral, missing, extra, or out-of-int32-domain argument is a compile-time error. Its exact `int32` result determines the type grid, not the argument's own numeric lexical representation.

**§ 14(4) — Contract order and parsing.** Existing ordered, repeated contract syntax is preserved. An explicit `scale N` may appear before or after another named-type contract and before `default`, provided the grammar yields an unambiguous sequence. A `range` bound is always an ordinary expression until its established terminating boundary. If an identifier spelled `scale` can be parsed as part of a legal ordinary `range` bound expression, the parser gives that expression precedence; it is a `ScaleContract` only at the next **confirmed** contract-head boundary. E.g. `range 0..scale` and `range 0..scale + 2` refer to an in-scope constant identifier `scale`, not a scale clause. To express a trailing open upper bound and a scale contract without ambiguity, write `scale 2 range 0..`. The lexer must not make the choice by raw token spelling alone, and the formatter must not change this parse. Parser recovery keeps invalid/missing `scale` arguments as a recoverable contract node without eating a following `range` or `default` clause.

**§ 14(5) — Applicability and duplicates.** `scale` applies only to named numeric types with underlying `decimal`/`decimal128` carrier; it is invalid on `int`, binary `float`, storage sites, fields and parameter declarations. Exactly one local explicit `scale` is allowed. A repeat of the **same inherited value** is permitted as an assertion (§ 10.2), but two local clauses are invalid even if equal. A conflicting inherited scale is invalid. The ordinary `range` grammar remains inclusive lower and inclusive/exclusive upper (`..`, `..<`); `scale` neither introduces a new range operator nor changes missing-bound syntax.

## § 15. Exact diagnostics and runtime failure mapping

**§ 15(1) — New advisory registry identity.** Allocate and register `A2012` / `types.inferred-decimal-scale` in the canonical diagnostics registry, after checking all active and retired IDs before integration. This proposed stable allocation was not present in main `00e3e266` on 2026-11-21. Family `types`; configurable, nonmandatory, default severity `Information`, never a warning/error by default. Emit it once for the named declaration **that actually inferred** a scale; do not emit it merely for an inherited fixed scale or a plain variable-scale decimal. Include structured arguments: named type, carrier width, preferred scale, selected scale, whether capacity reduced it, and the limiting bound with related source location when applicable. Message and help must explicitly suggest a legal source `scale <selected>` insertion. Register the stable name, arguments, explanations and any quick-fix code through `rules/tooling/diagnostics.md` §§ 5–7 and `internal/diagnostics/diagnostics.go`; LSP uses the same ID. Do not emit an unregistered diagnostic.

**§ 15(2) — Existing error identities.** Reuse registered errors where their meanings fit: `S1064` / `types.inapplicable-contract` for `scale` on a non-decimal base or storage site as applicable; `S1066` / `types.invalid-contract-argument` for a non-integer, out-of-`int32`, non-CTE or missing `scale` argument; `S1065` / `types.unsatisfiable-contract-set` for conflicting inherited/local scale, duplicate locally authored scales, inconsistent ranges, no grid member, or a finite bound outside the fixed-scale coefficient domain. A nonrepresentable **explicit default** uses existing `S1060` / `types.default-not-representable`; a representable default violating a `range` uses `S1059` / `types.default-violates-contract`. The existing `S1061` / `types.ambiguous-implicit-default` means a valid type lacks a unique implicit nearest-zero default, not that the type itself has an empty value set. Compiler messages must identify the type, original relevant bound, fixed scale, coefficient width and a concrete remedy. When a symbol has a more specific existing mandatory diagnostic (e.g. obsolete storage-site contract), retain that canonical diagnostic rather than emitting a competing `S1064`.

**§ 15(3) — Checked conversion error matrix.** After a legal numeric conversion relation has been established, validate the underlying carrier and **required fixed-scale representation** first, then declared value contracts in source order, as `rules/types/contracts.md` § 13 already requires. Runtime-dependent failures use existing `ConversionError`, without defining a new public enum:

| Cause | Exact error |
|---|---|
| Exact numeric value cannot be placed on the target grid without rounding | `ConversionError.PrecisionLoss` |
| Exact coefficient at target `S` is integral but outside the target signed coefficient domain | `ConversionError.OutOfRange` |
| Scale itself lies outside signed `int32` (only reachable in relevant generic/runtime conversion contexts, not a valid fixed-scale declaration) | `ConversionError.ScaleOverflow` |
| Intrinsically valid and exactly representable value violates a declared `range` | `ConversionError.Contract(ContractError.Violation { Kind: Range, DeclarationIndex: failing-source-contract-index })` |
| All checks succeed | Construct `T` with exactly `(coefficient, S)` |

Conversion relation failures remain compile-time errors. `try` exposes the typed error; ordinary panic-capable conversion uses the existing MD-131 panic model. A compile-time-proven invalid conversion is diagnosed rather than emitted as a runtime failure. The failure must not corrupt or partially replace destination storage.

**§ 15(4) — Named arithmetic error matrix.** A permitted arithmetic operation producing `T` uses the existing `ArithmeticError` (`core/error.sec`), not `ConversionError` and not a new `DecimalArithmeticError`: divisor zero maps to `ArithmeticError.DivisionByZero`; intrinsic arithmetic overflow uses its existing most-specific variant; an exactly evaluated mathematical result that cannot be placed at required fixed scale `S` **without rounding** maps to `ArithmeticError.PrecisionExhausted` (including nonintegral required coefficient); a selected scale outside int32 maps to `ArithmeticError.ScaleOverflow` only when no legal scale exists under the operation's actual policy. If intrinsic arithmetic succeeds and the `T` representation fits but a declared range fails, use the existing contract failure endpoint (`ConversionError.Contract` through `try`, `ContractFailure` panic without `try`) without changing its provenance or indexing. A single failure path cannot report the same event as both a precision and contract violation; intrinsic failures precede contract checks.

**§ 15(5) — `try` reconciliation.** Under owner decision MD-131, the untried runtime named-conversion/compound-update form is valid and may panic on contract failure; the tried form exposes the existing typed failure. Reconcile the older unconditional 'a conversion that may fail requires `try`' wording in `rules/types/types.md` § 25(5) with `rules/types/contracts.md` § 12 without renumbering either: general numeric fallible conversion rules still require their specified `try`, while MD-131's checked named-contract paths explicitly admit untried panic-capable forms. A fallible property setter on *plain* decimal retains its independently required `try`; fixed-scale named values have no setter at all (§ 12.2).

**§ 15(6) — Informational quick fix.** The `A2012` quick fix must insert a valid explicit `scale S` clause without changing range literal spelling, bound evaluation, inherited contracts, the final grid, default value, or source meaning. Where insertion after an open range could change parsing (§ 14.4), place it **before** the first `range`. If adding the clause would change some semantic property or violate applicability, suppress the quick fix and emit information only. The quick fix does not automatically rewrite numeric bounds or silently reduce a developer-declared scale.

## § 16. MLIR schema, integration and historical compatibility

**§ 16(1) — Active versus proposed schemas.** At baseline `00e3e266`, `rules/mlir/sec_mlir_dialect.md` declares **active schema 12** (installed 2026-10-19). `rules/mlir/dialect-versions/sec_mlir_dialect_v17.md` and P21 decimal operations are **proposed**, not proof of implementation. Do not mark v17 active or advance the active schema solely because MD-049 is normative. Any future schema additions use the owner-governed package/version process; this correction does not mint a new operation/type or silently change backend contracts.

**§ 16(2) — v2 historical scale inconsistency.** Historical `rules/mlir/dialect-versions/sec_mlir_dialect_v2.md` § 21 says `scale is non-negative`; the canonical source language since MD-066 permits the full signed int32 domain, and v3+ decimal attributes use signed `IntegerAttr(i32)`. Preserve the historical v2 snapshot and its verifier if needed to read legacy schema-v2 inputs; add an explicit **compatibility note** that v2's old nonnegative requirement is schema-local/historical and must **not** override the current Sec source language or silently force current modules into v2. Regression-test old v2 input acceptance/rejection separately from active v12 negative-scale behavior. Correct the active canonical source/dialect precedence where it is ambiguous.

**§ 16(3) — Existing constant and named-type forms.** Preserve `!sec.decimal` and `!sec.decimal128` with their signed-coefficient/signed-scale semantics. Modern `sec.const.decimal` carries `coefficient: StringAttr`, `scale: IntegerAttr(i32)`, `lexeme: StringAttr` with a verified concrete result type/nominal wrapper where that schema supports it. Neither the string literal nor source lexer spelling may replace the typed `(coefficient,scale)` value. Retain the nominal identity and fixed-scale contract obligation in the already existing named-type and contract metadata/proof pipeline, and add checked construction/guard only under the appropriate installed schema and owning package. No implicit float lowering, invented standalone fixed-decimal MLIR primitive, or unchecked nominal cast is permitted.

**§ 16(4) — Verifier fail-closed rule.** In every supported high-level schema and lowering path, verifiers must reject a nominal fixed-scale value whose coefficient/scale violates its associated resolved type contract, and reject contract checks with missing or mismatched provenance, width or reason. Constant folding must preserve observed scale; contract-free decimal carriers remain variable-scale. If an incomplete P21/P22 feature prevents correct verification or lowering, keep the feature explicitly unsupported in that path, with an actionable diagnostic and governance `partial` status. Do not report an untested schema capability as implemented.

## § 17. Normative acceptance matrix (source → type/scale → result)

**§ 17(1) — Reading the table.** `S` is the **resolved fixed type scale** (`variable` where none exists). `D=(c,s)` is the implicit default's **coefficient and scale**, not merely its display text. `A2012` means default `Information` unless suppressed/configured. A compile-time error prevents type creation; tests assert the registered error identity. Unless a `decimal128` carrier is shown, examples use signed 64-bit `decimal`. For forms with multiple declarations separated by `;`, parse them as separate source declarations in test fixtures, not a literal one-line Sec program.

| ID | Source or test setup | S | Required semantic result / diagnostic |
|---|---|---:|---|
| T01 | `type Negative decimal range -1.00..<0.000` | 3 | `D=(-1,3)`; 1000 legal coefficients `[-1000,-1]`; `A2012` |
| T02 | `type Pos decimal range 0.00..` | 2 | `D=(0,2)`; `A2012` |
| T03 | `type Neg decimal range ..<0.000` | 3 | `D=(-1,3)`; `A2012` |
| T04 | `type Wide decimal range 0.001..1000000000000000000` | 0 | `D=(1,0)`; `P=3`; capacity reduction; `A2012` |
| T05 | `type Wide128 decimal128 range 0.001..1000000000000000000` | 3 | `D=(1,3)`; capacity reduction **not** required; `A2012` |
| T06 | `type WideBad decimal scale 3 range 0.001..1000000000000000000` | — | `S1065` (out-of-capacity finite bound); no automatic lowering |
| T07 | `type Window decimal scale 2 range 0.001..<0.009` | — | `S1065`: nonempty mathematical interval but no grid member |
| T08 | `type Window decimal scale 2 range 0.001..<0.020` | 2 | `D=(1,2)`; sole member is `0.01` |
| T09 | `type Steps decimal scale -2 range ..<0` | -2 | `D=(-1,-2)` = -100 |
| T10 | `type Money decimal scale 2` | 2 | `D=(0,2)`; named type valid without `range` |
| T11 | `type Broad decimal range ..` | variable | No inferred fixed scale; no `A2012`; ordinary primitive-derived default |
| T12 | `type RangeZero decimal range 0.000..0.000` | 3 | `D=(0,3)`; `A2012` |
| T13 | `type Multi decimal range -100.00..100.00 range -10.000..10.000` | 3 | `D=(0,3)`; intersection `[-10,10]`; `A2012` |
| T14 | `type Base decimal scale 2; type Price Base range 1..10` | 2 | `D=(100,2)`; inherited fixed scale, no `A2012` on `Price` |
| T15 | `type Base decimal scale 2; type OK Base scale 2` | 2 | Valid identical inherited-scale assertion |
| T16 | `type Base decimal scale 2; type Bad Base scale 3` | — | `S1065`: conflicting inherited scale |
| T17 | `type Bad decimal scale 2 scale 2` | — | `S1065`: duplicate local scale contract |
| T18 | `type Root decimal; type Child Root range -1.00..<0.000` | 3 | `D=(-1,3)` on `Child`; `Root` remains variable |
| T19 | `type Base decimal range -1.00..<0.00; type Child Base range -1.000..<0.000` | 2 | `D=(-1,2)` on `Child`; no re-inference or additional `A2012` |
| T20 | `type Base decimal scale 2 range 0..10; type Bad Base range 11..12` | — | `S1065`: empty inherited/local intersection |
| T21 | `type NotDecimal int scale 2` | — | `S1064` |
| T22 | `type Bad decimal scale 2147483648` | — | `S1066` (out of signed int32) |
| T23 | `type Bad decimal scale 1.5` | — | `S1066` (wrong scale argument type) |
| T24 | `type Amount decimal scale 2 default 0.001` | — | `S1060` (inexact explicit default) |
| T25 | `type Amount decimal scale 2 range 0..10 default 11.00` | — | `S1059` (contract failure) |
| T26 | `let scale := 3` with a local binding | n/a | Valid ordinary identifier; no reserved-word diagnostic |
| T27 | `type Pos decimal scale (1 + 2) range 0..1` | 3 | `D=(0,3)`; legal CTE expression argument |
| T28 | `type Bad decimal scale RuntimeScale()` with runtime-only callee | — | Required-CTE/`S1066` diagnostic, no runtime scale contract |
| T29 | Literal `1.20m` versus `1.200m` | n/a | Numeric equality true; representation scales 2 versus 3 |
| T30 | Constant `1.20m + 2.3m` | n/a | Exact `(350,2)` |
| T31 | Constant `1.20m * 2.0m` | n/a | Exact `(2400,3)` |
| T32 | Constant `0.00m * 0.000m` | n/a | **Exact `(0,3)`, not `(0,5)`** |
| T33 | Constant `0.000m * 0.00m` | n/a | Exact `(0,3)`; operand-order independent |
| T34 | Constant `0.000m * 0.000m` | n/a | Exact `(0,3)`; no zero-product scale sum |
| T35 | Constant `0.00m * 1.234m` | n/a | Exact `(0,3)` |
| T36 | Constant `0.00m + 0.000m` | n/a | Exact `(0,3)`; normal addition rule |
| T37 | `type Amount decimal scale 2 range 0..1000; Amount(12.340m)` | 2 | Exact coefficient 1234; accepted, no rounding |
| T38 | Same type; runtime conversion from `12.345m` | 2 | `ConversionError.PrecisionLoss` |
| T39 | Same type; runtime conversion from `1000.01m` | 2 | `ConversionError.Contract(Range at local ordinal)` |
| T40 | Same type; runtime conversion from `1000000000000000000m` | 2 | `ConversionError.OutOfRange` before range contract |
| T41 | Same type; `Amount(0.01m) * Amount(0.01m)` | 2 | `ArithmeticError.PrecisionExhausted` (exact `0.0001` not on grid) |
| T42 | Same type; `Amount(999.99m) += Amount(0.02m)` | 2 | Contract failure; lhs unchanged 999.99 |
| T43 | Same type; `amount.Scale` | 2 | Getter returns `int32(2)` |
| T44 | Same type; `try amount.Scale = 3` | — | Compile-time read-only-member error |
| T45 | Same type; `amount.Rescale(3)` | n/a | `Result[decimal, PrecisionError]`; original `amount` unchanged |
| T46 | Same type; `amount.Normalize()` | n/a | `decimal`, not `Amount`; original unchanged |
| T47 | Same type; `amount.RoundTo(1)` | n/a | `Result[decimal, PrecisionError]`; any rounding remains explicit in method API |
| T48 | `type CTE decimal range (1.20m * 2.0m)..10m` | 3 | `D=(2400,3)`; preserved computed bound scale; `A2012` |
| T49 | Imported immutable decimal bound whose value is `(2400,3)` | 3 | Same inferred scale and default as T48; import does not erase scale |
| T50 | Repeated runtime and CTE multiplication of zero with `0.000m` | n/a | Scale remains 3 after every iteration (with initial zero scale ≤3) |
| T51 | `type Amount decimal scale -2147483648 range ..<0` | -2147483648 | Mathematical default `-1 × 10^2147483648` as compact `(−1,S)` if the lower bound is omitted; no huge power materialization |
| T52 | `type Amount decimal scale 2147483647 range ..<0` | 2147483647 | Mathematical default `−1 × 10^-2147483647` as compact `(−1,S)`; no huge power materialization |
| T53 | `let scale := 3` then `type Bound decimal range 0..scale` | 0 | `scale` is an upper-bound identifier (integer bound scale 0), not a `ScaleContract`; `D=(0,0)`; `A2012` |
| T54 | `type Steps decimal scale (0 - 3) range ..<0` | -3 | Valid contextual clause and computed negative signed scale; `D=(-1,-3)` = -1000 |
| T55 | `type Open decimal scale 2 range 0..` | 2 | Valid open upper bound and explicit scale; no `A2012` |

| T56 | `type Edge decimal scale 2 range 0.001..0.019` | 2 | Inclusive bounds between grid points; sole member `0.01`, `D=(1,2)` |
| T57 | `type Edge decimal scale 2 range 0.001..<0.010` | — | `S1065`; upper excludes the only candidate `0.01` |
| T58 | `type Edge decimal scale 2 range -0.019..-0.001` | 2 | Sole member `-0.01`, `D=(-1,2)` |
| T59 | `type Edge decimal scale 2 range -0.019..<-0.010` | — | `S1065`; upper excludes `-0.01`; lower excludes `-0.02` |
| T60 | `type Edge decimal scale 2 range 0.001..<0.021` | 2 | Legal members `0.01` and `0.02`; `D=(1,2)` |
| T61 | `type SignedMin decimal scale 0 range -9223372036854775808..0` | 0 | Valid signed i64 minimum coefficient; `D=(0,0)` |
| T62 | `type SignedMax decimal scale 0 range 0..9223372036854775807` | 0 | Valid signed i64 maximum coefficient; `D=(0,0)` |
| T63 | `type TooHigh decimal scale 0 range 0..9223372036854775810` | — | `S1065`; bound is a valid plain decimal using negative scale, but exceeds fixed-scale positive i64 capacity |
| T64 | `type NegativeCapacity decimal range -9223372036854775808..0.000` | 0 | `P=3`, inferred `S=0`, `D=(0,0)`; negative signed minimum is admitted; `A2012` |
| T65 | `type Huge decimal range 0.001..10000000000000000000` | -1 | `P=3`, inferred `S=-1`, `D=(1,-1)` = 10; upper exact 10^19, `A2012` |

**§ 17(2) — Required endpoint matrix.** In addition to T01–T65, test positive and negative endpoints at exact grid points and strictly between grid points, `..` versus `..<`, omitted lower/upper/both, a positive exclusive upper endpoint, an inferred scale stepwise reduced `3→2→1→0→−1`, and signed asymmetric coefficient boundaries (`−2^63` valid, `+2^63` not at `S=0`; corresponding signed 128-bit cases). Assert **exact** coefficient, scale, validity, and error identity; no test may only match a human-readable error substring.

**§ 17(3) — Pipeline and tooling matrix.** Exercise lexer/parser/recovery/CST/formatter, AST and Sema, both simple and executed CTE, immutable imports, type inference/inheritance and generic instantiation, compiler-known members, function/operator signatures, fixed-scale constructor and compound assignment, default synthesis for omitted fields, Semantic IR verification and round-trip, supported MLIR schemas and verifier (including historical v2 negative-scale rejection versus active v12 acceptance), runtime guards and failure mapping, CLI diagnostics, LSP completion/hover/quick fix, cold/incremental equivalence and cache invalidation. Round-trip tests must not rely on re-parsing a lost `Lexeme`.

**§ 17(4) — Resource bounds.** Test extreme signed-int32 `S` values and finite bounds with enormous exponent differences without allocating `10^2147483647`, proportional strings, or linear scans over the int32 scale domain. Implementations may use exact digit-count/exponent comparison, bounded multiprecision and established constant-time scale math. Resource exhaustion must be a deterministic diagnosed compiler capability failure under the owning CTE rules, never an invented semantic precision cap.

**§ 17(5) — Interpretation and evidence.** These are **acceptance requirements**, not claims of passing tests or implemented capabilities. Record each individual test's file/path, stage, execution evidence and remaining failures in the appropriate governance integration. A conformance test that is blocked by a separately unfinished compiler package remains blocked/partial; it cannot be silently omitted or marked passed.

## § 18. Normative integration, governance and `applied` housekeeping gate

**§ 18(1) — Rulebook ownership map.** Integrate all normative changes into the canonical owning books **without changing existing paragraph IDs**:

| Owner | Required normative action |
|---|---|
| `rules/foundations/grammar.md` § 17 | Add contextual `ScaleContract`; expression boundary, positioning, multiplicity and recovery |
| `rules/foundations/lexical_structure.md` § 7.3 | Establish `scale` as an ordinary identifier contextually interpreted **only** at contract head; exception to reserved contract words |
| `rules/types/types.md` §§ 12.1, 22.1, 25, 26–28 | Fixed-scale nominal invariant, inheritance, exact conversion and arithmetic result type, reconciled MD-131 untried form |
| `rules/types/contracts.md` §§ 4–6, 11–14 | Scale applicability, capacity, preserved exact boundaries, validation/source-order, inheritance/conjunction and runtime error ordering |
| `rules/types/default_values.md` §§ 4, 6, 10–14, Appendix A.5 | Explicit/inherited/inferred scale order, grid construction, nearest zero, explicit defaults, no-grid rejection |
| `rules/foundations/operators.md` § 27.1(5), §§ 27–27.2, § 98–99 | Exact zero-product `max` exception, fixed-scale arithmetic exactness/failure/result identity |
| `rules/library/core-library.md` § 10 | Named fixed-scale `.Scale` read-only, carrier methods and their unchanged public error contracts |
| `rules/compiler/compiler_known_members.md` §§ 10, 14 | Suppress named `.Scale` setter; enforce carrier-returning inherited methods and safe member substitution |
| `rules/compiler/semantic_ir.md` §§ 11–12, 58–59 | Immutable scale-bearing decimal constants, resolved contract/plan facts, no AST/lexeme re-inference |
| `rules/tooling/formatter.md` § 29 | Preserve all numeric literal spellings/trailing zeros; idempotent formatting |
| `rules/tooling/diagnostics.md` §§ 5–7 and diagnostic registry | `A2012` definition/severity/arguments; reuse error identities; LSP consistency |
| `rules/mlir/sec_mlir_dialect.md`, historical v2/v3 snapshots, and relevant active schema/plan books | Clarify signed-scale precedence; preserve historical schema compatibility; fail closed on unsupported nominal checked lowering |

**§ 18(2) — Governance coverage (not completion claim).** Add or update **all** action items in their relevant existing integration records rather than dumping everything into `frontend.default-values`. At minimum: `governance/types.yaml` (`frontend.default-values`, `frontend.type-contracts-v2`, `frontend.types-core` and their actual maintained IDs), diagnostics governance, core/compiler-known member integration, semantic IR/MLIR decimal/contract package integration, formatter/LSP, and CTE where owned. Link each to the exact normative paragraph(s), test vector IDs and current true implementation status. Preserve existing IDs; if an integration is split or an owner is missing, use the governance index and fragment synchronization policy instead of inventing a duplicate record. The `missing-decisions.yaml` MD-049 item can record the *normative decision as resolved* separately from implementation tasks.

**§ 18(3) — Required test documentation.** All **specified** tests in §§ 11 and 17, including both expected positive and negative results and diagnostic identities, must be represented in appropriate governance test/verification plans or referenced dedicated test specifications before the correction is marked `applied`. Tests do **not** have to have been run or passed merely for that housekeeping move, but their status must be honestly recorded (planned, partial, blocked, passed) and their owning action points and execution gate assigned.

**§ 18(4) — Exact `applied` gate (owner housekeeping decision 2026-11-21).** Move this correction to `rules/corrections/applied/` **only when all three conditions are satisfied**:

1. **Every normative change is integrated into its relevant canonical rulebook(s)**, including the zero-product arithmetic amendment, contextual syntax and historical schema precedence; all existing paragraph IDs remain stable.
2. **Every action item is represented in the relevant governance file(s)** with cross-reference to its rulebook and owning integration, without falsely claiming it was implemented.
3. **Every specified test is documented** with its expected results, owning verification task and honest status, even when implementation/test execution is still pending.

Passing the tests and completing the implementation are **not prerequisites** for moving a fully integrated correction to `applied`; they have their own implementation-governance completion gates. Conversely, a correction with merely approved decisions or an unmerged standalone MD does **not** qualify for `applied`. Never mark unexecuted verification `passed`.

**§ 18(5) — Handoff requirement.** On integration, update `missing-decisions.yaml` MD-049 as normatively resolved and link the correction's applied path; update the relevant governance action/task records without erasing prior historical progress. Do not rename or renumber existing rulebook paragraphs or registry IDs. Recheck current main HEAD before applying, refresh affected files if it changed since this baseline, and run syntax/YAML/diagnostic-registry validation when committing changes.

## § 19. MD-049 original-decision traceability and completion verdict

**§ 19(1).** Original `missing-decisions.yaml` MD-049 asks the policy for signed scale, fixed or unbounded search, exclusive upper zero, safe bounded computation and reconciliation of v2/v3 decimal constants. The answers are §§ 2–5, § 12.8, §§ 13–16 and test vectors T01, T03–T07, T29–T36, T48–T65. They expressly supersede the old MD description's interim sentence forbidding endpoint-scale inference, because the owner has now chosen bound-representation-driven fallback with coefficient-capacity reduction.

**§ 19(2).** The owner's five-point completeness audit maps as follows: point 1, § 4.5 and §§ 12, 15–17; point 2, §§ 10–11; point 3, § 12; point 4, § 13 and § 12.8; point 5, §§ 14–18. The special zero-product rule is normative only through integration into `operators.md`, not because this document merely proposes a code change. Housekeeping criteria are § 18.4.

**§ 19(3).** **Normative decisions complete for handoff; repository integration not yet performed.** This revision is a source document for the implementer to merge into the canonical rulebooks and governance. It is not itself an applied correction, compiler patch, passing conformance suite or evidence of status closure. If an implementation detail conflicts with an already locked rule, surface the exact normative conflict with path and paragraph; do not invent replacement source semantics or silently change unrelated language design.
