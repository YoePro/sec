---
name: sec-language
description: Write, explain, review, or implement Sec language constructs using this repository's canonical rulebooks. Use for Sec source and language semantics; consult governance separately for compiler capability.
---

# Sec language skill

Use this skill when generating, explaining, reviewing or implementing Sec. Apply the syntax and decision rules below directly; follow a linked rulebook when the task reaches a boundary described here. Examples are source-language examples, not a promise of native compiler support. Snippets without `module` are declaration or function-body fragments; names such as `Take` and `Release` stand for application-defined operations, not assumed library APIs.

## Establish authority

- Read [codex.md](codex.md) for repository instructions and access boundaries. In this checkout, `sec/` and `manual/` are outside ordinary compiler work unless explicitly in scope.
- Derive semantics from [rules/](rules/README.md), the owning rulebook and applicable [corrections](rules/corrections/README.md). An approved correction can supersede older prose; a proposal does not decide a rule.
- Read [missing-decisions.yaml](missing-decisions.yaml) for unresolved decisions and [governance/index.yaml](governance/index.yaml) for implementation ownership. Neither invents language semantics.
- Treat Sec as an independent language. Similarity to Go, Rust, C# or C does not establish behavior.

## Basic source and declarations

```sec
module main

type Point struct {
    X: int,
    Y: int
}

fn Add(left: int, right: int) int {
    return left + right
}

fn main() int {
    let origin := Point { X: 0, Y: 0 }
    let mut count: int := origin.X
    count = Add(count, 1)
    return count
}
```

- Use `let name := expression` for inferred initialization and `let name: T := expression` for an explicit type. Add `mut` when the binding needs mutation.
- Write `fn Name(argument: T) R { ... }`, with the return type after `)`, without a return arrow. Ordinary function definitions need bodies; interface requirements and permitted `extern` declarations are different forms.
- Separate struct fields with commas. Newlines alone do not separate consecutive fields.
- Write named types as `type Name UnderlyingType` and structs as `type Name struct { ... }`. Do not introduce `type Name = ...` or standalone Sec 0.2 struct syntax.
- User-defined nominal types and generic type parameters begin with an uppercase letter after visibility prefixes.
- Imports use logical paths, for example `import "net/http"` or `import http "net/http"`; do not substitute filesystem-relative imports.
- Visibility comes from prefixes: `Name` is public, `_name` module-internal, `__name` private. Private top-level functions are source-file scoped; member privacy follows the owning declaration's rules. Capitalization alone does not choose visibility.

References: [grammar](rules/foundations/grammar.md), [types](rules/types/types.md), [names and visibility](rules/foundations/names_scopes_visibility.md), [functions](rules/declarations/functions.md), [modules](rules/projects/modules.md).

## Binding and literal spelling: choose the exact form

| Intent | Canonical form | Do not substitute |
|---|---|---|
| Infer and initialize | `let n := 3` | `var n = 3`, `let n = 3` |
| Explicit type | `let n: uint := 3` | `let n: uint = 3` |
| Mutable binding | `let mut n := 3` | `mut let n := 3` |
| Inferred move initialization | `let destination :<- source` | `let destination <- source` |
| Typed move initialization | `let destination: T <- source` | `let destination: T :<- source` |
| Default initialization | `let mut value: T` when T is defaultable | An uninitialized immutable declaration |
| Reassignment | `n = 4` | Redeclaring `n` with `:=` |
| Module/static storage | `static let n: int := 3` | Treating every `let` in an impl as static |

A declaration such as `let literal, kind := Read()` does not destructure a multi-result value. Grouped declarations create independent bindings. Do not infer Go-style tuple returns from the punctuation.

Numeric family suffixes are adjacent, lowercase and case-sensitive:

| Literal | Selected family |
|---|---|
| `8i` | signed integer |
| `8u` | unsigned integer |
| `8g` | binary floating point |
| `8m` | exact decimal |
| `65t` | 8-bit `char` |
| `65r` | Unicode `rune` |

The suffix generally selects a family, not a width: `let n: int8 := 8i` and `let n: int256 := 8i` select different widths. `t` must fit `0..255`; `r` must be a Unicode scalar, excluding surrogates. Do not invent C suffixes or assume `char` means a Unicode scalar.

