package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

// parseBinaryProperties reads binary UCD properties without mistaking enumerated
// values (InCB; Linker, etc.) for booleans. Emoji has a different version header.
// Rules: rules/library/core-library.md — §11.3; UAX #44, UTS #51 data formats.
func (db *ucdDatabase) parseBinaryProperties(r io.Reader, name, version string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if name == "emoji-data" {
		if !strings.HasPrefix(string(data), "# emoji-data.txt\n") || !strings.Contains(string(data), "\n# Version: "+version+"\n") {
			return fmt.Errorf("emoji-data version header must identify %s", version)
		}
	}
	properties := map[string][]codePointRange{}
	header, err := ucdLines(bytes.NewReader(data), func(_ int, fields []string) error {
		if len(fields) < 2 || fields[1] == "" {
			return fmt.Errorf("missing property name")
		}
		value, err := parseCodePointRange(fields[0])
		if err != nil {
			return err
		}
		if len(fields) == 3 && name == "DerivedCoreProperties" && fields[1] == "InCB" {
			switch fields[2] {
			case "Linker", "Consonant", "Extend":
				return nil
			}
		}
		if len(fields) != 2 {
			return fmt.Errorf("unsupported property value for %s", fields[1])
		}
		properties[fields[1]] = append(properties[fields[1]], value)
		return nil
	})
	if err != nil {
		return err
	}
	if name != "emoji-data" {
		if err := checkVersionHeader(header, name, version); err != nil {
			return err
		}
	}
	if db.binaryProperties == nil {
		db.binaryProperties = map[string][]codePointRange{}
	}
	for name, ranges := range properties {
		if _, exists := db.binaryProperties[name]; exists {
			return fmt.Errorf("duplicate property %s", name)
		}
		db.binaryProperties[name] = scalarRanges(normalizeRanges(ranges))
	}
	return nil
}

// scalarRanges excludes surrogate code points from generated rune literals,
// including the General_Category Cs table, which is empty for scalar inputs.
// Rules: rules/types/types.md — rune; rules/library/core-library.md — §11.3.
func scalarRanges(ranges []codePointRange) []codePointRange {
	var out []codePointRange
	for _, r := range ranges {
		if r.end < 0xD800 || r.start > 0xDFFF {
			out = append(out, r)
			continue
		}
		if r.start < 0xD800 {
			out = append(out, codePointRange{r.start, 0xD7FF})
		}
		if r.end > 0xDFFF {
			out = append(out, codePointRange{0xE000, r.end})
		}
	}
	return out
}

type unicodeProperty struct {
	name, source string
	ranges       []codePointRange
}

