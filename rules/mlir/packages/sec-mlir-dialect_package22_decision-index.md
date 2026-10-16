# Package 22 Decision Index

- **Status:** Decision audit — no new P22 decision proposed
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package22_decision-index.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Numbering protocol

When a genuinely new unresolved normative fork arises in a future implementation package, create **one dedicated English Markdown file per decision**, using an ID such as `P22-D1`, `P22-D2` (or `P23-D1`, etc.), and link it from the package manifest. The user integrates decisions into their `MD-###` tracking. Do not duplicate an existing MD item under a new Pxx-Dy number.

## § 2. New P22 decisions

```text
None discovered during the 2026-10-16 P22 specification audit.
```

No P22-D1 file is created merely to occupy a number. This index can be updated if implementation uncovers a genuinely new normative choice.

## § 3. Existing unresolved dependencies

| Existing ID | Subject | P22 handling |
|---|---|---|
| MD-010 | Canonical regex language and contract integration | Block regex lowering; no substitute implementation. |
| MD-050 | Invalid UTF-8 semantic policy | Block paths depending on the decision, retain valid-UTF-8 paths. |
| P21-D1 | Decimal result scale policy | Remains owned by P21; P22 does not choose it. |
| P21-D2 | Decimal arithmetic error family | Remains owned by P21; conversions use already locked MD-012. |
| P21-D3 | `decimal128` core member parity | Remains owned by P21. |

## § 4. Already settled, not new questions

| ID | Resolved fact |
|---|---|
| MD-012 | Public checked-conversion error is `ConversionError`, with nested `ContractError`. |
| MD-014 | Platform-width float semantics supersede old hardcoded width assumptions. |
| MD-058 | String `.Len` counts Unicode scalars and value `SizeOf` counts UTF-8 bytes. |

## § 5. Decision-file template for future packages

```text
# Pxx-Dy — <Short Decision Title>
Status: Open
Created: YYYY-MM-DD
Revision: 1
Applies to: SEC-MLIR-Pxx
Canonical rulebooks: <paths>
Existing MD reference: <ID or none>

§ 1. Existing normative facts
§ 2. Genuine unresolved fork
§ 3. Options and consequences
§ 4. Technical recommendation
§ 5. Decision reserved for the Sec owner
§ 6. Required rulebook/governance/test synchronization after approval
```

Each decision is submitted for explicit approval. A package implementation agent must not assume a recommendation is accepted.