See [grammar — Let declarations](rules/foundations/grammar.md) and [types — Literal family suffixes](rules/types/types.md).

## Ownership, borrowing and methods

- `ref T` grants shared access; `ref mut T` grants exclusive mutable access. Mutable references are not implicitly copyable.
- Reference expressions use `ref value` and `ref mut value`. On an existing reference holder they reborrow its referent; `*reference` names the referent Place. Do not manufacture `ref ref T` merely to express a reborrow.
- Named ownership transfers are explicit: `Take(<-value)`. A move makes its source unavailable; it does not initialize it to zero or `None`. Move initialization uses `let destination :<- source`.
- Ordinary by-value parameters may receive explicit moves. A forced-consuming parameter is written `-> value: T` and requires the corresponding call-site transfer.
- Copyability follows the resolved type, not its apparent size. A type with custom `free` is move-only, even if its fields are scalars. A method named `Clone` does not grant implicit copyability.
- Arguments evaluate once, left to right. Transfers commit after all arguments are prepared successfully. Cleanup and borrowing obligations survive error paths.
- Methods live in `impl Type { ... }`. An ordinary method uses implicit `self`; receiver authority is inferred from its operations. Do not add a Rust-style self parameter or assume every mutating ordinary method needs a `mut fn` marker.
- `static fn` declares a receiver-free method called as `Type.Method(...)`. Ordinary methods require an instance. Ordinary whole-self consumption is forbidden; callable consumption is a separate contract.

References: [ownership](rules/memory/ownership.md), [copy and move](rules/memory/copy_move.md), [borrowing](rules/memory/borrowing.md), [references](rules/memory/references.md), [impl](rules/declarations/impl.md).

### Parameter mode is independent of receiver capability

| Parameter | Caller contract | Callee authority |
|---|---|---|
| `value: T` | Ordinary copy if permitted, fresh owned temporary, or explicit `<-source` | Owns its received value |
| `value: ref T` | Compatible reference or permitted implicit shared borrow | Shared access to caller storage |
| `value: ref mut T` | Mutable Place or permitted mutable reborrow; no conflicting live authority | Exclusive access to caller storage |
| `-> value: T` | Required explicit consumption from a named source | Forced ownership transfer, including copyable T |

Use canonical reference types in the parameter: `value: ref T`, not the legacy `ref value: T`. Never satisfy a mutable parameter by copying a `ref mut` holder or upgrading a shared reference.

```sec
fn Read(value: ref int) int {
    return *value
}

fn Write(value: ref mut int, replacement: int) void {
    *value = replacement
}

fn UseReferences() int {
    let mut number := 1
    Write(number, 2)             // implicit call-bounded mutable borrow
    let grant := ref mut number
    Write(grant, 3)              // reborrow; grant is not moved or copied
    return *grant
}
```

A returned reference must refer to storage that outlives the result. `return ref local` is not made safe by the result being a reference. A receiver-derived reference can outlive a method call only while its receiver storage and borrow remain valid. Returning one from a fresh temporary receiver cannot extend that temporary's lifetime.

`ref holder` and `ref mut holder` reborrow a reference holder's referent. They do not borrow the holder object itself. The mutable form requires an existing exclusive grant.

### Concrete methods, interface methods and properties

```sec
type Counter struct {
    _value: int
}

impl Counter {
    fn Read() int {
        return self._value
    }

    fn Increment() void {
        self._value += 1
    }

    static fn Zero() Counter {
        return Counter { _value: 0 }
    }

    property Value: int {
        get { return self._value }
        set next { self._value = next }
    }
}
```

- Call `Counter.Zero()` without an instance; call `counter.Increment()` with mutable receiver authority. `Counter.Increment()` does not manufacture a receiver.
- In concrete method bodies, `self` is implicit. A bare same-owner method call can use that existing receiver. A static method has no implicit instance.
- A property setter explicitly names its incoming value: `set next`. There is no implicit C#-style `value` binding. Access uses `counter.Value` and `counter.Value = expression`.
- `impl extends Counter { ... }` is an extension of the same nominal type under the impl/module rules; it is not inheritance.
- A custom destructor is `free { ... }` inside an impl, not a callable `fn Free()`. It makes the nominal type move-only. Do not invoke or synthesize destruction as an ordinary public method.

