# Foreign Function Interface

- **Status:** Normative
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/platform/ffi.md`
- **Replaces:** `rules/platform/ffi.txt`
- **Repository baseline reviewed:** `0f92cf4`

## 1. Purpose and scope

The Foreign Function Interface defines how Sec source code describes and interacts with foreign ABI functions and data.

Sec 0.1 FFI is designed primarily to consume existing C-compatible and system libraries such as operating-system APIs, OpenSSL, libc-like libraries, and similar native interfaces.

FFI is an ABI and binding boundary.

It is not a C or C++ source-language subsystem.

The Sec compiler does not:

- parse C or C++ header files;
- run a C preprocessor;
- interpret `#define`, `#if`, `#ifdef`, or other preprocessor constructs;
- parse C typedef declarations;
- compile C inline functions;
- import C source expressions.

A separate optional binding-generation tool may later parse foreign headers and emit ordinary Sec FFI source. Such a tool is outside the Sec language and outside this rulebook.

General export of Sec functions or libraries as named C ABI symbols is not part of Sec 0.1.

Callbacks required to consume foreign APIs are supported separately through C function-pointer adaptation.

## 2. Foreign declarations

An imported foreign function is declared with `extern` and has no Sec body.

Examples:

```sec
extern "C" fn GetVersion() C::int

extern "system" fn GetSystemValue() uint32
```

A body on an imported extern declaration is invalid in Sec 0.1.

```sec
extern "C" fn GetVersion() C::int {
    return 1
}
```

The compiler must reject unknown calling-convention strings.

The canonical calling conventions include:

```text
extern "C"
extern "system"
extern "Sec"
```

`extern "C"` uses the active target C ABI.

`extern "system"` uses the active target system ABI.

`extern "Sec"` uses the Sec ABI for an external declaration where that ABI is explicitly required.

The active `CompilationPlan` and its resolved ABI model are authoritative for physical ABI validation.

## 3. Foreign trust boundary and `unsafe extern`

Every foreign declaration is a compiler-unverified trust boundary.

This fact does not require repetitive `unsafe` syntax at every ordinary call site.

Example:

```sec
extern "C" fn GetProcessId() C::int

let processId := GetProcessId()
```

`unsafe extern` has a stronger meaning.

```sec
unsafe extern "C" fn CopyMemory(
    destination: RawPtr[byte],
    source: RawPtr[byte],
    length: c::stddef::size_t,
) RawPtr[byte]
```

`unsafe extern` means that the caller has additional proof obligations that are not fully expressed or verifiable by the declared Sec signature.

Calling an `unsafe extern` function requires an explicit unsafe context.

```sec
unsafe {
    CopyMemory(destination, source, length)
}
```

Therefore:

```text
extern
    foreign/compiler-unverified implementation boundary

unsafe extern
    foreign boundary plus additional caller proof obligations
```

`unsafe` does not disable ownership, borrowing, effects, cleanup, target validation, or ABI validation.

## 4. FFI declarations describe the foreign ABI, not C source syntax

A binding author writes the Sec representation required by the foreign ABI.

For example, a C header may internally use typedefs or macros, but the Sec binding expresses the resulting ABI directly.

The compiler does not need to know how the foreign header arrived at that representation.

A binding may expose safer or more nominal Sec wrapper names than the original foreign source when doing so does not falsify the raw ABI.

## 5. Fundamental C ABI scalar family

Compiler-known fundamental C ABI scalar types use uppercase `C::` qualification.

Examples include:

```sec
C::char
C::schar
C::uchar

C::short
C::ushort

C::int
C::uint

C::long
C::ulong

C::long_long
C::ulong_long

C::float
C::double
C::long_double

C::bool
```

`C::` is compiler-known foreign type-family qualification.

It is not ordinary member access and does not create a runtime value named `C`.

The physical representation of each type is resolved through the active C ABI model.

For example, `C::long` may have different widths on different targets.

## 6. C library and platform binding namespaces

C standard-library, implementation-specific, and platform-specific binding types use lowercase hierarchical `c::` qualification.

Example:

```sec
c::stddef::size_t
```

Other binding namespaces may define types such as:

```sec
c::stddef::ptrdiff_t
c::time::time_t
c::stdarg::va_list
c::posix::socklen_t
```

Such names are not automatically fundamental compiler primitives.

Their concrete definitions are supplied by the selected target/library binding environment.

