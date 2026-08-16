# Collections correction: `for` binding ownership modes

- **Status:** Applied normative correction
- **Applied:** 2026-08-16
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction path:** `rules/corrections/collections-for-correction.md`
- **Source:** `rules/control-flow/flowcontrol_for.md` revision 2.0
- **Target:** `rules/collections/collections.md`
- **Repository baseline reviewed:** `56be75d`

---

## 1. Ordinary iteration is not an implicit move

Where `collections.md` states that ordinary iteration must not implicitly move an owned element from a collection, synchronize it with the canonical `for` binding model:

```sec
for item in items {
}
```

is a by-value copy binding.

It is valid only when the yielded element type is implicitly copyable.

For move-only `T`, the loop is invalid.

The compiler must not silently reinterpret the binding as `ref T` and must not move the element out of the collection.

---

## 2. Shared element iteration

For sequential collection categories that can provide stable addressable elements, add the canonical shared form:

```sec
for ref item in items {
}
```

The binding has type:

```text
ref T
```

This form is valid for move-only `T` because it does not transfer ownership.

Normal borrow and backing-validity rules remain in force.

---

## 3. Mutable element iteration

For sequential collection categories with mutable element authority, add:

```sec
for ref mut item in items {
}
```

The binding has type:

```text
ref mut T
```

This permits mutation of the represented element.

It does not permit structural mutation of the iterated collection while iteration state or element references depend on the current structure.

Operations such as `Append`, `Clear`, `RemoveAt`, and equivalent structural operations remain subject to the active iteration/borrow restrictions.

---

## 4. Sequential index and element binding

Sequential collections may use:

```sec
for index, item in items {
}
```

```sec
for index, ref item in items {
}
```

```sec
for index, ref mut item in items {
}
```

The index remains a normal `int` value binding.

The reference modifier applies to the element position only.

---

## 5. Map iteration

Keep map iteration as exactly two logical positions:

```text
key, value
```

Canonical forms include:

```sec
for key, value in users {
}
```

```sec
for ref key, ref value in users {
}
```

```sec
for ref key, ref mut value in users {
}
```

The last form requires mutable authority for the map values.

Reject:

```sec
for ref mut key, value in users {
}
```

A stored map key must not be mutated in place because doing so may change its equality/hash identity while it remains stored.

A plain key or value binding requires that component type to be implicitly copyable.

A discarded component `_` does not require copying or moving that component merely to discard it.

Map iteration order remains unspecified for ordinary `map[K, V]`.

---

## 6. Set iteration

Canonical set forms are:

```sec
for value in values {
}
```

and:

```sec
for ref value in values {
}
```

Reject:

```sec
for ref mut value in values {
}
```

A stored set value participates directly in equality/hash identity and must not be mutated in place while stored.

Plain value binding requires the set element type to be implicitly copyable.

Set iteration order remains unspecified for ordinary `set[T]`.

---

## 7. Consuming extraction remains operation-specific

Do not add consuming semantics to ordinary `for` iteration in Sec 0.1.

Owning extraction remains explicit through collection operations such as the already-defined operations that remove and return owned values.

This correction does not introduce a new `Take`, `Drain`, consuming iterator, or similar public collection API.

Any such API requires its own collection design decision.
