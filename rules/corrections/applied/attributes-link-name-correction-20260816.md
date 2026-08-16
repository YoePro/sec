# Correction: register `@link_name` as a compiler-known FFI attribute

- **Status:** Applied 2026-08-16
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/foundations/attributes.md`

## Problem

The canonical attribute set is closed.

The FFI rulebook and compiler implementation already use:

```sec
@link_name("open")
extern "C" fn c_open(
    path: RawPtr[byte],
    flags: int32,
) int32
```

Therefore `@link_name` must be registered in the canonical compiler-known attribute set rather than existing as an undocumented exception.

## Attribute definition

Add:

```text
@link_name("foreign-symbol")
```

to the Sec 0.1 compiler-known attribute set.

### Allowed target

`@link_name` is valid on an `extern` declaration that introduces a foreign-link symbol.

It is not a renaming mechanism for ordinary Sec functions.

### Arguments

Exactly one string literal argument is required.

```sec
@link_name("open")
extern "C" fn c_open(...) int32
```

The foreign symbol string may contain characters permitted by the selected object/link environment even when those characters are not legal Sec identifiers.

The Sec declaration name remains the source-level name used for Sec name resolution.

### Semantic effect

`@link_name("symbol")` selects the foreign/link symbol associated with the declaration.

It does not:

- change the Sec declaration name;
- change calling convention;
- imply a library dependency;
- imply ownership;
- imply safety;
- imply effects;
- change ABI type compatibility.

### Duplicates

More than one `@link_name` on the same declaration is invalid.

### Conflicts

Two declarations that resolve to a conflicting explicit foreign symbol under the same applicable link domain are rejected according to FFI/linking rules unless a future explicit aliasing rule permits them.

### Diagnostics

Required diagnostics include at least:

```text
@link_name is valid only on extern declarations
```

```text
@link_name requires exactly one string literal argument
```

```text
duplicate @link_name attribute
```

### Formatter

The formatter preserves `@link_name` and its string argument.

### Implementation status

The existing FFI implementation already parses `@link_name`, retains it through AST/semantic function metadata, uses it for foreign symbol references, and diagnoses duplicate explicit link symbols.

Mutable implementation status remains outside the normative attribute rulebook.