[Functions §§17–24](rules/declarations/functions.md) own call entry and receiver rules; [properties](rules/declarations/properties.md) own accessor evaluation; [destruction](rules/memory/destruction.md) owns cleanup.

## Types, sequences and text

- Types are nominal where declared nominal; sharing a machine representation does not make types interchangeable.
- `int` and `uint` are target-sized; fixed-width integer families are also available. Consult conversion rules instead of assuming a cast wraps, truncates or cannot fail.
- `T[N]` is a fixed sequence, `T[]` an owning unsized sequence, and `ref T[]` / `ref mut T[]` borrowed slices. Use suffix sequence syntax, not Go's `[]T` or Rust's `[T; N]`.
- `byte`, `char`, `rune` and `string` have distinct contracts. Do not equate a byte, Unicode scalar, grapheme or display column.
- In string length contracts, `minLen`, `maxLen`, `exactLen` and `notEmpty` count Unicode scalars (RuneLen), without normalization. `minByteLen`, `maxByteLen` and `exactByteLen` count encoded bytes. Every `string` must contain valid canonical UTF-8 (types.md, “Valid UTF-8 invariant”, MD-050). Byte decoding must validate input; slicing must preserve scalar boundaries. Arbitrary bytes belong in byte storage, not an invalid string.
- `Option[T]` uses `Some(value)` and bare `None`, not `None()`. Contextual literals need the expected-type rules; do not guess overload selection from the return type.
- Unit-annotated types and quantities have their own dimension, scale and conversion rules. Consult the units rulebook before treating them as ordinary numbers.

References: [types](rules/types/types.md), [contracts](rules/types/contracts.md), [collections](rules/collections/collections.md), [units](rules/types/units.md).

### Nominal types, contracts and defaults

```sec
type UserId uint64
type Port int range 1..65535 default 8080
type Label string minLen 1 maxLen 32 maxByteLen 128
```

`UserId` is distinct from `uint64`. Likewise `type B int` followed by `type A B` declares distinct A, B and int types. An explicit conversion needs a language-defined relation; a cast is not permission for arbitrary representation reinterpretation. Potentially failing conversions require the owning checked-conversion/`try` contract.

A contract is written after the underlying type, not as a field annotation or an invented `where` clause. `default` is part of the type's default-value contract; do not select zero for a range excluding zero. Conjunctions of contracts must be satisfiable together. Regex syntax is owned by its dedicated decision/rulebook, not by a host regex library chosen by an AI.

### Sequences and slices: shape, ownership and ranges

```sec
let fixed: int[3] := [10, 20, 30]
let owned: int[] := []
let borrowed := ref fixed[0..<2]
let empty: int[] := []
```

`fixed` stores exactly three elements; `owned` starts as an empty owning unsized sequence; `borrowed` borrows a range and does not own or copy its backing elements. `[]` needs an expected type. Read-only versus mutable slices are expressed as `ref T[]` versus `ref mut T[]`.

- Sec 0.1 does not define nonempty array literals for owning dynamic `T[]`: `let data: int[] := [1, 2]` is invalid. Start with an empty mutable owner and populate it through the fallible collection operations, for example `try data.Append(1)`, in a compatible error context.
- `a..b` includes the upper endpoint; `a..<b` excludes it. Do not assume Rust's range spelling/inclusivity.
- Slices permit omitted bounds, such as `values[..<end]` or `values[start..]`, under the range rules.
- Bounds checks and stale-storage checks have separate semantics. Use the owning `try` range/check form when the failure must be fallible; unsafe access is not a substitute.
- Ordinary `values...` spread is non-consuming and must obey each receiving position's copy/borrow rules. It does not extract move-only owned elements implicitly.
- `list[T]`, `map[K, V]`, `set[T]`, `vector`, `matrix` and `tensor` have separate construction/member contracts. Do not extrapolate methods such as `Reserve` or `Push` from another language.

