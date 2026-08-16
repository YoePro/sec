# Collections

**Created:** 2026-08-11  
**Last updated:** 2026-08-11  
**Document revision:** 1  
**Language version:** Sec 0.1  
**Status:** Draft for Sec 0.1

## Purpose

This rulebook defines Sec collection semantics for:

```text
T[N]
T[]
ref T[]
ref mut T[]

list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]
```

It replaces the collection semantics previously split between:

```text
arrays-slices.txt
collections-shaped-types.md
```

Shaped-value semantics belong to `shaped-types.md`.

This rulebook preserves still-valid array and slice semantics from
`arrays-slices.txt` and synchronizes them with later Sec 0.1 decisions,
including P14-P18 Semantic IR/MLIR work.

Implementation status does not belong in this rulebook.

---

# 1. Collection design principles

Sec has a small first-class collection set whose semantics benefit materially
from compiler knowledge.

The collection hierarchy is intentionally simple:

```text
T[N]
    fixed-size owning contiguous sequence

T[]
    basic dynamic owning contiguous sequence

ref T[]
ref mut T[]
    borrowed contiguous sequence views

list[T]
    richer dynamic ordered contiguous collection

map[K, V]
    associative key/value collection

set[T]
    associative unique-value collection
```

`T[]` is deliberately useful without becoming a complete high-level collection
framework.

`list[T]` is the richer ordered collection.

More specialized data structures belong in libraries, for example:

```text
LinkedList[T]
OrderedMap[K, V]
OrderedSet[T]
```

The standard library may provide free generic algorithms over collection
interfaces or accepted collection categories.

The standard library does not gain privileged permission to add arbitrary
members to compiler-owned fundamental collection types.

Compiler-owned public members are defined by the language/compiler-known core
surface.

---

# 2. Compiler-known collection type constructors

The following lowercase constructors are compiler-known and require no import:

```text
list
map
set
```

Arrays and slices use language type syntax rather than named constructors:

```text
T[N]
T[]
ref T[]
ref mut T[]
```

Compiler-known collection names are not standard-library aliases.

---

# 3. Shared collection properties

Collection state that is naturally observed rather than executed is exposed as
a property, not a method.

The canonical property spelling uses an initial uppercase letter.

Do not use legacy spellings such as:

```text
.len
.ptr
IsEmpty()
```

when the canonical property is:

```text
.Len
.Ptr
.IsEmpty
```

## 3.1 `Len`

`Len` is a read-only property.

For all collections and slices in this rulebook:

```text
Len means logical element or entry count.
Len never means payload size in bytes.
```

The result type is:

```text
uint
```

Examples:

```sec
values.Len
view.Len
users.Len
activeUsers.Len
```

For `T[N]`, `Len` is the compile-time constant `N`.

For `T[]`, `ref T[]`, `ref mut T[]`, and `list[T]`, `Len` is the current
number of live logical elements.

For `map[K, V]`, `Len` is the current number of stored entries.

For `set[T]`, `Len` is the current number of stored unique values.

The compiler-known global:

```sec
len(value)
```

may remain available where the core member rules define a signed `int` result
for index arithmetic.

`len(value)` and `value.Len` describe the same logical count.

## 3.2 `IsEmpty`

`IsEmpty` is a read-only `bool` property.

It is equivalent to:

```text
Len == 0
```

but is exposed as a direct semantic observation.

Examples:

```sec
if values.IsEmpty {
    return
}
```

For `T[N]`, `IsEmpty` is compile-time constant.

## 3.3 `Ptr`

`Ptr` is a read-only unsafe property only for collection/view types whose
source contract guarantees contiguous addressable element storage.

In this rulebook it is available on:

```text
T[N]
T[]
ref T[]
ref mut T[]
list[T]
```

It is not available on:

```text
map[K, V]
set[T]
```

`Ptr` does not:

```text
transfer ownership
extend lifetime
pin storage
prevent later invalidation
make a safe reference
make an FFI descriptor stable
```

The result is governed by the raw-pointer and FFI rules.

A pointer obtained from relocatable storage becomes invalid when the owning
storage transition invalidates that backing incarnation.

For an empty sequence, the pointer value is implementation-defined within the
raw-pointer rules and must not be dereferenced.

## 3.4 Instance `SizeOf`

For contiguous sequence instances in this rulebook:

```text
value.SizeOf
```

is a read-only `uint` property describing the byte extent of the represented
logical payload.

It is not the descriptor size.

Conceptually:

```text
payload byte size = Len * element layout stride
```

The multiplication is not revalidated as an ordinary runtime arithmetic
expression when observing an already valid collection value. Storage creation,
growth, layout, and slicing must already have established a representable
payload extent.

Examples:

```sec
let values: int32[10]

values.Len       // 10
values.SizeOf    // payload byte extent
```

For a dynamic array or slice:

```sec
buffer.Ptr
buffer.SizeOf
```

form the ordinary explicit pointer/byte-count pair used by many FFI wrappers.

Associated type layout remains a different query:

```sec
T.SizeOf
SizeOf(T)
```

Those forms describe the physical layout size of one value/type `T` according
to the layout rulebook.

The exact general `SizeOf` rules are owned by
`compiler_known_members.md`.

---

# 4. `ToString`

Collection types may expose:

```sec
value.ToString()
value.ToString(format)
```

according to the common Sec formatting and compiler-known-member rules.

This rulebook does not define the general formatting grammar.

For ordinary array/slice collection values, the no-argument representation is
type-oriented rather than an implicit dump of all data.

Canonical examples:

```text
int[4].ToString()      -> "int[4]"
int[].ToString()       -> "int[]"
ref int[].ToString()   -> "ref int[]"
```

Exact `char[]` and `rune[]` sequence conversions are intentional special cases
defined by the compiler-known string conversion rules: their no-argument
`ToString()` may materialize the represented text.

A data-revealing formatting overload must borrow rather than consume the
sequence.

The common formatting rulebook owns the exact format strings and overload set.

---

# 5. Fixed arrays `T[N]`

## 5.1 Type syntax and identity

A fixed-size array uses postfix syntax:

```sec
T[N]
```

Examples:

```sec
byte[256]
int[4]
Packet[32]
```

`N` is the number of elements.

Array length is part of type identity.

These are distinct:

```text
int[4]
int[5]
uint[4]
```

`N` must be a non-negative compile-time integer representable by `uint`.

The compiler must reject:

```text
negative lengths
non-integer lengths
runtime-dependent lengths
constant-evaluation overflow
lengths not representable by uint
total layouts exceeding target layout limits
```

Zero-length arrays are valid.

## 5.2 Layout

A fixed array owns `N` contiguous elements of `T`.

The fixed array itself contains no hidden runtime:

```text
pointer
length field
capacity field
allocator identity
```

Its length comes from the type.

`T` must be sized.

The complete layout uses checked layout arithmetic.

## 5.3 Nested arrays

Postfix dimensions are written from outermost to innermost.

```sec
int[3][4]
```

means three outer elements, each of type:

```text
int[4]
```

Every dimension participates in type identity and layout checking.

## 5.4 Default construction

A mutable fixed array may omit its initializer only when every element can be
validly default-constructed.

Example:

```sec
let mut values: int[4]
```

constructs four valid default `int` elements.

A zero-length array is defaultable without constructing an element.

Partial undefined element storage must never become a readable safe array.

Construction of non-trivial elements tracks cleanup for any already initialized
prefix if later construction fails.

An immutable array requires an initializer.

## 5.5 Array literals

Array literal syntax:

```sec
[expression, expression, expression]
```

A trailing comma is allowed.

Target context provides element type and required length:

```sec
let values: byte[3] := [1, 2, 3]
let percentages: Percent[2] := [10, 20]
```

When no target type exists, the compiler may infer `T[N]` when:

```text
the literal contains at least one element
every element resolves to the same type
N equals the literal element count
```

Example:

```sec
let values := [10, 20, 30]
```

infers:

```text
int[3]
```

An empty literal requires target context:

```sec
let empty: int[0] := []
```

## 5.6 Spread in fixed-array literals

Spread is valid according to the canonical spread rules.

Example shape:

```sec
let all: int[6] := [1, ...middle, 6]
```

Multiple fixed-array spreads are allowed when the total result length is
compile-time known and exactly matches the target array length.

Spread does not create dynamic-length fixed arrays.

## 5.7 Fixed-array public surface

Read-only properties:

```text
Len
IsEmpty
Ptr
SizeOf
```

Compiler-known methods may include:

```text
ToString()
ToString(format)
```

There is no required:

```text
AsSlice()
AsMutableSlice()
Slice()
First()
Last()
```

Slice creation uses explicit `ref` or `ref mut` syntax.

Indexing, slicing, iteration, and equality are language operations rather than
methods.

---

# 6. Owning dynamic arrays `T[]`

## 6.1 Meaning

A dynamically sized owning array uses:

```sec
T[]
```

It is a first-class owning value type.

It is distinct from:

```text
T[N]
ref T[]
ref mut T[]
list[T]
```

Its length is runtime state and is not part of type identity.

Every `T[]` is:

```text
MoveOnly
non-trivially destructible
owning its live logical elements
```

regardless of whether `T` itself is copyable.

Ordinary assignment, parameter transfer, return, and aggregate transfer do not
implicitly deep-copy `T[]`.

## 6.2 Empty default

A mutable `T[]` may use the canonical empty default.

The empty default has:

```text
Len = 0
internal capacity = 0
backing storage = none
allocation = none
live elements = none
```

It is valid independently of whether `T` is defaultable.

Example:

```sec
let mut values: int[]
```