Ordinary local identifiers remain legal even when they use the same spelling.

```sec
let mut c: int := 24
let mut b := c

let size: c::stddef::size_t := 0
```

The local value `c` does not participate in resolving `c::stddef::size_t`.

The `::` mechanism may support additional foreign language families in future language versions, but those families are outside Sec 0.1 FFI scope.

## 7. C scalar types and Sec scalar types remain distinct

Representation equality does not create source-level type identity.

On a target where `C::int` is a signed 32-bit value, `C::int` and `int32` may have identical physical representation while remaining distinct Sec types.

```sec
let foreignValue: C::int := 42
let secValue: int32 := 42
```

Assignment does not become implicit merely because the current ABI gives both types the same representation.

## 8. C-to-Sec and Sec-to-C scalar conversions

Conversions use the ordinary Sec explicit conversion model.

```sec
let foreignValue: C::int := 42
let secValue := int32(foreignValue)

let other: int32 := 42
let foreignOther := C::int(other)
```

If the target type can represent the complete source value domain, the conversion is infallible.

If the conversion may lose range, it is checked and requires `try`.

```sec
let wide: int64 := ReadWideValue()
let narrow := try C::int(wide)
```

A representation-identical conversion may lower to no machine instruction.

Literal shaping remains distinct from runtime conversion.

```sec
let count: C::int := 10
```

is valid when the literal is representable.

## 9. ABI-stable Sec numeric scalars in FFI

Fixed-width Sec numeric scalars may appear directly in foreign signatures when they exactly describe the foreign ABI type and the active ABI model verifies compatibility.

Examples include:

```sec
byte
int8
uint8
int16
uint16
int32
uint32
int64
uint64
float32
float64
```

For example, a foreign API defined in terms of an exact 32-bit unsigned integer may use:

```sec
extern "C" fn CRC32(
    data: RawPtr[byte],
    length: uint32,
) uint32
```

When the foreign source type is C ABI-defined rather than fixed-width, the binding uses the appropriate `C::` type.

For a C `int`, use:

```sec
C::int
```

rather than hard-coding `int32` merely because the current target happens to use a 32-bit C `int`.

Sec `bool`, Sec `char`, and Sec `rune` do not automatically represent C `_Bool`, C `char`, or foreign wide-character types.

Use the correct C or binding type.

## 10. Raw pointers

`RawPtr[T]` is the raw pointer representation used at foreign boundaries where no safe Sec borrow contract is established.

A `RawPtr[T]`:

- may contain a foreign null address;
- does not imply pointee ownership;
- does not establish lifetime;
- does not establish non-nullness;
- does not establish bounds;
- does not establish aliasing guarantees;
- does not establish retention behavior.

Moving or consuming a `RawPtr[T]` never implies ownership transfer of the pointee.

## 11. `null`

`null` is not an ordinary Sec value.

It is an unsafe foreign/raw-pointer sentinel.

The token `null` may be used only lexically inside an unsafe context explicitly permitted by the unsafe/FFI rules.

Valid:

```sec
unsafe {
    if raw is null {
        return None
    }
}
```

Invalid:

```sec
if raw is null {
    return None
}
```

Null testing uses `is`.

Equality comparison with `null` is invalid.

```sec
if raw == null {
    ...
}
```

`null` has no standalone inferred type.

Invalid:

```sec
unsafe {
    let value := null
}
```

A target type may provide the required raw-pointer context.

```sec
unsafe {
    let value: RawPtr[Device] := null
}
```

A zero-valued character or byte used as a C string terminator is not the `null` sentinel.

## 12. Safe references in foreign parameters

An extern parameter may use `ref T` or `ref mut T` when the foreign contract is a non-null call-bounded borrow.

```sec
extern "C" fn Inspect(value: ref Header) C::int

extern "C" fn Modify(value: ref mut Header) C::int
```

The contracts are:

```text
ref T
    non-null
    shared
    valid for the call
    foreign code must not retain the address

ref mut T
    non-null
    exclusive mutable
    valid for the call
    foreign code must not retain the address
```

A foreign function may still be declared `unsafe extern` when other proof obligations remain.

```sec
unsafe extern "C" fn ProcessBuffer(
    first: ref mut byte,
    length: c::stddef::size_t,
) void
```

The reference proves the first byte's borrow contract, not that `length` bytes are valid.

