# MD-117 — Heterogeneous Native Variadic Type Packs

- **Status:** Normative design decision; canonical rulebooks synchronized 2026-10-27; governance and implementation pending (kept open per § 14(7))
- **Decision ID:** MD-117
- **Area:** `stdlib-formatting-heterogeneous-argument-model` (language mechanism; library API separate)
- **Created:** 2026-10-26
- **Last updated:** 2026-10-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Document kind:** Cross-rulebook correction / implementation contract
- **Canonical target rulebooks:** `rules/declarations/functions.md`, `rules/declarations/generics.md`, `rules/declarations/spread.md`, `rules/control-flow/flowcontrol_for.md`, `rules/memory/references.md`, `rules/memory/borrowing.md`, `rules/memory/lifetime_analysis.md`, `rules/memory/copy_move.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/monomorphization.md`, `rules/compiler/generics_lowering.md`, `rules/library/stdlib.md`, `rules/library/core-library.md`, `rules/platform/ffi.md`
- **Governance primary owner:** `governance/stdlib.yaml` for the originating stdlib dependency; language implementation facts must be integrated into their respective existing governance areas, without duplicate ownership
- **Replaces:** The unresolved MD-117 design alternatives, **not** the existing numbered paragraphs of canonical rulebooks

> **Integration instruction:** This book records the approved MD-117 language decisions. It is not a replacement for the canonical rulebooks. Integrate its clauses into those books **without renumbering any existing cited paragraphs or sections**; append new numbered material or use the repository's established stable amendment method. Confirm the current `main` HEAD before integrating. The exact spelling/semantics of `*` dereferencing are deliberately excluded: that question must not be inferred from this book.

---

## § 1. Purpose, scope, and design motivation

**§ 1(1).** Sec 0.1 shall support **heterogeneous native variadic arguments** in ordinary statically typed functions and methods. A call may supply zero or more values of different concrete types without implicit `any`, type erasure, boxing, dynamic reflection, or C varargs.

**§ 1(2).** The mechanism shall be a **general language feature** equally available to user code, stdlib, and core. No formatter name, `fmt.Printf` declaration, or other stdlib API receives privileged call semantics.

**§ 1(3).** The principal user-facing goal is to make the natural call syntactically simple while retaining all the static type, ownership, lifetime, and control-flow guarantees that the compiler can establish itself. Do not force redundant compile-time or ownership boilerplate on callers.

**§ 1(4).** Existing **homogeneous** native `name: ...T` semantics, where `T` is one concrete type, remain valid. The new type-pack facility supplements them; it does not reinterpret every ordinary `...T` as heterogeneous.

**§ 1(5).** This book specifies the general language mechanism needed by `stdlib/fmt`. The **public `fmt` API, exact formatting grammar, format-validation/error contracts, output destinations and materialization behavior** remain a separate stdlib specification task. These are **not** a reason to add `fmt`-specific language behavior, and they must have a separately tracked owner before the original broad MD-117 entry is fully removed.

## § 2. Generic type-pack declaration

**§ 2(1).** A standard generic type parameter uses `[T]`. A heterogeneous type-pack parameter uses `[...Types]`:

```sec
fn Trace[...Types](values: ...Types) void {
    // Each argument retains its concrete type.
}
```

**§ 2(2).** `Types` is an ordinary programmer-chosen identifier, **not** a keyword or a builtin type. Prefix `...` within the generic parameter declaration changes its kind from *one type* to *an ordered sequence of zero or more types*.

**§ 2(3).** For an invocation with arguments of types `string`, `int`, and `bool`, the inferred pack identity is the ordered sequence `(string, int, bool)`. Order, nominal type identity, reference kind, and other relevant semantic type distinctions are preserved.

**§ 2(4).** An empty pack is valid. A type pack is a compile-time generic entity, not a dynamic type, tuple object, storable container, source-level array, or first-class runtime type descriptor.

