# Applied Correction — Terminal Return Ownership Forwarding Through Construction

## Audit context

- Repository: `github.com/YoePro/sec`
- Audited: 2026-09-03
- Repository baseline reviewed: `998d8d1`
- Sec language version: `0.1`
- Primary rulebooks:
  - `rules/memory/ownership.md`
  - `rules/memory/copy_move.md`
- Related governance: `implementation-status.yaml`
- Related implementation: `internal/sema/analyzer.go`

## Classification

**Normative consistency correction and generalization.**

The current rule set contains an inconsistency:

- `memory/copy_move.md` already permits `return Ok(resource)` as a terminal
  ownership transfer without an inner `<-` marker.
- `implementation-status.yaml` already requires this behavior.
- `memory/ownership.md` still states the general constructor rule in a form that
  requires `<-` for reusable move-only payload sources without fully describing
  how terminal return context propagates through construction.

This correction makes the terminal-return rule general rather than
`Result`-specific.

---

## 1. Core rule

A non-reference `return` is an intrinsically terminal ownership-transfer context.

The terminal return context propagates recursively through structural owning
construction that directly forms the returned value.

Therefore an owned reusable local may be transferred into the returned value
without an explicit `<-` marker when that transfer is required to construct the
returned value.

Valid examples:

```sec
return resource
return Ok(resource)
return Err(errorValue)
return Some(resource)
return Packet.Data(resource)

return Response {
    Body: resource,
}

return Ok(Response {
    Body: resource,
})
```

The compiler determines the required copy or ownership transfer from:

- the source copy classification;
- the destination ownership requirements;
- the fact that the expression directly constructs the function result;
- ordinary borrowing, lifetime, availability, and storage restrictions.

The programmer must not be required to annotate every nested ownership edge
inside an already terminal return expression.

---

## 2. Explicit `<-` remains permitted

The explicit move marker remains legal inside a terminal return expression:

```sec
return Ok(<-resource)
```

and:

```sec
return Response {
    Body: <-resource,
}
```

These forms are documentation only when the same transfer is already implied by
the terminal return context.

They must not change ownership semantics relative to the concise forms.

The formatter must not insert `<-` merely because a terminal return forwards a
move-only value.

---

## 3. Non-terminal construction keeps ordinary copy/move semantics

Terminal-return forwarding does **not** change ordinary construction.

For a copyable reusable source:

```sec
let response := Ok(result)
Use(result)
```

`Ok(result)` copies the payload according to ordinary copy semantics.

The original `result` remains available.

For a source whose type is `@noCopy` or otherwise move-only:

```sec
let response := Ok(result)
```

is invalid when construction requires ownership of the source.

The ownership transfer must be explicit:

```sec
let response := Ok(<-result)
```

After that transfer:

```sec
Use(result)
```

is invalid because `result` is unavailable.

The compiler must never reinterpret ordinary non-terminal syntax as a hidden
destructive move merely because the source is move-only.

---

## 4. Copyable values in terminal return context

A copyable value remains copyable.

This correction does not make copyable types move-only and does not change
ordinary non-terminal copy semantics.

However, when the only remaining use of an owned local is as part of the
function result:

```sec
return Ok(result)
```

the language semantics need not require an observable intermediate copy of
`result`.

The compiler may forward the owned value directly into the returned carrier and
may elide representation copies when this preserves all language semantics.

There is no continuing source path after `return` on which the local binding can
be observed.

Semantic IR should therefore represent the operation as return ownership
forwarding rather than requiring an artificial source-level copy followed by a
second return transfer.

---

## 5. Recursive propagation through returned construction

Terminal return context propagates through nested structural construction.

Example:

```sec
return Ok(Response {
    Body: resource,
})
```

If `resource` is move-only, no inner `<-` is required.

Conceptually:

```text
resource
    -> Response.Body
    -> Ok payload
    -> function result
```

is one terminal ownership path.

This applies to compiler-known and language-defined structural owning
construction, including:

- struct/aggregate construction;
- union variant construction;
- `Option.Some`;
- `Result.Ok`;
- `Result.Err`;
- nested combinations of the above.

---

## 6. Return context through value-producing control flow

When a value-producing control-flow expression directly supplies the return
value, terminal return context propagates into every value-producing arm that
reaches that return.

Example:

```sec
return match state {
    State.Ready => Ok(resource)
    State.Failed => Err(errorValue)
}
```

Each continuing arm is analyzed as constructing the terminal function result.

A non-terminal match does not gain implicit destructive transfer:

```sec
let response := match state {
    State.Ready => Ok(resource)
    State.Failed => Err(errorValue)
}

// Execution continues here.
```

Ordinary copy/move rules apply in that form.

---

## 7. Ordinary function calls are not structural return forwarding

Terminal return context must not silently convert ordinary reusable call
arguments into consuming arguments.

For example:

```sec
return Wrap(resource)
```

does not by itself authorize an implicit destructive move into `Wrap`.

The call is validated according to the declared parameter mode of `Wrap`.

If the parameter is an ordinary by-value parameter:

- a copyable source is copied;
- a move-only or `@noCopy` reusable source is not silently consumed.

If the parameter is explicitly consuming, the normal consuming-call rules
apply.

The return context governs construction of the function result; it does not
rewrite unrelated function-call contracts.

---

## 8. Restrictions remain unchanged

Terminal-return forwarding does not bypass any other ownership or memory rule.

In particular, it does not permit:

- moving from borrowed storage without ownership authority;
- moving a move-only element out through ordinary indexing when collection
  extraction rules do not permit it;
- moving from fixed, static, foreign, volatile, or MMIO storage when ownership
  cannot be transferred;
- violating an active borrow;
- returning a reference to invalid local storage;
- moving an unavailable or partially unavailable Place as if it were complete;
- bypassing `@noCopy` copy restrictions;
- cloning a move-only value.

The return context removes redundant source syntax. It does not weaken semantic
validation.

---

## 9. Required rulebook changes

### `rules/memory/ownership.md`

Update the constructor/payload-transfer rules so that:

1. ordinary non-terminal construction keeps the explicit-move rule for reusable
   move-only sources;
2. terminal return context propagates recursively through returned structural
   construction;
3. `return Ok(resource)`, `return Some(resource)`, returned union payloads, and
   returned aggregates are covered by the same rule;
4. `<-` remains permitted but optional inside terminal return construction.

Update the return-boundary section so it describes nested return construction,
not only direct `return resource`.

### `rules/memory/copy_move.md`

Generalize the existing `Result`-specific return exception in §10.3.

The terminal-return rule should apply consistently to:

- §10.1 aggregate construction;
- §10.2 Option construction;
- §10.3 Result construction;
- §10.4 union construction.

Prefer one general rule in §9 or the beginning of §10, with carrier-specific
sections referring to it.

The existing rule that non-terminal plain constructor syntax must not become an
implicit destructive move remains unchanged.

---

## 10. Governance changes

Update `implementation-status.yaml`.

### `frontend.copy-move`

The current governance already requires `return Ok(resource)` to remain valid as
a terminal transfer.

Generalize that requirement to all structural returned construction.

Required tests must include at least:

```text
return Some(resource) accepts a reusable move-only local as terminal forwarding
return UnionVariant(resource) accepts a reusable move-only local as terminal forwarding
return Struct { Field: resource } accepts a reusable move-only local as terminal forwarding
return Ok(Struct { Field: resource }) recursively forwards ownership
explicit <- remains accepted in all equivalent terminal forms
non-terminal Some(resource) still rejects a reusable move-only source
non-terminal union payload construction still rejects a reusable move-only source
non-terminal aggregate field construction still rejects a reusable move-only source
copyable non-terminal Ok(result) copies and leaves result available
@noCopy non-terminal Ok(result) rejects hidden consumption
@noCopy non-terminal Ok(<-result) consumes and makes result unavailable
borrowed/indexed/volatile sources retain their existing extraction restrictions
```

