# MD-140 — Canonical Arena epoch exhaustion

| Document metadata | Value |
|---|---|
| Document ID | MD-140 |
| Document revision | 1.0 |
| Date | 2026-11-24 |
| Status | Owner-approved design; normative integration pending; implementation not verified |
| Language version | Sec 0.1; applicable to Sec 0.1.1 |
| Classification | Constraining — Arena epoch exhaustion strategy |
| Repository baseline | `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1` |
| Primary normative owner | `rules/memory/arena.md` |
| Related owners | Reference model, panic, runtime checks, effects, target profiles, Semantic IR and Sec MLIR P19 |
| Governance owner | `lowering.sec-mlir-package19` |

## § 1. Scope and approved decision

**§ 1(1)** MD-140 resolves the exhaustion strategy, the check obligation for retained Arena epochs, and the panic identity used on exhaustion. The following decisions were approved by the language owner on 2026-11-24:

1. Panic is the sole canonical exhaustion strategy for `Arena.Reset()` in Sec 0.1, across Hosted, RTOS and BareMetal.
2. Retained numeric epochs use checked advancement independently of width. Exhaustion is detected before the Reset transition commits. A compiler may eliminate an exhaustion check when it proves the check unnecessary, and may eliminate the epoch mechanism itself when the existing reference-model proof rules permit that elimination.
3. Exhaustion uses the distinct registered reason `panic.arena-epoch-exhaustion` and the canonical message `Arena validity epoch exhausted.`

**§ 1(2)** These decisions preserve the existing Arena API, reference validity contract, allocation model and profile panic policies. They introduce no source syntax, public error type, fallible Reset overload, new profile family or alternative reference-safety model.

**§ 1(3)** The existing 64-bit default deliberately makes exhaustion practically remote. That practical observation is not a proof of unreachability. It neither changes the arithmetic contract nor permits an unchecked retained epoch.

**§ 1(4)** This correction concerns Arena epoch exhaustion only. The other exhaustion strategies allowed for allocation identities, collection domains, reusable slots and other generation mechanisms by their owning rulebooks remain unchanged.

## § 2. Reset contract and profile resolution

**§ 2(1)** The source API remains exactly the existing Reset operation in `arena.md` § 35:

```sec
fn Reset() void
```

**§ 2(2)** Successful Reset retains the Arena owner, backing storage and ArenaDomain, ends the previous validity epoch, resets allocation cursors and establishes a distinguishable next validity epoch. All existing Reset preconditions and capacity, ownership and backing-storage behavior remain applicable (`arena.md` §§ 35–42).

**§ 2(3)** When the retained epoch cannot be advanced without exhaustion, the operation panics. It does not return normally, produce a `Result`, return an exhaustion error or silently leave the Arena unchanged and continue the source-level success path.

**§ 2(4)** No profile may select rekeying, domain recreation, retirement followed by continued Reset, or fallible Reset as an alternative exhaustion recovery strategy for this API. The general invalidation-domain model may still support those strategies for independently specified APIs. In particular, creating a different Arena through ordinary legal construction is not prohibited.

**§ 2(5)** The source-visible behavior is identical for every current profile family:

| Resolved family | Arena Reset exhaustion | Physical failure realization |
|---|---|---|
| Hosted | Panic | Selected verified panic policy |
| RTOS | Panic | Selected verified panic policy |
| BareMetal | Panic | Selected verified panic policy |

A derived or custom profile inherits this rule. It may choose only a conforming panic realization, not different Reset semantics.

**§ 2(6)** The required panic strategy is the existing strategy of `target_profiles.md` § 13 and `panic.md`. A target trap is a possible physical realization of the panic under that policy, not a separate language-level exhaustion outcome. No universal unwinder, heap allocation, panic containment mechanism or new runtime dependency is required by this correction.

**§ 2(7)** If the selected execution context forbids a reachable panic, the owning analysis must prove that the exhaustion path cannot be taken or that the epoch mechanism is legitimately eliminated. Otherwise the operation fails the existing context/effect requirements. A profile may not bypass that proof by selecting an unsafe unchecked Reset.

## § 3. Logical width and checked advancement

