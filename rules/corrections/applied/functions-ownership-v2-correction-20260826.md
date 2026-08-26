# Correction: explicit consuming arguments and non-consuming ordinary methods

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/functions.md`

---

## Correction

Synchronize function and method call ownership semantics with
`rules/memory/ownership.md` revision 2.0.

### Consuming parameters require an explicit call-site move

A parameter declared with `->` takes ownership of its argument:

```sec
fn Consume(-> value: Resource) void {
    ...
}
```

When the argument is a reusable source place, the call site must make the
transfer explicit:

```sec
Consume(<-resource)
```

This requirement is independent of whether `Resource` is copyable. The `->`
parameter contract is explicitly consuming.

A plain call must not silently consume a reusable source:

```sec
Consume(resource) // invalid
```

The same explicit-source rule applies when an ordinary by-value parameter would
otherwise need to consume a non-copyable reusable source. Fresh temporaries may
be forwarded without a move marker because no reusable source remains visible:

```sec
Consume(CreateResource())
```

Argument evaluation remains left-to-right. Ownership transfer for the outer
call commits only after all arguments required by that call are valid and ready.
Failure while evaluating a later argument must not prematurely consume an
earlier source for that outer call.

### Return boundaries

Returning an owned value is already an ownership-transfer boundary. Both forms
are valid:

```sec
return resource
```

```sec
return <-resource
```

The move marker is optional at a function return boundary. It must never be
required merely to restate that `return` transfers the returned value.

A fresh value received from a function may be initialized normally:

```sec
let resource := CreateResource()
```

No receiving-side move marker is required for the fresh return value.

### Ordinary methods do not consume whole `self`

Remove ordinary whole-instance consuming receiver semantics. A normal method
may require shared or mutable/exclusive receiver authority, but it may not end
the ownership of the complete instance merely by being called.

Whole-instance lifetime termination belongs to `free` and the destruction
rules.

A method with sufficient receiver authority may still move, discard, replace,
or reinitialize an owned member of `self` when ordinary partial-ownership and
borrow rules permit it:

```sec
impl Package {
    fn ReleasePayload() void {
        Destroy(<-self.Payload)
    }
}
```

After such an operation, `self` may be partially available; the method does not
thereby consume the complete receiver.

### Diagnostics

A missing call-site move marker is a semantic error. Diagnostics must name the
callee contract, identify the argument that would be consumed, and show the
canonical repair, for example `Consume(<-resource)`.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 is authoritative for availability,
move-marker requirements, delayed call-transfer commit, and method/member
ownership effects.