A string is not a byte array, even though its payload is UTF-8. `char` covers all 256 byte-sized values; `rune` covers Unicode scalars. String contracts count runes while value-form `SizeOf` counts encoded payload bytes. For example `"é"` has one rune and two UTF-8 bytes. `ToCharArray` is byte-preserving, not a conversion to Unicode graphemes. Read the current member declaration before choosing an index unit or error type.

See [collections](rules/collections/collections.md), [reference model](rules/memory/reference_model.md), [default values](rules/types/default_values.md) and [compiler-known members](rules/compiler/compiler_known_members.md).

## Fallibility and control flow

```sec
enum ReadError error {
    Missing
}

fn Read() Result[int, ReadError] {
    return Err(ReadError.Missing)
}

fn ReadOrZero() int {
    let value := try Read() {
        Err(_) => 0
    }
    return value
}
```

- Fallible operations expose `Result[T, E]`; concrete error types are declared error enums or unions. `error` is an open error channel, not a general object base type.
- Naked `try` propagates through a compatible enclosing fallible return contract. A local handler may recover with a value or terminate/return/propagate. For value-used `try`, every reachable handler path must satisfy that obligation.
- `try` protects its expression subtree, including call arguments and receivers. Do not assume only the outer call can fail.
- Use `while condition { ... }`, `for { ... }`, or `for value in values { ... }`. Index/element iteration uses `for index, value in values`. Do not invent C-style or condition-only `for` loops.
- `match` uses `pattern => ...` arms with the owning exhaustiveness, payload-binding and reachability rules. `switch` has separate case semantics.
- `defer`, `discard`, `panic`, unsafe contexts and must-use results each have explicit rules. Do not use `discard` to evade error handling, cleanup or ownership.

References: [error handling](rules/errors/errorhandling.md), [for](rules/control-flow/flowcontrol_for.md), [match](rules/control-flow/flowcontrol_match.md), [defer](rules/control-flow/defer.md), [unsafe](rules/memory/unsafe.md).

### Propagation, recovery and exact error identity

```sec
fn ReadAgain() Result[int, ReadError] {
    let value := try Read()
    return Ok(value)
}

fn ReadWithEarlyReturn() Result[int, ReadError] {
    let value := try Read() {
        Err(problem) => { return Err(problem) }
    }
    return Ok(value)
}

fn Find() Option[int] {
    return None
}

fn FindAgain() Option[int] {
    let value := try Find()
    return Some(value)
}
```

`Ok` and `Err` construct `Result`; `Some` and `None` construct `Option`. Do not return a bare T where the signature promises `Result[T, E]`. A `None` handler belongs to Option handling, not to an invented `Err(None)` variant.

For `let n := try Read() { ... }`, a recovering handler must yield `int`; a handler ending with unconditional `return` needs no int recovery value. A terminating exhaustive `match` inside the handler can satisfy that requirement. Do not append fake recovery values after code that already returns on every path.

In `try Wrap(left + right)`, both the addition and the call are protected. Distinct failure types can coexist. `Err(problem)` may not bind one fictitious common concrete error type for a heterogeneous failure set; use typed handlers or the permitted catch-all. A guard can fail and allow later handlers/residual propagation. Consult errorhandling.md §§11–15 before narrowing a failure set.

Concrete errors widen to the open `error` channel; unrelated concrete errors do not become interchangeable. Do not invent universal `Message`, `Code` or `Description` fields on errors.

### Matching and loops

```sec
fn OptionOrZero(value: Option[int]) int {
    match value {
        Some(number) => return number
        None => return 0
    }
}

fn CountElements(values: ref int[]) uint {
    let mut count: uint := 0
    for value in values {
        discard value
        count += 1
    }
    return count
}
```

- Use `Enum.Member` or the owning contextual `.Member` form where context permits; use `_` for a wildcard, not `default` in a match.
- A union variant's payload is matched with its declared form. Bare variant patterns do not invent bindings. Consult current shallow payload/open-error narrowing rules before nesting patterns.
- Exhaustiveness, arms shadowed by earlier arms and arms excluded by a proven active variant are distinct questions; do not assign all three the same reachability error.
- `for ref item in items` borrows elements; `for ref mut item in items` requires exclusive mutable access. Plain `for item` does not silently move a move-only element or convert it into a borrow.
- Map `for key, value in map` differs from sequence `for index, value in sequence`.
- `break` and `continue` preserve cleanup. Deferred work belongs to the exiting scope/invocation; `defer` is not allowed inside custom `free` in Sec 0.1.

