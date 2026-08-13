# Sec Properties

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 2.0
- **Language version:** Sec 0.1
- **Replaces:** `rules/declarations/properties.txt`
- **Canonical path:** `rules/declarations/properties.md`

## 1. Purpose

A property defines controlled member-like behavior for reading or assigning a value.

A field stores instance data.

A property defines behavior.

Properties therefore belong to implementations or interface contracts and never add stored instance data to the implemented type.

## 2. Property forms

An instance property is declared inside a primary `impl` or an `impl extends` fragment.

```sec
impl Vehicle {
    property TopSpeed: Speed {
        get {
            return self._speed
        }

        set speed {
            self._speed = speed
        }
    }
}
```

A property may contain:

- one `get`,
- one `set`, or
- one `try set`.

A property must contain at least one accessor.

`set` and `try set` are mutually exclusive because both define assignment behavior.

A property may not contain stored fields or arbitrary declarations.

## 3. Setter parameter is always explicit

A setter always declares the incoming value parameter explicitly.

```sec
property Name: string {
    set value {
        self._name = value
    }
}
```

The parameter name is not magical and is not reserved. Any valid identifier may be used.

```sec
property Name: string {
    set banana {
        self._name = banana
    }
}
```

The setter parameter has the declared property type.

Sec has no C#-style implicit setter-value binding.

This rule applies equally to:

- instance properties,
- static properties,
- interface property requirements.

A setter without an explicit parameter is invalid.

```sec
property Name: string {
    set {
        self._name = value
    }
}
```

Expected diagnostic:

```text
setter for Name must declare a value parameter
```

## 4. Getter semantics

A getter reads the property value.

```sec
property CurrentSpeed: Speed {
    get {
        return self._speed
    }
}
```

A getter:

- must return a value assignable to the property type;
- must return on every reachable path required by the normal function return rules;
- uses ordinary expression, ownership, borrowing, error, and effect rules;
- has implicit `self` for instance properties.

A getter is not a stored field read merely because its body reads a field. It is behavior and may perform any operation permitted by the surrounding rules.

A property without a getter is not readable.

## 5. Infallible setter semantics

An infallible setter is declared with `set`.

```sec
property TopSpeed: Speed {
    set speed {
        self._speed = speed
    }
}
```

The setter receives the value being assigned to the property.

```sec
vehicle.TopSpeed = requestedSpeed
```

The setter parameter is initialized from `requestedSpeed`.

An infallible setter must not produce an error result.

A property without either `set` or `try set` is read-only.

A property without `get` is write-only.

## 6. Fallible setter semantics

A fallible setter is declared with `try set`.

```sec
property TopSpeed: Speed {
    try set speed {
        if speed < MinimumSpeed {
            return Err(SpeedError.TooLow)
        }

        self._speed = speed
    }
}
```

Normal completion is the success path.

The effective setter error type is determined and validated according to the error-handling rules.

Assignment through a fallible setter requires explicit `try`.

```sec
try vehicle.TopSpeed = requestedSpeed {
    Err(error) => Handle(error)
}
```

Assignment without `try` is invalid when the selected setter is fallible.

```sec
vehicle.TopSpeed = requestedSpeed
```

Expected diagnostic:

```text
assigning fallible property TopSpeed requires try
```

A property may not declare both `set` and `try set`.

## 7. Compound assignment

Compound assignment to a property uses the property's getter and setter semantics.

```sec
vehicle.Count += 1
```

requires both:

- a readable property value, and
- an assignable setter.

Conceptually:

1. evaluate the receiver exactly once;
2. invoke/read the getter exactly once;
3. evaluate the right-hand side exactly once;
4. compute the compound result;
5. pass the computed value to the setter exactly once.

The compiler must preserve source evaluation order and side effects.

If the selected setter is fallible, compound assignment requires `try`.

```sec
try vehicle.TopSpeed += delta {
    Err(error) => Handle(error)
}
```

A write-only property cannot be the target of compound assignment because no getter exists.

## 8. Instance receiver

Instance property bodies have implicit `self`, following the ordinary `impl` rules.

```sec
impl Counter {
    property Value: int {
        get {
            return self._value
        }

        set next {
            self._value = next
        }
    }
}
```

The programmer does not declare `ref self` or `ref mut self`.

Sema infers the receiver access requirements from the property bodies and enforces ordinary ownership and borrowing rules.

## 9. Primary impl and `impl extends`

Properties may be declared in either:

```sec
impl Type {
    ...
}
```

or:

```sec
impl extends Type {
    ...
}
```

All fragments in the same module contribute to one combined implementation.

Property identity and duplicate checking therefore apply across the complete primary implementation plus all `impl extends` fragments.

A property may not be duplicated or conflict with another member in the target type's member namespace.

## 10. Static properties

A static property is declared with `static property`.

