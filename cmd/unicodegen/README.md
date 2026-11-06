# Unicode classification generation

Run from the repository root:

```sh
go run ./cmd/unicodegen
go run ./cmd/unicodegen -check
```

Generation is offline. The six checked-in Unicode 18.0.0 inputs are validated
against SHA-256 pins before any output is produced. Updating a Unicode version
requires reviewing the inputs and updating both the version and the pins.
`-input` chooses an input directory; `-output` chooses the unicode module file;
`-core-output` chooses the directory for all three generated core files.
`-check` reports every missing/stale artifact and never writes.

Sources:

- [UnicodeData.txt](https://www.unicode.org/Public/18.0.0/ucd/UnicodeData.txt)
- [DerivedCoreProperties.txt](https://www.unicode.org/Public/18.0.0/ucd/DerivedCoreProperties.txt)
- [PropList.txt](https://www.unicode.org/Public/18.0.0/ucd/PropList.txt)
- [emoji-data.txt](https://www.unicode.org/Public/18.0.0/ucd/emoji/emoji-data.txt)
- [SpecialCasing.txt](https://www.unicode.org/Public/18.0.0/ucd/SpecialCasing.txt)
- [CaseFolding.txt](https://www.unicode.org/Public/18.0.0/ucd/CaseFolding.txt)

The generated catalog exposes 112 scalar predicates on `rune` and as functions
in `unicode`: 30 General_Category classes, eight aggregate classes, 63 binary
properties and 11 common aliases (including `IsLetter`). Identical category
aliases share the same immutable range pool and binary search in core. The
stdlib wrappers store no second copy of the tables. Decimal digit zero values
are also generated for the numeric digit-value API.

Examples include `IsEmoji`, `IsEmojiPresentation`, `IsEmojiComponent`,
`IsEmojiModifier`, `IsEmojiModifierBase`, `IsExtendedPictographic`,
`IsWhitespace`, `IsCombiningMark`, `IsPunctuation`, `IsSymbol`, `IsNumber`,
`IsNoncharacter`, `IsAlphabetic`, `IsIDStart`, `IsIDContinue`, `IsLowercase`
and `IsCategoryLu`. See `rules/library/core-library.md` §11.3 for exact naming
and semantics, including ASCII-only legacy digit/hex helpers.

`Emoji` includes ASCII keycap bases (`#`, `*`, digits); it does not mean that a
scalar renders as a standalone emoji. Sequences and grapheme clusters require
a separate recognizer. Surrogates never enter generated rune ranges;
noncharacters remain valid scalars and have their own predicate.

Casing mappings and conditions are parsed and validated but the public
simple/full/contextual casing API is still tracked in `stdlib.unicode-tables`.
It is not part of this classifier catalog. The same owner separately tracks
executable standard-library behavior tests: data/generation tests and source
checking must not be described as native execution coverage.