is an initialized empty owner.

An immutable declaration still requires an initializer.

## 6.3 Internal capacity

A dynamic array may maintain compiler/internal capacity.

Capacity is not public source state.

Do not expose:

```text
Capacity
capacity
Reserve
Resize
```

as part of the Sec 0.1 dynamic-array API.

Exact growth factor and allocator over-allocation are not source semantics.

The fundamental invariant is:

```text
0 <= Len <= internal capacity
live initialized elements = [0, Len)
capacity-only slots contain no live T
```

## 6.4 Allocation and growth

A non-empty operation that requires new backing storage is allocation-capable.

It must have a resolved allocation context before backend lowering.

No hidden global heap is assumed.

Growth is transactional.

On growth failure:

```text
the original array remains valid
Len is unchanged
all existing live elements remain owned by the original array
no new backing becomes observable
```

Internal allocator failures remain `AllocationError` at the allocation layer.

The public collection operation maps resource/growth failure to the
collection-facing failure type defined in this rulebook.

## 6.5 Storage identity and invalidation

Direct references and slices derived from `T[]` depend on the current backing
incarnation.

Ordinary element mutation in place does not invalidate backing identity.

A backing relocation may:

```text
advance the storage epoch
or
replace the backing domain
```

according to the storage/reference rules.

A structural mutation that may invalidate an active direct dependency is
illegal while a conflicting reference or slice is live.

No runtime borrow checker is introduced.

## 6.6 Dynamic-array public surface

Read-only properties:

```text
Len
IsEmpty
Ptr
SizeOf
```

Methods:

```text
ToString()
ToString(format)

Append(value)
Clear()
RemoveAt(index)
```

No required methods:

```text
Push()
Reserve()
Resize()
Capacity()
AsSlice()
AsMutableSlice()
Slice()
First()
Last()
```

`Push` is not an alias for `Append`.

## 6.7 `Append`

Canonical semantic signature:

```text
Append(value) -> Result[void, CollectionError]
```

`Append(value)` appends one element at logical index:

```text
Len
```

and increases `Len` by one only after successful element transfer and any
required growth.

The operation may require allocation.

Source-level failure uses:

```text
CollectionError
```

Examples:

```sec
try values.Append(value)
```

or a local handler:

```sec
try values.Append(value) {
    Err(error) => Handle(error)
}
```

`Append` must be atomic with respect to collection validity.

The element is not lost on allocation failure.

A structural growth that relocates backing follows the ordinary reference/slice
invalidation rules.

## 6.8 `Clear`

`Clear()`:

```text
destroys every live element
uses reverse logical index destruction
sets Len to 0
retains reusable backing storage when present
does not allocate
does not expose capacity
```

Example:

```sec
values.Clear()
```

`Clear()` is infallible under ordinary destruction rules.

## 6.9 `RemoveAt`

Canonical semantic signature:

```text
RemoveAt(index) -> Option[T]
```

The index accepts the same integer index categories as ordinary `[]` indexing:

```text
signed integer index
unsigned integer index
compatible named integer index
```

Negative signed indexes are unsuccessful.

An invalid index does not produce an Error.

It returns:

```text
None
```

A valid index:

```text
moves the removed T out
shifts every later live element one logical slot to the left
decrements Len by one
leaves the former last slot uninitialized
does not default-construct a replacement
does not allocate
returns Some(removed)
```

Example:

```sec
let removed := values.RemoveAt(index)
```

Ignoring the returned `Option[T]` follows normal discard/destruction semantics.

`RemoveAt` never leaves a readable hole in `[0, Len)`.

For:

```text
[A B C D E F G H]
```

removing index `5` produces:

```text
[A B C D E G H]
```

and returns `Some(F)`.

This order-preserving compaction is intentionally O(number of later elements).

`T[]` is not a sparse collection.

---

# 7. Slices

## 7.1 Slice types

A slice is a non-owning borrowed view of contiguous initialized elements.

Canonical source types:

```sec
ref T[]
ref mut T[]
```

A slice:

```text
does not own elements
does not own capacity
does not choose an allocator
does not deallocate backing storage
does not implicitly copy elements
```

Its physical representation is compiler-defined and is not an FFI-stable
struct layout.

## 7.2 Slice sources

A slice may borrow any source that guarantees a contiguous, unit-stride,
initialized element range of exact element type `T` and whose lifetime and
relocation rules can preserve the borrow.

Canonical Sec 0.1 sources include:

```text
T[N]
T[]
list[T]
ref T[]
ref mut T[]
```

A slice is not a general view over:

```text
map[K, V]
set[T]
```

Shaped views use `tensor_view` and the shaped-type rules where appropriate.

## 7.3 Explicit slice creation

Slice creation is explicit.

Shared:

```sec
let view := ref values[..]
```

Mutable:

```sec
let writable := ref mut values[..]
```

Subrange:

```sec
let middle := ref values[1..<4]
let inclusive := ref values[1..3]
```

There is no required `AsSlice()` or `Slice()` method.

There is no implicit owner-to-slice conversion.

A `ref mut T[]` requires mutable borrowing authority over the source.

A shared slice may be borrowed from mutable storage without granting mutation.

## 7.4 Ranges

Slice ranges use Sec range syntax.

Exclusive upper bound:

```sec
source[start..<end]
```

Inclusive upper bound:

```sec
source[start..end]
```

Open forms:

```sec
source[..]
source[start..]
source[..end]
source[..<end]
```

Descending ranges are invalid.

A slice range does not reverse a sequence.

Empty slices are valid.

Semantic analysis normalizes valid ranges to:

```text
[start, endExclusive)
```

for borrow and bounds reasoning.

Negative endpoints are invalid.

## 7.5 Fallible slicing

Compile-time invalid constant ranges are compile-time errors.

Dynamic checked slicing may use the canonical fallible range path.

The owning range/index rule defines the exact `RangeError` cases.

Safe bounds checking remains required inside `unsafe`.

Unchecked traversal belongs to raw-pointer operations.

## 7.6 Reborrowing

A slice may be sliced again.

Shared reborrow:

```sec
let part := ref view[2..<5]
```

Mutable reborrow:

```sec
let part := ref mut writable[2..<5]
```

A mutable reborrow requires mutable source authority.

The result lifetime cannot exceed:

```text
the source slice lifetime
the ultimate storage-owner lifetime
the current backing/storage epoch
```

## 7.7 Slice public surface

Shared slice properties:

```text
Len
IsEmpty
Ptr
SizeOf
```

Shared slice methods may include:

```text
ToString()
ToString(format)
```

Mutable slices expose the same observations and additionally:

```text
Reverse()
Fill(value)
```

Slices do not expose structural mutation:

```text
Append
Insert
RemoveAt
Remove
Clear
Reserve
Resize
```

A slice cannot change the owner's collection length.

## 7.8 `Reverse`

`Reverse()` is valid on:

```text
ref mut T[]
```

It reverses the represented elements in place.

It:

```text
does not allocate
does not change Len
does not change storage identity
does not require T to be Copy
does not consume the slice
```

The compiler/core implementation may use move-based swap semantics.

`Reverse()` works for move-only `T`.

## 7.9 Slice `Fill`

`Fill(value)` is valid on:

```text
ref mut T[]
```

when `T` supports ordinary infallible semantic copying.

The operation:

```text
evaluates value exactly once
uses the value as the source for each replacement
visits logical elements in increasing index order
replaces every represented element
destroys each replaced owned value exactly once through normal replacement semantics
does not allocate
does not change Len
does not change storage identity
```

`Fill(value)` is unavailable when `T` is move-only or otherwise cannot be
repeatedly copied under the ordinary copy rules.

Member availability depends on `T`, not on runtime slice length.

---

# 8. Indexing

## 8.1 General rule

Indexing syntax is:

```sec
target[index]
```

For linear sequence types, indexing produces an element Place.

It does not inherently:

```text
copy
move
borrow
```

Use context determines the operation.

Examples:

```sec
let value := values[index]
values[index] = value
let element := ref values[index]
let element := ref mut values[index]
```

## 8.2 Valid index types

Valid index categories include:

```text
built-in signed integers
built-in unsigned integers
compatible named integer types without incompatible unit dimensions
```

Invalid categories include:

```text
floating point
decimal
bool
string
unit-bearing non-index values
```

Negative signed indexes are invalid/out of bounds.

## 8.3 Bounds

For `T[N]`:

```text
0 <= index < N
```

For `T[]`, slices, and `list[T]`:

```text
0 <= index < Len
```

A statically known invalid index is a compile-time error.

A dynamic index uses the normal checked access path.

The fallible access form uses the canonical `IndexError` family.

## 8.4 Replacement

Indexed assignment requires:

```text
mutable element Place
assignable replacement value
no conflicting borrow
valid index
fully evaluated replacement before old-value destruction
```

The old owned value is destroyed exactly once after the replacement is ready
and ownership transfer is valid.

## 8.5 Move-only indexed reads

Ordinary runtime indexing does not move a move-only element out of an owning
collection.

Borrow the element instead.

Dynamic-array `RemoveAt` is the explicit owning extraction operation defined by
this rulebook.

A slice never transfers ownership merely through indexing.

---

# 9. Iteration over linear sequences

Fixed arrays, dynamic arrays, slices, and lists participate in `for` iteration.

Iteration must preserve:

```text
index order
copy/move rules
borrow rules
element ownership
```

Iteration does not implicitly move move-only elements out of the collection.

Structural mutation that can invalidate the iterator's storage/range is not
permitted while the iteration borrow is active.

---

# 10. Equality

Fixed arrays support:

```text
==
!=
```

when `T` supports equality.

Comparison is element-by-element in increasing index order.

Both operands must have the same fixed-array type, including the same length.

Slices do not use ordinary `==`/`!=` for content equality because reference
identity and content equality are distinct concepts.

Dynamic-array/list/set/map content equality is not implied merely by their
storage representation.

Any public content-equality operation must be explicitly defined by the
relevant collection/core rules.

---

# 11. Function parameters and return values

A by-value fixed array follows normal copy/move classification derived from `T`.

A by-value `T[]` transfers the owning dynamic array by move.

A slice parameter borrows the source.

Examples:

```sec
fn Sum(values: ref int[]) int
fn ClearBytes(values: ref mut byte[]) void
fn Consume(values: int[4]) void
fn ConsumeOwned(values: Packet[]) void
```

Slice creation remains explicit at the call site unless another rule
specifically introduces a borrowing conversion.

Returned slices must refer to storage that remains valid after return.

A function must not return a slice into destroyed local storage.

---

# 12. Destruction

## 12.1 Fixed arrays

Destroy live elements in reverse index order.

## 12.2 Dynamic arrays

Destroy live elements in reverse logical index order.

Then:

```text
end the active backing invalidation domain
reclaim backing only through the resolved reclamation authority/plan
```

An Arena-backed dynamic array still owns element object lifetimes while Arena
policy owns raw backing reclamation.

Value ownership and raw backing reclamation are separate concerns.

## 12.3 Slices

Destroying a slice:

```text
destroys no element
reclaims no backing
ends only the reference/borrow value
```

## 12.4 Lists, maps, and sets

Owning collection destruction destroys all still-owned live elements/entries
exactly once according to the collection's defined traversal/destruction plan,
then performs backing reclamation according to its allocation/reclamation
authority.

Backend container implementation must not change source destruction semantics.

---

# 13. `list[T]`

## 13.1 Meaning

`list[T]` is a variable-length ordered linear collection.

Sec 0.1 guarantees contiguous logical element storage for the public list
contract.

This guarantee permits:

```text
indexing
explicit slices
Ptr
SizeOf
ordered iteration
```

`list[T]` is not a linked list.

## 13.2 Dynamic and bounded forms

Dynamic:

```sec
list[T]
```

may grow through an approved allocation strategy.

Bounded:

```sec
list[T, Capacity]
```

has runtime-variable `Len` and compile-time maximum element capacity.

A bounded list:

```text
never exceeds Capacity
performs no hidden growth allocation beyond its resolved bounded backing model
reports capacity exhaustion explicitly
is suitable for no-heap targets
```

`Capacity` must be greater than zero.

The exact physical representation is not source semantics.

## 13.3 Empty default

Both forms are defaultable independently of whether `T` is defaultable.

For dynamic `list[T]`, the empty state has:

```text
Len = 0
Capacity = 0
element storage = none
live elements = none
allocation = none
```

For bounded `list[T, Capacity]`, the empty state has:

```text
Len = 0
maximum capacity = Capacity
initialized elements = none
hidden growth allocation = none
```

Reserved bounded backing does not make any `T` live and does not default-
construct an element.

The canonical explicit empty forms are:

```sec
list[T] {}
list[T, Capacity] {}
```

They are compiler-known collection literals, not struct literals. The parser
and AST must preserve enough information for Sema to resolve that category
without routing it exclusively through named-struct construction.

## 13.4 List properties

Read-only properties:

```text
Len
IsEmpty
Ptr
SizeOf
Capacity
```

For `list[T, Capacity]`, `Capacity` is the compile-time maximum capacity exposed
as a read-only property.

For dynamic `list[T]`, `Capacity` is the currently available logical capacity
according to the list contract.

Unlike `T[]`, list capacity is intentionally source-visible.

## 13.5 Core list methods

The Sec 0.1 list surface includes:

```text
Append(value)
Insert(index, value)
RemoveAt(index)
Remove(value)
Clear()

Contains(value)
IndexOf(value)

Reverse()
Sort()
SortBy(compare)

ToString()
ToString(format)
```

Higher-level transformation algorithms may be supplied as free library
algorithms.

The standard library does not extend `list[T]` by adding hidden privileged
members.

## 13.6 `RemoveAt`

List `RemoveAt` follows the same order-preserving semantic model as dynamic
array `RemoveAt`:

```text
valid index -> Some(removed T)
invalid index -> None
later elements shift left
Len decrements
no hole remains
```

## 13.7 `Remove`

`Remove(value)` removes one matching value according to the list equality rules.

Its result is:

```text
bool
```

`true` means a value was removed.

`false` means no matching value existed.

Absence is not an Error.

## 13.8 `Contains`

`Contains(value)` returns `bool`.

It requires equality semantics for `T`.

## 13.9 `IndexOf`

`IndexOf(value)` returns:

```text
Option[uint]
```

The first matching logical index is returned.

No match returns `None`.

No Error is produced merely because the value is absent.

## 13.10 `Insert`

Canonical semantic signature:

```text
Insert(index, value) -> Result[bool, CollectionError]
```

`Insert(index, value)` is structural mutation.

Result meaning:

```text
Ok(true)
    insertion succeeded

Ok(false)
    the insertion position is invalid

Err(error)
    a valid insertion could not be completed because of a structural/resource
    failure
```

An invalid insertion position is an ordinary unsuccessful input and does not
become an `IndexError` merely because failure is possible.

Resource/growth failure uses `CollectionError`.

## 13.11 Sorting

`Reverse()` mutates in place and does not allocate.

`Sort()` is available when `T` satisfies the required ordering contract.

`SortBy(compare)` accepts the canonical comparison callable form defined by the
function/lambda rules.

Sorting must not silently copy move-only elements merely because an
implementation algorithm was written in copy-oriented form.

---

# 14. `map[K, V]`

## 14.1 Meaning

`map[K, V]` stores unique keys associated with values.

It provides:

```text
lookup
insertion
replacement
removal
iteration
owned key/value storage
stable key equality/hash requirements while stored
explicit allocation/capacity semantics
```

Dynamic:

```sec
map[K, V]
```

may grow through an approved allocation strategy.

Bounded:

```sec
map[K, V, Capacity]
```

has a compile-time maximum entry count.

No hidden global heap is assumed.

## 14.2 Representation and ordering

Hash-based behavior is the default semantic implementation family.

The physical representation is not source-visible.

The compiler may choose an implementation that preserves the required map
semantics.

Map iteration order is unspecified.

Programs must not depend on:

```text
insertion order
bucket order
sorted order
target-specific order
```

Ordered associative behavior belongs in a distinct type.

## 14.3 Map properties

Read-only properties:

```text
Len
IsEmpty
```

No required:

```text
Ptr
First
Last
```

`First` and `Last` would make an otherwise unspecified iteration order
observable and are therefore not part of the map base surface.

## 14.4 Indexed lookup

Canonical syntax:

```sec
users[key]
```

means lookup of the entry associated with `key`.

A successful lookup produces the value Place for the existing entry.

Normal read/borrow context then determines whether the value is copied,
borrowed, or mutably borrowed.

A missing key is a checked access failure using the canonical key-access error
family.

A fallible access form may use `try` according to the ordinary fallible-access
and bodyless-try rules.

A lookup does not insert.

## 14.5 Indexed assignment

Map mutation uses ordinary indexed assignment syntax.

Do not add a `.Set()` method.

Canonical upsert:

```sec
try users[id] = user
```

Semantics:

```text
existing key:
    replace V

missing key:
    insert K/V entry
```

If the operation requires structural growth and that growth cannot be
performed, it fails with the collection-facing structural failure type.

The operation does not use `:=`.

`:=` remains declaration/initialization syntax.

Explicit move assignment may use the canonical move assignment operator where
the general assignment rules permit it:

```sec
try users[id] <- user
```

The exact move behavior remains governed by the ownership and assignment rules.

## 14.6 Map iteration

Canonical form:

```sec
for key, value in users {
    ...
}
```

Iteration is over entries.

The iteration order is unspecified.

Ordinary iteration must not implicitly move owned keys or values out of the
map.

Stored keys must not be mutated in a way that changes equality/hash identity
while stored.

Structural mutation of the map while a conflicting iteration borrow is active
is invalid.

## 14.7 Core map operations

The core map surface includes:

```text
indexed lookup
indexed upsert assignment
Remove(key)
ContainsKey(key)
Clear()
iteration

ToString()
ToString(format)
```

Canonical removal result:

```text
Remove(key) -> Option[V]
```

`Some(value)` transfers ownership of the removed value.

`None` means the key was absent.

Absence is not an Error.

`ContainsKey(key)` returns:

```text
bool
```

Indexed upsert assignment has the operation-level failure shape:

```text
Result[void, CollectionError]
```

when structural growth/resource failure is possible.

---

# 15. `set[T]`

## 15.1 Meaning

`set[T]` stores unique values.

It provides:

```text
insertion
removal
membership
iteration
set operations
explicit allocation/capacity behavior
```

Dynamic:

```sec
set[T]
```

may grow through an approved allocation strategy.

Bounded:

```sec
set[T, Capacity]
```

has a compile-time maximum value count.

No iteration order is guaranteed unless a separate ordered set type defines it.

## 15.2 Set properties

Read-only properties:

```text
Len
IsEmpty
```

No required:

```text
Ptr
First
Last
```

## 15.3 Core set methods

The Sec 0.1 set surface includes:

```text
Add(value)
Remove(value)
Contains(value)
Clear()

Union(other)
Intersection(other)
Difference(other)
SymmetricDifference(other)

ToString()
ToString(format)
```

