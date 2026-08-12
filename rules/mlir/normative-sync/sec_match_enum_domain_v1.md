# Package 12 Normative Amendment - Enum Match Domain

## Status

Normative amendment for:

```text
rules/control-flow/flowcontrol_match.txt
rules/declarations/enums.txt
```

Package:

```text
SEC-MLIR-P12
```

Repository baseline:

```text
152c772
```

This amendment resolves an existing conflict between enum value semantics and
enum match exhaustiveness.

---

# 1. Existing facts that must remain true

The following existing enum rules remain unchanged:

```text
enum values have the enum type
an enum has an integer-backed semantic value
duplicate numeric values are valid aliases
alias names remain distinct declarations
explicit integer-to-enum conversion is allowed
explicit conversion may produce an enum value with no declared case name
enum equality/inequality compares values of the same enum type
```

No amendment removes numeric aliases or unnamed converted enum values.

---

# 2. Canonical enum pattern meaning

A pattern:

```sec
EnumType.Case
```

matches a subject enum value when:

```text
subject semantic numeric enum value == Case semantic numeric enum value
```

The declaration case identity is source provenance.

It is not an additional hidden runtime discriminant.

---

# 3. Alias-equivalent patterns

If:

```text
CaseA numeric value == CaseB numeric value
```

then:

```text
CaseA
CaseB
```

match the same enum subject set.

Therefore an unguarded arm for one fully covers the other's numeric pattern.

Example:

```sec
enum Status int {
    Good = 0
    Ok = 0
    Bad = 1
}
```

Invalid:

```sec
match status {
    Status.Good => A()
    Status.Ok => B()
    _ => C()
}
```

The second arm is unreachable.

---

# 4. Guarded aliases

Valid:

```sec
match status {
    Status.Good where condition => A()
    Status.Ok => B()
    _ => C()
}
```

The first guarded arm does not fully cover numeric value `0`.

If its guard is false, the later value-equivalent pattern remains available.

---

# 5. Duplicate/reachability key

For enum match analysis, coverage identity is:

```text
canonical arbitrary-precision numeric enum value
```

not declared case name.

Use declaration names only for diagnostics/provenance.

---

# 6. Enum runtime domain

Because explicit integer-to-enum conversion may produce an undeclared value,
the possible enum value domain is the full representable domain of the enum's
underlying representation.

Examples:

```text
bit[1]  -> 0..1
bit[2]  -> 0..3
uint8   -> 0..255
int8    -> -128..127
```

For target-sized `int`/`uint`, use the active CompilationPlan when width is
available.

If width/range is not available at the analysis point, the compiler must not
assume declared cases are the entire runtime domain.

---

# 7. Enum match exhaustiveness

An enum match is exhaustive when either:

```text
an unguarded catch-all _ exists
```

or:

```text
the compiler proves that the unique numeric values covered by unguarded enum
patterns cover the complete representable enum domain
```

Guarded arms do not count as full coverage.

Aliases contribute only one numeric coverage value.

---

# 8. Ordinary enums normally require catch-all

Example:

```sec
enum Direction {
    North
    East
    South
    West
}
```

The following is generally non-exhaustive:

```sec
match direction {
    Direction.North => A()
    Direction.East => B()
    Direction.South => C()
    Direction.West => D()
}
```

because explicit conversion can create other numeric `Direction` values.

Canonical exhaustive form:

```sec
match direction {
    Direction.North => A()
    Direction.East => B()
    Direction.South => C()
    Direction.West => D()
    _ => Other()
}
```

---

# 9. Full finite-domain exception

A catch-all is not required when full domain coverage is proven.

Example:

```sec
enum Bit: bit[1] {
    Zero = 0
    One = 1
}

match value {
    Bit.Zero => A()
    Bit.One => B()
}
```

is exhaustive.

Likewise a `bit[2]` enum may be exhaustive without `_` when unguarded numeric
patterns cover:

```text
0
1
2
3
```

regardless of how many alias declarations also exist.

---

# 10. Exhaustiveness implementation

Use arbitrary-precision range/cardinality logic.

Do not enumerate a huge integer domain.

Conceptually:

```text
coveredUniqueNumericCount == representableDomainCardinality
```

is sufficient when every covered numeric value is known representable.

For an enum whose representable domain cardinality is too large to equal the
finite declared pattern set, reject as non-exhaustive without enumeration.

---

# 11. Match after explicit conversion

Example:

```sec
let status: Status := Status(77)
```

A later match must handle the possibility of numeric value `77` even if no
declared Status case has that value.

A catch-all handles it.

Do not reinterpret explicit conversion as choosing the nearest or matching
declared case name.

---

# 12. Equality consistency

The match rule must remain consistent with same-enum equality.

If:

```text
Status.Good == Status.Ok
```

because both have the same semantic numeric value, then a match pattern for one
must also match a runtime subject whose numeric enum value equals the other.

No hidden provenance bit is introduced.

---

# 13. Required synchronization

Update:

```text
rules/control-flow/flowcontrol_match.txt
rules/declarations/enums.txt
Sema enum match coverage
enum match tests
manual examples derived from these rules
```

Do not change enum storage layout to solve match aliases.

Do not prohibit aliases.

Do not prohibit explicit unnamed enum values.

---

# 14. Diagnostic guidance

Recommended:

```text
unreachable enum match arm Status.Ok; numeric value 0 is already fully covered
```

and:

```text
non-exhaustive match for Status; enum may contain undeclared numeric values;
add _ or cover the complete underlying domain
```

Stable diagnostic IDs follow the normal diagnostic registry.
