# Sec Standard Library

## Purpose

The Sec standard library provides reusable modules, nominal types, algorithms,
and platform services above the language and core-library layers.

The standard library is not the definition of Sec's first-class type system.

It builds on:

```text
compiler-defined language semantics
compiler intrinsics
the mandatory core library
target capabilities
```

The standard library is imported explicitly.

It must remain usable across hosted, freestanding, RTOS, and embedded profiles
where the selected module's requirements can be satisfied.

---

# 1. Layer model

Sec has three implementation layers.

## 1.1 Compiler

The compiler defines language semantics that require direct knowledge of:

```text
type identity
representation
ownership
borrowing
addressability
layout
bounds
shape
strides
volatile behavior
target capabilities
Semantic IR
MLIR lowering
backend lowering
```

The compiler may implement an operation directly as an intrinsic or lowering
rule.

---

## 1.2 Core library

The core library is the mandatory first-class library layer.

It is available without import and is defined by `core-library.md`.

Core provides:

- fundamental behavior for built-in types;
- the public core surface of first-class language types;
- compiler-known nominal support types;
- language-level runtime error types;
- privileged impl blocks for compiler-owned lowercase types;
- the bridge between normal member lookup and intrinsic implementation.

Examples of first-class types whose fundamental API belongs in compiler/core
include:

```sec
string
list[T]
map[K, V]
set[T]
vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor_view[T, Rank]
```

Core may implement members through:

- ordinary Sec source;
- compiler intrinsics;
- direct Semantic IR operations;
- Sec-specific MLIR operations;
- standard MLIR dialects;
- target-specific lowering.

Core is not required to be implemented only in Sec source.

---

## 1.3 Standard library

The standard library is the reusable library-level layer above core.

It provides:

- higher-level algorithms;
- specialized collection types;
- numerical libraries;
- text and encoding facilities;
- time facilities;
- higher-level synchronization;
- operating-system integration;
- networking;
- formatting, I/O, logging, and predefined units.

A standard-library feature may also be implemented through:

- ordinary Sec source;
- compiler-recognized declarations;
- direct Semantic IR;
- Sec-specific MLIR operations;
- standard MLIR dialects;
- target-specific lowering;
- FFI;
- system calls;
- accelerator libraries.

The use of intrinsic or MLIR implementation does not make a standard-library
type a first-class language type.

Language status and implementation technique are separate concerns.

---

# 2. Core and stdlib boundary

The compiler/core layer owns the fundamental source-visible behavior required to
use a first-class language type correctly.

For example, the compiler/core layer owns the semantic foundation of:

```text
list length and capacity
map key and value ownership
set uniqueness
matrix shape
tensor strides and views
indexing
bounds checks
destruction
matrix multiplication
Semantic IR and MLIR mapping
```

The standard library may provide additional algorithms that operate on those
types.

Examples:

```text
sorting
search algorithms
graph algorithms
matrix decompositions
FFT
statistics
advanced text transformations
serialization
protocol implementations
```

The standard library must not redefine the language identity or fundamental
semantics of a built-in lowercase type.

It must not globally extend a built-in type unless a separate language rule
explicitly delegates that member surface to stdlib.

The normal model is:

```text
compiler/core
    defines and implements first-class members

stdlib
    consumes first-class types and adds higher-level nominal types and algorithms
```

---

# 3. No wrapper duplication

The standard library must not introduce a nominal wrapper solely to provide
members that belong on a first-class type.

Invalid design direction:

```sec
type List[T] struct {
    value: list[T],
}
```

when the only purpose is to recreate the fundamental `list[T]` API.

Likewise, a separate nominal `Matrix[T, R, C]` must not duplicate the
first-class:

```sec
matrix[T, R, C]
```

Nominal wrappers are valid when they add real domain semantics or invariants.

Examples:

```sec
Image[Pixel, Width, Height]
Grid[T, Rows, Columns]
Transform3D
```

---

# 4. Source-visible declarations