**§ 3(1)** The default logical epoch width is 64 bits on every target, independent of pointer width (`reference_model.md` § 33). Shorter widths remain selectable only at compilation time under § 34, with a verified stale-reference safety strategy. This correction does not add supported widths, change representation-policy syntax, or make width a public source type property.

**§ 3(2)** Let `W` be the resolved logical numeric epoch width, `E` the current epoch, and `MaxEpoch` the largest valid numeric epoch under the canonical representation. Advancement must use checked arithmetic. For a representation in which all unsigned values are available, `MaxEpoch = 2^W - 1`. Existing representation constraints, including any reserved values, remain authoritative; this correction does not introduce a reserved epoch or prescribe an initial epoch value.

**§ 3(3)** Conceptually, for the ordinary retained numeric epoch mechanism:

```text
validate Reset preconditions
if E cannot advance within the valid epoch space:
    panic with panic.arena-epoch-exhaustion
prepare the distinguishable next epoch
commit Reset transition
```

The check and arithmetic must operate on the resolved logical width, including on a narrower machine where a 64-bit epoch uses multiple machine words. The compiler must not substitute pointer-width arithmetic.

**§ 3(4)** Reaching the maximum valid epoch is not itself an exhaustion panic. The next attempted advancement beyond the available epoch space triggers the panic. Ordinary Arena allocations do not advance the epoch (`reference_model.md` § 30(3)); they must not fail merely because the current epoch is maximal.

**§ 3(5)** No wrapped value may become observable, even transiently as the published live epoch. An implementation may use checked addition, an equivalent bound comparison, or another verified mechanism with the same failure and ordering behavior. A nonnumeric physical representation must preserve the corresponding checked-advancement, exhaustion and non-revival guarantees where the logical mechanism remains required.

**§ 3(6)** Short width is not a reason to select a different exhaustion behavior. Its approved proof or exhaustion strategy must be consistent with this correction. A legacy profile or plan that selects Arena-specific rekeying or a fallible Reset on exhaustion must be reconciled before it can be accepted for this API.

## § 4. Reset commit and ownership

**§ 4(1)** The canonical Reset transition remains the one in `arena.md` § 42. Exhaustion detection and preparation of a valid next epoch precede every externally relevant part of the Reset commit.

**§ 4(2)** On the exhaustion path, no Reset-induced mutation has committed. In particular, Reset has not ended the old epoch, invalidated its allocations, advanced the epoch or state version, reset any cursor, released backing, replaced a domain, or made old storage available under a new epoch.

**§ 4(3)** This pre-commit rule concerns the failed Reset operation. The subsequent panic policy may perform ordinary scope cleanup, destroy the Arena, end its domain or terminate the execution. Such actions follow the panic/destruction rules and must not be mistaken for successful Reset or be suppressed by the pre-commit requirement.

**§ 4(4)** On the success path, the new epoch and all Reset state changes form one coherent semantic transition. Fixed, borrowed and growable Arenas retain the backing and capacity behavior already specified by `arena.md` §§ 40–41. No partial cursor reset or mixed old/new state becomes observable.

**§ 4(5)** The existing exclusive-access, final-use, dependency and stale-capable handle rules remain mandatory. Exhaustion checks do not replace borrow/dependency proofs. A validity-preserving dependency that crosses Reset is still a source-semantic error; the compiler must not legalize it by introducing a runtime panic check.

**§ 4(6)** Weak or explicitly stale-capable handles that may legally survive Reset retain their original expected domain/epoch. After a successful Reset they cannot resolve to an allocation of the new epoch merely because storage addresses were reused. Their own fallible resolution contract remains unchanged.

## § 5. Check elimination and epoch metadata elimination

**§ 5(1)** The default implementation contract for a retained epoch is checked advancement. This includes 64-bit epochs. Runtime exhaustion checking may be omitted only when sound, applicable proof establishes that the retained advancement cannot exhaust.

**§ 5(2)** An adequate proof may establish, for example, a bound on the number of advances from a known initial or current epoch over all relevant paths, including loops, calls and repeated uses of the same ArenaDomain. It must account for the resolved logical width and representation constraints.