Canonical insertion shape:

```text
Add(value) -> Result[bool, CollectionError]
```

Result meaning:

```text
Ok(true)
    value was inserted

Ok(false)
    value was already present

Err(error)
    a valid insertion could not be completed because of structural/resource
    failure
```

An already present value is not an Error.

`Remove(value)` returns:

```text
bool
```

`false` means the value was absent.

`Contains(value)` returns:

```text
bool
```

The non-mutating set-algebra methods:

```text
Union(other)
Intersection(other)
Difference(other)
SymmetricDifference(other)
```

produce a new owning set and therefore require `T` to support the ordinary
non-consuming copy semantics required to populate the result.

For a move-only `T`, these non-consuming set-algebra members are unavailable;
the programmer must use explicit ownership-moving logic instead.

When result construction is allocation/resource capable, the set-algebra
operation returns its resulting set through:

```text
Result[set[...], CollectionError]
```

with the concrete bounded/dynamic set type preserved as defined by the receiver
and operation rules.

No hidden element copying or hidden allocation is permitted.

## 15.4 Set iteration

Canonical form:

```sec
for value in activeUsers {
    ...
}
```

Iteration order is unspecified.

Ordinary iteration must not implicitly move an owned set element out.

Structural mutation during a conflicting active iteration borrow is invalid.

---

# 16. Collection errors

Sec does not create an Error merely because an operation can return a normal
negative answer.

General rule:

> If absence, rejection, or "not found" is an ordinary safe outcome, prefer
> `bool` or `Option[T]` over an Error.

Examples:

```text
RemoveAt(invalid index) -> None
Remove(missing value)   -> false
Contains(missing value) -> false
IndexOf(missing value)  -> None
set Add(existing value) -> successful non-insert outcome
```

Errors are reserved for failures that prevent the operation from fulfilling a
valid requested structural/resource action.

Collection-facing structural failures use:

```text
CollectionError
```

Sec 0.1 collection-level cases include at least:

```text
CollectionError.AllocationFailed
CollectionError.CapacityExceeded
CollectionError.SizeOverflow
```

The allocation subsystem may retain its own lower-level:

```text
AllocationError
```

A collection operation may map an underlying allocation failure to:

```text
CollectionError.AllocationFailed
```

This does not redefine allocation failures globally as collection failures.

Access failures remain in access-specific error families where a checked access
operation actually requires an Error.

Examples:

```text
IndexError
RangeError
key-access error family
```

Do not collapse all error domains into one global collection error merely to
simplify `Result[T, E]`.

Instead, collection APIs should avoid unnecessary Error production for normal
negative outcomes.

---

# 17. Compiler-known `fill`

The compiler-known `fill(...)` initializer is defined by
`compiler_known_members.md`.

This rulebook defines the collection consequences.

Examples:

```sec
let mut values: int[5] := fill(10)
let mut values: int[] := try fill(10, 5)
```

For a fixed target array:

```text
target extent comes from T[N]
no runtime count is required
no dynamic allocation is implied by fixed inline storage
```

For a dynamic owning array:

```text
count is explicit
a non-zero result may require allocation
allocation failure is explicit
the result owns count initialized values
```

A context-shaped `fill(...)` must not use return-type overload guessing.

The target type is part of semantic resolution.

String `fill(...)` behavior belongs to the compiler-known/string rules.

Slice `.Fill(value)` remains the in-place mutation operation defined in this
rulebook and is distinct from constructing a new collection with `fill(...)`.

---

# 18. FFI

Fixed arrays may cross an FFI boundary only when their complete layout is
compatible with the declared foreign ABI.

Dynamic arrays, lists, maps, sets, and slices do not have a general FFI-stable
descriptor guarantee.

A foreign wrapper uses explicit compatible components.

For contiguous data, the common pattern is:

```sec
unsafe {
    ForeignCall(values.Ptr, values.SizeOf)
}
```

`Ptr` provides the data address under raw-pointer rules.

Instance `SizeOf` provides the represented payload byte extent.

Neither operation transfers ownership.

Neither extends storage lifetime.

The FFI contract must still define:

```text
pointer validity
mutability
retention
ownership
element ABI compatibility
byte count meaning
```

For APIs that require element count rather than byte count, use:

```sec
values.Len
```

Do not substitute `Len` when the foreign ABI expects bytes.

---

# 19. Borrowing and structural mutation

Borrow rules apply uniformly across collection operations.

A shared borrow prevents conflicting mutation.

A mutable borrow requires exclusive mutable authority.

Structural operations that may relocate or invalidate storage cannot execute
while a conflicting reference, slice, iterator, or direct element reference is
live.

A `ref mut T[]` cannot originate from immutable storage.

A shared slice may originate from mutable storage but grants no mutation.

A slice never outlives:

```text
its source borrow
its ultimate storage owner
the backing incarnation/epoch on which it depends
```

