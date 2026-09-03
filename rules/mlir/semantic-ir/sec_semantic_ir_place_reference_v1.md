# Semantic IR Amendment - Places and Direct Safe References

## Status

Normative amendment for:

```text
rules/compiler/semantic_ir.md
```

Package:

```text
SEC-MLIR-P15
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13
SEC-MLIR-P14
```

This amendment defines the canonical target-independent Semantic IR model for
places and direct safe references.

---

# 1. Place identity

Semantic IR must distinguish:

```text
value
storage
place
reference
raw pointer
```

A place is an addressable semantic path.

It is not a runtime numeric address.

---

# 2. Stable place roots

Places use stable compiler root identities.

Source spelling is diagnostics metadata only.

A place root may correspond to:

```text
local
parameter
static
addressed storage
allocation
declared foreign storage
other compiler-recognized storage domain
```

---

# 3. Place projections

Canonical projection information includes:

```text
stored field identity
constant index
dynamic index identity
union variant payload identity
safe-reference dereference
```

Later slice/property projections may use the same place framework.

---

# 4. Constant index precision

Constant place indexes use arbitrary precision.

Do not truncate to host `int64`.

This requirement is shared with fixed-array Semantic IR.

---

# 5. Place relationships

Semantic IR/Sema use one canonical relationship domain:

```text
Same
Disjoint
Contains
ContainedBy
PotentiallyOverlapping
Unknown
```

Borrowing and future ownership/data-race passes must consume this common result.

---

# 6. Storage identity

Every known place/root may carry a compiler storage identity.

Storage identity is distinct from:

```text
numeric address
source variable name
allocation size
epoch value
```

Address reuse does not silently reuse stale semantic identity.

---

# 7. Invalidation domain

A reference may depend on an invalidation domain.

The domain has semantic identity independent of address.

A validity epoch describes one live incarnation of that domain.

---

# 8. Reference facts

Every direct safe reference Semantic IR value may carry facts including:

```text
shared versus mutable authority
referent TypeID
possible origin Places
storage identities
address space
BorrowID
LifetimeID
validity policy
epoch dependency
relocation class
provenance/trust class
```

These may live on the value, producing operation or analysis side table.

---

# 9. Direct reference types

Semantic IR supports:

```text
ref T
ref mut T
```

as separate semantic types.

They are non-null and non-owning.

`ref T` is copyable.

`ref mut T` is move-only.

---

# 10. Physical representation separation

Semantic IR direct references are not defined as:

```text
machine address
address + generation
fat pointer
capability
side-table key
```

Those are lowering choices.

---

# 11. Validity policy

Canonical direct-reference validity policies:

```text
proven
dynamic-epoch
```

A proven reference carries proof provenance.

A dynamic reference carries the required invalidation-domain/epoch dependency.

---

# 12. Reference creation

Reference creation is explicit Semantic IR.

It consumes a valid borrowable place and produces a reference.

It records:

```text
BorrowID
LifetimeID
origin
storage identity
authority
validity policy
relocation class
```

No ownership transfer of the referent occurs.

---

# 13. Reborrowing

Reborrow is explicit.

Safe derivation may only narrow:

```text
authority
spatial extent
lifetime
```

It must preserve compatible provenance, address space and epoch dependency.

Shared may not become mutable.

---

# 14. Place dereference

Dereferencing a safe reference produces a semantic place.

Shared reference:

```text
read-only place
```

Mutable reference:

```text
read/write place
```

For dynamic validity, the operation is guarded by the same reference validity
predicate.

---

# 15. Aggregate subplaces

Struct fields and fixed-array elements can be borrowed without copying the whole
aggregate.

The place path records the aggregate projection.

Physical offsets are not selected.

---

# 16. Fixed-array place bounds

Dynamic fixed-array element places require the same bounds semantics as P14.

The place bounds predicate and element-place construction operate on one
evaluated array place and one evaluated index.

---

# 17. Union payload place

Union payload place formation is valid only under the active-variant proof for
the same union semantic storage.

---

# 18. Reference whole-value read

A whole-value read through a reference is explicit.

P15 executable support is limited to copy-trivial T.

No silent move from shared reference.

---

# 19. Reference write

Write through mutable reference is replacement.

P15 supports only trivially replaceable T.

Non-trivial destruction/reinitialization remains deferred.

---

# 20. Shared reference copy

Semantic IR may explicitly represent a shared reference copy.

The copy preserves non-owning referent identity and compatible validity facts.

---

# 21. Mutable reference move

Semantic IR explicitly preserves move-only reference transfer.

Moving `ref mut` transfers holder/borrow obligation, not referent ownership.

---

# 22. Borrow lifetime end

Semantic IR represents resolved borrow end explicitly or through equivalent
canonical lifetime metadata.

The operation is compile-time semantic structure, not runtime borrow tracking.

---

# 23. Dynamic reference validity

Semantic IR provides a total validity predicate.

A dynamic validity use is guarded.

The false path of ordinary direct safe-reference use reaches the canonical
invalid-reference-generation failure endpoint.

---

# 24. Ordinary stale failure

Ordinary direct reference stale failure:

```text
does not return Result
does not use try
does not resume
```

It reaches panic/trap semantics with:

```text
panic.invalid-reference-generation
```

---

# 25. Reference equality

Semantic reference equality compares:

```text
same live storage identity
same referenced location
```

It is not raw address equality.

Dynamic references must satisfy their required validity before comparison.

---

# 26. Alternative origins

A reference may have a finite compile-time set of possible origins after CFG
merge.

This does not imply a runtime provenance tag.

Reborrow requiring precise ownership provenance may be rejected when the origin
set becomes unknown.

---

# 27. Returned-reference summary

Function Semantic IR metadata preserves enough origin information to instantiate
returned-reference facts at call sites.

Returning a local-origin direct reference remains invalid.

---

# 28. Invalidation metadata

Semantic IR establishes common identities/facts for future invalidation events:

```text
destroy
free
arena reset/release
collection storage replacement
relocation
foreign invalidation
```

Future packages must reuse this model.

---

# 29. Epoch width

The compilation plan records logical epoch policy.

Default logical width is 64 bits.

The reference source type does not include epoch width.

---

# 30. No runtime borrow checking

Borrow compatibility remains compile-time analysis.

No Semantic IR operation introduced here requires:

```text
runtime borrow counter
runtime owner lock
runtime alias tag
```

---

# 31. Verifier requirements

Semantic IR verifier must validate:

```text
place root/projection type consistency
authority does not increase
field/array/union projections are semantically valid
array element place is bounds-proven/guarded
union payload place is variant-proven
ref shared/mut authority
ref mut is never copied
reborrow authority narrowing
dynamic validity guard structure
reference compare compatibility
BorrowID/LifetimeID consistency
no use after resolved borrow end
returned-reference origin validity
no place escapes as a language value
```

---

# 32. Deterministic printer

Print:

```text
stable PlaceID/root ID
structured projection path
arbitrary-precision constant indexes
reference kind
BorrowID/LifetimeID
validity policy
epoch dependency identity
relocation class
```

Do not make source display spelling the semantic identifier.
