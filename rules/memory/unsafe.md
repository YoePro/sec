# Unsafe

## Status

This document is the canonical unsafe rulebook for Sec 0.1.

It defines:

- what `unsafe` means;
- which operations require an unsafe context;
- single-operation and block syntax;
- `unsafe fn` and `unsafe extern`;
- caller proof obligations;
- safe wrappers around unsafe implementation details;
- raw-pointer rules;
- unchecked construction and representation conversion;
- FFI and inline assembly trust boundaries;
- addressed storage and target knowledge;
- interaction with ownership, borrowing, arenas, effects, cleanup, generics,
  interfaces, and function values;
- trust provenance;
- diagnostics, formatter, LSP, tests, and implementation requirements.

Sec 0.1 does not define:

```text
unsafe struct
unsafe type
```

Unsafe applies to operations, caller contracts, and foreign or machine-level
trust boundaries.

It does not classify an entire data shape as inherently unsafe.

---

# Core principle

`unsafe` disables no compiler analysis.

It permits a compiler-known operation whose complete safety obligations cannot
be proven automatically.

The programmer accepts only the specific proof obligations attached to that
operation.

Unsafe does not mean:

```text
stop type checking;
stop ownership checking;
stop borrow checking;
stop effect analysis;
stop contract checking;
ignore moves or invalidation;
ignore cleanup rules;
ignore target rules;
permit arbitrary backend undefined behavior.
```

The compiler continues every analysis that remains applicable.

---

# Unsafe terminology

Sec distinguishes four concepts.

## Unsafe operation

An operation whose safety depends on conditions the compiler cannot fully
prove.

Examples include:

```text
raw-pointer dereference;
raw-pointer write;
raw-pointer arithmetic;
constructing a reference from a raw pointer;
constructing a slice from raw parts;
calling an unsafe function;
calling an unsafe foreign declaration;
inline assembly;
unchecked representation construction;
unchecked union or enum tag construction.
```

## Unsafe context

A lexically explicit source context in which unsafe operations may be written.

An unsafe context does not make ordinary surrounding operations unchecked.

## Unsafe function

A Sec function or method whose caller must satisfy additional proof obligations
that the compiler cannot fully verify at the call site.

## Trusted declaration

A declaration whose implementation or target fact lies outside ordinary Sec
source verification.

Examples include:

```text
foreign functions;
inline assembly contracts;
raw numeric target addresses;
compiler intrinsics;
target knowledge-pack facts.
```

---

# Unsafe context syntax

Sec supports both a single-operation form and a block form.

## Single-operation form

```sec
unsafe pointer.Write(value)
```

The unsafe context applies only to the immediately following operation.

The operation may be an expression statement or an expression used inside a
larger valid grammatical context.

Examples:

```sec
unsafe pointer.Write(value)
```

```sec
let value := unsafe pointer.Read()
```

```sec
return unsafe pointer.Read()
```

## Block form

```sec
let value := unsafe {
    pointer.Read()
}
```

Use braces when the unsafe context contains more than one operation or requires
multiple statements.

Example:

```sec
let value := unsafe {
    ValidateRawState(pointer)
    pointer.Read()
}
```

An unsafe block is an expression when its body produces a value according to
ordinary block-expression rules.

---

# Braces

Braces are optional for one unsafe operation.

Braces are required when the unsafe context contains more than one operation or
statement.

Valid:

```sec
unsafe pointer.Write(value)
```

Valid:

```sec
let value := unsafe {
    let raw := pointer.Read()
    Convert(raw)
}
```

Invalid:

```sec
unsafe
    Validate(pointer)
    pointer.Write(value)
```

The multi-operation form must use braces.

---

# Scope of an unsafe context

An unsafe context permits only compiler-classified unsafe operations within its
lexical extent.

It does not:

```text
change variable visibility;
extend lifetimes;
change move semantics;
change effect guarantees;
change target selection;
change error propagation;
make nested function bodies unsafe;
make called safe functions unsafe;
make later statements unsafe.
```

A closure or nested function declared inside an unsafe block does not inherit an
unsafe body merely because its declaration occurs there.

Its own unsafe operations still require explicit unsafe context.

---

# `unsafe fn`

Sec supports unsafe functions and methods.

Canonical form:

```sec
unsafe fn FromRawParts(
    pointer: RawPtr[byte],
    length: int,
) Buffer {
    // ...
}
```

An unsafe function states:

> Calling this function requires proof obligations that ordinary Sec typing and
> analysis do not fully establish.

Calling an unsafe function requires an unsafe context.

Example:

```sec
let buffer := unsafe Buffer.FromRawParts(pointer, length)
```

or:

```sec
let buffer := unsafe {
    Buffer.FromRawParts(pointer, length)
}
```

---

# Unsafe function bodies

The body of an `unsafe fn` is not implicitly an unsafe context.

Unsafe operations inside the function body remain explicit.

Example:

```sec
unsafe fn ReadRaw(pointer: RawPtr[byte]) byte {
    return unsafe pointer.Read()
}
```

This is intentionally more explicit than treating the entire body as unsafe.

It shows exactly which implementation operations depend on the caller's proof
obligations.

Invalid:

```sec
unsafe fn ReadRaw(pointer: RawPtr[byte]) byte {
    return pointer.Read()
}
```

when `RawPtr.Read()` is classified as unsafe.

---

# Caller proof obligations

Every unsafe function must have defined caller obligations.

Typical obligations include:

```text
pointer is non-null where required;
pointer is correctly aligned;
storage is valid for the required size;
storage contains initialized values of the declared representation;
storage remains live for the required lifetime;
read or write access is permitted;
no conflicting mutable or shared access exists;
length and capacity are correct;
foreign ABI assumptions are correct;
the operation is valid for the selected target;
required synchronization has been established.
```

The obligations are normative even when they are described in documentation
rather than encoded directly in the type system.

Violating an unsafe obligation removes only the guarantees that depend on that
obligation, subject to unavoidable backend consequences.

---

# Safety documentation

Public or reusable unsafe APIs should document their caller obligations.

Recommended form:

```sec
/**
 * Creates a buffer over foreign storage.
 *
 * Safety:
 * - pointer must be non-null and correctly aligned;
 * - pointer must be valid for length initialized bytes;
 * - storage must remain live for the buffer lifetime;
 * - no conflicting owner or mutable reference may exist.
 */
unsafe fn FromRawParts(
    pointer: RawPtr[byte],
    length: int,
) Buffer {
    // ...
}
```

