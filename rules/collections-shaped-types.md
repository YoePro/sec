# Collections and Shaped Types

## Purpose

This rulebook defines Sec's first-class collection and shaped-data type families.

It covers:

- `list`
- `map`
- `set`
- `vector`
- `matrix`
- `tensor`
- `tensor_view`
- `Shape`
- `Strides`
- `TensorLayout`
- matrix multiplication with the contextual `x` operator
- the division between compiler semantics and standard-library implementation
- expected MLIR representation and lowering

Existing arrays, slices, registers, and bit-backed enums remain governed by
their own rulebooks, but must be integrated with the type model described here.

---

# Design principles

## Small first-class set

Sec should provide only the collection and shaped types whose semantics benefit
materially from compiler knowledge.

More specialized data structures belong in the standard library.

The first-class set is:

```sec
list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]

vector[T, N]

matrix[T, Rows, Columns]

tensor[T, Dimensions...]

tensor_view[T, Rank]
```

Supporting nominal types include:

```sec
Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
MemorySpace
```

---

## Lowercase and uppercase type names

Fundamental language types and compiler-known type constructors begin with a
lowercase letter:

```sec
bool
int
string

list[T]
map[K, V]
set[T]

vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor_view[T, Rank]
```

Named and nominal types begin with an uppercase letter.

This applies to user types, core types, and standard-library types:

```sec
Result[T, E]
Option[T]
Task[T]
Thread[T]

Shape[Rank]
Strides[Rank]
TensorLayout[Rank]

Stack[T]
RingBuffer[T, Capacity]
OrderedMap[K, V]

CollectionError
ShapeError
LayoutError
```

The declaration form does not change the naming rule:

```sec
type Person struct {
    name: string,
}

type Response union {
    // variants
}

enum Permissions bit[8] {
    Read = 1,
    Write = 2,
}

type Control register[32] {
    Enabled: bit,
    Mode: bit[3],
    _: bit[28],
}
```

---

## Existing compiler-known generic forms

The new types must coexist with existing compile-time-parameterized forms:

```sec
type Control register[32] {
    Enabled: bit,
    Mode: bit[3],
    _: bit[28],
}

enum Permissions bit[8] {
    Read = 1,
    Write = 2,
}
```

`register[N]` and `bit[N]` are compiler-known language forms.

They are not standard-library generics.

Square brackets are resolved by context:

```text
list[T]
    element type

list[T, Capacity]
    element type and compile-time capacity

matrix[T, Rows, Columns]
    element type and compile-time dimensions

Shape[Rank]
    compile-time rank

register[N]
    compile-time register width

bit[N]
    compile-time bit width
```

The parser, AST, Sema, Semantic IR, formatter, and diagnostics must preserve the
different semantic categories.

---

# Type families

The collection model has three families.

```text
Linear collections
    array
    slice
    list

Associative collections
    map
    set

Shaped values and views
    vector
    matrix
    tensor
    tensor_view
    Shape
    Strides
    TensorLayout
```

---

# Existing arrays and slices

## Arrays

An array is fixed-length owned linear storage.

Its canonical type syntax remains defined by the array rulebook.

Arrays:

- own their elements;
- have compile-time-known length;
- do not grow;
- have canonical contiguous element storage unless another explicit layout rule
  says otherwise;
- may live in stack, static, arena, or other explicitly selected storage.

Arrays are not replaced by `list`.

---

## Slices

A slice is a non-owning linear view.

A normal slice contains:

- storage origin;
- start;
- runtime length;
- borrow mode.

A normal slice has unit element stride.

General non-unit, negative, or zero-stride views belong to `tensor_view`, not to
ordinary slice semantics.

---

# list

## Meaning

`list[T]` is a variable-length ordered linear collection.

```sec
let values: list[int]
```

The default semantic model is contiguous storage.

`list[T]` is not a linked list.

A linked list is a standard-library type:

```sec
LinkedList[T]
```

---

## Dynamic list

```sec
list[T]
```

has runtime-variable length and capacity.

A dynamic list must be associated with a permitted allocation strategy before an
operation may grow its storage.

The language must not assume a hidden global heap.

Permitted strategies may include:

- explicit allocator;
- arena;
- target runtime allocator;
- another allocation source approved by `allocation.txt`.

Growth is fallible.

## Empty defaults and literals

`list[T]` is defaultable independently of whether `T` is defaultable. Its
default is an initialized empty list with:

```text
length 0
capacity 0
element storage none
initialized elements none
dynamic allocation none
```

`list[T, Capacity]` is also defaultable. Its empty default has length zero,
maximum capacity `Capacity`, no initialized elements and no hidden growth
allocation. Reserved backing capacity does not construct elements.

Canonical explicit forms are:

```sec
list[string] {}
list[Packet, 32] {}
```