## 13. Foreign pointer returns

Raw foreign pointer returns use `RawPtr[T]`.

A raw extern declaration must not return `ref T` or `ref mut T`.

```sec
extern "C" fn GetCurrent() RawPtr[Device]
```

A returned foreign pointer does not by itself establish a Sec lifetime.

A wrapper validates nullability, ownership, lifetime, and other foreign invariants before exposing a safe Sec abstraction.

## 14. Stored foreign pointer fields

A stored pointer field in C-compatible data uses `RawPtr[T]`.

```sec
extern "C" type Node struct {
    value: C::int,
    next: RawPtr[Node],
}
```

Stored foreign pointer fields do not use `ref` or `ref mut`.

Their nullability, lifetime, aliasing, retention, and ownership are controlled by the foreign contract rather than by a call-bounded Sec borrow.

## 15. C-compatible structs

The canonical C struct declaration is:

```sec
extern "C" type Point struct {
    x: C::int,
    y: C::int,
}
```

This declares a nominal Sec type whose stored representation must match a C struct under the active C ABI.

Source field order is representation-significant and must not be reordered.

Natural C ABI padding and alignment are implicit.

Nested fields must themselves have a valid foreign representation.

Ordinary behavior may be defined separately through `impl`.

An ordinary Sec struct is not C-compatible merely because its fields happen to have the same sizes.

## 16. C struct initialization

Foreign representation does not imply C initialization semantics.

An `extern "C"` struct is not implicitly Defaultable merely because its fields are individually Defaultable.

Complete explicit construction is required unless an explicit binding/helper provides another valid initialization path.

```sec
let point := Point {
    x: 10,
    y: 20,
}
```

Omitting a foreign field is invalid unless a separate rule for that declaration explicitly provides initialization semantics.

## 17. Incomplete C structs

A bodyless C struct declaration represents an incomplete/opaque C struct.

```sec
extern "C" type Context struct
```

The type has no known concrete size or stored layout in Sec.

Valid uses include:

```sec
RawPtr[Context]
ref Context
ref mut Context
```

Invalid uses include by-value parameters, by-value returns, direct construction, and operations requiring a known size.

No separate `opaque` keyword is required.

## 18. C-compatible unions

The canonical C union declaration is:

```sec
extern "C" type Value union {
    integer: C::int,
    real: C::double,
}
```

An `extern "C"` union is not an ordinary Sec tagged union.

It has:

- overlapping C storage;
- no hidden Sec tag;
- no compiler-tracked active variant;
- layout determined by the active C ABI.

Direct member access is semantically foreign-unsafe because Sec does not know which member is active.

The explicit foreign type declaration already marks this representation boundary; Sec does not require repetitive explicit `unsafe` syntax solely for each member access.

Analysis and diagnostics must still identify such access as foreign union access.

Safe wrappers should hide raw union-member selection when the foreign protocol provides a tag or other discriminator.

## 19. C-compatible enums

The canonical C enum declaration is:

```sec
extern "C" type Color enum {
    Red = 1,
    Green = 2,
    Blue = 3,
}
```

An `extern "C"` enum uses the representation selected by the active C ABI.

A C enum is open over values representable by its resolved foreign representation.

Foreign code may provide a value that is not one of the declared symbolic members.

Code that interprets a foreign C enum must therefore account for unknown values where exhaustiveness is required.

```sec
match color {
    Color.Red => HandleRed()
    Color.Green => HandleGreen()
    Color.Blue => HandleBlue()
    _ => HandleUnknownColor()
}
```

This differs from ordinary closed Sec enums.

## 20. C bitfields

A C bitfield in an `extern "C"` data declaration uses the C base type followed by a bitfield width declarator.

```sec
extern "C" type Flags struct {
    enabled: C::uint bit[1],
    mode: C::uint bit[3],
    reserved: C::uint bit[28],
}
```

Inside this context, `bit[N]` is a C bitfield width declarator, not a standalone type.

The active C ABI determines:

- allocation unit;
- bitfield placement;
- ordering;
- packing;
- alignment effects.

Sec register `lsb-first`/`msb-first` semantics do not control C bitfield layout.

The C bitfield base type must be classified as bitfield-capable by the active ABI model.

Unnamed bitfields use `_`.

```sec
_: C::uint bit[4],
```

A zero-width bitfield is permitted only in an unnamed C bitfield position.