A `Safety:` section is strongly recommended.

It is not a hard grammar or language requirement in Sec 0.1.

Tooling may report missing safety documentation according to project policy.

The default diagnostic may be informational and configurable.

---

# `unsafe extern`

Canonical foreign declaration form:

```sec
unsafe extern "system" fn rawSysCall(
    number: int,
    argument1: uint64,
) int64
```

The tokens have separate meanings:

```text
unsafe
    caller has additional proof obligations;

extern "system"
    implementation uses foreign system linkage;

fn
    declares a callable function.
```

`unsafe extern` is not one indivisible keyword.

Other linkage forms remain defined by the FFI rulebook.

Bare foreign `...` is defined only by the FFI rules for final C varargs and is
not native Sec typed-variadic syntax.

Examples include:

```sec
unsafe extern "C" fn CCreateContext()
    Result[RawPtr[Context], NullError]
```

```sec
unsafe extern "system" fn rawSysCall(
    number: int,
    argument1: uint64,
) int64
```

---

# Calling unsafe extern functions

Calling an unsafe extern function requires an unsafe context.

Example:

```sec
let result := unsafe rawSysCall(number, argument)
```

The unsafe call does not automatically establish:

```text
correct arguments;
correct pointer validity;
correct ownership;
correct foreign lifetime;
correct effect claims;
correct ABI declaration.
```

Those obligations belong to the declaration and call site.

---

# Safe wrappers

A safe Sec function may use unsafe operations internally and present a safe API
when it validates or establishes every caller obligation.

Example:

```sec
fn CreateBuffer(
    pointer: RawPtr[byte],
    length: int,
) Result[Buffer, BufferError] {
    if pointer.IsNull() {
        return Err(BufferError.NullPointer)
    }

    if length < 0 {
        return Err(BufferError.InvalidLength)
    }

    let buffer := unsafe Buffer.FromRawParts(pointer, length)
    return Ok(buffer)
}
```

The caller of `CreateBuffer` does not require unsafe context if the wrapper
fully establishes the contract.

---

# Unsafe does not propagate through safe wrappers

Unsafe is not a transitive caller marker.

A safe function remains safe to call when it correctly encapsulates unsafe
implementation details.

Conceptually:

```text
unsafe implementation operation
    may be encapsulated by a safe function

safe caller
    does not inherit unsafe status
```

Actual effects still propagate.

A safe wrapper around a blocking foreign call remains blocking.

A safe wrapper around an allocating intrinsic remains allocating.

A safe wrapper around volatile access still has volatile-access effects.

---

# Trust provenance through wrappers

A safe wrapper does not erase internal trust provenance.

The compiler may record:

```text
caller-facing safety:
    safe

implementation provenance:
    depends on unsafe foreign call
```

This information may be used for:

```text
auditing;
security review;
ISR reports;
LSP navigation;
whole-program trust reports;
diagnostics;
certification profiles.
```

Trust provenance is not automatically part of the source-level function type.

---

# Rules unsafe cannot bypass

Unsafe context does not automatically permit:

```text
use after move;
copy of an @noCopy value;
use after arena reset;
use after arena release;
reference escape from local or arena storage;
conflicting ref and ref mut borrows;
ordinary type mismatch;
ignored required Result handling;
unhandled constrained-type assignment;
contract violation;
violation of @noPanic;
violation of @noAlloc;
violation of @noBlock;
blocking or suspension in ISR code;
panic-capable destructor or defer;
proven unreachable code;
invalid target selection;
invalid visibility access.
```

Only the precise unsafe operation transfers a specific proof obligation.

---

# Unsafe and effect analysis

Unsafe status and effects are separate dimensions.

Examples:

```text
safe and MayAllocate;
unsafe and noAlloc;
unsafe and MayBlock;
safe and MayAccessVolatile.
```

An unsafe context does not remove effects.

Example:

```sec
@noAlloc
fn Work() void {
    unsafe allocator.New[Value]()
}
```

This remains invalid because the operation contributes `MayAllocate`.

Example:

```sec
@noBlock
fn Work() void {
    unsafe BlockingForeignCall()
}
```

This remains invalid because the call contributes `MayBlock`.

---

# Unsafe and `@noPanic`

Unsafe does not make panic-capable code compatible with `@noPanic`.

Example:

```sec
@noPanic
fn Work() void {
    unsafe PanicCapableOperation()
}
```

The function remains invalid unless effect analysis proves the operation cannot
panic or an explicit trusted foreign contract supplies a valid no-panic claim.

Trust provenance must remain visible when such a claim is accepted.

---

# Unsafe and `@noAlloc`

Unsafe does not permit allocation inside `@noAlloc`.

This includes:

```text
heap allocation;
arena capacity consumption;
collection growth;
boxing;
closure environment allocation;
foreign allocation;
compiler helper allocation.
```

---

# Unsafe and `@noBlock`

Unsafe does not permit synchronous blocking inside `@noBlock`.

It also does not permit blocking or suspension inside `@isr` or
`@interruptSafe`.

---

# Unsafe and `try`

`try` and unsafe solve different problems.

```text
try
    converts supported failure into explicit fallible control flow;

unsafe
    accepts a proof obligation the compiler cannot fully establish.
```

An unsafe call may still return `Result`.

Example:

```sec
let result := try unsafe ForeignOperation(pointer)
```

The unsafe context accepts the foreign safety obligations.

`try` handles the declared explicit error.

Neither substitutes for the other.

---

# `RawPtr[T]`

`RawPtr[T]` is the primary boundary between safe Sec references and raw address
handling.

Possessing a `RawPtr[T]` is not automatically unsafe.

The unsafe classification depends on the operation performed with it.

---

# Safe raw-pointer operations

The following may be safe when their dedicated rule permits them:

```text
store a RawPtr value;
move a RawPtr value;
return a RawPtr value;
pass a RawPtr through FFI;
compare raw pointers for equality;
test a raw pointer for null;
convert between explicitly representation-compatible raw-pointer types without
  dereference;
format or inspect an address for diagnostics where permitted.
```

These operations do not assert that the pointed-to storage is valid.

---

# Unsafe raw-pointer operations

The following require unsafe context:

```text
dereference;
read through RawPtr;
write through RawPtr;
pointer arithmetic;
convert an arbitrary integer to RawPtr;
construct ref T;
construct ref mut T;
construct a slice from pointer and length;
assume alignment;
assume initialized representation;
adopt ownership from a pointer;
construct a callable function pointer from a raw address;
reinterpret unrelated pointer representations when validity is not proven.
```

The list is compiler-known and may grow through later rulebooks.

---

# Raw-pointer read

Conceptual:

```sec
let value := unsafe pointer.Read()
```

The programmer must guarantee at least:

```text
pointer is non-null where required;
pointer is aligned for T;
pointer addresses readable storage;
storage contains a valid initialized T;
storage remains live for the operation;
read does not violate ownership or alias rules;
address space permits the access.
```

The operation may also contribute effects such as:

```text
MayAccessVolatile;
MayIO;
MayUseNondeterministicInput.
```

The pointer operation definition determines the effects.

---

# Raw-pointer write

Conceptual:

```sec
unsafe pointer.Write(value)
```

The programmer must guarantee at least:

```text
pointer is non-null where required;
pointer is aligned for T;
pointer addresses writable storage;
storage is valid for one T;
write does not violate exclusive-access rules;
write does not overwrite a live value without valid destruction semantics;
address space permits the access.
```

---

# Raw pointer to shared reference

Constructing `ref T` from `RawPtr[T]` is unsafe.

The caller must guarantee:

```text
non-null where required;
correct alignment;
valid initialized T representation;
readable storage;
storage remains live for the complete borrow live range;
no conflicting mutable reference exists;
foreign code will not mutate the storage contrary to the shared-reference
  contract;
the reference does not escape its valid lifetime.
```

Unsafe context permits the construction.

It does not make the obligations optional.

---

# Raw pointer to mutable reference

Constructing `ref mut T` from `RawPtr[T]` is unsafe.

The caller must additionally guarantee:

```text
exclusive access for the complete mutable borrow live range;
writable storage;
no shared references conflict;
no foreign or hardware mutation violates exclusivity unless the type and
  volatile model explicitly permit it;
correct destruction and replacement behavior.
```

---

# Slice from raw parts

Constructing a slice or array view from raw pointer and length is unsafe.

Typical obligations:

```text
pointer is valid for length elements;
length is non-negative and representable;
required byte-size multiplication does not overflow;
all elements satisfy representation requirements;
alignment is correct;
storage remains live for the complete slice lifetime;
alias and mutability rules are satisfied;
the memory range belongs to one compatible allocation or target region where
  required.
```

---

# Pointer arithmetic

Raw-pointer arithmetic is unsafe.

It does not create a valid reference by itself.

The caller must establish:

```text
correct unit: bytes or elements as defined by the operation;
address calculation does not wrap into an invalid address;
result remains within the permitted storage object or target range;
alignment remains valid for later typed access;
object provenance remains valid where required by backend semantics.
```

Exact raw-pointer arithmetic APIs belong to the pointer rulebook.

---

# Null pointers

Testing a raw pointer for null may be safe.

Dereferencing null is never made valid by an unsafe context.

Unsafe merely transfers the proof obligation that null cannot be reached.

A compiler-proven null dereference remains a compile error even inside unsafe
code.

---

# Uninitialized storage

Reading uninitialized storage as `T` is invalid unless a dedicated unsafe
operation defines a valid uninitialized representation workflow.

Unsafe context does not automatically make every bit pattern a valid `T`.

A type may have:

```text
invalid bit patterns;
invalid enum tags;
invalid union states;
contract requirements;
ownership invariants;
foreign layout requirements.
```

---

# Unchecked construction

Bypassing normal construction, validation, or contracts is unsafe.

Conceptual:

```sec
let value := unsafe Percent.FromRepresentationUnchecked(raw)
```

The caller assumes responsibility for every invariant normally established by
construction.

If the value does not satisfy the invariant, the relevant Sec safety guarantees
no longer apply.

---

# Unsafe constructors

Types with invariant-bearing private or hidden fields should expose unsafe
constructors rather than making the entire type unsafe.

Example:

```sec
type RawSlice[T] struct {
    _pointer: RawPtr[T]
    _length: int
}

impl RawSlice[T] {
    unsafe fn FromRawParts(
        pointer: RawPtr[T],
        length: int,
    ) RawSlice[T] {
        return RawSlice[T] {
            _pointer: pointer,
            _length: length,
        }
    }
}
```

The unsafe boundary belongs to construction or use.

The type itself is not declared `unsafe`.

---

# No `unsafe struct`

Sec 0.1 does not define:

```sec
unsafe type RawSlice[T] struct {
}
```

or:

```sec
unsafe struct RawSlice[T] {
}
```

A struct is a data shape.

Its existence is not automatically an unsafe operation.

The safety boundary belongs to:

```text
construction;
raw interpretation;
methods with caller obligations;
FFI interaction;
layout assumptions;
dereference;
ownership adoption.
```

---

# Types containing `RawPtr`

A type containing `RawPtr` is not automatically unsafe.

Example:

```sec
type RawHandle struct {
    pointer: RawPtr[void]
}
```

It may be safe to:

```text
store the handle;
move the handle;
compare its pointer with null;
return it;
pass it back to foreign code.
```

Operations that interpret or dereference the pointer remain unsafe.

---

# No `unsafe type`

A named type is not declared globally unsafe in Sec 0.1.

Unsafe behavior is expressed through:

```text
unsafe functions;
unsafe methods;
unsafe constructors;
unsafe raw operations;
unsafe foreign declarations;
inline assembly;
trusted target declarations.
```

This keeps safety boundaries operation-specific.

---

# Layout

Foreign or hardware layout requirements use dedicated layout and target
mechanisms.

They do not use `unsafe struct`.

Conceptual future form:

```sec
@layout("C")
type NativePoint struct {
    x: int32
    y: int32
}
```

Exact layout syntax belongs to the layout and FFI rulebooks.

A compiler-verified layout attribute may be safe.

An unverified layout assumption carries trust provenance.

---

# `@address`

`@address` does not use an `unsafe` modifier.

Canonical:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

or:

```sec
@address(0x40021000)
let mut GPIOA: GPIORegisters
```

The declaration itself is already an explicit target binding.

---

# Knowledge-pack addresses