## Generics and heterogeneous packs

```sec
fn Count[...Types](values: ...ref Types) uint {
    return values.Len
}
```

`[T]` denotes one type parameter; `[...Types]` denotes an ordered compile-time sequence. Homogeneous `...T` and heterogeneous `...Types` are different contracts. Pack modes are owned `...Types`, shared `...ref Types` and exclusive `...ref mut Types`. Empty packs are valid. Type order and arity participate in specialization identity.

A heterogeneous pack is ephemeral, not a tuple object, heap container, `any[]`, C varargs or runtime type dictionary. Element typing and static iteration need concrete specialization; a runtime index cannot create a universal element type. Generic constraints are explicit, and return types do not select overloads. Do not assume an interface implies boxing or runtime dispatch: distinguish static generic constraints from permitted borrowed interface views.

Read [functions §38A](rules/declarations/functions.md), [generics](rules/declarations/generics.md), [interfaces](rules/declarations/interfaces.md) and [monomorphization](rules/compiler/monomorphization.md). Check current governance before promising pack-element iteration, spread, owned lowering or native execution; MD-117 support is still being expanded.

### Constraints and explicit interface conformance

```sec
interface Readable {
    fn Read() int
}

interface MutableCounter {
    mut fn Increment() void
}

fn ReadOne[T: Readable](source: ref T) int {
    return source.Read()
}
```

A concrete type declares conformance with `impl Device implements Readable { ... }` (or another permitted explicit declaration form). A same-named method alone is not structural conformance. Generic bodies may rely on the declared constraints, not arbitrary operations available only on one convenient test type.

Concrete methods use ordinary `fn` and inferred receiver authority; interface requirements use explicit `fn` for shared or `mut fn` for exclusive receiver capability because they have no body to inspect. Check the precise rule before using a consuming callable requirement as a consuming ordinary method.

Generic constraints specialize statically. Permitted borrowed interface views such as `ref Readable` are a different, runtime-dispatch mechanism. Do not replace one with boxing or an unconstrained storable interface object. `Iterator[T]` uses its own explicit conformance and concrete `Next` resolution rules; do not discover iterators by the method name alone.

### Pack decisions an AI must preserve

| Source form | Meaning |
|---|---|
| `[T]` with `values: ...T` | One element type, homogeneous variadic |
| `[...Types]` with `values: ...Types` | Ordered heterogeneous owned parameters |
| `[...Types]` with `values: ...ref Types` | Ordered shared borrows |
| `[...Types]` with `values: ...ref mut Types` | Ordered exclusive borrows; caller proves non-overlap |
| `values.Len` | Compile-time-known arity of a heterogeneous specialization |
| `values...` | Non-consuming spread under ordinary element rules |
| `<-values...` | Whole-pack consuming transfer from a complete owned native pack |

Only one trailing type pack and one corresponding trailing variadic parameter are admitted. `-> values: ...Types` is not a new pack mode. `<-values[0]` is not whole-pack transfer; array/slice consuming spread does not become valid because the punctuation looks similar.

Heterogeneous indexing requires a compile-time-known position. Heterogeneous `for` is expanded type-aware; source `break`, `continue`, `return`, `try` and cleanup keep their meanings. Do not turn a pack into a runtime array to avoid implementing expansion, or duplicate argument evaluation while specializing it.

Expected return context may fill missing scalar generic substitutions after a unique argument-selected overload, but cannot choose between overloads. Equivalent fixed signatures keep their specified preference over pack signatures. Check constraints before publishing a selected call or committing its outer moves.

Current support must be read from governance. In particular, unused/Len-only borrowed-pack support does not imply that element iteration, owned cleanup, spread or physical reference lowering is complete. Do not suppress an implementation boundary and silently compile different semantics.

