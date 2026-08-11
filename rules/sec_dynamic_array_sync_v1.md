# Package 18 Normative Synchronization - Owning Dynamic Arrays

## Status

Normative synchronization for:

```text
rules/collections.md
rules/default_values.md
rules/copy_move.md
rules/ownership.md
rules/core-library.md
rules/layout.md
rules/storage.md
rules/allocation.txt
```

Package:

```text
SEC-MLIR-P18
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13
SEC-MLIR-P14
SEC-MLIR-P15
SEC-MLIR-P16
SEC-MLIR-P17
```

This synchronization makes the ownership/default/storage implications of the
already-valid source type `T[]` explicit.

---

# 1. `T[]` is an owning value type

Canonical:

```sec
T[]
```

is a runtime-sized owning sequence descriptor.

It is distinct from:

```sec
T[N]
ref T[]
ref mut T[]
list[T]
```

---

# 2. `T[]` is MoveOnly

Canonical copy classification:

```text
T[] -> MoveOnly
```

This does not depend on T.

Reason:

```text
shallow descriptor copy would duplicate ownership
deep copy may allocate
implicit semantic copy must not allocate or fail
```

A future named duplication operation may provide explicit deep-copy semantics.

---

# 3. Empty default

`T[]` is defaultable through a canonical initialized empty state:

```text
length 0
capacity 0
no backing storage
no initialized elements
no dynamic allocation
```

The default is valid independently of T defaultability because no T object is
constructed.

---

# 4. Mutable declaration without initializer

Valid:

```sec
let mut values: byte[]
```

It receives the canonical empty default.

---

# 5. Immutable declaration correction

The existing illustrative:

```sec
let values: byte[]
```

must not be treated as a valid executable immutable declaration without an
initializer.

The general binding rule remains:

```text
immutable binding requires explicit initializer
```

Use a mutable declaration or an explicitly initialized owning-array expression
in examples.

---

# 6. No hidden allocation for default

The empty default allocates nothing.

This preserves the general rule:

```text
implicit default initialization must not dynamically allocate
```

A noalloc target/profile can therefore still represent empty `T[]`.

---

# 7. No implicit copy

These operations do not deep-copy `T[]`:

```text
assignment
parameter passing
return
branch transfer
aggregate construction
```

They use move semantics unless the source operation explicitly constructs a new
owner.

---

# 8. Capacity is not public

The owning descriptor may conceptually maintain capacity.

Current array/slice source rules expose no `capacity` member.

P18 keeps capacity compiler/internal.

Do not add:

```sec
values.capacity
values.Capacity()
```

through this synchronization.

---

# 9. Canonical public API

The approved public surface is:

```text
Append
Clear
RemoveAt
ToString
Len
IsEmpty
Ptr
SizeOf
```

for `T[]`. P18 may define the internal primitives needed to implement these
operations.

Do not add `Push`, `Reserve`, `Resize`, `Capacity`, `AsSlice`,
`AsMutableSlice`, or `Slice`.

Those internal primitives are not source members.

---

# 10. Length

Existing rules remain:

```text
T[].Len -> uint
len(T[]) -> int
```

The unresolved `uint`-to-`int` representability case remains the same issue
already documented for slices.

No silent narrowing is permitted.

---

# 11. Element ownership

The owning array owns exactly the initialized logical elements:

```text
indexes [0, length)
```

Capacity slots beyond length contain no live T object.

They are never safe-readable merely because storage exists.

---

# 12. Indexed move-out remains restricted

The existing initial rule remains:

```text
moving a move-only element out through runtime indexing is invalid
```

Borrow the element instead.

P18 does not add dynamic-index partial-move state.

---

# 13. Destruction

Destroying `T[]`:

```text
destroys initialized elements in reverse index order
ends its live backing invalidation domain
reclaims backing bytes only when the resolved reclamation authority belongs to
the owning value
```

Arena-backed bytes are reclaimed by Arena policy, not individually by `T[]`.

---

# 14. Value ownership and backing reclamation are separate

Canonical storage principle:

```text
dynamic array owns element object lifetimes
```

does not imply:

```text
dynamic array always individually frees the raw bytes
```

Use `BackingRelation` and `ReclamationAuthority`.

---

# 15. Allocation is explicit through the producer

A non-empty owning-array operation that creates new backing storage is
allocation-capable.

It must have:

```text
allocation context
AllocationError behavior
storage origin
reclamation strategy
```

explicit before backend lowering.

---

# 16. Allocation failure

Potentially failing owning-array allocation returns/uses:

```text
AllocationError
```

through ordinary Result/check flow.

No null return, allocation panic, or silent shortening.

---

# 17. Structural mutation and backing invalidation

Canonical storage transitions:

```text
element mutation in place:
    no backing transition

capacity growth without invalidation:
    no invalidating transition

reallocation preserving logical backing identity:
    AdvanceEpoch

logical backing replacement:
    EndDomain(old) + EstablishDomain(new)

owner destruction:
    EndDomain
```

---

# 18. Direct references and slices

Element references and slices derived from `T[]` depend on the current backing
incarnation.

A structural mutation that may invalidate them is not legal while conflicting
direct dependencies are live.

No runtime borrow checker is introduced.

---

# 19. Explicit slices from owning arrays

P16's temporary owning-source limitation is removed.

Valid explicit source forms include:

```sec
ref values[..]
ref mut values[..]
ref values[start..<end]
```

when ordinary mutability/borrow/range rules are satisfied.

---

# 20. No FFI descriptor guarantee

`T[]` has no FFI-stable physical descriptor layout.

Foreign wrappers must expose explicit compatible components/contracts.

---

# 21. Core materialization

P18 provides the owning-storage substrate needed by compiler/core operations
returning types such as:

```text
byte[]
rune[]
```

It does not by itself finalize unresolved public conversion signatures.

---

# 22. Required synchronization

Update:

```text
arrays-slices owning T[] copy/default/destruction text
default_values dynamic owning-array default
copy_move/ownership T[] classification
core intrinsic array length/member metadata
storage transition integration
Semantic IR docs
tests and derived manuals
```