A named target knowledge-pack address may be compiler-verified.

The compiler may validate:

```text
device availability;
address;
alignment;
address space;
register-block compatibility;
read and write capability;
access width;
reserved status.
```

When fully verified, ordinary typed access does not require unsafe context.

---

# Raw numeric addresses

A raw numeric `@address` value is valid only for the selected target's canonical
linear hardware address domain. It is not an unchecked or merely trusted
assertion. The compiler must validate the complete binding against canonical
target, platform, device, or project facts, including:

```text
address-domain applicability;
canonical region coverage;
complete bound extent;
alignment;
address space;
access width and permissions;
layout and representation compatibility;
known overlap and alias policy.
```

If those facts cannot be resolved, the `@address` declaration is rejected.
Neither an unsafe context nor raw numeric spelling waives validation.

---

# Typed addressed access

After an addressed declaration has been accepted, ordinary typed access does not
require unsafe context.

Example:

```sec
let status := GPIOA.status
GPIOA.control := command
```

The access remains:

```text
typed;
volatile;
checked against mutability;
checked against register layout;
tracked by effect analysis.
```

---

# Raw target access

Runtime-discovered hardware addresses use the checked mapping/resource model in
`rules/platform/hardware-register-access.md`, or explicit `RawPtr[T]` and unsafe
operations under the applicable low-level contract.

Converting an arbitrary integer address to `RawPtr[T]` and dereferencing it is
unsafe. Possessing the `RawPtr[T]` does not grant hardware privilege, mapping
authority, security-domain authority, resource ownership, or canonical endpoint
identity.

Example:

```sec
let pointer := unsafe RawPtr[Register].FromAddress(address)
let value := unsafe pointer.Read()
```

The dedicated `@address` declaration is preferred for stable module-level
hardware storage.

---

# Interrupt vectors

A named interrupt vector from target knowledge may be compiler-verified.

A raw numeric vector may still be validated against the selected target.

`@interrupt(vector: value)` does not automatically require unsafe syntax.

Unknown or unverified target facts retain trust provenance or are rejected
according to the target rulebook.

---

# FFI trust boundary

Foreign code is outside ordinary Sec body verification.

The compiler must not infer from `extern` alone that the function is:

```text
memory-safe;
noPanic;
noAlloc;
noBlock;
interrupt-safe;
non-retaining;
thread-safe;
ABI-correct.
```

The FFI rulebook defines the declaration contract.

---

# Foreign effect summaries

Unknown foreign effects are conservative.

Unless explicitly declared through a future trusted FFI effect form, a foreign
call may contribute:

```text
MayPanic or foreign abort/unwind;
MayAllocate;
MayBlock;
MaySuspend where applicable;
MaySpawn;
MayIO;
MayAccessVolatile;
MayMutateExternalState;
MayUseNondeterministicInput.
```

Unsafe context does not remove these effects.

---

# Trusted foreign effect claims

A future FFI form may declare narrower effects.

Conceptual only:

```sec
@noPanic
@noAlloc
@noBlock
unsafe extern "C" fn ReadRegister(
    address: RawPtr[uint32],
) uint32
```

For foreign code, these are trusted claims rather than proof from a Sec body.

The exact source syntax for trusted foreign effect claims is not decided here.

The compiler must retain provenance:

```text
effect guarantee trusted from foreign declaration
```

---

# Foreign ownership contracts

Foreign declarations may require facts such as:

```text
pointer borrowed only for the call;
pointer retained after return;
ownership transferred to foreign code;
ownership returned to Sec;
foreign code may mutate storage;
foreign code may invoke callback later;
foreign code may call concurrently;
foreign code may free storage.
```

Unknown ownership behavior is conservative.

Exact FFI annotation syntax belongs to the FFI rulebook.

---

# Inline assembly

Inline assembly is an unsafe operation and a trust boundary.

It requires explicit unsafe context unless a dedicated declaration form already
contains the unsafe marker according to the inline-assembly grammar.

The compiler cannot infer arbitrary machine-code behavior from assembly text.

---

# Inline assembly contract

Inline assembly must describe enough information for compiler correctness.

At minimum, the model must support:

```text
register inputs;
register outputs;
register clobbers;
memory reads;
memory writes;
volatile behavior;
control-flow behavior;
stack behavior;
possible trap or abort;
possible blocking;
possible I/O;
possible external mutation.
```

Exact syntax belongs to the inline-assembly rulebook.

---

# Inline assembly effects

Inline assembly effects remain active inside unsafe context.

Example:

```sec
@noBlock
fn Work() void {
    unsafe {
        asm ...
    }
}
```

The function is invalid when the asm contract includes blocking behavior.

---

# Compiler intrinsics

A compiler intrinsic may be classified as:

```text
safe;
unsafe;
compiler-verified for selected targets;
trusted target operation.
```

Unsafe intrinsics require unsafe context.

Their effect summaries come from compiler definitions and target knowledge.

---

# Interfaces

Unsafe status is part of interface method semantics.

Conceptual:

```sec
interface RawStorage {
    unsafe fn Adopt(
        pointer: RawPtr[byte],
        length: int,
    ) Buffer
}
```

Every implementation must preserve the caller-facing unsafe contract.

---

# Interface compatibility

A safe interface method may not be implemented by a method that requires
additional unsafe caller obligations.

Invalid:

```text
interface method:
    safe

implementation:
    unsafe
```

An implementation may safely implement an unsafe interface operation internally,
but calls through the interface remain unsafe because callers only know the
interface contract.

---

# Function values

Unsafe status is part of function-value compatibility.

An unsafe function must not silently convert to an ordinary safe function value.

Conceptually:

```text
unsafe fn(T) R
    is not assignable to
fn(T) R
```

A safe wrapper may expose a safe function value after establishing all required
conditions.

Exact function-type syntax for unsafe callables may be defined separately.

---

# Callbacks

A callback contract must preserve whether invocation is unsafe.

Generic or interface code must not call a callback with unknown safety status as
though it were safe.

An unsafe callback invocation requires unsafe context and satisfaction of the
callback's caller obligations.

---

# Generics

Unsafe status is preserved through generic constraints and specialization.

Generic code may invoke an operation safely only when the generic contract proves
that invocation is safe.

Otherwise:

```text
invocation requires unsafe context;
or
the generic code must reject the operation.
```