**§ 5(3)** Neither an assumed human-scale program duration, a benchmark, a likely reset rate, the default epoch width, nor a target word width is adequate proof. Unknown advance counts do not establish a bound. Proof must remain valid under the selected CompilationPlan, actual source paths and applicable analysis assumptions.

**§ 5(4)** Independently, `arena.md` §§ 38(7), 105 and `reference_model.md` §§ 33–36 already allow runtime epoch metadata to be eliminated when sound validity analysis proves that no runtime stale-reference or handle mechanism requires it. If the epoch mechanism is validly eliminated, its corresponding advancement and exhaustion check are unnecessary. This is the owner-approved existing proof-elimination rule, not an authorization to remove a required epoch merely to avoid its possible exhaustion.

**§ 5(5)** Eliminating a local reference's dynamic validation alone does not establish that the entire ArenaDomain can omit epoch metadata. Other surviving dependencies, escaped stale-capable handles, foreign retention, or unresolved aliasing/provenance must be accounted for. Absence of proof is not proof of absence.

**§ 5(6)** Proof-eliminated runtime metadata does not eliminate the logical invalidation and ordered Reset effects. Subsequent analyses must still respect allocation lifetime, borrow termination, domain identity, state-version and capacity transitions.

**§ 5(7)** A retained epoch may therefore have either a dynamic checked-advance path or a proven-safe advance without the dynamic check. A proof-eliminated epoch mechanism has no physical exhaustion branch. These are analysis outcomes under the single panic policy, not three user-selectable exhaustion strategies.

## § 6. Panic identity, effects and diagnostics

**§ 6(1)** Register the following target-independent panic reason in the canonical panic/diagnostic registry:

```text
panic.arena-epoch-exhaustion
```

Canonical message:

```text
Arena validity epoch exhausted.
```

**§ 6(2)** This reason is distinct from `panic.invalid-reference-generation`, ordinary arithmetic overflow, allocation failure, and explicit programmer panic. The failed operation is advancement of an exhausted Arena validity epoch, not dereference of a stale reference and not failure to obtain backing memory. Reusing an existing numeric ID with a new meaning is prohibited.

**§ 6(3)** Allocate a new stable numeric `PanicID` through the existing registry process (`panic.md` § 13). Do not invent a numeric value in the Arena rulebook or assign IDs independently in Sema, Semantic IR, the dialect or a target runtime. `PanicID` remains `uint32`; the five-field `PanicInfo` surface remains unchanged.

**§ 6(4)** Preserve source location and callable provenance for the Reset operation through lowering and panic observation. The minimum panic path is allocation-free under `panic.md` § 14. A profile's additional reporting capabilities remain optional and subject to its active constraints.

**§ 6(5)** Where exhaustion has not been proven impossible or the mechanism eliminated, effect analysis must preserve the corresponding potential panic with its reason/provenance. Existing `@noPanic`, context, ISR and other effect contracts apply. A compile-time capability or effect failure is distinct from the runtime panic itself.

**§ 6(6)** Proven elimination of this exhaustion path removes this particular panic cause; it does not establish that an entire callable is panic-free when independent operations may panic. The effect system must not infer safety solely from the 64-bit default.

**§ 6(7)** `try` does not catch Arena epoch exhaustion. `Reset()` remains a void-returning operation, not a language-defined fallible Reset expression. Where the ordinary `try` rules allow a larger enclosing expression for another reason, those rules do not convert this panic into a recoverable error. No new `try arena.Reset()` recovery form is introduced.

**§ 6(8)** Tooling should explain an unproven required safety guarantee using the Arena Reset source location, the resolved width and the active effect/profile restriction when available. It must not present exhaustion as an already observed runtime event merely because analysis cannot prove its absence.

## § 7. Canonical plan and Sema facts

**§ 7(1)** The immutable CompilationPlan supplies the resolved reference policy, logical epoch width, verified representation constraints and applicable panic policy. The Arena rulebook supplies the single exhaustion outcome. No compiler stage independently selects a different outcome.

**§ 7(2)** Extend or reconcile the existing reference/Arena plan model rather than introducing a second independent epoch-width source. `internal/layout/reference_policy.go` already defines the default logical width and the requirement for a proof/exhaustion strategy on shorter widths. Its current representation is an implementation baseline, not a complete proof that Arena Reset lowering conforms to MD-140.