```sec
impl Application {
    static property Mode: ApplicationMode {
        get {
            return Application._mode
        }

        set mode {
            Application._mode = mode
        }
    }
}
```

Static properties:

- have no instance receiver;
- use the same explicit setter-parameter rule;
- use the same `get`, `set`, and `try set` semantics;
- are accessed through the type.

```sec
let current := Application.Mode
Application.Mode = ApplicationMode.Safe
```

Static-property details that are common to other static members are defined by `static.md`.

## 11. Interface property requirements

Interfaces may require properties.

Interfaces do not require stored instance fields.

```sec
interface Positioned {
    property Position: Point {
        get
        set position
    }
}
```

An interface property requirement describes the observable contract.

- `get` requires readable access.
- `set name` requires infallible writable access.
- `try set name` requires fallible writable access.

The explicit setter parameter is required even in an interface declaration.

A setter requirement implies mutable receiver access for the setter operation.

The implementing type chooses whether the property is backed by a field, computed value, foreign state, register access, or another valid implementation strategy.

Conformance checks property type, accessor availability, fallibility, static/instance category, and receiver capability.

## 12. Property identity and namespace

A property has:

- a name,
- a declared property type,
- an optional getter,
- an optional infallible setter,
- an optional fallible setter,
- an effective setter error type when fallible,
- static or instance membership,
- source location.

Property names participate in the target type's member namespace.

Properties may not silently shadow or collide with:

- stored fields,
- register fields,
- methods where the member naming rules forbid the collision,
- another property,
- associated members that occupy the same namespace.

Member registration occurs before property bodies are analyzed, so property bodies may reference later-declared compatible members.

## 13. Interaction with constrained named types

Contracts belong to types, not variables.

```sec
type Percent int range 0..100
```

A property of a constrained named type uses the ordinary rules for that type.

```sec
type Settings struct {
    _volume: Percent,
}

impl Settings {
    property Volume: Percent {
        get {
            return self._volume
        }

        try set volume {
            self._volume = try volume
        }
    }
}
```

Fallibility caused by validating a constrained named type remains explicit through normal `try` rules.

The compiler may model contract-checked assignment using property-like internal operations, but that lowering does not create a source-level property declaration.

## 14. Effects and side effects

Properties are behavioral members and may have effects.

A getter is not assumed pure merely because it uses member-access syntax.

A setter is not assumed to be a simple field store.

Analysis and lowering must preserve:

- evaluation order,
- getter side effects,
- setter side effects,
- fallible setter behavior,
- ownership and borrowing,
- cleanup and `defer`,
- volatile or foreign effects when reached through the property implementation.

Effect classification follows the effect-analysis rules.

## 15. Lowering

Properties do not introduce hidden stored fields by themselves.

Semantic lowering must represent property access explicitly enough that later stages can distinguish:

- property read,
- property write,
- fallible property write,
- compound property update.

Backends must not infer property semantics from ordinary field access.

Lowering may inline or optimize property behavior when legal, but optimization must preserve the language-visible semantics.

## 16. Diagnostics

The compiler must diagnose at least:

- property missing a name;
- missing `:` after the property name;
- missing property type;
- property with no accessor;
- duplicate getter;
- duplicate setter;
- both `set` and `try set`;
- setter without an explicit parameter;
- getter result incompatible with the property type;
- getter missing required return;
- assignment to a read-only property;
- read from a write-only property;
- assignment type incompatible with the property type;
- fallible property assignment without `try`;
- compound assignment without a getter;
- duplicate/conflicting property across primary and extended impl fragments;
- interface property conformance mismatch;
- invalid receiver/borrow use in a property body.

Diagnostics should identify the property and the violated contract.

## 17. Best practice

- Use fields for stored representation and properties for controlled behavior.
- Prefer a field when no behavioral boundary is needed.
- Use a property when access must validate, compute, synchronize, translate, observe, or otherwise enforce behavior.
- Name setter parameters for clarity; `value` is conventional but never implicit.
- Avoid surprising effects in getters unless the abstraction genuinely requires them.
- Use `try set` when assignment can legitimately fail; do not hide failure behind an infallible setter.
- Keep related properties near the main behavior of the type; use `impl extends` when splitting a large implementation across files improves readability.

## 18. Cross-rulebook ownership

This rulebook owns property declaration and access semantics.

Related rules are owned elsewhere:

- primary/extended implementations and implicit `self`: `rules/declarations/impl.md`;
- static-member rules: `rules/declarations/static.md`;
- interface contracts and receiver capabilities: `rules/declarations/interfaces.md`;
- constrained named types and contracts: type/contract rulebooks;
- fallibility and `try`: error-handling rulebooks;
- ownership and borrowing: memory rulebooks;
- effects: effect-analysis rulebooks;
- semantic lowering: Semantic IR and MLIR rulebooks.