These are collection literals, not named-struct literals. Parser, AST, Sema,
formatter and lowering must retain that distinction. Empty construction never
allocates; later dynamic growth remains fallible and requires an approved
allocation context. This rule does not decide map or set literal syntax.

---

## Bounded list

```sec
list[T, Capacity]
```

has runtime-variable length and a compile-time maximum capacity.

```sec
let packets: list[Packet, 32]
```

The bounded form:

- performs no hidden growth allocation;
- never exceeds `Capacity`;
- may use inline, stack, static, arena, or explicit backing storage;
- reports capacity exhaustion explicitly;
- is suitable for MCU, RTOS, and no-heap profiles.

`Capacity` must be greater than zero.

---

## List properties

The public API should provide harmonized members where their meaning is clear:

```sec
values.Len()
values.Capacity()
values.IsEmpty()
```

Mutation and fallibility belong to the standard-library implementation of the
built-in type.

Exact member names must be synchronized with the core and standard-library
naming rules.

---

# map

## Meaning

`map[K, V]` stores unique keys associated with values.

```sec
let users: map[UserID, User]
```

The source-level semantics require:

- unique keys;
- lookup;
- insertion;
- replacement;
- removal;
- iteration;
- ownership of keys and values;
- stable key equality and hashing while stored;
- explicit capacity and allocation behavior;
- mutation-during-iteration checks.

---

## Dynamic map

```sec
map[K, V]
```

may grow only through an explicitly permitted allocation strategy.

Growth failure is explicit.

No hidden heap is assumed.

---

## Bounded map

```sec
map[K, V, Capacity]
```

has a compile-time maximum entry count.

```sec
let routes: map[RouteID, Route, 64]
```

The bounded form:

- performs no hidden growth allocation;
- never stores more than `Capacity` entries;
- reports capacity exhaustion explicitly;
- may use a compiler-selected fixed hash-table representation.

---

## Representation

The initial implementation should use hash-based semantics.

The physical representation is not part of the source contract unless this
rulebook explicitly guarantees it.

The compiler and standard library may select:

- open addressing;
- robin-hood hashing;
- another fixed-capacity hash strategy;
- target-specialized hashing;
- compile-time perfect hashing for immutable known maps.

The selected representation must preserve source semantics.

---

## Iteration order

`map` does not guarantee iteration order.

Programs must not depend on:

- insertion order;
- hash-bucket order;
- sorted order;
- target-specific order.

Ordered behavior belongs in the standard library:

```sec
OrderedMap[K, V]
```

This permits the compiler and standard library to improve the normal map
representation without changing source semantics.

---

## Key requirements

A key type must support:

```text
equality
hashing
stable key identity while stored
```

The final core interface or compiler-known contract names remain defined by the
map implementation rule.

Candidate nominal names include:

```sec
Equal
Hash
```

A stored key must not be mutated in a way that changes equality or hash.

The compiler should derive equality and hashing for eligible types.

---

# set

## Meaning

`set[T]` stores unique values.

```sec
let activeUsers: set[UserID]
```

It is a distinct source-level type even when its implementation shares map
infrastructure.

It provides:

- insertion;
- removal;
- membership;
- iteration over values;
- set operations;
- explicit allocation and capacity behavior.

---

## Bounded set

```sec
set[T, Capacity]
```

has a compile-time maximum value count.

The same bounded-storage principles as `map` apply.

---

## Iteration order

`set` does not guarantee iteration order.

Ordered behavior belongs in:

```sec
OrderedSet[T]
```

in the standard library.

---

# Shaped values

## General model

A shaped value has:

```text
element type
rank
shape
logical value semantics
optional materialized storage
```

A materialized view additionally has:

```text
storage origin
offset
strides
layout
memory space
borrow mode
```

Not every property occupies runtime storage.

Static information should remain in the type or Semantic IR and be erased when
unnecessary.

---

## Value versus storage view

The following are logical shaped values:

```sec
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
```

They do not promise one permanently observable physical memory layout.

The compiler may:

- keep a value in registers;
- scalarize it;
- fuse operations;
- eliminate temporaries;
- change physical layout before it becomes observable;
- lower it through MLIR tensor semantics;
- materialize a buffer only when needed.

The following is an explicit view of storage:

```sec
tensor_view[T, Rank]
```

A `tensor_view` preserves observable shape, offset, stride, layout, memory-space,
and borrow information.

---

# Shape

## Shape type

```sec
Shape[Rank]
```

represents one non-negative extent per axis.

```sec
let dimensions: Shape[3] := value.shape
```

Rank is part of the type.

A `Shape[3]` is not interchangeable with `list[uint]`.

Zero extents are valid.

A shaped value with at least one zero extent contains zero logical elements.

Negative extents are invalid.

The product of all extents must be checked for overflow.

---

## Static rank

Sec 0.1 requires rank to be known at compile time.

For:

```sec
tensor[T, D0, D1, D2]
```

the rank is three.