```sec
_: C::uint bit[0],
```

Its layout effect is defined by the active C ABI.

C bitfields are not independently addressable.

The following operations are invalid:

```sec
ref flags.mode
ref mut flags.mode
flags.mode.Ptr
```

Bitfield assignment uses Sec checked-range semantics and must not silently inherit C truncation/wrapping behavior.

C bitfields do not imply atomic, volatile, MMIO, or thread-safe semantics.

## 21. Fixed C arrays

A fixed C array stored inside C-compatible data maps to Sec fixed inline array storage.

```sec
extern "C" type Address struct {
    bytes: byte[16],
}
```

The array is inline storage.

It does not contain a Sec slice/list descriptor.

C array parameters are represented by their actual ABI pointer form rather than by-value Sec fixed arrays.

## 22. C flexible-array members

A C flexible-array member uses:

```sec
C::flex[T]
```

Example:

```sec
extern "C" type Packet struct {
    length: c::stddef::size_t,
    data: C::flex[byte],
}
```

A flexible-array member:

- is valid only in an `extern "C"` struct;
- must be the final stored field;
- has no Sec descriptor;
- has no implicit runtime length;
- does not automatically produce a safe Sec slice or view;
- obtains its usable extent from the foreign allocation/protocol contract.

`Packet.SizeOf` describes the fixed C struct portion according to the active C ABI and does not include runtime trailing elements.

The wrapper must establish the actual extent before creating any safe view over the trailing storage.

## 23. Explicit packing and alignment

`extern "C"` selects ordinary layout under the active C ABI.

Packed, over-aligned, explicitly offset, or otherwise overridden layouts use Sec's canonical explicit-layout mechanism.

FFI does not introduce a second packing/alignment syntax.

A packed or otherwise misaligned field may be read or written through compiler-supported unaligned operations when permitted by the target.

A `ref` or `ref mut` to a misaligned field is invalid unless the required alignment is proven.

## 24. C function-pointer types

A C ABI function-pointer type uses:

```sec
C::fn(ParameterTypes) ReturnType
```

Examples:

```sec
C::fn(C::int) void
C::fn(RawPtr[Context], C::int) C::bool
```

A `C::fn(...) R` value is not a native Sec callable type.

Compare:

```sec
fn(C::int) void
C::fn(C::int) void
```

The first is a native Sec callable.

The second is a C ABI function pointer.

A C function pointer may be null when obtained from foreign data.

Null testing follows the normal unsafe-only `is null` rule.

A call through a possibly-null C function pointer is valid only when non-nullness has been established.

## 25. C callbacks

Sec 0.1 supports callbacks required to consume foreign C APIs.

The explicit adapter is:

```sec
C::callback(callable)
```

`C::callback` produces a non-null `C::fn(...) R` value with the corresponding C ABI signature.

Example:

```sec
fn OnValue(value: C::int) void {
    ProcessValue(int32(value))
}

let callback := C::callback(OnValue)
```

The source callable must be:

- reusable;
- environment-free;
- non-capturing;
- compatible with the exact foreign-facing parameter and return types.

Capturing closures are not directly adaptable to C callbacks in Sec 0.1.

`mut fn` and `-> fn` callables are not directly adaptable.

The compiler may generate a private C ABI thunk/entry point required to implement `C::callback`.

This is callback interoperation, not general Sec-to-C symbol export.

No user-selected exported C symbol is created by `C::callback`.

When a C API provides an explicit userdata/context pointer, that pointer is the canonical state-transport mechanism for Sec 0.1 callbacks.

## 26. C variadic functions

C varargs are distinct from native Sec typed variadics.

Native Sec:

```sec
fn Sum(values: ...int) int {
    ...
}
```

C varargs:

```sec
unsafe extern "C" fn Printf(
    format: RawPtr[C::char],
    ...
) C::int
```

A bare final `...` is permitted only for C foreign variadic signatures and C function-pointer types.

A C variadic declaration must be `unsafe extern` because the fixed signature cannot express all caller obligations.

The bare `...` marker must be final.

## 27. C variadic arguments

Additional C variadic arguments must already be C-ABI-representable values.

An ordinary Sec scalar must be explicitly converted to the intended C type before being passed through a C vararg position.

The compiler applies the active C ABI's default argument promotions to already-C-typed values.

Examples include the required promotion of:

- C floating types such as `C::float` to the appropriate promoted C type;
- small C integer types to the appropriate promoted integer type.

The exact promotions are determined by the active C ABI model.

`ref` and `ref mut` are not passed directly through C varargs.

Use the correct raw foreign pointer representation where the foreign protocol expects a pointer.

C-compatible aggregates may be passed through C varargs only when the active ABI model defines and supports their variadic classification.

Sec spread does not apply to C varargs.

An untyped `null` cannot be passed as a C variadic argument because no target pointer type is available.

`va_list` is a target C binding type, for example under `c::stdarg`, and is never a native Sec variadic pack.

Format-string checking belongs to compiler analysis or known binding contracts rather than to core C varargs semantics.

## 28. Sec `string` is not a C string

Sec `string` has no direct C ABI representation.

The following is invalid as a raw FFI signature:

```sec
extern "C" fn Open(path: string) C::int
```

A C character pointer is represented by the corresponding pointer type, for example:

```sec
RawPtr[C::char]
```

Such a pointer does not by itself imply:

- text;
- encoding;
- NUL termination;
- ownership;
- length;
- mutability;
- non-nullness.

## 29. String encoding and termination

FFI defines no universal encoding for C `char*`.

A binding or wrapper must explicitly establish the foreign text encoding.

Possible foreign contracts include:

- UTF-8;
- ASCII;
- locale/code-page text;
- filesystem bytes;
- protocol-specific bytes;
- non-text binary data.

Conversion from Sec `string` to NUL-terminated foreign character storage must define:

- encoding;
- element type;
- terminator rules;
- embedded-zero behavior;
- ownership;
- allocation/storage;
- lifetime.

A generic encoding-agnostic `ToCString()` is not a complete FFI contract.

A helper library may provide C-string wrapper types, but their encoding/lifetime semantics must be explicit.

## 30. Embedded zero and C string terminators

A C string terminator is a numeric zero code unit in character storage.

It is not the FFI `null` pointer sentinel.

When converting Sec text to a NUL-terminated foreign string, an embedded zero that would terminate the foreign string early must be handled explicitly.

A conversion that rejects embedded zero is fallible.

The binding must not silently truncate Sec text at an embedded zero.

## 31. Length-delimited text and binary buffers

A pointer-plus-length foreign API is distinct from a NUL-terminated string API.

```sec
extern "C" fn ConsumeText(
    data: RawPtr[C::char],
    length: c::stddef::size_t,
) C::int
```

The wrapper must know what the length counts.

Possible units include:

- bytes;
- C character elements;
- UTF-16 code units;
- other protocol-defined units;
- capacity including or excluding a terminator.

Sec must not assume that a foreign length equals `string.Len`.

For binary buffers, raw declarations use the actual pointer and length parameters.

A safe wrapper may accept a Sec slice/view and explicitly provide the pointer and length.

Sec performs no hidden slice-to-pointer-plus-length decomposition in raw extern signatures.

## 32. Incoming foreign strings

A foreign pointer returned as character data remains a raw pointer.

The wrapper must establish:

- nullability;
- readable extent;
- termination or explicit length;
- lifetime;
- ownership;
- encoding.

Decoding invalid foreign text may be fallible.

A binding may define replacement semantics only when that behavior is explicit.

Wide-character representations are binding-specific.

FFI does not universally equate `wchar_t*` with UTF-16, UTF-32, or Sec `string`.

## 33. Fixed and flexible character arrays

A fixed foreign character array remains an array.

```sec
extern "C" type User struct {
    name: C::char[64],
}
```

It is not automatically a Sec `string`.

A flexible foreign character array also remains foreign character storage.

```sec
extern "C" type Message struct {
    length: c::stddef::size_t,
    data: C::flex[C::char],
}
```

The wrapper decides whether such storage represents text and which encoding/protocol applies.

## 34. Pointer-to-pointer data

Foreign pointer nesting is represented explicitly.

```sec
RawPtr[RawPtr[C::char]]
```

does not automatically become `string[]`.

The wrapper must know count/sentinel rules, element nullability, lifetime, encoding, and ownership before exposing a safe Sec collection.

## 35. Foreign resource ownership

Raw pointer and scalar-handle representations never imply resource ownership.

An owning foreign resource is represented by an ordinary nominal Sec wrapper type.

Example:

```sec
extern "C" type SSLContextRaw struct

type SSLContext struct {
    raw: RawPtr[SSLContextRaw],
}

impl SSLContext {
    free {
        SSLContextFreeRaw(self.raw)
    }
}
```

The safe wrapper may construct an owning Sec value only when the foreign contract establishes ownership.

A borrowed or library-owned foreign pointer must not be silently upgraded to an owning wrapper.

## 36. Ownership transfer to foreign code

Ownership transfer to foreign code uses ordinary Sec ownership semantics on the owning wrapper.

It is not expressed by consuming a `RawPtr[T]` and pretending that the pointee became owned by foreign code.

A wrapper that consumes an owning Sec resource is responsible for implementing the foreign success/failure transfer contract.

If the foreign operation accepts ownership only on success, the wrapper retains responsibility for the resource until success is established.

On failure, the wrapper must clean up, return an explicit owner, or otherwise follow its declared Sec API contract.

## 37. Foreign reference counting

A foreign object being reference-counted does not make its Sec wrapper implicitly Copyable.

An ordinary Sec copy must not hide a foreign retain/up-ref call.

Foreign retain/clone operations are explicit operations.

They may have effects or failure behavior and are modeled accordingly.

A resource-owning wrapper with lifecycle cleanup is move-only by default unless ordinary Sec copy semantics explicitly define an independent safe copy.

## 38. Foreign cleanup and fallible close

Lifecycle `free` performs mandatory cleanup and does not propagate user-visible errors.

When a foreign resource has a meaningful fallible close/shutdown/commit operation, the wrapper exposes an explicit Result-returning operation.

The wrapper must prevent double release after successful explicit close/transfer.

Foreign invalid/sentinel values are binding-specific.

`null` is only one possible foreign sentinel.

## 39. Foreign failure conventions

Raw extern declarations expose the actual foreign failure convention.

FFI does not automatically convert:

- negative results;
- status values;
- null pointers;
- errno-style state;
- Windows-style last-error state;
- OpenSSL-style error queues;
- out-parameter protocols

into `Result` or `Option`.

A Sec wrapper performs explicit normalization.

`try` does not apply to a raw foreign failure convention unless the declared Sec return type itself uses ordinary Sec fallible semantics.

## 40. Auxiliary foreign error state

Auxiliary error state is binding-specific.

A wrapper must retrieve foreign error state according to that API's contract before another operation may invalidate it.

Compiler/LSP analyses may use explicit binding contracts to diagnose invalid error-state sequencing.

They must not guess such semantics from names such as `errno`, `GetLastError`, or similar identifiers.

## 41. No unwind across FFI

No unwind or non-local control transfer may cross an active FFI boundary in either direction in Sec 0.1.

This includes:

- foreign exceptions crossing Sec frames;
- `longjmp` or equivalent non-local transfer across active Sec frames;
- Sec unwind crossing a C callback boundary.

`unsafe` does not make such control transfer legal.

A foreign library that may throw must use a foreign-side shim or equivalent boundary that translates the failure into an ordinary foreign status/error protocol before returning to Sec.

A Sec callback must explicitly translate fallible Sec behavior into the C callback protocol.

## 42. Foreign effects

Existing Sec effect-guarantee attributes may be applied to extern declarations when the attribute's target rules permit functions.

For ordinary Sec implementations, guarantees are compiler-verified.

For extern declarations, the same guarantee is a trusted foreign contract.

The compiler must preserve provenance showing that the fact came from a trusted foreign declaration.

Unknown foreign effects are treated conservatively.

`unsafe` does not erase or override effect analysis.

## 43. Foreign symbol names

When the foreign symbol differs from the Sec declaration name, use:

```sec
@link_name("foreign_symbol")
extern "C" fn SecName(...) C::int
```

`@link_name` affects the foreign/link symbol only.

It does not alter:

- the Sec declaration name;
- calling convention;
- safety;
- ownership;
- effects;
- library dependency;
- ABI compatibility.

## 44. Native library and object dependencies

Native library, import-library, framework, object-file, search-path, or equivalent dependencies belong to project/package/build metadata.

They are not repeated as ordinary per-function FFI annotations in Sec 0.1.

The `CompilationPlan` resolves logical native dependencies into the concrete linker inputs required by the selected target.

A reusable binding package may carry native dependency metadata through the package/build system.

The precise package-manifest syntax is owned by the build/package rules, not by FFI.

