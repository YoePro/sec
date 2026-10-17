# MD-057 — Borrowed interface parameters and `Iterator[T]`

- **Status:** Design decided; normative correction pending integration and implementation verification
- **Created:** 2026-10-17
- **Updated:** 2026-10-17
- **Revision:** 1.0
- **Sec version:** 0.1
- **Canonical correction path (proposed):** `rules/corrections/MD-057-interface-iterator-parameters.md`
- **Amends:** `rules/declarations/interfaces.md` §7, §9.1; `rules/control-flow/flowcontrol_for.md` §13, §37, §40, §42; `rules/declarations/generics.md` (clarification only)
- **Motivation:** Resolve MD-057 without special syntax for iterator arguments or introduction of `dyn`.
- **Replaces:** None. This is an additive correction to the listed rulebooks, not a renumbering of their sections.

## 1. Decision and scope

**§1.1.** A borrowed interface parameter is written using the ordinary Sec reference syntax and a named interface type:

```sec
fn Inspect(value: ref Reader) void
fn Update(value: ref mut Counter) void
fn Consume(values: ref mut Iterator[string]) void
```

**§1.2.** The named interface describes the *complete behavioral contract* that the concrete argument type must explicitly implement. No source-level `dyn` keyword, anonymous type parameter, or redundant generic parameter is required merely to accept different implementing types.

**§1.3.** Such a parameter is a **borrowed interface view**. Its underlying concrete type can differ between invocations, and dispatch of instance members is permitted through the view at runtime. The view is not an owner of the implementing object.

**§1.4.** This correction defines borrowed interface views for function and method parameters and their ordinary uses, including `Iterator[T]`. It does **not** create a generally owned, unsized, by-value interface object, define interface-valued stored fields, add implicit conversions for generic containers, or alter the rules for type-level/static members.

## 2. Receiver authority, lifetime, and cost

**§2.1.** `ref Interface` grants shared receiver authority. `ref mut Interface` grants exclusive mutable receiver authority according to Sec's existing borrowing and reborrowing rules. A member declared `mut fn` cannot be called through a shared view.

**§2.2.** The borrow is tied to the lifetime of the underlying implementing object and cannot escape it. Creating or passing a borrowed interface view never transfers ownership or authorizes implicit copying or destruction of that object.

**§2.3.** A borrowed view contains sufficient runtime information to invoke the concrete implementation, by an implementation-defined representation such as a data reference and dispatch table. Creation and invocation of a borrowed view must not inherently allocate on the heap. The compiler may devirtualize when proven safe.

**§2.4.** Conformance remains explicit (`impl Concrete implements Interface`). A matching set of method names without explicit conformance is insufficient.

**§2.5.** The interface must be fully instantiated wherever it has generic parameters, for example `Iterator[string]`. This feature does not provide runtime dispatch of method-level generic methods.

## 3. Named generics remain available

**§3.1.** A named generic constraint retains its existing *static* meaning:

```sec
fn CountStatic[I: Iterator[int]](values: ref mut I) uint {
    let mut count: uint := 0
    for value in values {
        count += 1
    }
    return count
}
```

**§3.2.** `I` is a concrete type parameter resolved at specialization. The generic form is preferred when a signature must preserve the same concrete type across multiple positions, expose that type in a result, or deliberately specialize operations.

**§3.3.** The non-generic interface view and generic constraint do not mean the same thing internally. The former permits runtime dispatch; the latter performs static type substitution and concrete specialization.

**§3.4.** Two independent `ref Reader` parameters need not refer to objects of the same concrete type. To require the same concrete type, use a shared named generic parameter.

## 4. `Iterator[T]` is not an exception to borrowed interface views

**§4.1.** Preserve the existing compiler-known interface declaration and mandatory explicit conformance:

```sec
interface Iterator[T] {
    mut fn Next() Option[T]
}
```

**§4.2.** `Iterator[T]` may now appear in a mutable borrowed interface parameter, exactly like any other applicable interface:

```sec
fn Count(values: ref mut Iterator[int]) uint {
    let mut count: uint := 0
    for value in values {
        count += 1
    }
    return count
}
```

**§4.3.** The caller supplies a mutable borrow of a concrete type that explicitly implements `Iterator[int]`. The callee may advance the borrowed iterator by calling its `mut fn Next()` through the interface view, but cannot take ownership of the underlying iterator.

**§4.4.** `for value in values` is supported when `values` is `ref mut Iterator[T]`. The compiler lowers this to successive calls of the interface view's `Next()`, ending at `None`; each `Some(value)` yields an owned `T`, following the existing §37 protocol. It does not add a borrowing element protocol, implicit moves from collections, a second iteration factory protocol, or more than one iterator binding.

**§4.5.** Existing iteration over a **concrete** `Iterator[T]` implementer retains statically resolved `Next()` and the no-implicit-allocation guarantee. An explicit named generic parameter also retains static dispatch. Only iteration via the borrowed interface view uses runtime interface dispatch.

**§4.6.** Amend `flowcontrol_for.md` §40 by removing the blanket prohibition on "runtime-dispatched Iterator interface values" **only for borrowed `ref mut Iterator[T]` views covered by this correction**. Do not silently authorize owned or otherwise unspecified interface values.

## 5. Canonical examples and limitations

**§5.1.** This is the short, readable interface form for an interchangeable implementation:

```sec
fn ReadAll(source: ref mut Reader) void {
    // Calls allowed by Reader's declared mutable receiver contract.
}
```

**§5.2.** The iterator-specific analogue uses the same syntax and ordinary borrow rules:

```sec
fn Process(values: ref mut Iterator[string]) void {
    for value in values {
        Use(value)
    }
}
```

Here `Use` is illustrative. The public signature of any actual `stdlib/str` function (such as `Join` or `FromRunes`) remains owned by that standard-library rulebook. This correction does not choose whether a particular library operation borrows, owns, or consumes its iterator.

**§5.3.** This correction does not make `Iterator[T]` a by-value type, and does not imply that `fn Process(values: Iterator[T])` has the same ownership semantics as `fn Process(values: ref mut Iterator[T])`.

**§5.4.** Members dependent on a statically known concrete `Self`, including type-level/static constructors, retain their existing rules. Whether a future interface needs additional restrictions for first-class value storage or broader dynamic interface usage is outside MD-057; do not invent such behavior as part of this amendment.

## 6. Required rulebook synchronization

**§6.1.** Amend `rules/declarations/interfaces.md` §7 with the borrowed interface view semantics in §§1–3 and the distinct static-generic form. Preserve existing §7 examples and all existing explicit-conformance and receiver-capability requirements.

**§6.2.** Amend `rules/declarations/interfaces.md` §9.1: compiler-known `Iterator[T]` remains special in how the compiler *recognizes* iteration, not in whether a borrowed interface view is permitted. Refer to `flowcontrol_for.md` §37 for the two dispatch paths.

**§6.3.** Amend `rules/control-flow/flowcontrol_for.md` §13, §37, §40, and §42 to distinguish concrete statically dispatched iteration from borrowed-interface dynamically dispatched iteration. Preserve the paragraph and section identifiers already referenced by source code and tests.

**§6.4.** Clarify (without changing) `rules/declarations/generics.md` that a constrained generic type parameter is statically specialized, whereas an ordinary borrowed interface parameter is a distinct runtime interface view. Do not introduce `dyn` or implicit anonymous generics.

**§6.5.** Synchronize any `stdlib/str` signatures that illegally use an unowned `Iterator[T]` parameter as though it were a concrete, sized value. Library-specific API ownership remains a separate concern.

## 7. Compiler requirements

**§7.1.** Name/type resolution must recognize interface types (including concrete instantiations such as `Iterator[string]`) as valid targets of `ref` and `ref mut` parameters.

**§7.2.** Call checking must verify explicit interface conformance, receiver mutability, and the lifetime of the concrete storage. No cast may silently turn shared authority into exclusive authority.

**§7.3.** Semantic IR must distinguish (a) a borrowed runtime interface view with an interface method call, and (b) a named generic specialization with a resolved concrete method call. It must preserve the interface identity, generic arguments, borrow authority, and concrete witness/dispatch information as needed for lowering.

**§7.4.** Lowering must support runtime method dispatch for borrowed interface views without mandatory heap allocation. It must preserve normal `Option[T]` ownership, `for` control flow, cleanup, and side-effect order.

**§7.5.** Diagnostics should identify missing explicit conformance, missing mutable receiver authority, escaping borrows, invalid interface member calls, and the distinction between an unsupported owned interface value and a supported borrowed interface view. Diagnostics should offer the shortest valid repair when possible.

## 8. Minimum regression tests

**§8.1.** Positive: `ref Reader` accepts two distinct concrete implementing types in separate calls, without a generic parameter or `dyn`.

**§8.2.** Positive: `ref mut Counter` calls a `mut fn`; shared `ref Counter` is rejected for that call.

**§8.3.** Positive: two independent `ref Reader` parameters may receive different concrete types; a named generic requiring one shared type rejects such mismatches.

**§8.4.** Positive: `ref mut Iterator[int]` accepts a concrete explicitly implementing iterator, calls `Next()` and iterates with `for` until `None`.

**§8.5.** Positive: concrete iterator loops and named generic iterator loops continue to use statically resolved concrete `Next()` calls.

**§8.6.** Negative: matched method spelling without `implements Iterator[int]` does not satisfy the borrowed iterator interface parameter.

**§8.7.** Negative: shared `ref Iterator[int]` cannot call `mut fn Next()` and cannot be used as a mutable iterator source.

**§8.8.** Negative: an iterator interface borrow cannot outlive its concrete owner, and cannot overlap incompatible exclusive borrows.

**§8.9.** Negative: by-value `Iterator[int]` is not silently treated as either an owned type or an implicit anonymous generic parameter.

**§8.10.** Check for no mandatory heap allocation in interface-view construction/dispatch, plus `for` break/continue/return cleanup parity and debug/Semantic IR differentiation.

## 9. Implementation and governance status

**§9.1.** **Design status: decided.** This text records the chosen surface syntax and semantic distinction; it does not claim the compiler already supports runtime-dispatched borrowed `Iterator[T]` views.

**§9.2.** Implementation state must be audited against parser, Sema, Semantic IR, lowering, diagnostics, and tests. Existing static iterator support does not demonstrate runtime interface-view support.

**§9.3.** Update the appropriate governance fragment and MD-057 backlog entry only after syncing the canonical rulebooks. Preserve any failures as implementation gaps or identified bugs rather than marking functionality implemented without tests.