// unicodeProperties gives every General_Category (including aggregate classes)
// and every binary property a stable API spelling and sorted scalar ranges.
// Rules: rules/library/core-library.md — §11.3; rules/library/stdlib.md — Unicode classification.
func unicodeProperties(db *ucdDatabase) []unicodeProperty {
	var out []unicodeProperty
	for _, cat := range strings.Fields("Lu Ll Lt Lm Lo Mn Mc Me Nd Nl No Pc Pd Ps Pe Pi Pf Po Sm Sc Sk So Zs Zl Zp Cc Cf Cs Co Cn L LC M N P S Z C") {
		categories := []string{cat}
		if len(cat) == 1 || cat == "LC" {
			categories = nil
			for _, c := range strings.Fields("Lu Ll Lt Lm Lo Mn Mc Me Nd Nl No Pc Pd Ps Pe Pi Pf Po Sm Sc Sk So Zs Zl Zp Cc Cf Cs Co Cn") {
				if strings.HasPrefix(c, cat) || cat == "LC" && (c == "Lu" || c == "Ll" || c == "Lt") {
					categories = append(categories, c)
				}
			}
		}
		out = append(out, unicodeProperty{"Category" + cat, "General_Category=" + cat, scalarRanges(unicodeCategoryRanges(db, categories...))})
	}
	aliases := map[string]string{"LowerLetter": "Ll", "UpperLetter": "Lu", "TitleLetter": "Lt", "UnicodeDigit": "Nd", "Control": "Cc", "Punctuation": "P", "Symbol": "S", "CombiningMark": "M", "Number": "N", "Separator": "Z"}
	for name, cat := range aliases {
		for _, p := range out {
			if p.name == "Category"+cat {
				out = append(out, unicodeProperty{name, p.source, p.ranges})
				break
			}
		}
	}
	for source, ranges := range db.binaryProperties {
		name := strings.ReplaceAll(source, "_", "")
		switch source {
		case "White_Space":
			name = "Whitespace"
		case "Hex_Digit":
			name = "UnicodeHexDigit"
		case "ASCII_Hex_Digit":
			name = "AsciiHexDigit"
		case "Noncharacter_Code_Point":
			name = "Noncharacter"
		}
		out = append(out, unicodeProperty{name, source, ranges})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// handwrittenRuneProperties identifies existing rune declarations whose bodies
// delegate to generated lookups; the remaining properties are generated below.
// Rules: rules/library/core-library.md — §§11.2–11.3.
var handwrittenRuneProperties = map[string]bool{
	"LowerLetter": true, "UpperLetter": true, "TitleLetter": true, "UnicodeDigit": true,
	"Control": true, "Punctuation": true, "Symbol": true, "CombiningMark": true,
	"Emoji": true, "Whitespace": true, "Noncharacter": true,
}

// renderCoreProperties emits one shared immutable range pool, one binary search,
// named lookup functions and rune properties. Identical tables share storage.
// Rules: rules/library/core-library.md — §11.3.
func renderCoreProperties(db *ucdDatabase, properties []unicodeProperty) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by go run ./cmd/unicodegen. DO NOT EDIT.\n// Unicode Character Database %s; scalar classification (UAX #44, UTS #51).\n\nmodule core\n\n", db.version)

	var pool []codePointRange
	positions := map[string][2]int{}
	shared := map[string][2]int{}
	for _, p := range properties {
		key := fmt.Sprint(p.ranges)
		pos, ok := shared[key]
		if !ok {
			pos = [2]int{len(pool), len(p.ranges)}
			pool = append(pool, p.ranges...)
			shared[key] = pos
		}
		positions[p.name] = pos
	}
	fmt.Fprintf(&out, "// Inclusive scalar pairs; property offsets count pairs, not individual entries.\nlet _unicodePropertyRanges: rune[%d] := [\n", 2*len(pool))
	for _, r := range pool {
		fmt.Fprintf(&out, "    0x%06Xr, 0x%06Xr,\n", r.start, r.end)
	}
	fmt.Fprint(&out, "]\n\n")
	fmt.Fprintln(&out, `/** Binary search in one property's scalar ranges. Rules: rules/library/core-library.md §11.3. */
fn _UnicodeHasProperty(ch: rune, first: int, count: int) bool {
    let mut low := first
    let mut high := first + count - 1
    while low <= high {
        let middle := low + (high - low) / 2
        let offset := middle * 2
        let start := _unicodePropertyRanges[offset]
        let end := _unicodePropertyRanges[offset + 1]
        if ch < start {
            high = middle - 1
        } else if ch > end {
            low = middle + 1
        } else {
            return true
        }
    }
    return false
}`)
	for _, p := range properties {
		pos := positions[p.name]
		fmt.Fprintf(&out, "\n/** Unicode %s; UAX #44/UTS #51. Rules: rules/library/core-library.md §11.3. */\nfn _UnicodeIs%s(ch: rune) bool {\n    return _UnicodeHasProperty(ch, %d, %d)\n}\n", p.source, p.name, pos[0], pos[1])
	}
	fmt.Fprintln(&out, "\nimpl extends rune {")
	for _, p := range properties {
		if !handwrittenRuneProperties[p.name] {
			fmt.Fprintf(&out, "    /** Unicode %s; scalar property, no allocation. Rules: rules/library/core-library.md §11.3. */\n    property Is%s: bool {\n        get {\n            return _UnicodeIs%s(self)\n        }\n    }\n\n", p.source, p.name, p.name)
		}
	}
	fmt.Fprintln(&out, "}")
	return out.Bytes()
}

// renderUnicodeModule exposes the same generated scalar classification through
// stdlib/unicode without copying the core range data.
// Rules: rules/library/stdlib.md — Unicode classification; rules/library/core-library.md — §11.3.
func renderUnicodeModule(db *ucdDatabase, properties []unicodeProperty) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by go run ./cmd/unicodegen. DO NOT EDIT.\n// Unicode Character Database %s.\n\nmodule unicode\n", db.version)
	properties = append([]unicodeProperty{{name: "Letter", source: "General_Category=L"}}, properties...)
	for _, p := range properties {
		fmt.Fprintf(&out, "\n/** Unicode %s on a scalar; no allocation. Rules: rules/library/core-library.md §11.3. */\nfn Is%s(ch: rune) bool {\n    return ch.Is%s\n}\n", p.source, p.name, p.name)
	}
	return out.Bytes()
}