The first owned tensor form requires compile-time-known extents.

Runtime-dynamic owned tensor dimensions are a later extension.

`tensor_view[T, Rank]` may carry runtime shape values while retaining static
rank.

---

# Strides

## Strides type

```sec
Strides[Rank]
```

contains one element stride per axis.

```sec
let strides: Strides[2] := view.strides
```

Stride values are measured in elements, not bytes.

Target byte addresses are derived from:

```text
base address
element offset
element strides
element size
```

---

## Stride values

A stride may be:

```text
positive
    forward traversal

negative
    reversed traversal

zero
    broadcasted axis
```

Canonical owned dense storage uses positive non-zero strides.

Zero and negative strides are primarily view semantics.

---

## Canonical row-major layout

The default materialized dense layout is row-major.

For:

```sec
matrix[T, Rows, Columns]
```

the canonical element strides are:

```text
row stride       Columns
column stride    1
```

For a rank-N dense tensor:

- the last axis has unit stride;
- each preceding stride is the product of later extents.

Statically derivable strides require no runtime metadata.

---

## Addressing

A tensor-view element address is conceptually selected by:

```text
offset + sum(index[axis] * stride[axis])
```

in element units.

The compiler must validate:

- each index;
- arithmetic overflow;
- negative-stride bounds;
- zero-stride aliasing;
- backing-storage range;
- memory-space compatibility.

---

## Aliasing and mutation

A mutable view must be proven non-overlapping for its permitted index domain.

A zero-stride broadcast view aliases one element through multiple logical
indexes and is therefore shared-only by default.

Invalid:

```sec
ref mut tensor_view[T, Rank]
```

when the selected view has overlapping logical indexes.

A negative-stride reversed view may be mutable when every logical index maps to
a unique element.

---

# TensorLayout

## Purpose

Simple strides can represent:

- row-major layout;
- column-major layout;
- padded rows;
- transpose;
- reversal;
- subsampling;
- broadcasting.

Strides alone cannot represent every layout.

Examples requiring a richer description include:

- tiled layout;
- blocked layout;
- Morton or Z-order;
- accelerator swizzles;
- compressed sparse formats;
- banked memory.

The nominal type:

```sec
TensorLayout[Rank]
```

describes the layout category and associated metadata.

Initial conceptual categories include:

```text
DenseRowMajor
DenseColumnMajor
Strided
Tiled
Sparse
TargetSpecific
```

The exact enum/union representation belongs in core or stdlib.

---

## Sec 0.1 public view restriction

The initial `tensor_view[T, Rank]` implementation should support affine strided
views.

More advanced sparse and target-specific views may use dedicated layout
metadata and operations later.

This restriction keeps:

```sec
view.strides
```

well-defined in the initial implementation.

---

# MemorySpace

A shaped value or view may reside in a nominal memory space such as:

```text
host memory
stack
static storage
arena
shared memory
device memory
GPU memory
accelerator memory
MMIO-backed storage where explicitly permitted
```

The source syntax is not locked by this document.

Relevant memory-space information must remain in Semantic IR.

A view must not cross an incompatible memory-space boundary without an explicit
transfer.

---

# vector

## Meaning

```sec
vector[T, N]
```

is a rank-one shaped value.

```sec
let position: vector[float64, 3]
```

Semantically:

```text
rank 1
shape [N]
```

It is not a growable collection.

Growable ordered storage is:

```sec
list[T]
```

---

## Vector orientation

A plain `vector[T, N]` is treated as a column vector for matrix multiplication.

Therefore:

```sec
matrix[T, R, C] x vector[T, C]
```

is valid.

A vector used as a row vector requires an explicit row view or conversion.

The final API may provide a nominal row-view type or a method such as:

```sec
value.AsRow()
```

The exact name remains part of the vector stdlib API.

---

## Vector operations

Potential vector operations include:

```text
Dot
Outer
Magnitude
Normalize
Cross for valid dimensions
```

These are standard-library members or compiler-recognized intrinsics.

They do not all require dedicated language operators.

---

# matrix

## Meaning

```sec
matrix[T, Rows, Columns]
```

is a rank-two shaped value.

```sec
let transform: matrix[float32, 4, 4]
```

Semantically:

```text
rank 2
shape [Rows, Columns]
```

---

## Matrix operations

The type supports or may expose:

```text
row view
column view
transpose
diagonal
trace
matrix multiplication
determinant where defined
inverse where defined
```

Basic shape-aware operations may be compiler-known.

Numerical algorithms such as inverse, decomposition, eigensolvers, and
factorization belong in the standard library.

---

# tensor

## Meaning

```sec
tensor[T, Dimensions...]
```

is the general rank-N shaped value.

```sec
let image: tensor[float32, 3, 224, 224]
let volume: tensor[float64, 64, 64, 64]
```

The number of dimensions determines rank.

The dimensions are compile-time integer values in Sec 0.1.

