# Sec MLIR Dialect

## Status

Normative detailed dialect specification for the currently implemented Sec MLIR
surface.

Dialect schema version: `1`

This rulebook is subordinate to:

```text
rules/sec_mlir.md
```

and to all higher-authority Sec language/domain rulebooks and
`rules/semantic_ir.txt`.

It defines representation only.

It must not introduce or redefine Sec language semantics.

---

# 1. Scope of dialect schema version 1

Schema version 1 intentionally defines only the dialect foundation required by
Sec MLIR implementation Package 1.

Defined in version 1:

```text
dialect namespace
dialect version marker
named type representation
distinct type representation
common metadata key names
source-location policy
basic structural verification
```

Not defined in version 1:

```text
Sec semantic operations
ownership operations
lifetime operations
reference operations
storage operations
checks
failure operations
aggregate operations
allocation operations
string operations
hardware operations
concurrency operations
ABI lowering
Semantic IR import
standard MLIR lowering
LLVM lowering
```

A future version of this rulebook must define those surfaces before they are
implemented.

---

# 2. Dialect identity

MLIR dialect namespace:

```text
sec
```

C++ namespace:

```text
::sec
```

The Sec dialect is not runtime-extensible.

Sec operations and types must be statically registered.

---

# 3. Dialect schema version

Compiler-generated high-level Sec MLIR modules use the module attribute:

```mlir
sec.dialect_version = 1 : i32
```

The version identifies the Sec MLIR schema, not the Sec source-language version.

Schema version 1 does not define migration between versions.

Development tools may parse hand-written test modules without the marker.

A production Semantic IR importer, when implemented, must emit the marker.

---

# 4. Named type

Canonical textual form:

```mlir
!sec.named<"type-id", base-type>
```

Example:

```mlir
!sec.named<"main::Speed", i64>
```

Parameters:

```text
identity: StringAttr
base: Type
```

The identity is an opaque compiler identity.

The displayed identity is not interpreted by MLIR lowering.

The base type records the already-selected lower representation type available
at this MLIR level.

`!sec.named` preserves Sec named-type identity.

Two named types are identical only when both their identity and base type are
identical.

A named type must not be treated as implicitly interchangeable with its base
type.

The existence of the base parameter does not authorize an implicit cast.

Verifier requirements:

```text
identity must not be empty
base must be a valid MLIR type
base must not be NoneType
```

The type does not itself encode:

```text
contracts
units
ownership
copy classification
destruction
ABI
layout offsets
```

Those are represented or resolved by their applicable compiler layers.

---

# 5. Distinct type

Canonical textual form:

```mlir
!sec.distinct<"type-id", base-type>
```

Example:

```mlir
!sec.distinct<"main::CustomerID", i64>
```

Parameters:

```text
identity: StringAttr
base: Type
```

The identity is an opaque compiler identity.

`!sec.distinct` preserves complete Sec distinct-type identity.

Two distinct types are identical only when both their identity and base type are
identical.

A distinct type must not be treated as implicitly interchangeable with its base
type.

Verifier requirements:

```text
identity must not be empty
base must be a valid MLIR type
base must not be NoneType
```

---

# 6. Common metadata keys

Schema version 1 reserves these attribute names:

```text
sec.symbol_id
sec.type_id
sec.layout_ref
sec.synthesized
sec.dialect_version
```

Their values use standard MLIR attributes:

```text
sec.symbol_id       StringAttr
sec.type_id         StringAttr
sec.layout_ref      StringAttr
sec.synthesized     BoolAttr
sec.dialect_version IntegerAttr(i32)
```

The strings are opaque values supplied by earlier compiler layers.

MLIR code must not infer Sec semantics by parsing identity strings.

This version does not standardize a cross-compilation identity serialization.

---

# 7. Source provenance

Normal MLIR `Location` is the canonical representation of source file, line and
column information.

Example:

```mlir
loc("example.sec":12:8)
```

The Sec dialect must not duplicate ordinary source coordinates into a
Sec-specific attribute.

Compiler-synthesized IR may carry:

```text
sec.synthesized = true
```

when a synthesized marker is useful.

Later dialect schema versions may define richer origin metadata only when the
information cannot be represented adequately by normal MLIR locations and the
higher-authority Sec rules require it.

---

# 8. Verification boundary

Sec MLIR verification validates representation invariants.

It does not repeat source semantic analysis.

Schema version 1 verification includes:

```text
registered dialect/type recognition
non-empty named type identity
non-empty distinct type identity
valid base type
valid textual representation
```

It does not perform:

```text
name lookup
overload resolution
ownership analysis
borrow analysis
lifetime analysis
contract validation
unit algebra
layout calculation
ABI selection
failure-policy selection
```

Invalid Semantic IR must be rejected before Sec MLIR construction.

---

# 9. Parser and printer

The canonical parser/printer forms for schema version 1 are:

```mlir
!sec.named<"identity", base-type>
!sec.distinct<"identity", base-type>
```

Round-tripping through the canonical MLIR parser and printer must preserve:

```text
type kind
identity
base type
source locations
reserved Sec metadata attributes
```

The printer need not preserve insignificant whitespace.

---

# 10. Evolution rule

New Sec types, attributes, operations, interfaces or traits must not be added
only because an implementation needs a convenient placeholder.

Before a new Sec dialect construct is implemented:

1. the corresponding Sec semantics must already be defined by higher-authority
   rulebooks;
2. this rulebook must define the representation contract;
3. the implementation must conform to that contract;
4. tests must verify the contract.

The dialect may reuse standard MLIR dialect constructs whenever they preserve all
remaining Sec obligations exactly.

---

# 11. Package 1 completion

Dialect schema version 1 is considered implemented when:

```text
the dialect registers
the Package 1 types parse
the Package 1 types print
the Package 1 types verify
invalid Package 1 forms are rejected
normal MLIR locations are preserved
reserved metadata survives round-trip
a Sec-aware MLIR driver can load the dialect
```

No Semantic IR import or lowering is required for schema version 1.