**§ 2(5).** For the first Sec 0.1 implementation, a function/method generic declaration may have **at most one type pack, at the end of its generic parameter list**, and at most one corresponding native variadic parameter, at the end of its value-parameter list. This is the narrow extension of the existing one-trailing-variadic rule; no arbitrary pack expansions in fields, return-type packs, or nested generic argument lists are introduced here.

**§ 2(6).** Generic type packs in this version are inferred from ordinary concrete call arguments and statically known pack spreads. The book does not introduce an independent runtime generic-pack value or new explicit pack-literal syntax.

## § 3. Parameter modes: owned, shared, mutable

**§ 3(1).** The following forms are valid for a generic type pack:

```sec
fn ReceiveOwned[...Types](values: ...Types) void {
}

fn ReceiveShared[...Types](values: ...ref Types) void {
}

fn ReceiveMutable[...Types](values: ...ref mut Types) void {
}
```

**§ 3(2).** The forms are pointwise equivalents of ordinary Sec parameter modes:

- `...Types`: each expanded argument is by-value, copying or transferring ownership as its concrete type requires;
- `...ref Types`: each expanded argument provides a shared borrow;
- `...ref mut Types`: each expanded argument provides an exclusive mutable borrow.

**§ 3(3).** Call-site implicit borrowing, explicit `<-` for transfers from named move-only values, temporary-value lifetimes, and the ordinary rules for mutable exclusivity apply to **each expanded argument**. Type-pack syntax must not bypass them.

**§ 3(4).** A `...ref mut Types` pack may contain multiple mutable references only if they satisfy the usual non-overlap and lifetime proofs. The compiler must reject a call that would give two conflicting exclusive borrows, before entering the callee.

**§ 3(5).** A native variadic parameter's **structure** remains read-only. Mutability of an object reachable via an explicitly granted `ref mut` is a separate right. Mutating that referent does not imply permission to mutate, resize, or rearrange the pack itself.

**§ 3(6).** The previously prohibited `->values: ...T` (or `->values: ...Types`) syntax is **not** introduced. Ordinary by-value variadic parameters already own their received values. Whole-pack transfer is an explicitly marked **call-site** operation (§ 7), not another parameter mode.

## § 4. Invocation, arity, and inference

**§ 4(1).** Calls use ordinary Sec argument syntax:

```sec
Trace("Hello", 42, true)
```

**§ 4(2).** For a heterogeneous pack, the final expanded argument count and each expanded concrete type must be statically known at the call site after legal spread expansion. Runtime **values** may be arbitrary and evaluated at runtime.

**§ 4(3).** A homogeneous `...T` retains the existing permission to accept an eligible runtime-length spread. A runtime-length source cannot be used to infer a heterogeneous type pack whose number and position-specific types are not statically known. It must not be silently converted into `any` or a runtime type-erased argument sequence.

**§ 4(4).** Each argument is type-checked according to the receiving concrete parameter mode. Normal literal-context typing and explicit conversion rules continue to apply. A pack does not introduce implicit coercions among unrelated types.

**§ 4(5).** Ordinary overload resolution is preserved, including its existing preference for an otherwise equivalent fixed-arity candidate and its prohibition against selecting solely by return type. Pack matching does not add runtime overload dispatch. Ambiguous candidates remain a compile error.

**§ 4(6).** A generic function body must be valid for each demanded concrete specialization. Operations used on a pack element require an existing valid operation, method, overload, or explicitly satisfied generic constraint for that concrete element. There is **no implicit structural `FormatArg` interface** or `Printf` exemption.

## § 5. Pack value, indexing, and iteration

**§ 5(1).** A heterogeneous pack remains an ephemeral compiler-known parameter pack, not an ordinary collection. It has `values.Len`, ordered access, and iteration but does not promise contiguous storage or expose `values.Ptr`.

**§ 5(2).** `values.Len` is compile-time-known for a heterogeneous specialization, including the empty pack. When indexing a heterogeneous pack, an index used to select a position with a concrete static type must be compile-time-known. A runtime index must not return a fictitious universal type or silently erase type identity.

