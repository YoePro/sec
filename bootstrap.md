# Sec Compiler Bootstrap Plan

## Purpose

This document defines the initial bootstrap plan for the Sec compiler.

The compiler already contains approximately 60,000–70,000 lines of Go code.
The bootstrap effort should therefore not begin by designing a second compiler
from scratch.

The existing Go implementation is the reference implementation and is
translated incrementally into Sec.

The bootstrap has two simultaneous purposes:

1. make the Sec compiler self-hosting;
2. verify that Sec is capable of implementing a real compiler and its required
   system-level support.

The first bootstrap implementation may use provisional functions placed in:

```text
core/bootstrap.sec
stdlib/bootstrap.sec
```

These files are temporary staging areas.

When a function, type or API has been reviewed and accepted, it should be moved
to its final core, standard-library or platform package.

Bootstrap declarations must not silently become permanent language or library
API merely because the bootstrap compiler depends on them.

---

## General strategy

The bootstrap proceeds by translating the existing Go compiler one component at
a time.

The Go implementation remains executable throughout the bootstrap process and
serves as the behavioral reference.

Each translated Sec component should be tested against the corresponding Go
component before the next compiler layer depends on it.

The intended progression is:

```text
Go compiler
    -> compiles first Sec bootstrap components
    -> compiles a partial Sec compiler
    -> compiles the complete Sec compiler
    -> Sec compiler compiles itself
```

The bootstrap must remain incremental.

A partially translated compiler must not require the complete language,
standard library or ownership model before it can be tested.

---

## Bootstrap stages

### Stage 0 — Current Go compiler

The current Go compiler is the stage-0 compiler.

It is responsible for:

- compiling bootstrap Sec code;
- producing the first runnable Sec-written compiler components;
- providing reference lexer, parser, AST, semantic-analysis and code-generation
  output;
- producing diagnostics used for comparison;
- exposing missing compiler or library capabilities discovered during
  translation.

The Go compiler remains authoritative until a translated Sec component has
passed equivalence tests.

---

### Stage 1 — Bootstrap execution environment

Before translating major compiler components, Sec must support the minimal
execution environment required by compiler code.

This stage introduces provisional declarations in:

```text
core/bootstrap.sec
stdlib/bootstrap.sec
```

The initial environment must support:

- process entry;
- command-line arguments;
- process exit codes;
- standard output;
- standard error;
- reading complete files;
- writing complete files;
- basic path handling;
- dynamic storage;
- owned text construction;
- dynamic sequences;
- basic key-value lookup;
- typed errors through `Result`;
- deterministic diagnostics.

The initial functions may be implemented using:

- Sec source;
- Semantic IR support;
- MLIR directly;
- target-specific lowering;
- a small FFI layer;
- compiler intrinsics.

The implementation language is less important than obtaining stable Sec-level
semantics.

No bootstrap function should bypass normal Sec type checking merely because its
implementation is compiler-provided or written directly in MLIR.

---

### Stage 2 — Lexer translation

The lexer is the first major compiler component translated from Go to Sec.

It is a suitable first component because it exercises:

- file input;
- byte and rune processing;
- enums;
- structs;
- functions and methods;
- loops and conditions;
- slices or dynamic sequences;
- string construction;
- diagnostics;
- `Result` handling.

The Sec lexer must produce a canonical token stream.

The token stream should include at least:

- token kind;
- source lexeme or normalized value;
- file identity;
- byte offset;
- line;
- column;
- source span length.

The Go and Sec lexers must be run on the same source corpus.

Their canonical token streams must match.

Invalid source files must also be compared so that lexical diagnostics and
recovery behavior are verified.

---

### Stage 3 — Parser and AST translation

After the token stream is stable, the parser and AST are translated.

The parser should consume the Sec lexer output rather than use an independent
bootstrap-only token model.

The first parser equivalence target is a canonical AST dump.

The dump must avoid unstable information such as:

- memory addresses;
- allocation order when semantically irrelevant;
- internal map iteration order;
- generated identifiers that are not normalized.

The Go and Sec parsers must produce equivalent canonical AST output for:

- valid source files;
- invalid source files;
- parser-recovery tests;
- all existing language feature tests.

The parser must not depend on semantic analysis to decide syntax that belongs in
the grammar.

