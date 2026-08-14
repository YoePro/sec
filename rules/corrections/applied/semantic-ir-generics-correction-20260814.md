# Correction: generic monomorphization in Semantic IR

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** Semantic IR rulebook

Semantic IR must preserve the distinction between:

- generic source templates;
- canonical concrete monomorphized instances.

For a concrete instance, preserve at least:

```text
generic declaration identity
ordered concrete generic arguments
resolved constraints
concrete substituted signature/type
canonical instance identity
instantiation source location
```

Representation-dependent lowering must not receive unresolved generic type parameters.

ABI-facing and backend-facing callable/type representations are concrete monomorphized instances.

Repeated requests for the same generic declaration with the same ordered concrete arguments must resolve to the same canonical instance.