**§ 5(3).** `for` needs **no special `static` keyword**. For a heterogeneous pack, the compiler expands the iterations in type-aware fashion. For ordinary runtime iterables, `for` keeps ordinary runtime iteration semantics. Both must preserve the same observable language behavior.

**§ 5(4).** Existing loop binding modes apply **without change**:

```sec
for value in values {
    // By-value binding: legal only when this element is implicitly copyable.
}

for ref value in values {
    // Shared borrow of the current element or its referent by normal rules.
}

for ref mut value in values {
    // Exclusive borrow/reborrow only when mutable authority exists.
}
```

**§ 5(5).** A plain `for value` shall **not** silently borrow a move-only value and shall **not** move it out of the pack. If any demanded iteration requires an unavailable implicit copy, that specialization is invalid. Use `for ref value` for observation.

**§ 5(6).** In a pack whose element is already `ref T`, ordinary shared-reference/reborrow rules apply. In a pack of `ref mut T`, `for ref mut value` may create an exclusive **reborrow of the referent**, without moving the reference-holder out of the pack. All lifetime and exclusivity rules still apply. A nested `ref ref T` is not manufactured merely because a loop uses `ref` syntax.

**§ 5(7).** `break`, `continue`, `return`, `defer`, `try`, `panic`, source-order evaluation, borrow live ranges, and deterministic cleanup retain their normal **observable semantics**. Static pack expansion and backend loop unrolling are implementation techniques, not new source-language control flow. An iteration whose body is not reached must not execute its runtime side effects.

## § 6. Evaluation, call transfer commit, and destruction

**§ 6(1).** Arguments and spread sources are evaluated **exactly once in their existing left-to-right semantic order**. The resulting expanded argument sequence preserves that order.

**§ 6(2).** The ordinary **call-transfer-commit** rule remains in force: ownership transfer to callee-owned parameters commits only after successful preparation of all arguments. If preparation fails, no partial ownership transfer into the callee or stale moved source state is permitted; completed temporaries are cleaned up according to normal Sec rules.

**§ 6(3).** After call entry, each owned element has one owner and one destruction responsibility. On ordinary exit and error paths, all remaining owned elements are destroyed exactly once according to the existing deterministic destruction order. Borrowed referents are **not** destroyed by the borrowing pack.

**§ 6(4).** Static pack expansion must not duplicate argument evaluation, implicit moves, allocators, externally visible operations, or destruction. The native pack itself shall not require hidden heap allocation merely to represent the variadic argument list.

## § 7. Non-consuming spread and whole-pack consuming spread

**§ 7(1).** The existing postfix `values...` spread is **non-consuming**. It may copy copyable values or pass/reborrow compatible references according to the destination, but it may not silently extract move-only elements or violate exclusive borrowing.

**§ 7(2).** The new explicitly consuming call-site form is:

```sec
fn Forward[...Types](values: ...Types) void {
    TakeAll(<-values...)
}
```

**§ 7(3).** `<-values...` is legal **only for a complete, currently owned native variadic parameter pack**. The target must accept the entire expanded sequence with valid concrete type and ownership modes. The transfer is one logical transaction, and it invalidates the source pack after successful call-transfer commit.

**§ 7(4).** This is **not** permission for individual moves such as `<-values[0]` or partial pack consumption. It does **not** permit consuming spread of arrays, slices, lists, tuples, or structs, nor a general consuming spread operator in other contexts.

**§ 7(5).** **Motivation for the intentional asymmetry:** transferring a complete pack preserves one owner for every element without introducing a partially moved pack state. Moving one selected element would require per-position availability, iteration, and cleanup state that Sec 0.1's ephemeral pack contract does not define. This is a narrow, reasoned exception to non-consuming spread rather than a general weakening of ownership.

**§ 7(6).** Shared-reference packs may be forwarded by ordinary legal non-consuming spread (`values...`) because shared references can be propagated without transferring ownership of their referents. Mutable-reference forwarding may reborrow exclusively only when all borrow and overlap proofs succeed; it shall never implicitly copy a `ref mut`.

