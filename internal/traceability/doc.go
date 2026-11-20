// Package traceability holds the source-to-rulebook traceability check
// (governance.source-rulebook-traceability; rules/README.md "Implementation
// traceability"). Functions that implement, validate, analyze or lower
// normative Sec behavior cite their rulebook in their doc comment. Functions
// written before the rule existed are listed in testdata/baseline.txt; the
// list only shrinks as references are added, and code is never removed
// because a reference is missing.
package traceability
