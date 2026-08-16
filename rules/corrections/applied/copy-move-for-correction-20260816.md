# Copy/move correction: Sec 0.1 `for` iteration

- **Status:** Applied normative correction
- **Applied:** 2026-08-16
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction path:** `rules/corrections/copy-move-for-correction.md`
- **Source:** `rules/control-flow/flowcontrol_for.md` revision 2.0
- **Target:** `rules/memory/copy_move.md`
- **Repository baseline reviewed:** `56be75d`

---

## 1. Resolve iterator-ownership ambiguity for Sec 0.1

Any general copy/move text that lists a moved element as a possible ordinary Sec 0.1 `for` binding must be narrowed.

For Sec 0.1, the source forms are:

```sec
for item in items {
}
```

```sec
for ref item in items {
}
```

```sec
for ref mut item in items {
}
```

Their ownership meanings are fixed:

```text
plain item
    by-value copy
    requires an implicitly copyable yielded type

ref item
    shared borrow

ref mut item
    exclusive mutable borrow
```

A plain loop binding is not contextually upgraded to a move for move-only `T`.

---

## 2. No consuming `for` syntax in Sec 0.1

Conceptual consuming-iterator examples such as:

```sec
for item in move items {
    Consume(item)
}
```

must be marked as future design material rather than Sec 0.1 source syntax.

Sec 0.1 also does not define:

```sec
for -> item in items {
    Consume(item)
}
```

The absence is intentional.

---

## 3. Move-only elements

For a move-only element type `T`, this is invalid:

```sec
for item in items {
    Consume(item)
}
```

The compiler must not:

- move the element from the source collection;
- silently borrow it;
- silently clone or duplicate it.

Use:

```sec
for ref item in items {
    Inspect(item)
}
```

for non-owning access.

Use an explicit collection-specific extraction/removal operation when ownership must leave the collection.

---

## 4. Future consuming iteration

The copy/move rulebook may retain a non-normative future-design discussion of consuming iterators, but it must state that such a feature is outside Sec 0.1 and requires separate rules for:

- collection ownership transfer;
- partial consumption state;
- early `break`;
- `continue`;
- function return and Result propagation;
- destruction of remaining elements;
- collection backing reclamation;
- fixed-array partial initialization;
- map/set extraction invariants.

No future conceptual syntax is reserved by this correction.