---

# 20. Analysis requirements

Sema and analysis must model collection operations semantically rather than as
ordinary opaque method calls.

At minimum analysis must understand:

```text
logical Len changes
structural mutation
allocation capability
possible relocation
storage epoch/domain invalidation
borrow conflicts
element destruction
element move/copy requirements
normal absence versus Error
bodyless try propagation from fallible collection mutation
iteration structural-mutation restrictions
map key immutability requirements
```

For:

```sec
while values.Len > 5 {
    let removed := values.RemoveAt(5)
}
```

analysis may prove that `RemoveAt(5)` succeeds on every entered iteration even
though its static return type remains `Option[T]`.

The proof may optimize checks or improve diagnostics but must not change source
type semantics.

---

# 21. Diagnostics

Diagnostics should distinguish:

```text
invalid access
normal unsuccessful removal
borrow conflict
structural mutation during borrow/iteration
allocation failure
bounded capacity exhaustion
move-only element extraction
invalid slice range
invalid mutable slice source
obsolete member spelling
obsolete helper method
```

Migration diagnostics should explicitly identify replaced collection members.

Examples:

```text
`.len` has been replaced by `.Len`

`IsEmpty()` is now the read-only property `.IsEmpty`

`AsSlice()` was removed; use `ref value[..]`

`AsMutableSlice()` was removed; use `ref mut value[..]`

`Slice(...)` was removed; use explicit range slicing with `ref` or `ref mut`
```

---

# 22. Parser, Sema, IR, and lowering boundaries

The compiler must preserve distinct semantic categories for:

```text
FixedArrayType(T, N)
DynamicArrayType(T)
SliceType(T, shared/mutable)
ListType(T, optional bounded capacity)
MapType(K, V, optional bounded capacity)
SetType(T, optional bounded capacity)
```

Do not lower one category as another merely because a backend representation
could be shared.

Indexing must retain Place semantics.

Collection structural operations must remain visible in Semantic IR until:

```text
allocation effects
ownership transfer
destruction
borrow invalidation
storage transitions
error propagation
```

have been verified.

The backend does not choose source semantics.

## 22.1 Required default-construction coverage

Conformance tests must cover:

```text
default numeric fixed array
default struct fixed array
non-defaultable fixed-array element
zero-length fixed array
empty owning dynamic array without allocation
empty dynamic and bounded list literals
empty list whose element type is non-defaultable
safe slice rejection without a valid origin
distinct collection-literal and struct-literal resolution
```

---

# 23. Superseded APIs

The following older collection API spellings are not canonical Sec 0.1 surface:

```text
value.len
value.ptr
IsEmpty()

AsSlice()
AsMutableSlice()
Slice()
Iterator()

Push()
```

Use:

```text
value.Len
value.Ptr
value.IsEmpty

ref value[..]
ref mut value[..]

Append()
```

Iteration uses Sec `for` syntax rather than requiring an explicit public
`Iterator()` member.

`First` and `Last` are not base collection members.

Individual non-collection types may define similarly named members through
their own rulebooks when their semantics justify them.

---

# 24. Related rulebooks

This rulebook must be synchronized with:

```text
types.md
lexical_structure.md
grammar.md
compiler_known_members.md
core-library.md
default_values.md
declarations/spread.md
allocation.txt
storage.md
layout.md
reference_model.md
borrowing.txt
ownership.md
copy_move.md
destruction.txt
errorhandling.txt
platform/ffi.md
shaped-types.md
semantic_ir.txt
sec_mlir.md
sec_mlir_dialect.md
sec_mlir_lowering.md
```

P18's dynamic-array implementation package remains the implementation substrate
for owning `T[]`, subject to the later public-API correction that references
this rulebook.

---

# 25. Sec 0.1 summary

```text
T[N]
    fixed owning contiguous array
    Len/IsEmpty/Ptr/SizeOf
    indexing/slicing/iteration
    no AsSlice/Slice helpers

T[]
    simple owning growable array
    MoveOnly
    Len/IsEmpty/Ptr/SizeOf
    Append
    Clear
    RemoveAt -> Option[T]
    explicit slices

ref T[]
ref mut T[]
    borrowed contiguous views
    Len/IsEmpty/Ptr/SizeOf
    Reverse and Fill only on ref mut T[]

list[T]
    richer ordered contiguous collection
    sequence operations plus search/sort members
    source-visible Capacity

map[K, V]
    associative collection
    indexed lookup
    indexed upsert assignment
    no Set()
    for key, value in map
    unspecified order

set[T]
    unique associative collection
    insertion/removal/membership/set operations
    unspecified order

Len
    logical entry count

instance SizeOf
    represented payload bytes for contiguous sequences

T.SizeOf / SizeOf(T)
    type/layout size

normal absence
    bool or Option

structural/resource failure
    CollectionError
```