**§ 7(3)** The resolved Arena Reset plan must retain, as applicable:

```text
ArenaDomain identity
writable Arena Place and ownership/dependency proof
resolved logical epoch width and representation policy
current epoch or required dynamic epoch source
checked advancement / proof-eliminated check / proof-eliminated metadata
proof provenance for any elimination
canonical exhaustion panic identity and applicable panic policy
state version before and after successful Reset
capacity/cursor transition and backing retention
ordered invalidation effect and source provenance
```

These are semantic requirements, not prescribed Go field names, wire encodings or public API declarations.

**§ 7(4)** A proof referring to a different ArenaDomain, width, source snapshot or relevant plan configuration is not reusable without canonical compatibility validation. Incomplete required layout, reference-policy or panic facts must be reported through the existing unproven/capability boundary and must not produce unchecked executable Reset code.

**§ 7(5)** Sema and the existing owning analyses establish source legality and proof facts. Semantic IR construction and lowering consume those facts; they must not invent a proof, repeat liveness analysis as a substitute for missing facts, or assume that absent metadata means proven elimination.

## § 8. Semantic IR and Sec MLIR P19

**§ 8(1)** Preserve the existing P19 Arena Reset plan (§ 74), `ArenaResetOp` (§ 94) and `sec.arena.reset` (§ 115) as the canonical high-level Reset representations. This correction fixes their exhaustion semantics; it does not prescribe a new opcode or a new dialect schema number.

**§ 8(2)** The high-level operation has no ordinary source result. Its success path retains the required `AdvanceEpoch` logical effect, state-version transition and cursor reset. Its dynamic exhaustion path terminates normal Reset continuation through the registered panic reason, without committing the Reset transition.

**§ 8(3)** For the retained checked mechanism, an illustrative lowering is:

```text
entry:
    read current epoch
    if exhausted:
        branch to exhaustion
    else:
        branch to advance

exhaustion:
    panic(panic.arena-epoch-exhaustion, source provenance)
    no normal successor

advance:
    compute representable next epoch
    commit coherent Reset transition
    continue normally
```

This is conceptual control flow, not a new Sec syntax or a required dialect operation spelling. An equivalent checked arithmetic implementation is valid if failure ordering and commit semantics are preserved.

**§ 8(4)** A lower-level optimizer must not move cursor mutation, domain invalidation or publication ahead of an unresolved exhaustion check. It must not remove a dynamic panic branch on the grounds that 64-bit exhaustion is unlikely. It must preserve canonical panic policy, identity and source provenance through any terminal failure lowering.

**§ 8(5)** IR verification must reject an unchecked retained advancement without valid proof, a required check omitted without proof, elimination without adequate provenance, an incorrect panic reason, a fallible Reset result, an unauthorized rekey strategy, or a normal success continuation from the exhaustion endpoint.

**§ 8(6)** A backend may use an existing terminal panic/failure mechanism if it represents this distinct registered reason and the selected policy. If the active Semantic IR, dialect, runtime or target lowering does not yet support the required behavior, fail closed with the existing implementation-capability diagnostic. Do not treat this design correction as proof that P19 or its dependencies are implemented.

**§ 8(7)** P19-07 and P19-20 may now consume the approved single exhaustion policy. Existing package dependencies, acceptance gates, serialization/verifier rules and required conformance tests remain in effect. No package or schema support is declared implemented by this document.

## § 9. Required conformance tests

**§ 9(1)** The following vectors are normative requirements for integration. Test identifiers are local to this correction. Test scaffolding may use verified synthetic epoch state, a controlled short-width test profile, or IR/runtime fixtures to reach exhaustion without executing enormous numbers of Reset operations. Such instrumentation does not introduce public mutable epoch access or a source-level epoch-width selector.

