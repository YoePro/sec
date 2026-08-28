# Correction 30 — Ownership revision-2 refinements for availability tests, call-transfer reservation, and dynamic ownership state

## Audit context

- Repository: `github.com/YoePro/sec`
- Audited: `2026-08-28`
- Primary rulebook: `rules/memory/ownership.md`
- Status: Normative
- Created: `2026-08-26`
- Last updated: `2026-08-26`
- Document revision: `2.0`
- Sec language version: `0.1`
- Rulebook-declared repository baseline reviewed: `b3315f6`
- Current audit source: repository `main`

## Classification

This correction addresses three internal revision-2 ownership issues:

1. `is not available` currently collapses every successful negative availability
   test to `Unavailable`, which is not correct for `PartiallyAvailable`,
   `Uninitialized`, or richer conditional aggregate state;
2. call-transfer commit defines delayed ownership commitment but does not define
   the required reservation of a reusable source while later arguments are
   evaluated;
3. Section 21.3 describes runtime ownership state as if it were needed only by a
   later `is available` query, although Sections 23–25 also require conditional
   discard, replacement, and destruction.

The first and third are internal rulebook consistency corrections.

The second is both a specification gap and an implementation issue: current
ordinary-call Sema evaluates all arguments before applying moved-call argument
state, but has no corresponding pending-transfer reservation.

This correction does not replace the revision-2 availability model. It makes the
existing model precise.

---

# 1. `is available` must test whole availability of the exact Place

Section 5 defines:

```text
Uninitialized
Available
PartiallyAvailable
Unavailable
ConditionallyAvailable
```

and permits a richer internal lattice.

Section 18 states:

```text
Whole-value operations require the whole value to be Available.
Sub-place operations require only the addressed sub-place to be Available.
```

Therefore availability is a property of the exact tested Place.

For:

```sec
let package := LoadPackage()
let payload :<- package.Payload
```

the state is:

```text
package.Payload    Unavailable
package.Header     Available
package            PartiallyAvailable
```

The whole Place `package` is not `Available`, even though some owned sub-places
remain.

The canonical meaning is therefore:

> `place is available` is true exactly when the tested Place has a complete
> currently owned value available for an ordinary operation on that Place on the
> current runtime path.

For an aggregate Place this requires the aggregate to be wholly `Available`
according to its recursive availability mask.

---

# 2. `place is not available` must not collapse every negative state to `Unavailable`

Section 21 currently states that inside the true branch of:

```sec
place is not available
```

ownership analysis refines the tested Place to `Unavailable`.

That is too strong because `is not available` is the logical negation of
`is available` and can therefore be true for several distinct states.

## Partial aggregate example

```sec
let package := LoadPackage()
let payload :<- package.Payload

if package is not available {
    Use(package.Header)
}
```

The condition is true because the complete `package` value is not available.

But the compiler must not refine the whole root to `Unavailable`, because
`package.Header` remains owned and available.

Collapsing the root to `Unavailable` would erase still-owned sub-place state and
could corrupt:

- legal sibling-field access;
- later convergence;
- replacement/reinitialization;
- destruction responsibility.

## Uninitialized example

```sec
let mut resource: Resource

if resource is not available {
    resource = OpenResource()
}
```

The test can be statically true without pretending that the place previously
held a value that was moved, discarded, or detached.

`Uninitialized` and `Unavailable` remain distinct semantic facts.

---

# 3. Canonical availability-test refinement

The normative refinement is:

```text
true branch of `place is available`
    -> tested exact Place is Available

false branch of `place is available`
    -> tested exact Place is not wholly Available;
       preserve every still-possible sub-place/state alternative

true branch of `place is not available`
    -> same refinement as the false branch above

false branch of `place is not available`
    -> tested exact Place is Available
```

The compiler may internally represent the negative branch as a state set,
recursive mask, `NotWhollyAvailable`, or another richer fact.

That internal representation need not become a new source-visible availability
state.

Only when prior facts prove that the tested Place can be exactly one of:

```text
Available
Unavailable
```