**§ 7(7).** The consuming operation is supported for **both homogeneous and heterogeneous owned native parameter packs**, with no different ownership semantics between them.

## § 8. Pack lifetime and returned-reference provenance

**§ 8(1).** The ephemeral pack itself cannot escape the invocation, be stored as a first-class persistent value, or be captured by an escaping closure. A reference into storage owned by the ephemeral pack cannot escape if that storage ends with the invocation.

**§ 8(2).** A reference **carried by** a pack is distinct from a reference **to the pack's own storage**. A shared or mutable reference ultimately originating in caller-owned storage may be returned or forwarded when the compiler proves that the referent's lifetime, storage validity, and borrow authority outlive the returned reference.

**§ 8(3).** Such proofs use the **ordinary returned-reference provenance model**, including finite sets of possible caller-side origins and per-position projections. No new source lifetime annotation is introduced by heterogeneity.

**§ 8(4).** A return of `ref mut T` must preserve the exclusive grant and reject any overlapping simultaneously usable mutable authority. A mutable reborrow used for a returned reference must be related to the original caller-side borrow, not treated as a new owner.

**§ 8(5).** A heterogeneously typed pack does not synthesize a common return type. A returned reference has an exact Sec return type; a runtime selection among different element types cannot silently become `any` or an invented universal reference.

**§ 8(6).** **Dereference-operator syntax explicitly deferred:** MD-117 establishes *what may be returned*, not whether a given expression must spell that access `value`, `*value`, or another form. Resolve the previously discussed `*` question in the relevant references/expressions rulebook **separately**, without silently changing this provenance decision.

## § 9. Specialization, Semantic IR, and lowering

**§ 9(1).** A heterogeneous pack's ordered concrete type sequence is part of the canonical generic specialization identity. `(string, int)` and `(int, string)` produce distinct concrete semantic specializations; empty and nonempty packs are distinct. Nominal identities and parameter ownership modes are preserved.

**§ 9(2).** No runtime generic type dictionary, universal erased pack ABI, or implicit boxing is introduced. Equivalent concrete specializations may share machine code only where existing monomorphization rules prove the observable semantics identical.

**§ 9(3).** Semantic IR shall preserve, at minimum:

- declaration of a type-pack generic parameter, its ordered resolved type sequence, and resolved pack length;
- each expanded value's concrete type, source position, by-value/shared/exclusive mode, and origin;
- source-order evaluation, spread provenance, and call-transfer-commit boundary;
- whole-pack ownership transfer and source invalidation versus non-consuming spread;
- non-escaping pack lifetime, referent lifetime/provenance, and return-origin sets;
- statically expanded loop iterations and their source-level control-flow effects;
- per-element copy/move/borrow/destruction obligations;
- a canonical specialization identity sufficient for incremental compatibility.

**§ 9(4).** These facts must be verified before lowering. The MLIR/LLVM backends may select registers, stack slots, direct arguments, or other legal physical representations, but must not reinterpret the source-level ownership or control-flow semantics from the machine representation.

**§ 9(5).** A selected CompilationPlan/target ABI may affect lowering and reuse compatibility, never the semantic type identity or source-level meaning of the pack. Cache reuse must fail closed when the specialization or relevant target facts differ.

## § 10. Diagnostics and tooling

**§ 10(1).** Diagnostics must state the failing pack position and concrete type, and explain whether the error concerns type inference, invalid runtime-length heterogeneous spread, forbidden element move, unauthorized mutable borrow, expired lifetime, failed returned-reference provenance, invalid whole-pack consumption, or unsupported formatting operation within a specialization.

**§ 10(2).** An error in a concrete expansion must identify both the template/source location and the concrete instantiation or argument position. The compiler should suggest a legal `for ref`, `for ref mut`, explicit `<-` whole-pack spread, or ordinary conversion when appropriate. It must not suggest a hidden `any` or boxing workaround.