---

### Stage 4 — Symbol collection and type resolution

The next translated layer is declaration collection and type resolution.

This includes:

- module declarations;
- imports;
- built-in types;
- named types;
- functions;
- overload sets;
- enums;
- structs;
- unions;
- interfaces;
- impl blocks;
- generic declarations;
- constants when supported;
- source-level visibility metadata.

The first comparison format should be a canonical symbol and type dump.

The Sec implementation must preserve nominal type identity.

Map or table iteration order must never affect semantic output or diagnostics.

---

### Stage 5 — Semantic analysis

Semantic analysis is translated in focused groups rather than as one large
conversion.

Recommended order:

1. declarations and scopes;
2. expression typing;
3. assignment and mutability;
4. function calls and overload resolution;
5. control-flow validation;
6. `Result`, `try`, `match`, `Ok` and `Err`;
7. properties and methods;
8. arrays, slices and indexing;
9. generics;
10. ownership, copy and move;
11. borrowing and reference-origin checks;
12. destruction and cleanup planning;
13. FFI and unsafe validation.

Each group must have dedicated valid and invalid test files.

The comparison should primarily use:

- stable diagnostic IDs;
- source spans;
- resolved semantic types;
- canonical Semantic IR.

Diagnostic text may be compared separately because wording improvements should
not necessarily invalidate semantic equivalence.

---

### Stage 6 — Semantic IR translation

The Sec compiler must produce explicit Semantic IR before backend lowering.

Semantic IR must make compiler decisions explicit, including:

- resolved calls;
- conversions;
- constructors;
- property reads and writes;
- copies;
- moves;
- borrows;
- destruction;
- cleanup edges;
- error propagation;
- bounds checks;
- contract checks;
- interface dispatch when required;
- concrete generic instantiations.

The backend must not reconstruct source-language semantics from low-level
operations.

Canonical Semantic IR is the preferred equivalence boundary between the Go and
Sec compilers.

---

### Stage 7 — MLIR and backend translation

Backend translation should begin only after the corresponding Semantic IR is
stable.

The initial backend target should remain Linux AMD64.

The bootstrap backend must support the subset needed to compile the compiler
itself before attempting complete target coverage.

MLIR may be produced by:

- Sec code calling validated MLIR bindings;
- compiler-provided operations;
- direct textual MLIR generation during the first bootstrap phase;
- a temporary bootstrap writer.

Direct MLIR generation is acceptable during bootstrap, but the Sec-level API and
ownership behavior must still be defined.

The first backend comparison should use normalized MLIR or LLVM IR rather than
linked binary bytes.

Binary comparison may be added after unstable metadata has been removed or
normalized.

---

### Stage 8 — First self-hosted compiler

The first complete Sec-written compiler is stage 1 of self-hosting.

The build chain becomes:

```text
sec-go -> sec-stage1
sec-stage1 -> sec-stage2
sec-stage2 -> sec-stage3
```

The primary fixed-point requirement is:

```text
normalized output from sec-stage2
    ==
normalized output from sec-stage3
```

The comparison should include:

- diagnostics;
- canonical AST where applicable;
- canonical Semantic IR;
- normalized MLIR or LLVM IR;
- produced executable behavior;
- the full valid and invalid compiler test suite.

Byte-identical binaries are desirable but are not the first correctness
criterion.

---

## Provisional bootstrap files

### `core/bootstrap.sec`

This file contains provisional declarations that are tightly coupled to the
language, compiler or primitive type system.

Possible initial responsibilities:

- primitive string length and storage access;
- primitive slice length and storage access;
- compiler-known allocation primitives;
- compiler-known process termination;
- primitive conversion helpers;
- low-level memory copying and comparison;
- compiler-only construction of fundamental collection storage;
- built-in bootstrap errors that cannot yet be imported from their final
  package.

Functions placed here must be candidates for one of:

- permanent core behavior;
- a compiler intrinsic;
- a target platform primitive;
- removal after bootstrap.

`core/bootstrap.sec` must not become a miscellaneous convenience library.

---

### `stdlib/bootstrap.sec`

This file contains provisional library-level APIs needed by the compiler.

Possible initial responsibilities:

- complete-file reading;
- complete-file writing;
- standard output and standard error;
- command-line argument access;
- basic path operations;
- dynamic byte buffers;
- text builders;
- provisional dynamic sequences;
- provisional maps and sets;
- deterministic diagnostic rendering;
- temporary-file support when backend invocation requires it.

Functions placed here should later move into packages such as:

```text
fmt
io
os
path
collections
text
process
```

The final package structure is not decided merely by the bootstrap placement.

---

## Current implementation status

### Implemented in the Go frontend

The stage-0 Go compiler and LSP can now resolve external Sec project sources
using the existing source-level inclusion model.

Implemented resolver behavior:

- `sec/core/*.sec` is loaded from the compiler repository before user analysis;
- `sec/stdlib/<module>/*.sec` and target-specific stdlib files remain resolved
  from the compiler repository;
- `platform/...` imports remain resolved from the compiler repository;
- source files under an external project with `.sec/sec.toml` can import
  project-local Sec modules by source path;
- local project imports are tried as:
  - `<project-root>/<import>.sec`
  - `<project-root>/<import>/<basename>.sec`
  - every `.sec` file directly under `<project-root>/<import>/`
- the LSP uses the same project-root rule for local imports, so editor analysis
  and CLI analysis agree.

This supports compiling Sec projects from outside the Go compiler repository
when their source files are passed to the stage-0 compiler, for example:

```text
sec sema <project-dir>
```

### Not yet implemented

- parsing `.sec/sec.toml` import declarations;
- package metadata or compiled library artifacts;
- stable project diagnostics for unresolved imports;
- command-specific source-root flags;
- recursive target filtering for project-local imported modules beyond the
  current source-file target checks;
- dependency graph reporting for bootstrap builds.

---

## Output functions

Bootstrap code must use the already selected Sec output syntax.

Examples:

```sec
fmt.print("compiling ")
fmt.println(path)
fmt.eprintln("compilation failed")
```

Bootstrap code must not introduce alternative global output functions such as:

```sec
Print(...)
Println(...)
StdoutWrite(...)
StderrWrite(...)
```

Low-level output operations may exist internally, but normal Sec source must use
the approved `fmt` member syntax.

The initial bootstrap implementation may provide only a limited set of accepted
argument types, but the source-level names and call shape must remain consistent
with the intended standard library.

The minimum initial surface should include:

```sec
fmt.print(value)
fmt.println(value)
fmt.eprint(value)
fmt.eprintln(value)
```

Multiple arguments and formatting rules may be added after the basic bootstrap
path works.

---

## Initial capability requirements

The following capabilities are required before substantial translation of the
Go compiler can proceed.

### Process

Required behavior:

- obtain command-line arguments;
- return or set a process exit code;
- write diagnostics to standard error;
- write ordinary output to standard output.

The exact final API names remain provisional except for the established `fmt`
syntax.

### Files

Required behavior:

- read a complete file as bytes or string;
- write a complete file from bytes or string;
- report typed I/O errors;
- preserve binary data exactly;
- distinguish empty files from failed reads.

Streaming file handles are useful but are not a prerequisite for translating
the lexer.

### Text

Required behavior:

- obtain string length;
- traverse bytes;
- traverse runes where required;
- compare strings;
- create substrings or source views;
- build owned strings efficiently;
- convert integral values to diagnostic text;
- preserve source text exactly.

The bootstrap should avoid unnecessary Unicode normalization.

The compiler must distinguish byte offsets from rune or display-column
positions.

### Dynamic sequences

Required behavior:

- create an empty sequence;
- append values;
- query length;
- index values;
- mutate elements when permitted;
- reserve capacity;
- iterate deterministically;
- destroy owned elements correctly.

The first implementation may use a provisional collection type, but the type
must not silently allocate without returning or propagating allocation failure
unless allocation failure policy has been explicitly defined.

### Maps and sets

Required behavior:

- insert;
- lookup;
- replace;
- remove when needed;
- test membership;
- iterate in a way that does not affect deterministic compiler output.

Hash-map iteration order must not determine:

- diagnostics;
- emitted symbols;
- AST order;
- Semantic IR order;
- generated code order.

Where output order matters, keys must be sorted or declarations must retain
source order separately.

### Allocation

The bootstrap requires dynamic allocation for compiler data structures.