The compiler must not infer a stable safe callable contract only because current
specializations happen to be safe.

---

# Unsafe and ownership

Unsafe does not disable ownership analysis.

The compiler still enforces:

```text
move validity;
source invalidation;
ownership transfer;
double-destruction prevention;
@noCopy policy;
owner lifetime;
arena ownership;
value initialization state.
```

A dedicated unsafe operation may transfer responsibility for one ownership fact,
such as adopting a foreign allocation.

That operation must define the obligation precisely.

---

# Ownership adoption

Creating an owned Sec value from raw or foreign storage is unsafe unless a
compiler-known API proves the ownership transfer.

Typical obligations:

```text
caller owns the storage;
no other owner will release it;
correct allocator and deallocator are paired;
storage contains a valid value;
foreign code no longer retains ownership;
destruction may run exactly once.
```

---

# Unsafe and borrowing

Unsafe does not globally disable borrow checking.

Normal `ref` and `ref mut` conflicts remain errors.

Constructing a reference from raw storage transfers the obligation to establish:

```text
valid lifetime;
valid aliasing;
valid mutability;
valid storage;
no conflicting foreign access.
```

After the reference is constructed, ordinary Sec borrow rules apply.

---

# Unsafe and escape analysis

Unsafe does not permit returning a reference to dead storage.

If the compiler can prove that a reference escapes invalid storage, it rejects
the program even inside unsafe context.

Unsafe construction may accept a lifetime assumption the compiler cannot prove,
but an explicit contradiction remains an error.

---

# Unsafe and arenas

Unsafe does not permit:

```text
use after arena reset;
use after arena release;
reference escape beyond arena lifetime;
allocation from a released arena.
```

An unsafe operation may construct an arena-backed reference only when the caller
accepts the corresponding lifetime obligation.

Effect and arena analyses continue afterward.

---

# Unsafe and copy/move

Unsafe does not permit implicit copy of an `@noCopy` type.

Unsafe does not permit use after explicit move.

Unsafe does not change whether a type is:

```text
copyable;
move-only;
relocatable;
pinned.
```

Dedicated low-level operations may define explicit representation transfer, but
they must not silently become ordinary copy semantics.

---

# Unsafe and contracts

Unsafe construction may bypass normal contract checking only through a dedicated
unchecked operation.

An unsafe context around ordinary construction does not suppress the contract.

Invalid conceptual usage:

```sec
let value := unsafe Percent(1000)
```

when ordinary `Percent` construction validates its contract.

Use an explicitly unsafe unchecked constructor when such a low-level operation
is intentionally provided.

---

# Unsafe and destruction

Unsafe operations may appear in destructors only through explicit unsafe
context.

Example:

```sec
unsafe ForeignRelease(pointer)
```

The destructor remains subject to every destruction rule.

In particular:

```text
destructor remains noPanic;
effect guarantees remain active;
double release remains invalid;
invalid ownership remains invalid.
```

---

# Unsafe and `defer`

Unsafe operations in deferred code require explicit unsafe context.

Example:

```sec
defer {
    unsafe ForeignRelease(pointer)
}
```

The deferred body remains subject to:

```text
noPanic cleanup requirements;
@noAlloc where applicable;
@noBlock where applicable;
ownership validity;
call-graph effects.
```

---

# Panic during unsafe cleanup

Unsafe does not permit cleanup code to panic when cleanup is required to be
noPanic.

A foreign release function with unknown panic or unwind behavior cannot be used
in such cleanup without a valid trusted contract.

---

# Compile-time evaluation

Ordinary compile-time evaluation rejects unsafe operations.

The compiler does not ordinarily execute:

```text
raw-pointer dereference;
FFI call;
inline assembly;
addressed storage access;
raw target memory access;
unsafe ownership adoption.
```

A compiler-known intrinsic may be compile-time evaluable only when explicitly
specified.

Unsafe context does not force compile-time execution.

---

# Target-dependent unsafe code

Unsafe code is analyzed after target and configuration selection.

Only active source contributes to the active unsafe and effect graph.

A trusted claim may be valid on one target and invalid on another.

Declared guarantees must hold for every active target variant.

---

# Redundant unsafe contexts

Nested unsafe context is valid but redundant when the inner context adds no new
lexical boundary.

Example:

```sec
unsafe {
    unsafe pointer.Write(value)
}
```

Default diagnostic:

```text
information: redundant unsafe context
```

The diagnostic may be promoted to warning through configuration.

---

# Unsafe context without unsafe operations

An unsafe context containing no unsafe operation is valid.

Example:

```sec
unsafe {
    let value := 10
}
```

Default diagnostic:

```text
information: unsafe context contains no unsafe operation
```

The diagnostic may be promoted to warning through configuration.

This assists refactoring and keeps unsafe boundaries narrow.

---

# Unsafe scope size

Small unsafe contexts are recommended.

Preferred:

```sec
let value := unsafe pointer.Read()
```

Less desirable:

```sec
unsafe {
    // Hundreds of lines of mostly ordinary code.
}
```

Large unsafe contexts remain valid unless another rule is violated.

Tooling may report an informational or configurable warning based on project
policy.

---

# Formatter behavior

The formatter preserves explicit unsafe boundaries.

It must not:

```text
remove unsafe;
add unsafe automatically;
expand one-operation unsafe into a block without formatting need;
collapse a multi-operation block into an invalid short form;
move ordinary code into or out of unsafe context;
rewrite safety semantics.
```

Canonical examples:

```sec
let value := unsafe pointer.Read()
```

```sec
let value := unsafe {
    Validate(pointer)
    pointer.Read()
}
```

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

---

# Modifier order

Sec currently does not assume a `pub` modifier.

Visibility and scope use the language's established `_` and `__` conventions.

Canonical unsafe foreign order is:

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

Canonical Sec unsafe function order is:

```sec
unsafe fn FromRawParts(...) Buffer {
}
```

Future modifiers must define their order without changing these meanings.

---

# Parser representation

Unsafe syntax should have explicit AST representation.

Conceptual:

```go
type UnsafeExpression struct {
    Token      lexer.Token
    Expression ast.Expression
}

type UnsafeBlockExpression struct {
    Token lexer.Token
    Body  *ast.BlockStatement
}
```

Function declarations should retain:

```go
IsUnsafe bool
```

Foreign linkage remains separate.

---

# Semantic representation