Diagnostics must not recommend adding `<-` when the source is already on a valid
terminal return-construction path.

### `semantic-ir.copy-move`

Semantic IR must represent nested terminal construction as return forwarding
without requiring source syntax markers at each constructor boundary.

The representation must still preserve:

- source Place identity;
- copy classification;
- ownership destination;
- partial availability;
- destruction responsibility;
- restricted storage origin;
- explicit `<-` when diagnostically useful.

The semantic result of:

```sec
return Ok(resource)
```

and:

```sec
return Ok(<-resource)
```

must be ownership-equivalent.

The same requirement applies to aggregate, Option, union, and recursively nested
returned construction.

---

## 11. Frontend implementation changes

`internal/sema/analyzer.go` currently has Result-specific terminal ownership logic
inside `analyzeResultReturnStatement`.

Result-specific code should continue to validate:

- `Result[T, E]` shape;
- `Ok` success type;
- `Err` error type;
- error widening;
- Result-specific diagnostics.

However, terminal ownership forwarding should move into a general
return-expression ownership context.

The ownership analyzer should recursively analyze an expression that constructs
a returned owned value and mark eligible reusable owned sub-Places as forwarded
to the caller.

Conceptually:

```text
AnalyzeReturn(expression)
    establish terminal return ownership context
    analyze expression type and validity
    propagate return ownership context through structural construction
    validate each source Place
    commit TransferToReturn ownership effects
```

Do not implement this as a growing list of special cases for `Ok`, `Some`,
individual union variants, and struct literals.

---

## 12. Frontend tests

Add focused tests for these cases.

### Copyable non-terminal construction

```sec
let result := CreateCopyable()
let response := Ok(result)
Use(result)
```

Expected: valid; `result` remains available.

### Move-only non-terminal construction

```sec
let resource := CreateMoveOnly()
let response := Ok(resource)
```

Expected: invalid; no hidden consume.

### Explicit non-terminal move

```sec
let resource := CreateMoveOnly()
let response := Ok(<-resource)
Use(resource)
```

Expected: final use rejected.

### Move-only terminal Result construction

```sec
let resource := CreateMoveOnly()
return Ok(resource)
```

Expected: valid.

### Move-only terminal Option construction

```sec
let resource := CreateMoveOnly()
return Some(resource)
```

Expected: valid.

### Move-only terminal union construction

```sec
let resource := CreateMoveOnly()
return Payload(resource)
```

Expected: valid when `Payload` owns that value.

### Move-only terminal aggregate construction

```sec
let resource := CreateMoveOnly()

return Container {
    Value: resource,
}
```

Expected: valid.

### Recursive terminal construction

```sec
let resource := CreateMoveOnly()

return Ok(Container {
    Value: resource,
})
```

Expected: valid.

### Illegal extraction remains illegal

A terminal return must still reject ownership extraction from a source that is
not legally movable, including an ordinary indexed read of a move-only
collection element when the collection rules forbid that extraction.

---

## 13. Rulebook status synchronization

After this correction is merged into the canonical rulebooks:

1. update `language-rulebook-status.md` notes for `memory/ownership.md` and
   `memory/copy_move.md` if their revisions or synchronization status change;
2. update `implementation-status.yaml` with the generalized requirements;
3. verify frontend tests and Semantic IR governance;
4. move this correction to:

```text
rules/corrections/applied/return-context-ownership-forwarding-correction-YYYYMMDD.md
```

The pending correction must not remain in `rules/corrections/` after all
normative target books have been updated.

---

## 14. Normative summary

> Ordinary expressions copy copyable reusable values by default and never hide a
> destructive move of a reusable move-only value. A non-reference `return` is an
> intrinsically terminal ownership-transfer context, and that context propagates
> recursively through structural construction of the returned value. Therefore
> `<-` is never required solely to describe ownership forwarding along a valid
> terminal return-construction path.

This keeps the source language simple while retaining strict compiler ownership
analysis.