**§ 10(3).** The formatter, CST/parser, LSP, syntax reference, hover, signature help, and diagnostics registry shall understand `[...Types]`, `...ref Types`, `...ref mut Types`, type-pack inference, and `<-values...` without changing the grammar of ordinary arrays or spreads.

**§ 10(4).** This correction reserves **no new diagnostic ID**. Assign new IDs only through the existing centralized diagnostics registration procedure after checking for existing equivalent diagnostics.

## § 11. Compatibility with normal functions, `main`, and FFI

**§ 11(1).** The same normal native Sec parameter and inference rules apply to user-authored code, stdlib code, and compiler-visible core declarations. No additional source-language permission is granted to a function merely because it lives in `fmt` or `log`.

**§ 11(2).** Ordinary statically declared heterogeneous parameters on a `main` function do not call for a different general pack type system. The mechanism by which the selected platform supplies process-entry arguments or invokes a Sec entry function is governed separately by the existing entry-point and platform contracts; MD-117 does not invent a runtime ABI for arbitrary OS arguments.

**§ 11(3).** Native Sec type packs are **not** foreign C variadics. Direct foreign variadic calls remain unsafe and require ABI-compatible values and C's argument promotion and representation rules. No `fmt` usage relaxes FFI requirements.

## § 12. Mandatory documentation correction from `functions.md` § 33

**§ 12(1).** Change the existing example of two named move-only arguments from:

```sec
ConsumeAll(first, second)
```

to:

```sec
ConsumeAll(<-first, <-second)
```

**§ 12(2).** The explanatory text must explicitly state that `<-` is required for each **named** move-only argument, whereas a fresh temporary uses the normal fresh-value rule. This is a consistency fix, not a new move rule.

## § 13. Canonical rulebook synchronization (preserve paragraph identifiers)

**§ 13(1).** Update `rules/declarations/functions.md` §§28–36 and relevant call/IR sections with both homogeneous and heterogeneous forms, pack ownership, compile-time hetero arity, consuming spread, and the `ConsumeAll` example correction.

**§ 13(2).** Update `rules/declarations/generics.md` where it currently says generic packs are absent and every variadic instantiation has one concrete type; retain that statement **only for ordinary `[T]`-based homogeneous variadics**. Add the distinct `[...Types]` model and its specialization rules. Update `rules/compiler/monomorphization.md` and `rules/compiler/generics_lowering.md` correspondingly.

**§ 13(3).** Update `rules/declarations/spread.md` to define `<-pack...` **only** for complete owned native parameter packs, explicitly retaining the non-consuming general spread policy and the no-partial-moves rule for arrays, structs, and ordinary collections.

**§ 13(4).** Update `rules/control-flow/flowcontrol_for.md` to make heterogeneous pack expansion a compiler-known iterable category while preserving the existing loop binding, control-flow, and cleanup rules and without adding `for static`.

**§ 13(5).** Update memory rulebooks only where needed to distinguish a pack-backed reference from a caller-origin reference carried in a pack. Do not alter the separately governed `*` dereference syntax in MD-117.

**§ 13(6).** Update `rules/compiler/semantic_ir.md` for concrete type sequence, lifetime/provenance, expansion and transfer facts. The platform/FFI book needs only an explicit native-vs-foreign separation. `rules/library/stdlib.md` and `rules/library/core-library.md` should reference the general language facility, not duplicate it or invent formatting-only argument rules.

**§ 13(7).** Integrate implementation status with the existing governance schema and stable IDs. The **originating dependency** belongs in `governance/stdlib.yaml`; generic, declaration, parser, semantic, control-flow, ownership, lowering and FFI fragments own their respective actual implementation facts. Do not mark not-yet-implemented features implemented or create duplicate parallel ownership entries.

## § 14. Conformance tests and acceptance criteria