Sema should represent:

```text
unsafe operation kind;
unsafe lexical context;
unsafe caller contract;
trust provenance;
effect summary;
source location;
required obligations;
resolved target facts.
```

Unsafe and effect properties must not be stored as one boolean.

---

# Unsafe operation registry

The compiler should maintain a central registry of unsafe operation kinds.

Each entry should define:

```text
stable internal identity;
source operation;
required unsafe context;
caller obligations;
effects;
trust provenance;
allowed targets;
diagnostics;
safe alternatives where available.
```

Do not distribute unsafe classification through unrelated ad hoc checks.

---

# Initial unsafe operation set

The initial compiler-known set includes:

```text
RawPointerRead
RawPointerWrite
RawPointerArithmetic
RawPointerFromInteger
ReferenceFromRawPointer
MutableReferenceFromRawPointer
SliceFromRawParts
AssumeAlignment
AssumeInitialized
AdoptRawOwnership
UncheckedTypeConstruction
UncheckedRepresentationCast
UncheckedEnumTag
UncheckedUnionTag
UnsafeFunctionCall
UnsafeForeignCall
InlineAssembly
UnsafeIntrinsic
CallableFromRawAddress
```

Exact internal names may differ.

The semantic categories must remain distinguishable.

---

# Diagnostics

Suggested diagnostic families:

```text
unsafe.required
unsafe.unnecessary
unsafe.redundant
unsafe.unclosed
unsafe.invalid-context
unsafe.call-requires-context
unsafe.operation-requires-context
unsafe.function-body-still-explicit
unsafe.interface-mismatch
unsafe.function-value-mismatch
unsafe.generic-constraint-unknown
unsafe.foreign-effect-unknown
unsafe.foreign-ownership-unknown
unsafe.raw-pointer-null
unsafe.raw-pointer-alignment
unsafe.raw-pointer-lifetime
unsafe.raw-pointer-alias
unsafe.raw-pointer-range
unsafe.uninitialized-read
unsafe.invalid-representation
unsafe.unchecked-construction
unsafe.target-trust
unsafe.address-unverified
unsafe.cleanup-effect
unsafe.compile-time-forbidden
unsafe.missing-safety-documentation
```

Final stable IDs belong to the diagnostics registry.

---

# Required diagnostic quality

Diagnostics should state:

```text
which operation is unsafe;
why unsafe is required;
which proof obligation the programmer accepts;
which effect remains active;
which target or foreign claim is trusted;
where the nearest unsafe context begins;
which safe alternative exists, when canonical.
```

Example:

```text
raw pointer dereference requires unsafe context

required obligations:
    pointer must be aligned for Value
    storage must contain an initialized Value
    storage must remain live for the read

help:
    use `unsafe pointer.Read()` after establishing these conditions
```

---

# Effect diagnostic example

```text
error: `ReadStatus` does not satisfy @noBlock

ReadStatus
  -> calls unsafe foreign function `device_read`
  -> foreign declaration may block

unsafe context does not suppress MayBlock
```

---

# Provenance diagnostic example

```text
information: function safety depends on a trusted raw target address

address declaration:
    hardware.sec:12

selected target:
    device = controller-a
```

---

# LSP behavior

The LSP should display:

```text
unsafe-operation explanation;
caller obligations;
unsafe function status;
foreign trust status;
effect summary;
trust provenance;
nearest unsafe context;
safe-wrapper boundary;
raw-address verification status;
target knowledge source;
interface and function-value compatibility.
```

---

# LSP completion

Inside an unsafe context, completion remains ordinary completion.

The LSP may additionally identify operations that are permitted only because of
the unsafe context.

At an unsafe call site, completion may show:

```text
Unsafe function
Caller obligations:
    pointer valid for length bytes
    storage live for returned buffer lifetime
```

---

# LSP code actions

Possible code actions include:

```text
wrap operation in `unsafe ...`;
wrap multiple selected statements in `unsafe { ... }`;
navigate to safety documentation;
create a safe wrapper skeleton;
show trusted-effect chain;
replace raw address with knowledge-pack name where an exact verified mapping
  exists;
remove redundant unsafe context;
```

The LSP must not insert unsafe automatically without an explicit user action.

---

# No mandatory runtime

Unsafe is a compile-time language and analysis feature.

It does not require:

```text
runtime unsafe flags;
dynamic trust checks;
mandatory pointer metadata;
mandatory provenance objects;
mandatory generational references;
exception machinery;
a general Sec runtime.
```

Specific profiles may add runtime checks independently.

---

# Relationship to backend undefined behavior

Sec should minimize backend undefined behavior.

Unsafe is not permission to lower arbitrary source into unrestricted backend UB.

Where practical, unsafe operations should lower to:

```text
defined target instructions;
explicit traps;
validated intrinsics;
well-specified foreign ABI operations;
carefully bounded backend assumptions.
```

When a false unsafe obligation necessarily violates backend assumptions, the
compiler and documentation must describe the risk honestly.

Only the guarantees depending on the violated obligation are intentionally
surrendered at the language level, though memory corruption may cause wider
physical consequences.

---

# Current implementation status

## Known implemented or established syntax

The current language direction already uses forms such as:

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

and:

```sec
unsafe extern "C" fn CCreateContext()
    Result[RawPtr[Context], NullError]
```

`RawPtr[T]` is the established foreign raw-pointer abstraction.

The exact repository implementation must be verified before changing status
entries.

---

# Partly implemented

Likely existing foundations include:

```text
unsafe token or modifier handling;
extern linkage declarations;
RawPtr type recognition;
FFI declaration parsing;
inline assembly planning or partial support;
addressed-storage parsing;
ownership and borrow analysis foundations;
effect and call-graph foundations.
```

Codex must inspect current repository state rather than assume completion.

---

# Not implemented unless repository code proves otherwise

```text
single-operation unsafe expression;
unsafe block expression;
unsafe fn for Sec bodies;
explicit unsafe-call checking;
unsafe function-value compatibility;
unsafe interface compatibility;
unsafe generic callable constraints;
central unsafe-operation registry;
caller-obligation metadata;
trust provenance graph;
raw-pointer operation classification;
unchecked constructor classification;
foreign effect trust declarations;
foreign ownership contracts;
inline assembly effect trust integration;
raw numeric address provenance;
unsafe-aware formatter;
unsafe-aware LSP;
complete diagnostics;
compile-time evaluation rejection;
trust reports.
```