| ID | Case | Required outcome |
|---|---|---|
| T01 | Hosted, retained epoch at maximum, Reset | Canonical exhaustion panic; no Reset commit. |
| T02 | RTOS, same case | Same reason and pre-commit semantics; resolved panic policy. |
| T03 | BareMetal, same case | Same logical panic; conforming target realization. |
| T04 | Derived/custom profile attempts recoverable Reset exhaustion | Profile/API strategy rejected; no alternative success behavior. |
| T05 | Profile attempts rekey on Reset exhaustion | Not a conforming Reset strategy. |
| T06 | Default plan on 64-bit target | Logical epoch width 64. |
| T07 | Default plan on 32-bit target | Logical epoch width 64, independent of pointer width. |
| T08 | Approved shorter-width plan with retained epoch | Same checked exhaustion rule and reason. |
| T09 | Shorter width without required safety justification | Plan validation rejects unsafe selection. |
| T10 | Retained epoch immediately below maximum | Successful Reset reaches maximum without panic. |
| T11 | Next Reset at maximum | Panic before any Reset-induced mutation. |
| T12 | Ordinary allocation at maximum epoch | No exhaustion panic solely due to epoch value. |
| T13 | 64-bit advance on narrow target crosses low-word boundary | Correct full-width advancement, no truncation. |
| T14 | 64-bit maximum on narrow target | No wrapped publication; canonical exhaustion panic. |
| T15 | Fixed Arena successful Reset | Owner/domain/backing retained; cursor reset and epoch advance. |
| T16 | Borrowed Arena successful Reset | Existing backing borrow retained; correct epoch/cursor transition. |
| T17 | Growable Arena successful Reset | All acquired segments retained; coherent cursor reset. |
| T18 | Fixed Arena exhaustion | Old epoch and cursor state unchanged by Reset before panic cleanup. |
| T19 | Borrowed Arena exhaustion | No Reset-induced release or backing-borrow end before panic cleanup. |
| T20 | Growable Arena exhaustion | No segment unlink, partial cursor reset or premature publication. |
| T21 | Live strong reference crosses Reset | Existing source-semantic dependency error. |
| T22 | Live nested Arena, slice, or deferred dependency crosses Reset | Existing dependency error, not exhaustion recovery. |
| T23 | Ordinary reference has proven final use before Reset | Dependency does not block otherwise legal Reset. |
| T24 | Surviving stale-capable handle after successful Reset | Original expected epoch cannot resolve to new allocation. |
| T25 | Surviving stale-capable handle at attempted exhausted Reset | No revival under a wrapped/new published epoch. |
| T26 | Retained default-width epoch, unbounded Reset count | Dynamic exhaustion check required. |
| T27 | Sound bound below exhaustion for all relevant paths | Check may be eliminated, logical transition preserved. |
| T28 | Bound depends on unresolved loop/call behavior | Not sufficient to eliminate check. |
| T29 | Proof valid for old domain or incompatible plan | Cannot justify check elimination. |
| T30 | All required validity proofs permit epoch metadata omission | No physical epoch advance or exhaustion check required; logical Reset effects preserved. |
| T31 | Local reference proof but another live stale-capable mechanism needs epoch | Required epoch metadata retained. |
| T32 | Unknown metadata requirement | Not treated as proof of elimination. |
| T33 | `@noPanic` callable with unproven retained exhaustion path | Existing no-panic proof fails. |
| T34 | `@noPanic` callable with proven nonexhaustion and no other panic | Exhaustion itself does not violate contract. |
| T35 | Proven exhaustion safety but unrelated reachable panic | Other panic effect remains. |
| T36 | Context forbids panic; retained exhaustion cannot be excluded | Existing context/effect diagnostic. |
| T37 | Attempt to catch Reset exhaustion with `try` | No recoverable exhaustion channel. |
| T38 | Stale direct reference failure in independent operation | Existing `panic.invalid-reference-generation` identity unchanged. |
| T39 | Arena exhaustion observed through supported panic policy | Registered Arena-specific `PanicID`; canonical Reset provenance. |
| T40 | Arena exhaustion reporting on minimum runtime-free path | No allocation required for minimum panic behavior. |
| T41 | Cleanup-capable panic profile destroys Arena after failed Reset | Ordinary cleanup; not a successful Reset transition. |
| T42 | Trap-based profile | Defined panic realization; no invented unwinding guarantee. |
| T43 | IR fixture lacks proof for omitted retained check | Verification fails closed. |
| T44 | IR fixture carries wrong reason or normal exhaustion continuation | Verification fails. |
| T45 | Lowering attempts to move cursor reset before exhaustion check | Invalid lowering; test must detect the change. |
| T46 | Active dialect/runtime cannot lower the required panic endpoint | Focused implementation limitation; no executable weakened Reset. |
| T47 | Reset metadata elimination with later storage allocation | Logical lifetime/invalidation and ordered effects preserved. |
| T48 | Existing non-Arena slot/allocation exhaustion policy | Unchanged by MD-140. |

