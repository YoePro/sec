# Storage cross-reference correction — fixed-address bindings

**Status:** Applied  
**Applied:** 2026-08-13  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Document revision:** 1  
**Sec language version:** 0.1  
**Target:** `rules/memory/storage.md`  
**Related rename:** `rules/platform/registers.txt` -> `rules/platform/fixed-address-bindings.md`

Update cross-references that currently attribute addressed-register volatility,
MMIO specialization, or fixed-address platform semantics to `registers.txt`.

They must instead refer to:

```text
rules/platform/fixed-address-bindings.md
```

The semantic ownership after the rename is:

```text
rules/memory/storage.md
    abstract fixed-address storage contract

rules/platform/fixed-address-bindings.md
    source-level @address / MMIO / volatile fixed-address binding semantics

rules/declarations/registers.md
    register[N] type and bit-layout semantics
```

Do not otherwise change the canonical storage rule that fixed-address placement
is orthogonal and does not by itself imply volatility or mutability.

Where the current text says that an `@address` register binding is volatile
because `registers.txt` defines that additional rule, retain the semantic
statement but replace the ownership reference with
`rules/platform/fixed-address-bindings.md`.