The first implementation may use:

- a C allocator through FFI;
- operating-system virtual-memory functions;
- a simple Sec allocator;
- compiler-provided bootstrap allocation operations.

The initial choice should minimize unrelated bootstrap risk.

Allocation behavior must still define:

- ownership;
- failure reporting;
- alignment;
- reallocation behavior;
- deallocation pairing;
- destruction of contained values.

Raw pointers alone must not represent ownership.

### Errors

Bootstrap operations must use typed errors.

The initial compiler implementation should use:

```sec
Result[T, E]
Ok(value)
Err(error)
try operation()
match result {
    Ok(value) => ...
    Err(error) => ...
}
```

Bootstrap helpers must not introduce hidden exceptions or untyped global error
state.

---

## Translation order for the existing Go compiler

The exact package names depend on the current repository structure, but the
recommended component order is:

1. source positions and spans;
2. token kinds and token values;
3. lexer;
4. diagnostics data model;
5. AST data structures;
6. parser;
7. canonical AST printer;
8. symbols and scopes;
9. type model;
10. declaration collection;
11. expression semantic analysis;
12. control-flow analysis;
13. generic instantiation;
14. ownership and borrowing analysis;
15. Semantic IR;
16. MLIR lowering;
17. LLVM lowering and linking;
18. command-line driver;
19. project and manifest handling;
20. formatter and auxiliary tools.

The command-line driver should not be translated first.

The compiler core should be callable as ordinary functions so that Go and Sec
implementations can be compared directly through test harnesses.

---

## Translation rules

The Go implementation is translated semantically, not mechanically line by
line.

During translation:

- Go slices must be mapped to the correct Sec ownership or view type;
- Go maps must not introduce nondeterministic output;
- Go interfaces must be mapped only after the required Sec interface semantics
  exist;
- Go `error` values must become typed Sec errors;
- Go `defer` must follow Sec defer and cleanup semantics;
- Go pointers must not automatically become Sec references;
- nil must become an explicit `Option`, `Result`, union variant or FFI-only raw
  pointer state;
- implicit Go zero values must be replaced with valid Sec initialization;
- Go garbage-collected ownership assumptions must be made explicit;
- mutation must be declared explicitly;
- copies and moves must be resolved by Sec type semantics.

Translation is also a design audit.

A Go implementation pattern must not automatically become a Sec language or
library pattern.

---

## Testing strategy

### Equivalence tests

Every translated component must have an equivalence harness.

The same input is processed by both implementations.

The harness compares stable data rather than implementation-specific memory
layout.

Examples:

```text
lexer          -> canonical token stream
parser         -> canonical AST
sema           -> diagnostics and resolved types
Semantic IR    -> canonical IR
backend        -> normalized MLIR or LLVM IR
executable     -> stdout, stderr and exit code
```

### Existing test corpus

All existing valid and invalid `.sec` files should be included in bootstrap
comparison.

Additional bootstrap-specific tests should cover:

- empty files;
- very large files;
- invalid UTF-8 where relevant;
- long identifiers;
- large token counts;
- allocation failure injection;
- file read and write failure;
- deterministic output across repeated runs;
- source-position correctness;
- diagnostics containing multiple related spans.

### Determinism

Running the same compiler stage repeatedly on the same source and target must
produce the same canonical output.

The bootstrap must detect accidental dependence on:

- pointer values;
- allocator order;
- hash randomization;
- thread scheduling;
- filesystem enumeration order;
- locale;
- host-specific path separators where target-independent output is expected.

---

## Temporary APIs and approval

Every declaration in a bootstrap file should be marked conceptually as one of:

```text
candidate-core
candidate-stdlib
candidate-platform
candidate-intrinsic
temporary-bootstrap-only
```

The exact annotation syntax may be added later.

Until then, the classification should be documented next to the declaration.

Before moving a declaration out of a bootstrap file, review:

- whether the name fits Sec conventions;
- whether the operation belongs in core, stdlib or platform code;
- whether ownership is explicit;
- whether failure behavior is explicit;
- whether the API works on hosted and freestanding targets;
- whether the operation requires unsafe;
- whether it allocates;
- whether it is deterministic;
- whether it exposes implementation details;
- whether the compiler is its only real consumer.