may the true branch of `is not available` refine directly to `Unavailable`.

For a `ConditionallyAvailable` leaf whose only runtime alternatives are
`Available` and `Unavailable`, the runtime split may naturally refine to those
two exact states.

For partial or conditional aggregate masks, the negative branch must retain the
surviving mask.

An availability test observes/refines ownership state. It does not itself move,
discard, detach, or destroy a value and must not invent a new
`UnavailableReason`.

---

# 4. Call-transfer commit requires a pending source reservation

Section 14 correctly establishes:

```text
arguments evaluate left-to-right;
outer-call transfer commits only after every argument has evaluated successfully
and the call is ready to enter the callee;
failure of a later argument must not commit the prepared outer-call transfer;
effects already completed inside argument evaluation are not rolled back.
```

However the rule does not define what happens to an earlier reusable move source
while later arguments are still being evaluated.

Consider:

```sec
Use(
    <-resource,
    Inspect(resource),
)
```

If `<-resource` leaves `resource` ordinarily `Available` until final commit,
the second argument may reuse the same source after it has already been prepared
for destructive transfer.

More dangerous forms include:

```sec
Use(
    <-resource,
    Mutate(resource),
)
```

```sec
Use(
    <-resource,
    Other(<-resource),
)
```

Marking the source as an ordinary committed `Unavailable` immediately is also
wrong because failure of a later outer argument must not commit the outer call
transfer.

A distinct pending/reserved fact is therefore required.

---

# 5. Canonical pending-transfer rule

When evaluation of an outer-call argument prepares an explicit destructive
transfer from a reusable Place:

```sec
<-source
```

the compiler creates a **pending call-transfer reservation** for that source
Place.

This is a compiler-owned transactional fact, not a new source-visible
availability state.

Conceptually:

```text
before argument
    source is Available

after successful preparation of <-source
    caller still owns source until outer-call commit
    PendingTransfer(source, outerCall) exists

while reservation exists
    overlapping later sibling argument evaluation may not:
        read
        copy
        borrow
        move
        discard
        replace
        reinitialize
        otherwise consume/mutate the reserved ownership

if later argument evaluation fails before outer-call entry
    cancel this outer-call reservation
    caller retains the source ownership state that remains after any
    independently completed earlier-argument effects

if every argument succeeds and call entry is ready
    commit all prepared outer-call transfers
    source becomes Unavailable in caller
    callee receives ownership
```

The reservation follows canonical Place overlap.

Therefore this may be legal when fields are proven independent:

```sec
Use(
    <-package.Payload,
    Inspect(package.Header),
)
```

while this is not:

```sec
Use(
    <-package.Payload,
    Inspect(package),
)
```

Unsupported or ambiguous aliasing remains conservative.

---

# 6. Nested calls have nested transfer transactions

Each call owns its own pending-transfer transaction.

Example:

```sec
Outer(
    <-a,
    Inner(<-b, Load()),
)
```

If `Inner` successfully commits ownership of `b`, that transfer is an effect
completed during evaluation of an outer argument and is not rolled back merely
because `Outer` later fails before entry.

The pending reservation for `a` belongs to `Outer` and is canceled if `Outer`
cannot commit.

This preserves Section 14's existing rule that completed effects inside argument
evaluation are not rolled back.

---

# 7. Current Sema consequence

At the audited source, ordinary direct-call analysis:

1. evaluates all argument expressions in `callArgumentTypes`;
2. resolves the overload;
3. only then calls `markMovedCallArguments`.

There is no pending-transfer/reservation state between an earlier prepared
`<-source` argument and evaluation of later sibling arguments.

The implementation therefore needs a reservation phase, not merely delayed
final move marking.

Compiler-known calls and constructors that consume arguments must follow the
same transaction rule where they have equivalent multi-argument ownership
semantics.

---

# 8. Dynamic ownership state is not only for explicit availability queries

Section 21.3 currently says a conditionally available Place may require runtime
state if the program later asks `is available`.

That wording is too narrow.

The same rulebook requires runtime path-dependent behavior for:

```text
discard of ConditionallyAvailable
replacement/reinitialization of ConditionallyAvailable
scope-exit destruction of conditional ownership
partial aggregate cleanup
other ownership-dependent operations that must know whether a value is still owned
```

Section 23 requires conditional destruction/no-op before discard convergence.
Section 24 requires conditional destruction before replacement. Section 25
requires destruction to follow the value that remains owned.

Those operations may require SSA/drop state even when source code never spells
`is available`.

The corrected principle is:

> A genuinely conditional ownership fact may require runtime state when any
> later ownership-dependent operation cannot be implemented correctly from
> static control-flow facts alone.

Such operations include, but are not limited to:

```text
is available / is not available
discard
replacement/reinitialization
destruction/cleanup
ownership-sensitive transfer/return
another rulebook-defined ownership decision
```

Section 31's ABI restrictions remain unchanged.

The static-ownership policy in Section 22 applies at the first point where any
such operation genuinely requires forbidden dynamic state, not only at an
availability query.

---

# 9. Required test corrections/additions

The current revision-2 test item:

```text
`is not available` refines true path to unavailable
```

must be narrowed.

Required coverage:

1. leaf `Available` -> `is available` true refinement;
2. binary `Available|Unavailable` conditional leaf -> `is not available` true
   refines to `Unavailable`;
3. `PartiallyAvailable` aggregate -> whole `is available` is false;
4. `PartiallyAvailable` aggregate -> whole `is not available` is true without
   erasing still-owned sibling fields;
5. `Uninitialized` mutable Place -> `is not available` is statically true while
   preserving uninitialized provenance;
6. conditional partial aggregate preserves its recursive mask on the negative
   branch;
7. availability tests never invent `Moved`, `Discarded`, or `Detached` reason;
8. earlier `<-source` call argument reserves against later sibling read;
9. reservation blocks later sibling borrow;
10. reservation blocks later sibling second move;
11. reservation is Place-sensitive and permits proven-disjoint sibling fields;
12. failure of a later argument cancels only the outer-call reservation;
13. a committed nested-call ownership effect is not rolled back by outer-call
    failure;
14. successful outer call commits every prepared transfer exactly once;
15. conditional discard can require runtime ownership state without any
    `is available` expression;
16. conditional replacement can require runtime ownership state without an
    availability query;
17. conditional scope-exit cleanup can require runtime ownership state without
    an availability query;
18. static-ownership policy rejects the actual ownership-dependent operation
    requiring dynamic state.

---

# 10. Governance implications

The root governance must explicitly track:

```text
availability-state representation
UnavailableReason/provenance
recursive partial aggregate masks
conditional availability and conditional-partial masks
availability-test parsing and refinement
pending call-transfer reservations and commit/cancel
runtime ownership-state requirement analysis
static-ownership policy
conditional discard/replacement/destruction
return forwarding
receiver/member ownership
custom-free partial-move prohibition
closure capture ownership
Result/try/match ownership integration
Semantic IR ownership facts
backend cleanup/ownership verification
mentor diagnostics
formatter/LSP ownership presentation
```

The old `frontend.copy-move` entry predates ownership revision 2 and is not a
complete implementation plan for this rulebook.

A dedicated `frontend.ownership-v2` governance entry is preferred so
`copy_move.md` classification work and `ownership.md` availability/transfer
state do not remain conflated.

---

# 11. Source-to-rulebook traceability

Under the traceability rule introduced by `correction25.md`, touched semantic
code should cite precise ownership sections.

Call analysis helpers involved in:

```text
argument evaluation
pending move reservation
Place overlap
call transfer commit/cancel
move-source marking
```

should cite:

```text
rules/memory/ownership.md §13
rules/memory/ownership.md §14
rules/memory/ownership.md §18
```

Availability analysis should cite:

```text
rules/memory/ownership.md §5
rules/memory/ownership.md §18
rules/memory/ownership.md §§20-25
rules/memory/ownership.md §§31-32
```

Focused regression tests should cite `correction30.md` where practical.