Even when a standard-library operation lowers directly through the compiler or
MLIR, it must have a source-visible declaration.

Conceptually:

```sec
module math

fn FFT[T](values: ref tensor_view[T, 1]) Result[tensor[T, Dynamic], FFTError]
```

The exact syntax and dynamic tensor support depend on their own rulebooks.

The declaration provides:

- name;
- generic parameters;
- parameter and result types;
- ownership and borrowing;
- failure behavior;
- effects;
- target requirements;
- documentation;
- tooling visibility.

The implementation may then be selected from:

```text
Sec body
intrinsic implementation
MLIR implementation
target-specialized implementation
external library adapter
```

The compiler must validate that the selected implementation matches the
source-visible declaration.

---

# 5. Imports

Standard-library modules are imported by their module path.

Example:

```sec
import "fmt"
import "collections"
import "math"
```

No project or repository prefix is required for standard-library modules.

Import resolution must prefer the standard-library namespace only according to
the project/module resolution rules.

A local or external module must not silently replace a standard-library module.

There are no hidden stdlib imports.

Core is always present.

Stdlib is imported explicitly.

---

# 6. Module organization

Standard-library source resides under:

```text
sec/stdlib
```

One directory represents one module according to the project/module rules.

Current repository modules include:

```text
fmt
io
log
units
unicode
```

The expected top-level standard-library areas include at least:

```text
collections
math
text
encoding
time
sync
os
net
fmt
io
log
units
```

A large area may contain submodules.

Examples:

```text
encoding/json
encoding/utf8
encoding/base64

math/linear
math/statistics
math/signal

net/http
net/dns
net/tls
```

Exact submodule names are established by their own API work.

---

# 7. Naming

Standard-library modules use lowercase names:

```text
collections
math
text
encoding
time
sync
os
net
```

Standard-library nominal types begin with an uppercase letter:

```sec
Stack[T]
RingBuffer[T, Capacity]
OrderedMap[K, V]
Complex[T]
EncodingError
```

Functions, fields, methods, properties, constants, and enum values follow the
general naming rulebook and their category-specific rules.

Standard-library code must not use keywords, modifiers, or reserved language
names as declarations.

---

# 8. Implementation techniques

## 8.1 Ordinary Sec implementation

Portable algorithms should normally be implemented in Sec when this provides
correct behavior and adequate optimization.

Examples include:

```text
collection adapters
iterators
small utility algorithms
domain wrappers
pure text transforms
fallback numerical algorithms
```

Generic Sec source should be specialized and inlined where the compiler can
prove it safe and beneficial.

---

## 8.2 Compiler intrinsics

A stdlib declaration may be compiler-recognized when ordinary Sec cannot express
the operation correctly or efficiently.

Examples may include:

```text
special target instructions
accelerator dispatch
operating-system transition helpers
specialized atomic primitives
high-level numerical operations retained for optimization
```

Intrinsic identity must not depend only on a user-spellable function name.

The compiler should bind the trusted stdlib declaration through an internal,
stable intrinsic identity.

---

## 8.3 Direct MLIR implementation

A standard-library operation may lower directly to MLIR.

Possible dialects include:

```text
arith
math
vector
tensor
linalg
memref
sparse_tensor
gpu
spirv
llvm
target-specific dialects
Sec-specific dialects
```

Examples:

```text
matrix decomposition kernels
FFT
convolution
Unicode scanning kernels
bulk memory algorithms
cryptographic target operations
```

A direct MLIR implementation must preserve the Sec declaration's:

- ownership;
- borrowing;
- failure;
- effects;
- shape;
- layout;
- target;
- cleanup;
- diagnostic contract.

MLIR does not define the source API.

The stdlib rulebook and module declarations do.

---

## 8.4 Target-specialized implementation

A module may provide target-specific implementations.

Examples:

```text
os file operations
network sockets
high-resolution clocks
thread synchronization
SIMD math
GPU kernels
hardware encoders
```

