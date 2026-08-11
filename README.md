# Sec Programming Language

> **Readable by default. Safe by design. Low-level when needed.**

Sec is a statically typed programming language designed for application, systems, embedded and bare-metal development.

Its central idea is simple:

**The programmer expresses intent. The compiler handles as much mechanical complexity as it can prove safely.**

Sec combines deterministic ownership, strong type identity, contracts, units, explicit error handling and low-level hardware access without requiring garbage collection or exposing compiler bookkeeping as source-level ceremony.

Sec is influenced by languages including C, C++, C#, F#, Go, Rust, Zig, Ada and Vale, but it is not a derivative of any one of them. Ideas are adopted, changed or rejected according to Sec's own design goals.

The language is under active development and is **not yet production ready**.

---

## Design goals

Sec aims to provide:

- **Readable code** without unnecessary semantic bureaucracy.
- **Strong static analysis** without requiring programmers to understand compiler theory.
- **Deterministic ownership and destruction** without garbage collection.
- **Static proof first**, with checked runtime semantics when a safety condition depends on runtime data.
- **Explicit fallibility** through `Result`, `try` and typed errors.
- **Nominal types and contracts** for domain correctness.
- **First-class units** with dimensional checking.
- **Predictable cost** with no hidden heap allocation, expensive copying or ownership transfer.
- **Low-level access** for FFI, syscalls, memory-mapped hardware and bare-metal targets.
- **One language model across targets**, from hosted applications to microcontrollers.
- **A sophisticated compiler, not sophisticated ceremony.**

Sec deliberately keeps advanced implementation concepts such as regions, data-flow states and most lifetime bookkeeping inside the compiler.

---

## A first Sec program

```sec
module main

import fmt

fn main() int {
    fmt.Println("Hello from Sec")
    return 0
}
```

Functions use explicit parameter and return types. `void` is used when no value is returned.

```sec
fn Add(left: int, right: int) int {
    return left + right
}

fn Notify(message: string) void {
    fmt.Println(message)
}
```

---

## Variables

Variables are immutable by default.

```sec
let name := "Anna"
let count: int := 10

let mut retries := 0
retries += 1
```

Typed mutable variables may be declared without an initializer:

```sec
int mut: x, y, z
```

Multiple declarations may also be written together:

```sec
let width := 1920, height := 1080
```

Mutability must be requested explicitly.

---

## Named types and contracts

Named types have their own identity even when they share the same underlying representation.

```sec
type Percent int range 0..100
type Age int range 0..130

let progress: Percent := 75
let age: Age := 50
```

`Percent` and `Age` are different types.

Runtime values require explicit conversion:

```sec
fn ReadPercent(value: int) Result[Percent, ContractError] {
    let percent := try Percent(value)
    return Ok(percent)
}
```

Contracts belong to types, not individual variables.

This allows domain rules to travel with the type instead of being repeated at every use site.

---

## Units

Sec units carry semantic meaning and participate in dimensional analysis.

```sec
unit Meter decimal physical
unit Second decimal physical
unit Speed decimal physical
```

Unit metadata defines dimensions and conversion relationships.

```sec
impl Meter {
    dimension: [length^1]
    scale: 1
    system: SI
}

impl Second {
    dimension: [time^1]
    scale: 1
    system: SI
}

impl Speed {
    dimension: [length^1, time^-1]
    scale: 1
    system: SI
}
```

The compiler can then validate unit algebra:

```sec
fn CalculateSpeed(distance: Meter, elapsed: Second) Speed {
    return distance / elapsed
}
```

An accidental assignment of a speed to a distance is a type error rather than a convention the programmer must remember.

---

## Structs and impl

Data belongs in the type declaration.

Behavior belongs in `impl`.

```sec
type Point struct {
    X: int
    Y: int
}

impl Point {
    fn IsOrigin() bool {
        return X == 0 && Y == 0
    }
}
```

