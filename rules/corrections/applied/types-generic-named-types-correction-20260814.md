# Correction: generic named types

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** type declaration rules

Generic named types are canonical Sec 0.1.

```sec
type ID[T] int
```

Concrete instantiations preserve nominal identity.

```sec
ID[User]
ID[Product]
```

are different concrete named types even when their underlying representation is the same.

A generic named type may use its parameter as the represented type where the ordinary type rules permit the resulting concrete declaration.

```sec
type Wrapped[T] T
```

After substitution, normal named-type rules govern:

- conversions;
- contracts;
- defaultability;
- copy/move behavior;
- size and alignment;
- interface conformance;
- ABI representation.

Do not retain stale wording that generic primitive-derived/named types are merely postponed.