---

# Required tests

Create or update:

```text
unsafe_valid.sec
unsafe_invalid.sec
unsafe_functions_valid.sec
unsafe_functions_invalid.sec
unsafe_extern_valid.sec
unsafe_extern_invalid.sec
unsafe_raw_ptr_valid.sec
unsafe_raw_ptr_invalid.sec
unsafe_construction_valid.sec
unsafe_construction_invalid.sec
unsafe_interfaces_valid.sec
unsafe_interfaces_invalid.sec
unsafe_function_values_valid.sec
unsafe_function_values_invalid.sec
unsafe_generics_valid.sec
unsafe_generics_invalid.sec
unsafe_effects_valid.sec
unsafe_effects_invalid.sec
unsafe_cleanup_valid.sec
unsafe_cleanup_invalid.sec
unsafe_address_valid.sec
unsafe_address_invalid.sec
unsafe_compile_time_valid.sec
unsafe_compile_time_invalid.sec
```

---

# Syntax tests

Test:

```text
single-operation unsafe statement;
single-operation unsafe expression;
unsafe block expression;
block with multiple statements;
block returning value;
unclosed unsafe block;
unsafe without operation;
redundant nested unsafe;
unsafe fn;
unsafe method;
unsafe extern "system";
unsafe extern "C".
```

---

# Raw-pointer tests

Test:

```text
store RawPtr safely;
move RawPtr safely;
compare RawPtr with null safely;
pass RawPtr through FFI safely;
read requires unsafe;
write requires unsafe;
arithmetic requires unsafe;
integer conversion requires unsafe;
ref construction requires unsafe;
ref mut construction requires unsafe;
slice from raw parts requires unsafe;
explicit null contradiction rejected;
explicit invalid alignment rejected;
use after known lifetime rejected;
conflicting known alias rejected;
uninitialized read rejected.
```

---

# Unsafe function tests

Test:

```text
unsafe call outside context rejected;
unsafe call in single-operation context accepted;
unsafe call in block accepted;
unsafe function body still requires explicit context;
safe wrapper accepted;
unsafe status does not propagate to safe caller;
effects do propagate through safe wrapper;
caller obligations shown in diagnostics and LSP.
```

---

# Interface and function-value tests

Test:

```text
safe interface method cannot have unsafe implementation;
unsafe interface method may have safe internal implementation;
call through unsafe interface remains unsafe;
unsafe function does not convert to safe function value;
safe wrapper can produce safe callable;
unknown generic callback safety is conservative.
```

---

# Effect tests

Test:

```text
unsafe allocation violates @noAlloc;
unsafe blocking call violates @noBlock;
unsafe panic-capable call violates @noPanic;
unsafe suspension violates @isr;
volatile effect remains through safe wrapper;
foreign unknown effects remain conservative;
trusted effect provenance is retained.
```

---

# Cleanup tests

Test:

```text
unsafe operation in defer requires context;
unsafe operation in destructor requires context;
cleanup remains noPanic;
blocking foreign release rejected in @noBlock cleanup;
unknown foreign unwind rejected in destructor;
double release remains rejected.
```

---

# Address tests

Test:

```text
knowledge-pack address verified;
raw numeric address accepted with trust provenance;
ordinary typed addressed access does not require unsafe;
raw integer-to-pointer conversion requires unsafe;
misaligned raw address diagnosed;
known invalid device address diagnosed;
address trust shown in LSP.
```

---

# Compile-time evaluation tests

Test:

```text
raw pointer read rejected in compile-time evaluation;
FFI call rejected;
inline assembly rejected;
addressed storage access rejected;
compiler-known explicitly supported intrinsic accepted;
unsafe context does not override compile-time restriction.
```

---

# Binary tests

Verify:

```text
unsafe introduces no runtime support;
no dynamic unsafe flag is emitted;
safe wrappers add no mandatory runtime;
trust provenance may be omitted from release binary unless requested;
raw-pointer operations lower directly according to target rules;
unused FFI and unsafe helpers are not linked.
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
attributes.md
effect_analysis.md
runtime_checks.md
panic.md
ownership.md
copy_move.md
memory_model.md
arena rulebook
allocation rulebook
borrow and escape rules
functions rulebook
interfaces rulebook
generics rulebook
closures and lambdas rulebook
defer rulebook
destruction rulebook
FFI rulebook
inline assembly rulebook
addressed-storage rulebook
hardware-register-access rulebook
register rulebook
interrupt and ISR rulebook
target knowledge rulebook
compiler pipeline rulebook
Semantic IR rulebook
formatter.md
lsp.md
diagnostics rulebook
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add the rulebook

Add:

```text
rules/memory/unsafe.md
```

Update:

```text
language-rulebook-status.md
rules/compiler/rules_implementations.txt
```

Mark the rulebook Written.

Do not mark all unsafe functionality implemented.

---

## A.2 Inspect existing syntax

Locate current parsing and AST support for:

```text
unsafe;
extern;
linkage strings;
RawPtr;
FFI declarations;
inline assembly;
@address.
```

Preserve already canonical syntax.

Do not invent `pub`, `unsafe struct`, or `unsafe type`.

---

## A.3 Parse unsafe contexts

Implement:

```sec
unsafe operation
```

and:

```sec
unsafe {
    // ...
}
```

The block must integrate with ordinary block-expression semantics.

---

## A.4 Parse `unsafe fn`

Support Sec functions and methods:

```sec
unsafe fn Name(...) ReturnType {
}
```

The body is not implicitly unsafe.

---

## A.5 Preserve `unsafe extern`

Support canonical:

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

Keep unsafe status separate from linkage.

---

## A.6 Add semantic unsafe context tracking

Track lexical unsafe depth or an equivalent context representation.

Require context for every compiler-known unsafe operation.

Do not disable unrelated diagnostics inside the context.

---

## A.7 Add unsafe operation registry

Centralize operation kinds, obligations, effects, and diagnostics.

Do not implement unsafe requirements through method-name matching alone.

Raw-pointer APIs and intrinsics should carry semantic classification.

---

## A.8 Add caller obligation metadata

Unsafe functions and intrinsics should expose structured caller-obligation data
for diagnostics, docs, and LSP.

Free-form safety documentation remains recommended rather than mandatory.

---

## A.9 Implement RawPtr classifications

Distinguish safe handling from unsafe interpretation.

Implement at least:

```text
read;
write;
arithmetic;
integer conversion;
reference construction;
mutable-reference construction;
slice construction;
ownership adoption.
```

---

## A.10 Integrate effects

Unsafe context must not suppress:

```text
MayPanic;
MayAllocate;
MayBlock;
MaySuspend;
MaySpawn;
MayIO;
MayAccessVolatile;
MayMutateExternalState;
MayUseNondeterministicInput.
```

Retain trust provenance.

---

## A.11 Integrate FFI

Treat unknown foreign effects and ownership conservatively.

Prepare representation for future trusted effect and ownership declarations.

Do not invent their source syntax.

---

## A.12 Integrate address provenance

Named knowledge-pack address:

```text
compiler-verified where possible.
```

Raw numeric address:

```text
compiler-validated canonical-linear target binding;
reject when complete region/storage validation cannot be established.
```

Do not require `unsafe @address`.

---

## A.13 Integrate interfaces and function values

Preserve unsafe caller status through:

```text
interface methods;
method implementations;
function values;
callbacks;
generic callable constraints.
```

Reject silent unsafe-to-safe conversion.

---

## A.14 Integrate cleanup

Require explicit unsafe context inside:

```text
defer;
destructors;
cleanup helpers.
```

Continue all cleanup effect checks.

---

## A.15 Compile-time evaluation

Reject ordinary unsafe operations in compile-time evaluation.

Allow only explicitly supported compiler-known intrinsics.

---

## A.16 Diagnostics

Add stable diagnostics for:

```text
missing unsafe context;
redundant unsafe context;
empty unsafe context;
unsafe function-value mismatch;
interface safety mismatch;
unknown foreign effect;
unknown ownership;
raw-pointer obligation;
trusted target provenance;
compile-time rejection.
```

Default redundant and empty-context diagnostics to information.

Permit severity configuration.

---

## A.17 Formatter

Support canonical formatting for:

```sec
unsafe pointer.Write(value)
```

```sec
let value := unsafe {
    pointer.Read()
}
```

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

Do not add or remove unsafe semantics.

---

## A.18 LSP

Add:

```text
unsafe-operation hover;
caller-obligation hover;
trust-provenance navigation;
wrap-in-unsafe code action;
remove-redundant-unsafe code action;
function safety compatibility diagnostics;
knowledge-pack address replacement where exact and safe.
```

---

## A.19 Tests

Run:

```text
go test ./...
compiler build
LSP build
formatter tests
fixture validation
unsafe test suite
effect test suite
FFI tests
binary dependency tests
```

Do not claim completion while unsafe syntax, RawPtr classification, effect
integration, or diagnostics are partial.

---

# Appendix B — Canonical unsafe table

| Construct | Requires unsafe context | Meaning |
|---|---:|---|
| Store or move `RawPtr[T]` | No | Preserve an uninterpreted raw address |
| Compare `RawPtr[T]` with null | No | Inspect pointer value without dereference |
| Raw-pointer read | Yes | Read typed storage under caller obligations |
| Raw-pointer write | Yes | Write typed storage under caller obligations |
| Raw-pointer arithmetic | Yes | Compute a new raw address |
| Integer to raw pointer | Yes | Assert an address interpretation |
| Raw pointer to `ref T` | Yes | Assert shared-reference validity |
| Raw pointer to `ref mut T` | Yes | Assert exclusive mutable-reference validity |
| Slice from raw parts | Yes | Assert range, lifetime, layout, and alias validity |
| Call `unsafe fn` | Yes | Accept the function's caller obligations |
| Call `unsafe extern` | Yes | Accept foreign caller and ABI obligations |
| Inline assembly | Yes | Accept machine-level trust obligations |
| Unchecked construction | Yes | Bypass ordinary invariant validation |
| Named knowledge-pack `@address` | No | Compiler-validated target binding where possible |
| Raw numeric `@address` | No extra syntax | Compiler-validated binding in the target's canonical linear hardware address domain |
| Typed access through accepted `@address` | No | Volatile typed target access |

---

# Final canonical summary

`unsafe` permits specific compiler-known operations whose complete safety
obligations cannot be proven automatically.

It disables no compiler analysis.

Sec supports:

```sec
unsafe pointer.Write(value)
```

for one operation, and:

```sec
let value := unsafe {
    pointer.Read()
}
```

for a block or value-producing unsafe context.

Braces are required when more than one operation or statement belongs to the
unsafe context.

Sec supports:

```sec
unsafe fn Name(...) ReturnType {
}
```

and:

```sec
unsafe extern "system" fn rawSysCall(number: int, argument1: uint64) int64
```

Calling either requires unsafe context.

The body of an unsafe function is not implicitly unsafe.

Every unsafe operation inside it remains explicit.

Safe functions may encapsulate unsafe implementation details after establishing
all caller obligations.

Unsafe does not transitively mark callers.

Actual effects still propagate.

`RawPtr[T]` is not inherently unsafe to store, move, compare, return, or pass
through FFI.

Dereference, write, arithmetic, integer conversion, reference construction,
slice construction, ownership adoption, and similar interpretation operations
are unsafe.

Sec 0.1 does not define `unsafe struct` or `unsafe type`.

Types containing raw pointers are not automatically unsafe.

Unsafe construction uses unsafe constructors or factory functions.

`@address` is not prefixed with unsafe.

Named endpoint and raw numeric `@address` bindings are compiler-validated
against canonical target/platform/device/project contracts. Numeric spelling is
limited to the target's canonical linear hardware address domain and does not
create a trust bypass.

Ordinary typed access to accepted addressed storage does not require unsafe
context.

Unsafe does not suppress `MayPanic`, `MayAllocate`, `MayBlock`, `MaySuspend`, or
any other effect.

Unsafe operations in `defer` and destructors still require explicit unsafe
context and remain subject to all cleanup rules.

Unsafe status is preserved in interfaces, function values, callbacks, and
generics.

Ordinary compile-time evaluation rejects unsafe operations.

Redundant and empty unsafe contexts are valid and produce informational
diagnostics by default; configuration may promote them to warnings.

Public unsafe API safety documentation is strongly recommended, not a hard
language requirement.

Unsafe and trust provenance require no mandatory Sec runtime.
