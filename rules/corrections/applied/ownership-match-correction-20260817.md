# Ownership correction — `match` payload transfer and guard commit

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-17
- **Last updated:** 2026-08-17
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/memory/ownership.md`

---

## Correction

The ownership rulebook's existing match-specific Place and availability model must incorporate the following normative Sec 0.1 behavior.

### Whole-payload plain binding

For:

```sec
Some(value)
```

where the payload type is `T`:

- implicitly copyable `T` is copied;
- move-only `T` is moved when the subject is an owned reusable Place from which ownership can legally be transferred;
- a fresh temporary subject may forward the payload directly;
- no hidden cloning or implicit borrowing is permitted.

### Borrowed subject

A subject available only through `ref` or `ref mut` cannot transfer ownership of a move-only payload.

Mutable borrowing authority is not ownership authority.

Move-only payload access through a borrowed subject requires the appropriate explicit pattern borrow.

### Guard commit point

For a guarded move-only by-value binding:

```sec
Some(value) where condition => body
```

the move is not committed when the pattern test succeeds.

The binding is a candidate arm binding while the guard is evaluated.

The move commits only when the guard succeeds and the arm is selected.

A consuming use of the prospective move-only binding inside the guard is invalid.

A guard may inspect or borrow it according to normal rules.

When the guard is false:

- that rejected arm contributes no committed payload move;
- later arms continue from the actual subject state after any real guard side effects;
- branch-local candidate borrows from the rejected arm end before the next arm is tested.

### Shallow field destructuring

Plain by-value shallow field bindings are copy-only.

Move-only fields must use `ref` / `ref mut` or whole-payload ownership binding.

Shallow field destructuring must not create a partially moved reusable union value.

### Match merge

Every continuing arm contributes its ownership state to the post-match merge.

The merge is mandatory language-safety analysis.

Examples include:

```text
Available + Available
    Available

Moved + Moved
    Moved

Available + Moved
    ConditionallyAvailable
```

A later whole-value use requires definite availability on every continuing path.

Terminating arms do not contribute a post-match state.

### Diagnostics and provenance

Ownership state created by `match` should retain sufficient provenance to identify:

- subject Place;
- active variant/payload Place;
- arm index or source location;
- binding receiving ownership;
- guard-success commit point;
- post-match merge reason.

These facts are shared with diagnostics and LSP tooling.

## Cross-reference

Canonical source semantics are defined by:

```text
rules/control-flow/flowcontrol_match.md
```