This variadic dimension form is compiler-known and does not automatically imply
general variadic generic parameters for user-defined types.

---

## Element types

A tensor may store any type satisfying the shaped-storage contract.

Arithmetic requires stronger element contracts.

Example:

```text
tensor[Pixel, ...]
    storage and indexing

tensor[float32, ...]
    storage, indexing, arithmetic, reductions, and contraction
```

The compiler must not assume every storable element type is numeric.

---

# tensor_view

## Meaning

```sec
tensor_view[T, Rank]
```

is a non-owning affine strided view.

```sec
fn ProcessImage(image: tensor_view[float32, 3]) void {
}
```

A view contains or derives:

```text
borrowed storage origin
element offset
runtime Shape[Rank]
runtime or static Strides[Rank]
TensorLayout[Rank]
MemorySpace
element type
borrow mode
```

---

## View forms

A tensor view may represent:

- complete tensor storage;
- tensor slice;
- submatrix;
- row;
- column;
- transpose;
- reversed axis;
- strided sampling;
- broadcast view;
- reshape when layout permits it.

Mutability comes from the reference:

```sec
ref tensor_view[T, Rank]
ref mut tensor_view[T, Rank]
```

Separate mutable and immutable view type names are not required.

---

## Properties

A shaped view exposes:

```sec
view.rank
view.shape
view.strides
view.layout
view.memorySpace
view.isContiguous
```

When statically known, these may be compile-time values.

---

## Contiguity

Contiguity means that the view describes one canonical uninterrupted element
range for its selected layout.

Examples:

```text
owned dense matrix
    normally contiguous

row view
    normally contiguous

column view
    normally non-contiguous

transposed view
    normally non-contiguous

broadcast view
    non-contiguous and overlapping
```

Operations requiring contiguous storage must state that requirement.

The compiler should prove contiguity statically where possible.

---

# Operators

## Ordinary multiplication

The `*` operator retains ordinary scalar or elementwise multiplication.

Valid conceptual cases include:

```sec
scalar * scalar
shapedValue * scalar
scalar * shapedValue
left * right
```

For two shaped values, elementwise multiplication requires identical shapes.

```sec
let products := left * right
```

does not mean matrix multiplication.

The result element type follows normal scalar multiplication and unit algebra.

---

## Matrix multiplication operator

Sec uses the contextual infix operator:

```sec
x
```

for linear-algebraic multiplication.

Example:

```sec
let result := left x right
```

The formatter requires one space on each side:

```sec
left x right
```

`x` remains a valid identifier in ordinary identifier positions:

```sec
let x := 10
```

It is recognized as a contextual operator only between expressions in an
operator position.

It is not a globally reserved identifier.

---

## Precedence and associativity

`x` has multiplicative precedence.

It has the same precedence level as:

```text
*
/
%
```

It is left-associative.

Parentheses should be used when dimensions or intended grouping would otherwise
be unclear.

---

## Valid matrix multiplication forms

### Matrix by matrix

```sec
matrix[L, Rows, Inner] x matrix[R, Inner, Columns]
```

produces:

```sec
matrix[ProductElement[L, R], Rows, Columns]
```

`ProductElement[L, R]` means the scalar product type derived by Sema.

The product terms must also be addable into one result element.

Example:

```sec
let result: matrix[float32, 3, 2] :=
    left x right
```

when:

```text
left  matrix[float32, 3, 4]
right matrix[float32, 4, 2]
```

### Matrix by vector

```sec
matrix[L, Rows, Columns] x vector[R, Columns]
```

produces:

```sec
vector[ProductElement[L, R], Rows]
```

A plain vector is treated as a column vector.

---

## Invalid or explicit cases

The following are not implicitly defined in Sec 0.1:

```sec
vector[T, N] x vector[T, N]
vector[T, N] x matrix[T, N, M]
tensor[T, ...] x tensor[T, ...]
matrix[T, R, C] x scalar
scalar x matrix[T, R, C]
```

Use explicit operations:

```sec
left.Dot(right)
left.Outer(right)
row.AsRow() x matrix
left.Contract(right, ...)
matrix * scalar
```

The exact contraction API belongs to the tensor stdlib implementation.

---

## Dimension checking

The contraction dimension must match exactly.

Invalid:

```sec
matrix[float32, 3, 4] x matrix[float32, 5, 2]
```

Expected error:

```text
matrix multiplication requires matching inner dimensions, got 4 and 5
```

When dimensions are compile-time-known, the error is compile-time.

---

## Element-type checking

The scalar multiplication of the two element types must be valid.

All product terms accumulated into one output element must support compatible
addition.

Units participate in ordinary Sec unit algebra.

Example conceptually:

```text
Meter x Scalar -> Meter
Meter x PerSecond -> Speed
```

The compiler must derive the result element type from scalar operations rather
than requiring both matrices to have an identical element type.