`self` is available to instance behavior without requiring repetitive receiver declarations where the language can determine the receiver from context.

Struct fields that have defaults do not all need to be written explicitly during construction.

---

## Properties

Properties expose behavior through member syntax without becoming stored fields.

```sec
type Motor struct {
    _speed: Speed
}

impl Motor {
    property CurrentSpeed: Speed {
        get {
            return _speed
        }

        set value {
            _speed = value
        }
    }
}
```

A setter may be fallible:

```sec
impl Motor {
    property RequestedSpeed: Speed {
        get {
            return _speed
        }

        try set value {
            _speed = value
            return Ok()
        }
    }
}
```

Fallible assignment remains visible at the call site:

```sec
try motor.RequestedSpeed = requested {
    Err(error) => Handle(error)
}
```

---

## Result and try

Recoverable errors are explicit in function signatures.

```sec
fn ParsePort(text: string) Result[Port, ParseError] {
    // ...
}
```

`try` unwraps a successful result and propagates a compatible error:

```sec
fn Connect(text: string) Result[Connection, ParseError] {
    let port := try ParsePort(text)
    return Ok(Connection.Open(port))
}
```

Errors can instead be handled locally:

```sec
let port := try ParsePort(input) {
    Err(ParseError.InvalidNumber) => Port(8080)
    Err(error) => return Err(error)
}
```

`match` is available when all alternatives should be explicit:

```sec
match ParsePort(input) {
    Ok(port) => Use(port)
    Err(error) => Report(error)
}
```

Sec does not use hidden exceptions for ordinary typed error handling.

---

## Arrays and slices

Fixed arrays and borrowed slices are distinct concepts.

```sec
let values: int[4] := [10, 20, 30, 40]

let first := values[0]
let middle := values[1..<10]
```

Array sizes are compile-time values.

Slices are non-owning views:

```sec
fn Sum(values: ref int[]) int {
    let mut total := 0

    for value in values {
        total += value
    }

    return total
}
```

Constant indexes can be checked at compile time. Dynamic indexes use the language's checked runtime semantics when the compiler cannot prove the access safe.

---

## References and ownership

Sec distinguishes ownership from borrowing.

```sec
fn Inspect(value: ref Buffer) void {
    // shared access
}

fn Modify(value: ref mut Buffer) void {
    // exclusive mutable access
}
```

The compiler tracks ownership, moves and borrows.

The normal source language does not require explicit lifetime annotations.

Unsafe raw addresses use a separate type:

```sec
let address: RawPtr[byte]
```

`RawPtr[T]` is not a reference and does not imply ownership.

---

## Interfaces

Interfaces define explicit contracts.

```sec
interface Printable {
    fn Print() void
}

type Report struct implements Printable {
    Title: string
}

impl Report {
    fn Print() void {
        fmt.Println(Title)
    }
}
```

Implementation is explicit. Sec does not use accidental structural conformance.

Interfaces may also be used as generic constraints.

```sec
fn PrintValue[T: Printable](value: ref T) void {
    value.Print()
}
```

---

## Generics

Generics are compile-time constructs and are monomorphized.

```sec
type Pair[A, B] struct {
    First: A
    Second: B
}

fn Identity[T](value: T) T {
    return value
}

let number := Identity(42)
let text := Identity[string]("Sec")
```

Generic parameters do not exist at runtime and do not imply boxing, type erasure or dynamic dispatch.

---

## Lambdas

Anonymous functions use the same `fn` model as named functions.

```sec
let double := fn(value: int) int {
    return value * 2
}

let result := double(21)
```

Captures are explicit:

```sec
let factor := 10

let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

Sec does not silently capture surrounding local variables.

---

## Registers and hardware

Sec has first-class register types for memory-mapped hardware and fixed-width bit layouts.

```sec
type Control register[8] {
    Enabled: bit
    Mode: bit[3]
    _: bit[4]
}
```

A register can be bound directly to hardware storage:

```sec
@address(0x40021000)
let mut control: Control