Target selection must happen through compiler-known target/profile mechanisms.

User code must not accidentally bind to an implementation for another target.

A portable fallback may be used when it preserves the same documented
semantics.

---

## 8.5 FFI and system calls

Stdlib may use:

```text
extern declarations
RawPtr[T]
system calls
platform APIs
trusted unsafe code
```

Such code must obey:

- ABI rules;
- ownership rules;
- effect declarations;
- panic boundaries;
- target-profile restrictions;
- error translation rules.

Foreign errors should be translated into stable Sec nominal error types where
the module API promises portability.

---

# 9. Runtime and allocation policy

Importing stdlib must not automatically require:

```text
garbage collection
a hidden global heap
a task scheduler
an operating system
exceptions
a managed runtime
```

Each API must make its requirements explicit.

A module or operation may require:

```text
allocator
arena
thread support
task support
clock
filesystem
network stack
Unicode tables
device memory
external library
```

The compiler must reject an unavailable required capability or select a valid
alternative implementation.

Bounded and static forms must remain available where the module contract
promises no allocation.

No stdlib operation may silently allocate only because a similarly named
operation does so in another language.

---

# 10. Error placement

Language-level runtime errors belong in:

```text
sec/core/error.sec
```

or the canonical `core/errors.sec` location selected by the repository.

Examples include errors required directly by language operations:

```text
bounds
capacity of a first-class bounded collection
shape
layout
thread creation
fundamental conversion
```

Standard-library-specific errors belong to their owning module unless they are
promoted into core because they are broadly required by language semantics.

Examples:

```sec
io.IOError
encoding.EncodingError
net.NetworkError
time.ClockError
math.MathError
collections.PriorityQueueError
```

A stdlib module must not add every domain error to core.

Error names are nominal types and begin with uppercase letters.

---

# 11. collections

The `collections` module provides higher-level collection data structures and
algorithms above the first-class:

```sec
list
map
set
```

Expected nominal types include:

```sec
Stack[T]
Queue[T]
Deque[T]
LinkedList[T]
RingBuffer[T, Capacity]

BinaryHeap[T]
PriorityQueue[T, Priority]

OrderedMap[K, V]
OrderedSet[T]
MultiMap[K, V]
MultiSet[T]
FlatMap[K, V]
FlatSet[T]

BitSet[N]
BloomFilter[T, Bits]
Trie[K, V]
RadixTree[K, V]

Tree[T]
BinaryTree[T]
Graph[Node, Edge]
DirectedAcyclicGraph[Node, Edge]
```

`OrderedMap` and `OrderedSet` provide ordering semantics that first-class `map`
and `set` deliberately do not guarantee.

`RingBuffer[T, Capacity]` must support a bounded no-allocation implementation.

ISR-safe or lock-free variants require explicit contracts and must not be
implied by the base `RingBuffer` name.

The module may also provide algorithms over first-class collections without
globally extending their core member surface.

---

# 12. math

The `math` module provides advanced numerical algorithms and nominal numerical
types.

Expected types may include:

```sec
Complex[T]
Quaternion[T]
Polynomial[T]
```

Expected areas include:

```text
elementary functions
trigonometry
logarithms and exponentials
linear algebra
matrix decomposition
linear solvers
eigenvalues
singular value decomposition
statistics
FFT
convolution
signal processing
automatic differentiation
```

The first-class:

```sec
vector
matrix
tensor
```

remain compiler/core types.

The `math` module supplies advanced algorithms over them.

Compiler and MLIR recognition is encouraged where it retains high-level
optimization opportunities.

No mandatory BLAS, GPU, or external runtime dependency is implied.

---

# 13. text

The `text` module provides higher-level text processing above core `string`,
`char`, and `rune`.

Expected areas include:

```text
Unicode normalization
Unicode segmentation
case mapping
search algorithms
pattern processing
string building
text comparison and collation
tokenization
```

Core retains only the minimal behavior required for the fundamental string type.