Named-type and explicit-conversion rules still apply.

---

## No generic operator overloading yet

The `x` operator is compiler-known for shaped linear-algebra types in Sec 0.1.

User-defined arbitrary `x` operator overloading is not introduced by this
rulebook.

A future operator-interface design may generalize it separately.

---

## Tensor contraction

The `x` operator is not general tensor contraction.

General rank-N contraction must be explicit:

```sec
left.Contract(right, ...)
```

This prevents hidden axis selection and accidental shape interpretation.

The compiler and MLIR backend may still preserve contraction as a high-level
operation.

---

# Broadcasting

## Scalar broadcasting

Scalar-to-shaped multiplication and other selected scalar operations may
broadcast the scalar:

```sec
let scaled := matrix * 2.0
```

This is unambiguous.

---

## Shaped broadcasting

Different shaped values are not automatically broadcast in Sec 0.1.

The programmer must request it explicitly:

```sec
let expanded := right.BroadcastTo(left.shape)
let result := left + expanded
```

This makes shape changes visible while preserving compiler optimization
opportunities.

Broadcast views normally use zero strides and are shared-only.

---

# Indexing

Indexing remains part of each type's complete implementation.

Examples:

```sec
value[index]
matrix[row, column]
tensor[i, j, k]
```

The compiler must:

- validate index count against rank;
- validate index types;
- perform compile-time bounds checks where possible;
- emit explicit runtime bounds operations otherwise;
- use shape and stride metadata;
- preserve place mutability;
- enforce alias and overlap restrictions.

---

# Iteration

Iteration must support the relevant forms:

```text
value iteration
index iteration
index and value iteration
map entry iteration
axis iteration
```

Structural mutation during iteration is forbidden unless a specific iterator
contract permits it.

Element mutation is permitted only when storage stability, borrowing, and
aliasing rules permit it.

Map and set iteration must not promise ordering.

---

# Ownership and allocation

## Owned types

The following normally own their storage:

```text
array
list
map
set
vector
matrix
tensor
```

The following are non-owning:

```text
slice
tensor_view
```

The following are small nominal descriptor values:

```text
Shape
Strides
TensorLayout
MemorySpace
```

---

## Copy and move

Copyability is derived from:

- element type;
- storage representation;
- allocator ownership;
- active borrows;
- explicit copy policy.

Dynamic collections must not be implicitly deep-copied unless the final copy
rule explicitly permits it.

Moving transfers storage ownership.

Views remain tied to backing-storage lifetime.

---

## Destruction

Owned collections destroy each initialized element exactly once.

Maps destroy each stored key and value exactly once.

Sets destroy each stored value exactly once.

Views do not destroy backing elements.

Descriptor values are normally trivially destructible.

---

## No hidden heap

No first-class collection or shaped type may assume a hidden global heap.

Any operation requiring allocation must use:

- an attached allocator;
- an arena;
- explicit target runtime allocation;
- another approved allocation source.

Bounded forms must not allocate merely because they become full.

---

# Standard-library responsibility

## Compiler and standard library split

These types are first-class language types:

```sec
list
map
set
vector
matrix
tensor
tensor_view
```

Their type identity, syntax, validation, ownership model, Semantic IR, and MLIR
mapping are compiler-defined.

Their public member APIs and reusable algorithms must also be declared and
implemented in the standard library.

The standard library receives privileged permission to define implementations
for built-in lowercase types, conceptually:

```sec
impl list[T] {
    // public collection API
}

impl map[K, V] {
    // public associative API
}

impl matrix[T, Rows, Columns] {
    // public matrix API
}
```

Ordinary user modules may not globally extend built-in types.

---

## Mandatory stdlib implementation

This rulebook is not considered fully implemented until stdlib contains working
implementations for at least:

```text
list
map
set
vector
matrix
tensor
tensor_view
Shape
Strides
TensorLayout
```

The stdlib implementation must include:

- constructors;
- destruction;
- core member operations;
- fallible capacity/allocation behavior;
- iteration;
- copy/move integration;
- indexing helpers where not intrinsic;
- equality where semantically valid;
- hashing support for map/set keys;
- matrix and vector operations;
- explicit tensor reshape, transpose, broadcast, reduction, and contraction
  foundations;
- target-independent tests;
- bounded no-allocation variants.

Compiler intrinsics may replace selected stdlib bodies during lowering.

The source-visible API must still exist in stdlib.

---

## Compiler access to stdlib declarations

AST and Sema must load compiler-known stdlib declarations for these built-in
types.

This follows the same general principle as compiler-known core members on
fundamental types.

The compiler must not hard-code every public method signature independently of
stdlib source.

Intrinsic operations may be marked or registered so Sema can validate the
stdlib declaration against compiler expectations.

---

## Completion requirement

A collection feature is not fully implemented merely because parser and Sema
accept the type.

Full implementation requires:

1. lexer/parser support where syntax changes are needed;
2. AST representation;
3. semantic type resolution;
4. ownership, borrowing, lifetime, and destruction rules;
5. stable diagnostics;
6. Semantic IR operations;
7. MLIR lowering;
8. target lowering or explicit unsupported-target diagnostics;
9. `core/errors.sec` runtime errors;
10. stdlib type API and implementations;
11. valid and invalid Sec integration tests;
12. Go compiler unit tests where useful;
13. formatter support;
14. LSP/type-display support;
15. implementation-status update in this rulebook.

---

# Standard-library data structures

The following are standard-library nominal types and begin with uppercase
letters.

They are not first-class language type constructors.

## Linear and queue structures

```sec
Stack[T]
Queue[T]
Deque[T]
LinkedList[T]
RingBuffer[T, Capacity]
```

`RingBuffer` is especially important for:

- MCU;
- RTOS;
- ISR deferred work;
- audio;
- networking;
- telemetry;
- bounded producer/consumer pipelines.

An ISR-safe ring buffer may use a separate stdlib type or explicit policy.

---

## Priority structures

```sec
BinaryHeap[T]
PriorityQueue[T, Priority]
```

Ordering and comparison policy belong to stdlib contracts.

---

## Ordered and specialized associative structures

```sec
OrderedMap[K, V]
OrderedSet[T]
MultiMap[K, V]
MultiSet[T]
FlatMap[K, V]
FlatSet[T]
```

`OrderedMap` and `OrderedSet` provide ordering that built-in `map` and `set` do
not guarantee.

`FlatMap` and `FlatSet` may use sorted contiguous storage and are useful for
small collections and embedded systems.

---

## Bit and lookup structures

```sec
BitSet[N]
BloomFilter[T, Bits]
Trie[K, V]
RadixTree[K, V]
```

`BitSet[N]` is a collection of independently addressable bits.

It is not the same as:

```sec
enum Permissions bit[8] {
}
```

where `bit[8]` defines enum representation width.

---

## Trees and graphs

```sec
Tree[T]
BinaryTree[T]
Graph[Node, Edge]
DirectedAcyclicGraph[Node, Edge]
```

These remain library-owned unless compiler implementation experience proves a
language-level need.

---

## Numerical types and algorithms

```sec
Complex[T]
Quaternion[T]
Polynomial[T]
```

Potential stdlib numerical modules include:

```text
matrix decomposition
linear solvers
eigenvalues
singular value decomposition
FFT
convolution
statistics
signal processing
image processing
automatic differentiation
neural-network operators
```

The compiler may recognize selected stdlib operations as intrinsics while the
source API remains in stdlib.

---

## Domain-shaped wrappers

```sec
Grid[T, Rows, Columns]
Image[T, Width, Height]
Volume[T, X, Y, Z]
```

These may wrap `matrix`, `tensor`, or `tensor_view` while adding domain
invariants and APIs.

---

## Sparse layout policies

Potential stdlib layout policy types include:

```sec
CsrLayout
CscLayout
CooLayout
BlockSparseLayout
```

A sparse matrix remains mathematically a `matrix`.

A sparse rank-N value remains mathematically a `tensor`.

Sparsity is represented through layout or encoding, not by replacing the
mathematical type family.

---

# Core errors

All language-level runtime error types required by these features must be
declared in:

```text
core/errors.sec
```

The initial set should include or be refined from:

```sec
enum CollectionError {
    CapacityExceeded
    AllocationFailed
    InvalidOperation
    UnsupportedOperation
}

enum IndexError {
    Negative
    OutOfBounds
    RankMismatch
}

enum ShapeError {
    NegativeExtent
    RankMismatch
    DimensionMismatch
    ElementCountOverflow
    InvalidReshape
    InvalidBroadcast
}

enum LayoutError {
    InvalidStride
    AddressOverflow
    OutOfStorageBounds
    OverlappingMutableView
    UnsupportedLayout
    IncompatibleMemorySpace
}
```

The final API may split errors more narrowly.

Compiler diagnostics are not runtime error values and do not belong in
`core/errors.sec`.

---

# Semantic analysis

Sema must validate at least:

- lowercase built-in type constructor names;
- type and compile-time value argument counts;
- capacities;
- ranks;
- dimensions;
- element storage eligibility;
- map/set key contracts;
- allocation capability;
- ownership and move state;
- view borrow lifetime;
- index rank and bounds;
- shape compatibility;
- stride arithmetic;
- contiguity requirements;
- mutable overlap;
- memory-space compatibility;
- `*` elementwise semantics;
- `x` matrix multiplication dimensions and element algebra;
- tensor contraction arguments;
- iteration mutation restrictions;
- stdlib intrinsic declaration consistency.

---

# Semantic IR

Semantic IR must preserve collection and shaped semantics explicitly.

At minimum, it should support operations equivalent to:

```text
ListCreate
ListLen
ListCapacity
ListGet
ListSet
ListInsert
ListRemove
ListDestroy

MapCreate
MapLookup
MapInsert
MapRemove
MapIterate
MapDestroy

SetCreate
SetContains
SetInsert
SetRemove
SetIterate
SetDestroy

ShapeCreate
StridesCreate
TensorCreate
TensorCopy
TensorMove
TensorDestroy
TensorElementPlace
TensorViewCreate
TensorViewSlice
TensorViewTranspose
TensorViewBroadcast
TensorReshape
TensorReduce
TensorContract

VectorDot
VectorOuter
MatrixMultiply

BoundsCheck
ShapeCheck
LayoutCheck
MemorySpaceTransfer
```

Semantic IR must record:

- element type;
- capacity;
- rank;
- static and runtime shape;
- strides;
- offset;
- layout;
- memory space;
- ownership;
- borrow mode;
- allocation strategy;
- index and bounds proof;
- operator-selected scalar algebra;
- source location.

MLIR lowering must not reconstruct these semantics from pointer arithmetic.

---

# MLIR

## Principle

MLIR provides lowering and optimization mechanisms.

It does not define Sec source semantics.

The pipeline is conceptually:

```text
Sec source
    -> AST
    -> Sema
    -> Sec Semantic IR
    -> Sec collection/shaped MLIR dialect
    -> standard MLIR dialects
    -> bufferization and target lowering
    -> LLVM, SPIR-V, or another backend
```

Language validation must be complete before MLIR lowering.

---

## Logical shaped values

Sec `vector`, `matrix`, and `tensor` should remain logical shaped values while
high-level optimization is useful.

Expected standard MLIR forms include:

```text
builtin ranked tensor
tensor dialect operations
linalg structured operations
shape information
vector dialect where appropriate
```

MLIR tensor values model abstract value semantics.

This permits:

- fusion;
- tiling;
- vectorization;
- destination-style optimization;
- temporary elimination;
- shape propagation;
- target-independent algebraic transformations.

---

## Bufferization

Bufferization converts logical tensor semantics into physical memory-buffer
semantics.

Sec ownership and lifetime analysis must guide this process.

The bufferization pass must not invent ownership rules.

Expected physical forms include:

```text
memref
offset
sizes
strides
memory space
explicit allocation/deallocation decisions
```

A `tensor_view` naturally maps to a memref-like descriptor or equivalent
strided-view representation.

---

## vector lowering

Sec `vector[T, N]` is a source mathematical rank-one value.

It may lower to:

- MLIR vector;
- ranked tensor;
- memref;
- scalarized values;
- target SIMD registers.

The backend chooses according to target and operation.

Sec source semantics do not require every `vector` to become one MLIR vector
value.

---

## Matrix multiplication

The source expression:

```sec
left x right
```

must be preserved as a high-level `MatrixMultiply` operation through Semantic IR.

It should lower to an appropriate structured MLIR operation, normally through
Linalg or an equivalent optimized path.

It must not be immediately expanded into scalar loops before shape-aware
optimization.

The backend may later select:

- tiled loops;
- SIMD;
- BLAS-compatible calls where allowed;
- GPU kernels;
- accelerator operations;
- static unrolled code for small fixed matrices.

No mandatory runtime or external BLAS dependency is implied.

---

## Sparse tensors

Sparse layout metadata should lower through MLIR sparse-tensor encodings and
operations where supported.

The Sec compiler must preserve:

- logical shape;
- sparse encoding;
- index widths;
- storage ordering;
- mutability;
- allocation policy.

Unsupported sparse layouts must fail explicitly.

---

## list, map, and set

MLIR does not define Sec's source semantics for general `list`, `map`, and `set`.

Sec should define a custom high-level dialect for these operations.

The custom dialect may then lower to:

- fixed inline storage;
- arena storage;
- heap storage;
- compiler-generated loops;
- target library calls;
- specialized immutable representations.

MLIR dialect extensibility is an implementation tool, not a reason to expose
backend details in Sec syntax.

---

## GPU and accelerator lowering

Shaped operations may later lower through MLIR GPU, SPIR-V, NVVM, or another
target path.

Memory-space transfers must remain explicit in Semantic IR.

The compiler must not silently move data between host and device memory when the
transfer has observable cost or failure.

---

# Diagnostics

Examples:

```text
list capacity must be greater than zero
```

```text
map key type MutableBuffer does not provide stable hashing while stored
```

```text
matrix multiplication requires matching inner dimensions, got 4 and 5
```

```text
operator x requires matrix or matrix-vector operands
```

```text
elementwise multiplication requires identical shapes, got [3, 4] and [4, 3]
```

```text
mutable tensor view overlaps itself through zero stride on axis 0
```

```text
tensor element count overflows target address size
```

```text
view stride reaches outside backing storage
```