A provisional API may be renamed or removed without compatibility guarantees.

---

## MLIR implementation policy

Core and standard-library bootstrap operations may be implemented directly in
MLIR when this is the clearest or safest first implementation.

Suitable candidates include:

- primitive memory operations;
- process entry and exit glue;
- integer formatting primitives;
- string length and pointer extraction;
- slice length and pointer extraction;
- low-level allocation wrappers;
- system-call wrappers;
- volatile operations;
- target ABI adaptation.

Direct MLIR implementation must still have a declared Sec signature and defined
Sec semantics.

MLIR implementation must not:

- bypass ownership rules;
- invent implicit allocation;
- return undocumented null values;
- hide recoverable failure;
- use a different calling convention than declared;
- make target-dependent behavior appear target-independent.

Where practical, more behavior should later move from handwritten MLIR to Sec
source after the compiler can compile it reliably.

---

## Initial milestone

Current implementation status for file input:

- `io.Open(path)` is declared for Linux/amd64 and returns `Result[io.File,
  io.IOError]`.
- `io.File.Read(buffer)` and `io.ReadFileInto(path, buffer)` are declared and
  semantically valid for caller-provided `ref mut byte[]` storage.
- `io.File.Write(data)`, `io.File.Flush()`, `io.File.Seek(offset, whence)` and
  lowercase aliases `read`, `write`, `flush`, `seek`, `close` are present as
  early file infrastructure. `Seek` currently returns `IOError.Unsupported`.
- `io.File.Close()` is declared, semantically valid and marks the file closed.
- Sema reports a local `io.File` that reaches scope exit without `Close()` or
  ownership transfer by return.
- The Linux/amd64 implementation routes through the existing raw syscall
  surface and normal Sec `Result` values.
- `io.ReadFile(path) Result[string, io.IOError]` exists as the intended
  complete-file API, but currently returns `IOError.Unsupported`.

Pending for complete-file reads:

- reviewed CString construction for general runtime paths;
- owned dynamic byte storage;
- byte-array-to-string materialization;
- final lowering/runtime support for the file IO path.

The first bootstrap milestone is complete when a Sec program can:

1. receive a source-file path from the command line;
2. read the complete file;
3. tokenize it using a Sec implementation translated from the Go lexer;
4. store all tokens dynamically;
5. report lexical diagnostics through the approved `fmt` syntax;
6. emit a canonical token stream;
7. return a correct process exit code;
8. match the Go lexer on the complete lexer test corpus.

This milestone proves that the following foundation works together:

```text
process arguments
file input
strings and source text
allocation
dynamic sequences
control flow
Result handling
diagnostics
fmt output
backend lowering
```

Only after this milestone should the Sec parser become dependent on the Sec
lexer.

---

## Second milestone

The second bootstrap milestone is complete when the Sec lexer and parser:

- parse the complete supported grammar;
- produce a canonical AST;
- match the Go compiler for valid files;
- match parser diagnostics and recovery for invalid files;
- compile and run using only approved bootstrap dependencies.

---

## Third milestone

The third bootstrap milestone is complete when the Sec implementation can:

- collect declarations;
- resolve names and types;
- execute the initial semantic checks;
- emit stable diagnostics;
- produce canonical Semantic IR for the supported bootstrap subset.

At that point, translation can proceed from front-end equivalence toward full
self-hosting.

---

## Non-goals of the first bootstrap phase

The first bootstrap phase does not require:

- all target platforms;
- all integer widths;
- decimal arithmetic;
- physical units;
- concurrency;
- `spawn` or `await`;
- runtime interface values;
- full closure support;
- complete reflection;
- package distribution;
- stable public library compatibility;
- optimal generated code;
- byte-identical compiler binaries.

These features may be implemented when required by the translated compiler or
by the broader language roadmap.

The bootstrap should not delay self-hosting merely to complete unrelated
language features.

---

## Guiding rule

The bootstrap files exist to discover the smallest correct foundation required
for Sec to implement Sec.

They are not shortcuts around language semantics.

Every provisional operation must eventually be:

- accepted and moved to its proper location;
- replaced by a better accepted API;
- reduced to a compiler intrinsic;
- restricted to a platform implementation;
- or removed after bootstrap.