Large Unicode tables and higher-level algorithms belong in `text` or an
appropriate submodule.

A text operation that allocates must expose allocation and failure behavior.

---

# 14. encoding

The `encoding` area provides conversion between structured values, text, and
binary representations.

Expected subareas include:

```text
UTF encodings
base encodings
hex
JSON
binary serialization
character encodings
compression adapters where appropriate
```

Possible module paths include:

```text
encoding/utf8
encoding/base64
encoding/json
```

Encoding errors belong to the encoding module unless the operation is required
by a fundamental core conversion.

Compile-time validation should be used for literal format descriptions where
the format rule permits it.

---

# 15. time

The `time` module provides clocks, calendar/time representations, timers, and
related conversions.

Expected areas include:

```text
monotonic clocks
wall clocks
instants
durations where not already represented by the units system
timers
deadlines
calendar values
time zones where supported
```

Clock availability is target-dependent.

A bare-metal target may provide only monotonic ticks or an explicit hardware
clock.

The module must distinguish monotonic measurement from wall-clock time.

Timer integration with tasks, threads, and `select` must preserve the
concurrency rules.

---

# 16. sync

The `sync` module provides higher-level synchronization constructs above
first-class compiler/core concurrency primitives.

Possible nominal types include:

```sec
Semaphore
Barrier
Latch
Once
WaitGroup
Condition
ReadWriteMutex[T]
```

The exact set remains subject to their own rulebooks and implementation work.

Compiler/core retains fundamental synchronization semantics such as:

```text
atomics
Mutex[T]
channels
task and thread completion
select registration
memory ordering
```

The `sync` module must not hide blocking, allocation, or scheduler
requirements.

ISR-safe synchronization requires an explicit ISR-safe API.

---

# 17. os

The `os` module provides operating-system integration.

Expected areas include:

```text
files
directories
paths
environment
terminal/process environment
permissions
system information
platform services
```

Process spawning is intentionally deferred and must not be treated as completed
merely because an `os` module exists.

`os` is unavailable or reduced on targets without an operating system.

Target-specific platform detail may be exposed through explicit platform
submodules or platform views.

Portable APIs should translate native errors into stable Sec errors while
retaining native detail where useful.

---

# 18. net

The `net` module provides networking abstractions and protocol support.

Expected areas include:

```text
addresses
sockets
datagrams
streams
DNS
network interfaces
protocol adapters
higher-level protocol modules
```

Possible submodules include:

```text
net/dns
net/http
net/tls
```

Availability depends on the target network stack.

The module must define:

- blocking and nonblocking behavior;
- task/thread integration;
- cancellation;
- timeout behavior;
- ownership of buffers and handles;
- platform error translation;
- allocation requirements.

---

# 19. Existing modules

The repository currently contains standard-library module directories for:

```text
fmt
io
log
units
```

This rulebook does not redefine their detailed APIs.

Their responsibilities are conceptually:

## fmt

Formatting and formatted output.

It may use compiler-known format validation and optimized formatting paths.

## io

General I/O interfaces and operations.

Concrete filesystem operations belong to `os` or a suitable submodule.

## log

Logging abstractions built on formatting, I/O, time, and target facilities.

Logging must not be a hidden compiler side effect.

## units

Predefined physical and domain units above the language's unit system.

The unit system itself remains a first-class language capability.

---

# 20. Module dependencies

Core must not depend on stdlib.

Stdlib modules may depend on core.

Stdlib modules may depend on other stdlib modules when the dependency graph
remains explicit and acyclic.

Example conceptual dependencies:

```text
log
    -> fmt
    -> io

encoding/json
    -> text
    -> io where stream support is used

net/http
    -> net
    -> io
    -> encoding
```

Circular module initialization dependencies are invalid.

The module and initialization rulebooks define exact dependency and
initialization behavior.

---

# 21. API design requirements

Every public stdlib API must document:

```text
ownership and borrowing
copy and move behavior
allocation
blocking
suspension
thread safety
ISR safety
panic behavior
typed failures
target support
complexity where material
determinism
ordering guarantees
```

An API must not promise more portability than its implementations provide.

A target-specific limitation must be:

- statically rejected;
- represented by a target capability;
- or returned as a documented runtime error when runtime variability is real.

---

# 22. Public type stability

Stdlib nominal type identity is part of the source API.

Changing:

```text
type name
generic parameter order
ownership behavior
error type
layout guarantee
method contract
```

may be a breaking language-library change.

Physical representation is not stable unless the API or ABI rule explicitly
guarantees it.

Compiler-recognized stdlib declarations must use stable internal identities that
survive harmless source refactoring.

---

# 23. Testing requirements

Every stdlib module must provide:

```text
valid Sec integration tests
invalid API-use tests where compile-time rejection is expected
unit tests
target-specific tests
allocation-policy tests
ownership and destruction tests
error-path tests
concurrency tests where relevant
MLIR/lowering tests for intrinsic implementations
fallback-versus-specialized equivalence tests
```

A specialized intrinsic or MLIR implementation must be behaviorally equivalent
to the portable implementation for the documented domain.

Bounded no-allocation types must be tested under a profile that forbids hidden
allocation.

---

# 24. Documentation requirements

Every public module, type, function, method, property, and error must have
source-visible documentation.

Documentation must state when an operation:

- allocates;
- blocks;
- suspends;
- can fail;
- requires unsafe;
- is target-specific;
- requires a runtime service;
- has ordering or determinism guarantees;
- uses approximate numerical behavior.

Compiler-generated API documentation must include intrinsic and MLIR-backed
declarations exactly like ordinary Sec declarations.

---

# 25. Completion criteria

The standard-library layer is not complete merely because directories or
declaration stubs exist.

A stdlib module is considered implemented only when:

1. its public API is declared;
2. ownership and failure semantics are defined;
3. at least one valid implementation exists for each claimed target profile;
4. unavailable targets receive explicit diagnostics;
5. compiler or MLIR intrinsics match the public declarations;
6. fallback and specialized implementations are tested;
7. required errors are declared in the correct module;
8. allocation and runtime requirements are explicit;
9. documentation is complete;
10. integration tests pass.

The complete collection ecosystem additionally requires:

```text
compiler/core implementation of first-class list, map, set, vector, matrix,
tensor, and tensor_view

stdlib implementation of the higher-level collection types and algorithms
promised by the collections module
```

The two requirements are related but distinct.

---

# 26. Implementation status

## Implemented

The repository currently contains initial standard-library areas:

```text
fmt
io
log
units
```

Their presence does not by itself claim complete API or target coverage.

Compiler/core already has an established mechanism for compiler-known behavior
and privileged built-in implementations through `core-library.md`.

The `io` package currently includes Linux/amd64 file and directory declarations:

- `io.Open(path) Result[io.File, io.IOError]`;
- `io.Create(path)`, `io.OpenAppend(path)` and
  `io.OpenReadWrite(path, create)` for writable handles;
- `io.File.Read(buffer) Result[uint, io.IOError]` for caller-provided
  `ref mut byte[]` storage;
- `io.File.Write(data) Result[uint, io.IOError]`;
- `io.File.WriteAll(data)` and `io.File.WriteString(data)`, including partial
  write handling and interrupted-syscall retry;
- `io.File.ReadExact(buffer)` for fixed-size records and `io.Copy(source,
  destination, buffer)` for allocation-free streaming copies;
- `io.File.Flush() Result[void, io.IOError]` through `fsync`;
- `io.File.Seek(offset, origin) Result[uint, io.IOError]` through `lseek` and
  the `io.SeekOrigin` enum;
- `io.File.Truncate(size)` through `ftruncate`;
- `io.ReadFileInto(path, buffer) Result[uint, io.IOError]` and
  `io.File.ReadAllInto(buffer)`, which read through EOF, reject caller-buffer
  truncation with `IOError.TooLarge`, and close on success and failure;
