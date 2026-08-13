# Correction: `static.md` sync with `impl` revision 2.0

**Target:** `rules/declarations/static.md`  
**Source rule:** `rules/declarations/impl.md` revision 2.0  
**Date:** 2026-08-13

## Replace stale explicit-self wording

Remove or rewrite statements such as:

```text
An ordinary method belongs to an instance and must declare self.
method Create must declare self or be marked static
```

Canonical Sec instance methods have compiler-provided implicit `self`.

Use examples like:

```sec
impl Counter {
    fn Increment() void {
        self.value += 1
    }
}
```

Do not use canonical examples containing:

```sec
ref self
ref mut self
```

The compiler derives and validates receiver access/mutability from the body and
ownership/borrowing semantics.

## Preserve explicit `static`

Keep the existing rule that type-level members are explicitly marked `static`:

```sec
impl Counter {
    static fn CreateDefault() Counter {
        return Counter {
            value: 0,
        }
    }
}
```

A static member has no instance `self`, and use of `self` in a static member is
invalid.

Do not infer static membership merely because a method body does not use `self`.

## Clarify the word `init`

`static.md` currently states that Sec must not introduce hidden `init()` startup
behavior for runtime static initialization.

Preserve that semantic rule, but rewrite it so it cannot be confused with the
new explicit impl lifecycle member:

```text
Sec must not introduce hidden module/static startup initializers.
The explicit `init(...)` lifecycle member defined by impl.md is instance
construction and does not authorize hidden static/module initialization.
```

## Related-rule reference

Update references from `impl.txt` to `impl.md` revision 2.0.