```text
dynamic collection growth requires an allocator or arena
```

```text
OrderedMap is a standard-library type; builtin map does not guarantee order
```

Diagnostics must have stable IDs.

---

# Tests

Required Sec integration tests include at least:

```text
list_valid.sec
list_invalid.sec

map_valid.sec
map_invalid.sec

set_valid.sec
set_invalid.sec

vector_valid.sec
vector_invalid.sec

matrix_valid.sec
matrix_invalid.sec

tensor_valid.sec
tensor_invalid.sec

tensor_view_valid.sec
tensor_view_invalid.sec
```

Every invalid test case must document:

```sec
/* Expected error: ...
 * Reason: ...
 */
```

Tests must cover parser, AST, Sema, ownership, analysis, Semantic IR, MLIR, and
stdlib behavior as each phase becomes implemented.

---

# Implementation status

## Implemented

According to the current relevant Sec rule documents:

- fixed postfix array type parsing is implemented;
- unsized sequence and slice-reference type parsing is implemented;
- AST distinguishes fixed arrays from unsized sequence types;
- Sema resolves fixed arrays and accepts `ref T[]` and `ref mut T[]`;
- Sema rejects bare unsized `T[]` in ordinary value positions;
- parser and AST support mixed type/value arguments for `list`, `map`, `set`,
  `vector`, `matrix`, `tensor`, `tensor_view`, `Shape`, `Strides` and
  `TensorLayout`;
- Sema recognizes `list[T]`, `list[T, Capacity]`, `map[K, V]`,
  `map[K, V, Capacity]`, `set[T]`, `set[T, Capacity]`, `vector[T, N]`,
  `matrix[T, Rows, Columns]`, `tensor[T, Dimensions...]`,
  `tensor_view[T, Rank]`, `Shape[Rank]`, `Strides[Rank]`,
  `TensorLayout[Rank]` and `MemorySpace` as compiler-known types;
- Sema validates type-argument counts and compile-time integer argument counts
  for those collection-shaped type constructors;
- Sema rejects non-positive bounded collection capacities;
- Sema rejects negative shaped dimensions and ranks;
- `set[T]` is accepted in type positions despite `set` also being a contextual
  property-setter keyword;
- `list[T]`, `set[T]`, `map[K, V]` and `vector[T, N]` participate in the
  current `for` iterable type inference;
- LSP type completion sees the new compiler-known types through the intrinsic
  type registry;
- VS Code TextMate grammar highlights the new collection-shaped type names;
- `register[N]`, `bit`, and `bit[N]` parsing is implemented;
- bit-backed enum widths are preserved through AST and Sema;
- register widths and field widths are validated;
- bit-backed enum widths have MLIR and LLVM scalar-width lowering;
- the compiler has an active MLIR pipeline for an existing scalar/numeric
  subset.

These existing implementations are dependencies and must not be regressed.

---

## Not Implemented

The following are not considered implemented by this rulebook yet:

- parsing, AST distinction, Sema construction and lowering for the canonical
  empty `list[T] {}` and `list[T, Capacity] {}` literals;
- other collection constructors, map/set literals and mutation APIs;
- shaped constructors, literals and member APIs for `vector`, `matrix`,
  `tensor`, `tensor_view`, `Shape`, `Strides` and `TensorLayout`;
- `for` iterable type inference for shaped types beyond rank-one
  `vector[T, N]`;
- contextual `x` parsing;
- matrix multiplication Sema;
- shape and stride analysis;
- mutable-overlap analysis;
- collection and shaped Semantic IR;
- collection and shaped MLIR lowering;
- sparse layout lowering;
- GPU or accelerator lowering;
- runtime collection errors in `core/errors.sec`;
- required stdlib implementations for the first-class types;
- planned standard-library structures listed by this document;
- complete indexing, slicing, and bounds lowering for the existing array/slice
  rules;
- formatter support and complete LSP member/signature support for these types;
- required valid and invalid integration tests.

This section must be updated as implementation progresses.

A type or operation may move to `Implemented` only when all applicable
completion requirements in this rulebook are satisfied.

---

# Required synchronization

This rulebook must be synchronized with:

```text
types.txt
generics.txt
arrays-slices.txt
registers.txt
enums.txt
allocation.txt
ownership.md
borrowing.txt
references.txt
lifetime_analysis.txt
copy_move.md
destruction.txt
functions.txt
impl.txt
interfaces.txt
flowcontrol_for.txt
spread.txt
units.txt
diagnostics.txt
semantic_ir.txt
mlir.txt
mlir-optimize.txt
compiler_pipeline.txt
rules_implementations.txt
core-library.md
formatter.md
default_values.md
language_philosophy.txt
core/errors.sec
stdlib collection modules
stdlib numerical modules
```

The type names and status must also be synchronized with the compact manual,
keyword/predeclared-type lists, VS Code grammar, formatter, and LSP.
