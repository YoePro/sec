# Sec Interfaces

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-14
- **Document revision:** 2.0
- **Replaces:** `rules/declarations/interfaces.txt`
- **Sec language version:** 0.1
- **Canonical path:** `rules/declarations/interfaces.md`

## 1. Purpose

An interface defines a behavioral contract that a concrete Sec type may explicitly implement.

Interfaces define required behavior and associated API surface. They do not define or extend the stored instance representation of an implementing type.

## 2. Interface declaration

```sec
interface Reader {
    fn Read(buffer: ref mut byte[]) Result[int, IOError]
}
```

An interface may declare:

- instance methods,
- mutable instance methods,
- consuming instance methods,
- static methods where meaningful,
- properties,
- associated or nested type declarations where permitted by the corresponding declaration rules.

An interface must not require stored instance fields.

If a contract requires readable or writable state, express that requirement through a property or method.

## 3. Receiver capability

Interface methods have no implementation body from which receiver requirements can be inferred. Therefore the receiver capability is part of the interface contract.

### 3.1 Shared receiver

```sec
interface Counter {
    fn Value() int
}
```

Plain `fn` requires only shared/non-mutating receiver access.

### 3.2 Mutable receiver

```sec
interface Counter {
    mut fn Increment() void
}
```

`mut fn` requires mutable/exclusive receiver access.

### 3.3 Consuming receiver

```sec
interface Buffer {
    -> fn IntoBytes() byte[]
}
```

`-> fn` consumes ownership of the receiver. After a successful call, the original owned receiver is no longer usable.

A consuming interface method cannot be called through `ref` or `ref mut`, because borrowing does not transfer ownership.

### 3.4 Static member

```sec
interface Parsable {
    static fn Parse(value: string) Result[Self, ParseError]
}
```

`static fn` is a type-level member and has no instance receiver.

### 3.5 Concrete implementations

Concrete implementation methods use ordinary `fn` syntax with implicit `self`.

```sec
impl CounterImpl implements Counter {
    fn Value() int {
        return self.value
    }

    fn Increment() void {
        self.value += 1
    }
}
```

Sema infers the concrete receiver requirement from the method body and verifies that the implementation does not require stronger receiver capability than the interface contract permits.

### 3.6 Receiver and parameter ownership are independent

Receiver capability does not replace the ownership contract of ordinary
parameters. For example:

```sec
interface Sender {
    mut fn Send(-> message: Message) Result[void, SendError]
}
```

requires mutable/exclusive receiver access and independently consumes
`message`. Interface conformance must preserve both contracts.

## 4. Explicit conformance

Concrete interface conformance is declared on the primary implementation.

```sec
type FileReader struct {
    handle: FileHandle,
}

impl FileReader implements Reader, Closeable {
    fn Read(buffer: ref mut byte[]) Result[int, IOError] {
        ...
    }

    fn Close() void {
        ...
    }
}
```

The `implements` list belongs to the primary `impl`.

`impl extends Type` fragments contribute behavior to the same implementation but do not redeclare the interface list.

```sec
impl extends FileReader {
    fn Helper() void {
        ...
    }
}
```

The type declaration remains focused on representation.

## 5. Interface inheritance

An interface may implement one or more other interfaces.

```sec
interface ReadWriter implements Reader, Writer {
}
```

An interface that implements another interface inherits that interface's behavioral requirements.

Duplicate or conflicting inherited requirements are compile-time errors unless the declarations are semantically identical under the applicable member compatibility rules.

## 6. Conformance requirements

A concrete type conforms to an interface only when all required members are present and compatible.

Conformance checks include, where applicable:

- member name,
- member category,
- parameter count and parameter types,
- parameter borrow, consuming, and variadic modes,
- generic constraints,
- result type,
- fallibility,
- property access contract,
- receiver capability,
- static versus instance membership,
- consuming ownership behavior.

Return type is not an overload discriminator.

Concrete receiver behavior may be less demanding than the interface contract, but never more demanding.

Examples:

- interface `fn` + concrete method requiring mutation: invalid;
- interface `mut fn` + concrete non-mutating method: valid;
- interface `-> fn` + concrete method that consumes the receiver: valid;
- interface `-> fn` + concrete method that does not consume the receiver: valid if all other contract requirements are satisfied.

## 7. Interface values and borrowing

Calls through an interface value obey the receiver capability declared by the interface.

```sec
fn Inspect(value: ref Counter) void {
    value.Value()
}
```

A shared borrow cannot call `mut fn` or `-> fn`.

```sec
fn Update(value: ref mut Counter) void {
    value.Increment()
}
```

A mutable borrow may call shared and mutable methods, but cannot call a consuming method.

An owned interface value may call consuming methods.

## 8. Properties instead of required fields

Interfaces do not declare required stored fields.

Use properties when the contract requires state exposure.

```sec
interface Positioned {
    property Position: Point {
        get
        set position
    }
}
```

The implementing type decides how the value is represented or computed.

Interface property accessors have no body. A setter still declares its incoming
value parameter explicitly:

```sec
interface Configurable {
    property Mode: Mode {
        get
        try set mode
    }
}
```

`get` requires readable access. `set name` requires infallible writable access,
and `try set name` requires fallible writable access. A setter requirement
implies mutable receiver access. Conformance checks the property type, accessor
availability, setter fallibility, receiver capability, and static versus
instance category.

## 9. Generics

Interfaces may be used as generic constraints according to the generic rules.

```sec
fn Print[T: Printable](value: ref T) void {
    ...
}
```

`T: Printable` constrains substitution; it does not declare implementation.
Constraint satisfaction requires valid explicit conformance such as
`impl Document implements Printable`.

Interfaces may themselves declare generic parameters:

```sec
interface Sink[T] {
    mut fn Write(value: T) void
}
```

`Sink[byte]` is one concrete interface contract. A method requirement may use
the interface's generic parameters but may not introduce additional
method-level generic parameters, because interface dispatch must remain finite
and Sec has no runtime generic dispatch.

Invalid:

```sec
interface Mapper[T] {
    fn Map[U](value: T) U
}
```

Interface inheritance and explicit implementation participate in generic
constraint satisfaction.

## 10. Associated and nested declarations

Interfaces may own associated or nested declarations where the corresponding declaration rules permit them.

Such declarations do not add stored instance data to implementing types.

The implemented type's stored representation remains defined solely by its type declaration.

## 11. Primary implementation and module ownership

Ordinary `impl` ownership rules apply.

A concrete type's primary implementation and `impl extends` fragments belong to the type's defining module.

Interface conformance declared by that primary implementation is therefore also owned by the defining module.

Cross-module ordinary implementation of imported foreign types is not permitted.

## 12. Diagnostics

The compiler must diagnose at least:

- missing required interface member,
- incompatible member signature,
- incompatible property contract,
- receiver capability stronger than the interface contract,
- invalid static/instance mismatch,
- invalid consuming call through a borrow,
- duplicate interface in an `implements` list,
- incompatible inherited interface requirements,
- `implements` declared on an `impl extends` fragment,
- required stored field declared directly in an interface,
- ordinary interface implementation outside the type's defining module.

## 13. Best practice

- Keep interfaces focused on observable behavior and API contracts.
- Prefer properties over exposing representation requirements.
- Use plain `fn` whenever mutation is not part of the contract.
- Use `mut fn` only when callers must provide mutable/exclusive receiver access.
- Use `-> fn` only when consuming ownership is part of the semantic contract.
- Keep the `implements` list on the primary `impl` so programmers have one clear conformance entry point.
- Prefer small, composable interfaces and use interface inheritance only when the semantic relationship is real.

## 14. Cross-rulebook ownership

This rulebook owns interface declaration, receiver capability in interface contracts, interface inheritance, and explicit conformance placement.

The following are defined elsewhere:

- ordinary and extended implementation structure: `rules/declarations/impl.md`;
- function and method signatures: functions rulebook;
- properties: properties rulebook;
- borrowing and ownership: borrowing/ownership rulebooks;
- generics and interface constraints: generics rulebook;
- lifecycle construction/destruction: `rules/declarations/impl.md` and destruction rules.
