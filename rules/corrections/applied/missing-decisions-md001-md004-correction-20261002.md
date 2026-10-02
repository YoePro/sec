# Correction: Missing Decisions MD-001 through MD-004

**Status:** Applied correction  
**Applied:** 2026-10-02  
**Created:** 2026-10-02  
**Updated:** 2026-10-02  
**Revision:** 1  
**Sec version:** 0.1  
**Canonical path:** `rules/corrections/applied/missing-decisions-md001-md004-correction-20261002.md`  
**Replaces:** None

## 1. Purpose

1.1. This correction records the normative decisions for missing-decision entries MD-001 through MD-004.

1.2. This correction contains only decisions that have been explicitly accepted.

1.3. Owning rulebooks and governance fragments shall be synchronized with these decisions before the corresponding entries are removed from `missing-decisions.yaml`.

## 2. MD-001 — Visually confusable identifiers

2.1. Identifier identity remains based on exact NFC-normalized spelling. Confusable detection does not change identifier identity.

2.2. Visually confusable identifier detection shall use Unicode Technical Standard #39 (UTS #39), including the Unicode confusable skeleton data from `confusables.txt`.

2.3. The normative confusable-data version shall be the same Unicode version used by the compiler for its other Unicode tables.

2.4. A visually confusable collision in a namespace and scope where the declarations would conflict is a compile-time error, not a warning.

2.5. The confusable-identifier error is not suppressible.

2.6. Identifiers in distinct scopes that do not otherwise conflict remain legal. In particular, local identifiers in separate functions may have visually confusable spellings.

2.7. Unit symbols occupy a separate unit-symbol namespace.

2.8. A unit symbol therefore does not conflict by name with an ordinary identifier. For example, unit symbol `<s>` and an ordinary variable named `s` may coexist.

2.9. Confusable comparison shall not treat the unit-symbol namespace and the ordinary identifier namespace as one shared namespace.

## 3. MD-002 — Named type declaration syntax

3.1. The canonical Sec 0.1 syntax for declaring a named type is:

```sec
type Name TypeReference
```

3.2. A named type declaration has exactly one underlying type.

3.3. Examples of valid declarations are:

```sec
type A int
type B string
type C float
```

3.4. A named type remains distinct from its underlying type. Chained named types remain distinct identities.

3.5. This is valid when `B` is already a type:

```sec
type B int
type A B
```

`int`, `B`, and `A` are three distinct types.

3.6. A declaration shall not name multiple underlying types. Forms such as these are invalid:

```sec
type A int string
type A B int
```

3.7. The legacy forms using `=` are not valid Sec 0.1 syntax:

```sec
type Name = ExistingType
type Name = First Second
```

3.8. When such a legacy form is recognized unambiguously, the compiler should emit a focused migration diagnostic rather than an unrelated generic parser error.

3.9. Migration handling for legacy syntax follows the policy in Section 4.

## 4. MD-003 — Legacy syntax migration policy

4.1. Sec 0.1 accepts only the canonical Sec 0.1 syntax defined by the active rulebooks.

4.2. Recognizable legacy Sec syntax is invalid Sec 0.1 syntax and shall receive a focused migration diagnostic when the intended migration can be determined reliably.

4.3. Default formatting shall not silently convert invalid legacy syntax into canonical syntax.

4.4. When sloppy correction is explicitly enabled, the formatter may rewrite a recognized legacy form only when the transformation is unambiguous and semantics-preserving.

4.5. If a legacy form cannot be rewritten without ambiguity or possible semantic change, sloppy correction shall not guess. The compiler or formatter shall instead report a focused migration diagnostic.

4.6. Prefix sequence spelling such as `[]byte` is not classified as legacy Sec syntax because it was never normative Sec syntax. The canonical Sec 0.1 spelling is `byte[]`.

4.7. For Sec 0.1, declarations such as structs continue to use the `type` introducer:

```sec
type User struct {
    id: int
}
```

4.8. The planned Sec 0.2 spelling:

```sec
struct User {
    id: int
}
```

is future canonical syntax and is not a Sec 0.1 legacy form.

## 5. MD-004 — Runtime string concatenation and interpolation

5.1. Runtime string concatenation and runtime string interpolation are fallible materialization operations.

5.2. A concatenation or interpolation expression is semantically one materialization operation that produces one resulting `string`.

5.3. The language does not require observable intermediate `string` values for individual concatenation steps.

5.4. The compiler may therefore measure all parts, allocate or otherwise prepare final storage once, write all parts into that storage, and produce the final string, provided observable language semantics are preserved.

5.5. Runtime materialization failure is propagated through Sec's ordinary fallible-operation mechanism. A runtime string materialization that may fail therefore requires the normal `try` handling required for a fallible expression.

5.6. A string concatenation or interpolation that is fully resolved at compile time may be folded into static materialized data and is not a runtime fallible operation.

5.7. Backend optimizations may remove temporaries or allocations only when doing so preserves the specified success, failure, ownership, and destruction behavior.

5.8. This correction does not select the allocator or allocation context used by runtime string materialization. That remaining choice must be specified separately before lowering invents allocator-selection semantics.

## 6. Synchronization requirements

6.1. The owning rulebooks for MD-001 through MD-004 shall be updated to record these decisions normatively.

6.2. Affected governance fragments shall be synchronized after the rulebooks have been corrected.

6.3. `missing-decisions.yaml` entries MD-001 through MD-003 may be removed after their owning rulebooks and governance fragments are synchronized.

6.4. MD-004 shall not be considered fully resolved solely by this correction if implementation still requires an unspecified allocator or allocation-context policy.