**§ 14(1). Positive syntax/inference:** Parse/type-check empty, one, and mixed `(string, int, bool)` packs; mixed named types; call from ordinary user modules; call from stdlib; normal homogeneous `...int`; a valid fixed-arity overload taking precedence over an equally valid variadic overload.

**§ 14(2). Negative syntax/type checks:** Duplicate/misplaced type pack, second/trailing-invalid variadic, unresolved specialization, heterogeneous pack from a runtime-length source, illegal erased-type inference, invalid concrete operation in one expanded iteration, invalid mutable aliases, `->values: ...Types`, and wrong call-mode requirements.

**§ 14(3). Iteration:** Verify zero iterations for empty pack; position/type specialization; ordinary `for value` copies copyable values and rejects move-only element binding; `for ref` on move-only values; `for ref mut` reborrow for `ref mut` elements; `break`, `continue`, nested `return`, `defer`, `try`, and panic/cleanup equivalence with expanded and unexpanded execution.

**§ 14(4). Ownership/spread:** Verify left-to-right evaluation once; rollback on failed argument preparation; consumption of named move-only values requires `<-`; legal `values...` for copyable or shared references; legal complete owned `<-values...`; invalid single-element move-out; invalid consuming spread from arrays/collections; no duplicate destruction; rejected reuse after successful whole-pack transfer.

**§ 14(5). Lifetimes:** Reject pack escape and returned references to expiring owned elements; permit provable caller-origin shared and exclusive reference return; reject a possible invalid origin; reject conflicting exclusive borrows; ensure fresh borrowed temporaries survive through nested calls but cannot escape beyond their original safe lifetime.

**§ 14(6). Lowering and tooling:** Check Semantic IR contains ordered type/provenance/transfer information, specialization and target compatibility, no mandatory pack heap allocation, reliable diagnostics, formatter idempotence, LSP hover/signature help, and native-vs-C-vararg distinction.

**§ 14(7). Acceptance:** MD-117 language **design** may be marked *decided* immediately from this correction. It may be marked *integrated/implemented* only after canonical rulebooks, governance, and tests reflect the feature. The separate stdlib formatting public API and the dereference-operator spelling are **not settled by this book** and must not be silently counted as complete.

---

## § 15. Decision ledger

| Topic | Decision | Status |
|---|---|---|
| Generic heterogeneous pack | `[...Types]` with ordered concrete types | Approved |
| Parameter forms | `...Types`, `...ref Types`, `...ref mut Types` | Approved |
| Uniform user/core/stdlib behavior | No function-specific special rule for `fmt` | Approved |
| Iteration | Existing `for`, compiler chooses expansion strategy | Approved |
| Loop binding | Existing by-value / `ref` / `ref mut` semantics | Approved |
| Semantic equivalence | No observable differences due to expansion/unrolling | Approved |
| Whole-pack consuming spread | `<-values...` for full owned native packs only | Approved |
| Element moves | No implicit or partial move-out from packs | Approved |
| Borrowing and lifetime | Existing provenance and exclusivity rules | Approved |
| Return of caller-origin references | Permitted with complete lifetime/authority proof | Approved |
| `*` dereference expression spelling | No decision here | Deferred separately |
| Public `fmt` API and grammar | No library API or format contract decided here | Separate stdlib task |
| Compiler implementation | No implementation claim | Pending verification |

### References checked for the design baseline

- `rules/declarations/functions.md` §§18, 28–36, 42–44 (including §33 example).
- `rules/declarations/generics.md` §§2, 16, 31–33.
- `rules/declarations/spread.md` §§1, 3–4, 7–8.
- `rules/control-flow/flowcontrol_for.md` §§4–7.
- `rules/memory/references.md` §§2–3, 9.
- `rules/compiler/monomorphization.md` §9 and type-specialization invariants.

*Repository note:* This decision package is based on the rules retrievable from the public `main` URLs and the explicit user-approved MD-117 decisions. A trustworthy current `main` HEAD hash was not available from the public browsing view, so the integrator must recheck HEAD and any newer rulebook changes before applying amendments.