- `io.WriteFile(path, data)` and `io.WriteStringFile(path, data)`;
- `io.File.Close() Result[void, io.IOError]`;
- lowercase `File` method aliases `read`, `write`, `flush`, `seek` and `close`;
- `io.OpenDirectory(path) Result[io.Directory, io.IOError]` and
  `io.Directory.Next() Result[Option[io.DirectoryEntry], io.IOError]`, backed by
  a fixed caller-owned iterator buffer and Linux `getdents64`; dot entries are
  filtered and malformed records are rejected;
- `io.ReadDirectoryInto(path, entries)`, which reads the complete directory
  into caller-owned `DirectoryEntry[]` storage and reports `IOError.TooLarge`
  instead of returning a partial list;
- `io.Access`, `io.Exists`, `io.CreateDirectory`, `io.RemoveFile`,
  `io.RemoveDirectory` and `io.Rename` for non-recursive path operations;
- semantic scope-exit diagnostics that require local `io.File` and
  `io.Directory` values to be closed explicitly or returned to transfer
  ownership;
- `@noCopy` policies on file and directory handles so one descriptor cannot be
  duplicated by ordinary value copy;
- close-on-exec on every high-level file and directory open, preventing handle
  inheritance across subsequently spawned processes;
- a bounded, NUL-terminating Linux path bridge that rejects embedded NUL and
  overlong paths before invoking the kernel;
- typed mapping for common path, descriptor, capacity, blocking, memory,
  seek, filesystem and write failures through `io.IOError`;
- a provisional `io.ReadFile(path) Result[string, io.IOError]` API that returns
  `IOError.Unsupported` until owned dynamic storage and string materialization
  are implemented.

The `unicode` package currently provides `unicode.IsLetter(ch: rune) bool`.
It is generated from Go's Unicode 15.0.0 `unicode.Letter` range table, uses
static range data and binary search, performs no allocation, and has no runtime
Unicode-data dependency. Ranges with a Go stride other than one are expanded by
the generator because current Sec `rune` operations intentionally do not expose
integer arithmetic.

## Not implemented

This rulebook is not considered fully implemented until at least the following
stdlib areas exist with documented APIs and tests:

```text
collections
math
text
encoding
time
sync
os
net
```

The following are also incomplete until implemented and tested:

- intrinsic identity and declaration validation for stdlib operations;
- direct MLIR-backed stdlib implementation contracts;
- target-profile availability diagnostics;
- module dependency and initialization validation;
- public documentation generation;
- the higher-level collection types listed by this rulebook;
- owned dynamically sized complete-file results and allocating directory-list
  convenience APIs;
- complete cross-target fallback testing.

---

# 27. Required synchronization

This rulebook must be synchronized with:

```text
core-library.md
foundations/language_philosophy.md
language-rulebook-status.md
names_scopes_visibility.md
types.md
functions.md
declarations/generics.md
declarations/interfaces.md
impl.md
properties.md
errorhandling.txt
allocation.txt
ownership.md
borrowing.txt
copy_move.md
destruction.txt
collections.md
shaped-types.md
types/units.md
concurrency.md
threads.md
tasks.txt
channels.md
select.md
mutex.md
atomics.md
blocking.md
scheduling.md
platform/ffi.md
projects.txt
modules.md
compiler/initialization.md
target_profiles.md
platform_model.md
platform/abi.md
semantic_ir.txt
mlir.txt
mlir-optimize.txt
compiler_pipeline.txt
diagnostics.txt
formatter.md
compiler_testing.md
sec/core
sec/stdlib
```

The living `language-rulebook-status.md` must change `stdlib.md` from Candidate
to Written when this rulebook is added to the repository.

`core-library.md` remains the canonical core rulebook and must be updated
separately where its current wording assigns first-class collections to stdlib
or does not yet describe direct MLIR-backed core implementations.