Dynamic runtime lookup APIs such as `dlopen`, `dlsym`, `LoadLibrary`, or `GetProcAddress` are ordinary foreign APIs, not special compile-time FFI linking syntax.

## 45. FFI-legal types

The compiler must distinguish:

```text
FFI-representable
    the ABI model can describe the physical representation

FFI-legal in this position
    Sec FFI rules permit the type in this exact source position
```

Types commonly legal in appropriate foreign positions include:

- fundamental `C::` scalar types;
- resolved `c::` binding scalar/ABI types;
- ABI-stable fixed-width Sec numeric scalars;
- `RawPtr[T]`;
- call-bounded `ref T` and `ref mut T` parameters;
- `C::fn(...) R`;
- complete `extern "C"` structs;
- `extern "C"` enums;
- `extern "C"` unions;
- fixed arrays in C data;
- `C::flex[T]` in its restricted final-field position.

## 46. Types forbidden from direct raw FFI use

The following do not cross a C/system ABI directly unless another explicit foreign representation has been defined:

- Sec `string`;
- native Sec dynamic collections;
- `list[T]`;
- `map[K, V]`;
- `set[T]`;
- vector/matrix/tensor ownership descriptors;
- `Option[T]`;
- `Result[T, E]`;
- interface values;
- ordinary Sec tagged unions;
- ordinary Sec structs without explicit foreign representation;
- native Sec callable values;
- capturing closures;
- unresolved generic templates;
- types with hidden Sec ownership/runtime state that has no declared foreign ABI representation.

Sec `bool`, `char`, and `rune` do not automatically substitute for C boolean/character representations.

## 47. Incomplete and position-restricted types

An incomplete foreign type may appear behind `RawPtr`, `ref`, or `ref mut` where the foreign contract permits it.

It may not be passed or returned by value.

`ref` and `ref mut` may appear as call-bounded foreign parameters.

They do not appear as raw foreign returns or stored C pointer fields.

Fixed arrays may represent inline C data fields but do not represent C array parameters by value.

## 48. Generics

An unresolved generic extern declaration is invalid.

```sec
extern "C" fn Process[T](value: T) void
```

Only concrete monomorphized types may acquire a concrete ABI.

A concrete generic Sec type is not automatically FFI-compatible.

It must independently satisfy the foreign representation rules applicable to its use.

## 49. Target and ABI validation

FFI validation occurs before target code generation.

The compiler must reject a declaration or use when:

- the calling convention is unavailable;
- the foreign representation is not supported by the selected ABI;
- an incomplete type is used by value;
- a required foreign data layout cannot be represented;
- a C bitfield layout is unsupported by the selected C ABI model;
- a variadic aggregate has no valid target classification;
- a C function-pointer signature is unsupported;
- required target/native dependency metadata cannot be resolved.

Frontend semantic rules must not hard-code one target's register or aggregate classification.

The active ABI model owns physical parameter/return classification.

## 50. Compiler representation

The AST/Semantic IR must preserve enough information to distinguish:

- calling convention;
- `unsafe extern` caller obligations;
- foreign symbol name;
- foreign type family/namespace identity;
- C struct/union/enum/incomplete type kind;
- C bitfield width and base type;
- flexible-array membership;
- C function-pointer type;
- C callback adaptation;
- C varargs marker;
- raw pointer versus call-bounded reference;
- trusted foreign effect provenance;
- target/native dependency references;
- source location for diagnostics.

Semantic IR must preserve these semantics until ABI-aware lowering can legally choose a physical representation.

## 51. Parser requirements

The parser must support at least:

```text
extern "C" fn ...
unsafe extern "C" fn ...
extern "system" fn ...
extern "Sec" fn ...

extern "C" type Name struct { ... }
extern "C" type Name struct
extern "C" type Name union { ... }
extern "C" type Name enum { ... }

C::name
c::namespace::name

C::fn(...) R
C::callback(...)

field: C-type bit[N]
field: C::flex[T]

fixed-parameters, ...
```

The parser must reject:

- extern functions with Sec bodies in Sec 0.1;
- malformed or unknown calling-convention strings;
- C varargs on non-C foreign declarations;
- non-final C varargs markers;
- invalid flexible-array positions;
- zero-width named bitfields;
- unsupported foreign declaration forms.

## 52. Sema requirements

Sema must:

- resolve the selected calling convention;
- resolve `C::` fundamental types through the active C ABI model;
- resolve `c::` binding types through the selected target/library environment;
- validate C-to-Sec and Sec-to-C conversions;
- validate FFI legality per position;
- distinguish raw pointer semantics from call-bounded reference semantics;
- enforce null lexical restrictions;
- validate complete/incomplete foreign data usage;
- validate C struct, union, enum, bitfield, fixed-array, and flexible-array rules;
- validate C function-pointer types and non-null call proofs;
- validate `C::callback` environment-free reusable callable requirements;
- validate C varargs and default promotions;
- validate trusted foreign effect declarations;
- validate foreign symbol and target metadata;
- reject unresolved generic ABI forms;
- produce target-aware diagnostics.

## 53. Diagnostics

FFI diagnostics should identify the foreign declaration, exact parameter/field/return position, calling convention, and selected target/ABI when relevant.

Required diagnostic classes include at least:

```text
unknown foreign calling convention
```

```text
extern function declarations may not have a Sec body
```

```text
calling unsafe extern function requires unsafe context
```

```text
null may be used only inside unsafe foreign/raw-pointer context
```

```text
null is tested with `is`, not equality
```

```text
Sec string has no direct C ABI representation
```

```text
ordinary Sec struct Point is not C-compatible; declare an explicit extern "C" representation
```

```text
incomplete foreign type Context cannot be passed by value
```

```text
foreign references may not be returned as ref; use RawPtr[T] and establish lifetime in a wrapper
```

```text
stored foreign pointer fields must use RawPtr[T], not ref/ref mut
```

```text
C bitfield is not independently addressable
```

```text
flexible-array member must be the final field of an extern "C" struct
```

```text
native Sec callable cannot be used as a C function pointer; use C::fn or C::callback
```

```text
capturing callable cannot be adapted with C::callback in Sec 0.1
```

```text
C variadic declaration must be unsafe extern
```

```text
C variadic argument is not ABI-representable
```

```text
foreign unwind across Sec frames is not permitted
```

```text
foreign representation is unsupported by the selected ABI
```

Diagnostics should recommend a safe Sec wrapper when raw foreign semantics are exposed into ordinary application code.

## 54. Best practice

Bindings should separate raw ABI declarations from safe/domain-oriented Sec APIs.

Prefer:

```text
raw extern declaration
    exact foreign ABI representation

ordinary Sec wrapper
    null normalization
    encoding conversion
    ownership
    cleanup
    error translation
    domain types
    safe references/views
```

Use `ref`/`ref mut` in foreign parameters when the foreign contract truly is a non-null call-bounded borrow.

Use `RawPtr[T]` when nullability, retention, ownership, or lifetime cannot be expressed as a call-bounded borrow.

Keep raw foreign handles encapsulated inside nominal owning wrappers.

Do not make foreign ref-count operations implicit copies.

Do not assume C character pointers are text.

Do not hard-code `C::int`/`C::long` widths.

Do not make ordinary Sec representation accidentally ABI-significant.

Do not hide dynamic allocation, foreign retention, or unsafe caller obligations behind ordinary type conversion.

## 55. Cross-rulebook ownership

This rulebook owns:

- source-level foreign declarations;
- `unsafe extern` semantics;
- FFI legality rules;
- C ABI type-family syntax;
- C-compatible data declaration forms;
- C function-pointer and callback adaptation semantics;
- C varargs source semantics;
- FFI null boundary rules;
- foreign string/buffer boundary semantics;
- foreign resource wrapper requirements;
- foreign error normalization requirements;
- trusted extern effect claims;
- foreign symbol-name semantics.

Other rulebooks own:

- physical ABI register/stack/aggregate classification: ABI rules;
- target/ABI selection and `CompilationPlan`: platform model;
- general storage layout, explicit packing/alignment, and layout queries: layout rules;
- `RawPtr[T]` primitive operations: raw-pointer rules;
- ownership/copy/move/destruction: memory/lifecycle rules;
- `ref`/`ref mut` borrow semantics: borrowing/reference rules;
- `Result`, `Option`, and `try`: error-handling/type rules;
- ordinary function/callable semantics: functions/lambda rules;
- effect lattice and guarantee verification: analysis/attribute rules;
- package/build native dependency syntax: build/package rules;
- general Sec-to-C exported symbols: outside Sec 0.1.