## Lambdas and callable authority

```sec
let twice := fn(value: int) int { return value + value }
let step: fn(int) int := twice
```

The lambda introducer is ordinary `fn`. Captures use `capture(...) fn ...`; callable capability is inferred from capture/body behavior. Function types distinguish `fn(...) R`, `mut fn(...) R` and `-> fn(...) R`. Those describe authority over the callable environment; `-> argument: T` describes a parameter instead. Do not write `mut fn` as the lambda expression introducer or invent Rust capture syntax.

See [lambda functions](rules/declarations/lambda-functions.md) before returning a capturing closure, moving a capture or coercing callable authority.

## Units, unsafe operations and specialized domains

- Quantity syntax is `5<s>` with adjacency; `5 < s` is a comparison. A unit implementation is `impl <s> { ... }`; `impl s` targets an ordinary type, not a unit. Unit metadata, dimensions, scale and explicit rounding rules belong to [units](rules/types/units.md). A duration is not interchangeable with an arbitrary time-unit carrier.
- Use `unsafe expression`, `unsafe statement` or `unsafe { ... }` for explicitly delimited unsafe work. `unsafe fn` expresses caller obligations; its body is not implicitly an unsafe context under [unsafe §§4–7](rules/memory/unsafe.md).
- FFI names use `c::...`, not C-looking Sec aliases. [FFI](rules/platform/ffi.md) and [ABI](rules/platform/abi.md) own the target binding environment, checked conversions and foreign vararg promotions. A native heterogeneous pack is not C varargs.
- [Hardware registers](rules/platform/hardware-register-access.md), [fixed-address bindings](rules/platform/fixed-address-bindings.md), [allocation](rules/memory/allocation.md) and [concurrency](rules/concurrency/concurrency.md) have dedicated contracts. Do not replace their capabilities, failures or effects with ordinary integer/pointer operations.
- Atomic order arguments must be compile-time-known and operation-valid; consult [atomics](rules/concurrency/atomics.md) rather than allowing arbitrary runtime order selection.

## Common invalid translations

| Imported assumption | Sec correction |
|---|---|
| `type Id = int` creates a transparent alias | `type Id int` creates a distinct nominal type |
| `fn F() -> int` | `fn F() int` |
| `public` / `private` modifiers | Identifier visibility prefixes |
| `[]int`, `[int; 3]` | `int[]`, `int[3]` |
| `None()` / `null` for any missing value | Bare `None` for Option; null is an FFI sentinel under its own rules |
| A named move-only argument moves automatically | Explicit `<-source` |
| `ref r` borrows a reference holder object | Reborrow r's referent |
| Mutating concrete method must say `mut fn` | Body inference on concrete methods; explicit capability on interface requirements |
| A type with `Clone` is implicitly copyable | Copy classification follows the language's type/ownership rules |
| `try` catches every call regardless of its declared channel | Resolved failure sets, local handlers and compatible propagation |
| Every interface implies a boxed runtime object | Static constraints versus explicitly permitted borrowed views |
| Pack specialization may erase types into `any[]` | Ordered concrete positions and canonical specialization identities |

## Verify against the current compiler

For an existing source file, use the repository's current CLI, for example:

```bash
go run ./cmd/compiler check path/to/example.sec --target linux-amd64
go run ./cmd/yamlstatus -id frontend.functions-v2
go run ./cmd/yamlstatus -id semantic-ir.core-v2
```

Parsing, Sema, Semantic IR, verified Sec MLIR and execution are separate evidence levels. A capability diagnostic is an implementation limitation, not automatically an invalid language construct. Report exactly the stage tested.

The compilation track is Sema → Semantic IR → Sec/standard MLIR → LLVM dialect → LLVM IR → clang. Do not create an alternate backend to bypass unsupported lowering. Compiler work packages such as P15 name implementation packages, not Sec source modules.

For compiler changes, preserve rulebook traceability, add focused source fixtures under `testdata/`, update the owning governance through `cmd/yamlstatus`, and sync shared work through [the compiler handoff](mlir/compiler-packages-handoff.md). Do not claim a broad remaining item closed when only one supported subset has grown.