control.Enabled = true
```

`@address` means external volatile storage.

`mut` determines whether Sec code may write it.

The compiler is responsible for masks, shifts, volatile access and preservation of unrelated register bits.

This gives low-level hardware control without requiring normal application code to manipulate raw pointers manually.

---

## Unsafe and FFI

Unsafe operations are explicit and localized.

```sec
extern "C" fn write(
    fd: int32,
    buffer: RawPtr[byte],
    length: uint
) int64

unsafe {
    let result := write(fd, data, length)
}
```

`unsafe` does not turn semantic analysis off.

Type checking, ownership rules, visibility, control flow and error handling still apply.

Foreign APIs should normally be wrapped in typed Sec functions that establish ownership, nullability and error semantics.

---

## Deterministic cleanup

Sec uses deterministic ownership and destruction.

Owned values are cleaned up when their lifetime ends unless ownership has been transferred.

```sec
fn Process() Result[void, IOError] {
    let file := try File.Open("input.txt")
    let buffer := try Buffer.Create(4096)

    return Ok()
}
```

Cleanup requirements are determined by the compiler and occur on all relevant control-flow exits.

`defer` adds explicit cleanup behavior:

```sec
fn Work() void {
    defer {
        Log("leaving Work")
    }

    RunTask()
}
```

Sec does not require garbage collection, reference counting or manually written cleanup for ordinary owned values.

---

## Compiler analysis

Sec treats compiler analysis as part of the language experience rather than an optional afterthought.

The compiler architecture includes or plans analyses for areas such as:

- ownership and borrowing
- escape behavior
- effects
- definite assignment
- control flow
- destruction and cleanup
- recursion
- stack usage
- ISR safety
- concurrency
- common pitfalls

The language server can expose useful analysis interactively while deeper project analysis can be run explicitly.

The purpose is not merely to reject invalid programs, but to explain why code is valid, unsafe, expensive or suspicious when that information is useful to the programmer.

---

## Compiler architecture

The implementation is structured around:

```text
Source
  ↓
Lexer / Parser
  ↓
AST
  ↓
Semantic Analysis
  ↓
Semantic IR
  ↓
Sec MLIR dialect
  ↓
LLVM
  ↓
Native code
```

Semantic rules belong to Sec, not to LLVM or MLIR.

The backend may change without changing the meaning of valid Sec source programs.

---

## Targets

Sec is intended to support both hosted and freestanding environments.

Planned and developing targets include but is not limited to:

- Linux
- Windows
- macOS
- BSD systems
- FreeRTOS
- bare-metal ARM Cortex-M
- RISC-V

Target profiles may restrict facilities such as heap allocation, threads or operating-system services, but they must not silently redefine core language semantics.

---

## Repository structure

Important repository areas include:

```text
cmd/          compiler commands
internal/     compiler implementation
rules/        normative language and compiler specifications
stdlib/       standard library
platform/     target-specific platform support
runtime/      minimal runtime facilities where required
testdata/     language and compiler test programs
```

The files under `rules/` are the normative specification.

Manuals, examples and other documentation are derived from those rules and are not normative when they disagree with them.

---

## Project status

Sec is in active language and compiler development.

Large parts of the lexer, parser and semantic analyzer already exist, together with an evolving Semantic IR and MLIR-based lowering architecture.

Not every documented feature has complete backend support yet.

A rule may therefore be:

- specified,
- parsed,
- semantically validated,
- represented in Semantic IR,
- lowered through MLIR,
- or fully executable,

without all of those implementation stages necessarily being complete.

The rulebooks and implementation-status documents in the repository provide the detailed status.

---

## Philosophy in one sentence

> **Make the programmer state the decisions that matter, let the compiler derive the bookkeeping, and never hide semantics that matter for correctness or cost.**