**§ 9(2)** T01–T03 require equivalent semantic outcomes, not identical native instructions or identical panic containment. T11, T18–T20 and T25 must distinguish Reset-induced mutations from effects of subsequent panic cleanup. T27 and T30 must exercise distinct proof-elimination cases.

**§ 9(3)** Test success in synthetic short-width or boundary-state fixtures does not establish all target/profile capabilities. Executable conformance must cover the actual supported lowering paths and their declared policies. Track frontend, analysis, Semantic IR, dialect and executable coverage independently.

## § 10. Normative integration and governance

**§ 10(1)** Integrate the following changes without renumbering existing stable sections. Bump affected document revisions and update their revision histories according to the repository's established governance conventions.

| Rulebook / artifact | Required reconciliation |
|---|---|
| `rules/memory/arena.md` | Replace § 39(1)'s alternative strategies with the single panic policy; retain §§ 39(2–3). Clarify §§ 35, 38, 40, 42 and 105 for checked advancement, pre-commit exhaustion and existing proof elimination. |
| `rules/memory/reference_model.md` | Preserve §§ 33–38's general width and generation rules; make § 40 Arena-specific and refer to the canonical panic strategy in `arena.md`. Do not remove retirement/rekey/fallible strategies for unrelated domains. |
| `rules/platform/target_profiles.md` | Clarify § 13 and relevant reference policy resolution: profile selects the physical panic realization, not a different Arena Reset exhaustion outcome. |
| `rules/errors/panic.md` | Add the distinct Arena epoch exhaustion reason category to § 13; keep the existing registry ownership and metadata shape. |
| `rules/errors/runtime_checks.md` and owning effect-analysis rules | Cross-reference checked advancement, proof elimination, panic effects and the absence of a `try` recovery channel. |
| `rules/compiler/semantic_ir.md` | Reconcile Arena Reset requirements and failure/proof facts under the existing operation model. |
| `rules/mlir/packages/sec-mlir-dialect_package19.md` | Replace § 42's profile-selected alternatives; reconcile §§ 39, 41, 74, 94, 115 and 152 with §§ 3–9 of this correction. |
| Canonical panic registry and implementation consumers | Allocate one stable reason ID and synchronize effects, IR, diagnostics, runtime and lowering. |
| `missing-decisions.yaml` | Resolve MD-140 after normative integration; preserve decision provenance. |
| `governance/` and P19 implementation tracking | Update the owning fragment for `lowering.sec-mlir-package19` and related requirement owners, distinguishing design closure from verified implementation. |

**§ 10(2)** Retain source API declarations and stable public types. No new core `ArenaError` case, panic metadata field, `Reset` overload, epoch access property or source-level trap operation is authorized by this correction.

**§ 10(3)** Integrate one implementation stage at a time. Verify the canonical reason registration and reference-policy/plan facts; then frontend/effect analysis; then Semantic IR and verifier; then P19 dialect and native/runtime lowering with applicable dependencies. At each stage, execute the relevant regression and conformance tests before advancing. Existing unsupported-capability diagnostics remain until the owning implementation is actually available.

**§ 10(4)** Record implementation status based on verified evidence, not the existence of this correction. This document closes the design question. It does not assert that any source implementation, dialect schema, panic registry entry, runtime path or test has already been changed or executed.

**§ 10(5)** The numeric reason assignment, internal plan field names and exact physical lowering are delegated to existing implementation owners. The illustrative arithmetic representation and synthetic test method do not authorize additional public language features. The pre-commit cleanup distinction and proof-provenance requirements are derived clarifications of the accepted decisions and existing rules.
