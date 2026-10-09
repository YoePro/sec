# String length units and byte contracts

- **Status:** Applied 2026-10-09
- **Authority:** Explicit language-owner decision in the implementation session.
- **Affected rules:** rules/types/contracts.md; rules/compiler/compiler_known_members.md; rules/foundations/grammar.md; rules/foundations/lexical_structure.md; rules/types/types.md; rules/library/core-library.md.

1. `string.Len` equals `RuneLen` and counts Unicode scalar values, without
   normalization or grapheme counting. Byte-oriented operations use `ByteLen`.
2. On strings, `minLen`, `maxLen`, `exactLen` and `notEmpty` use RuneLen.
   Collection length contracts retain logical element counting.
3. Add reserved contextual string-only contracts `minByteLen`, `maxByteLen`,
   `exactByteLen`, each with a nonnegative semantic-CTE integer argument.
4. Compile-time and runtime checks share units, source order and first-failure
   behavior. Add corresponding ContractKind variants without replacing the
   ConversionError.Contract channel. Mutable implementation status stays in
   governance/types.yaml.
