package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Verify the serialized shared pool and published lookup offsets together,
// independently of output currency: a wrong offset could otherwise regenerate
// deterministic but incorrect classifiers from correct input ranges.
// Rules: rules/library/core-library.md — §11.3.
func TestGeneratedPropertyLookupOffsets(t *testing.T) {
	db := loadRepositoryUCD(t)
	properties := unicodeProperties(db)
	source := string(renderCoreProperties(db, properties))
	var scalars []rune
	for _, m := range regexp.MustCompile(`0x([0-9A-F]{6})r`).FindAllStringSubmatch(source, -1) {
		value, err := strconv.ParseInt(m[1], 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		scalars = append(scalars, rune(value))
	}
	lookups := map[string][]string{}
	for _, m := range regexp.MustCompile(`fn _UnicodeIs([A-Za-z0-9]+)\(ch: rune\) bool \{\s*return _UnicodeHasProperty\(ch, ([0-9]+), ([0-9]+)\)`).FindAllStringSubmatch(source, -1) {
		lookups[m[1]] = m
	}
	for _, p := range properties {
		m, ok := lookups[p.name]
		if !ok {
			t.Fatalf("missing lookup %s", p.name)
		}
		first, _ := strconv.Atoi(m[2])
		count, _ := strconv.Atoi(m[3])
		if count != len(p.ranges) || 2*(first+count) > len(scalars) {
			t.Fatalf("invalid lookup bounds for %s", p.name)
		}
		for i, r := range p.ranges {
			if scalars[2*(first+i)] != r.start || scalars[2*(first+i)+1] != r.end {
				t.Fatalf("%s routes to the wrong data at range %d", p.name, i)
			}
		}
	}
}

// Classification is exact Unicode property membership rather than block
// approximation, including noncharacters, case distinctions and emoji bases.
// Rules: rules/library/core-library.md — §11.3(1–5).
func TestUnicodePropertyExamples(t *testing.T) {
	properties := map[string][]codePointRange{}
	for _, p := range unicodeProperties(loadRepositoryUCD(t)) {
		properties[p.name] = p.ranges
	}
	for _, tc := range []struct {
		name  string
		value rune
		want  bool
	}{
		{"Emoji", '0', true}, {"EmojiPresentation", '0', false},
		{"Emoji", '#', true}, {"Emoji", 0x1F600, true}, {"Emoji", 0x1F02C, false},
		{"EmojiPresentation", 0x1F600, true}, {"EmojiModifier", 0x1F3FB, true},
		{"EmojiModifierBase", 0x1F44D, true}, {"EmojiComponent", 0x200D, true},
		{"ExtendedPictographic", 0x1F600, true}, {"Emoji", 'A', false},
		{"UpperLetter", 0x100, true}, {"LowerLetter", 0x100, false},
		{"LowerLetter", 0x101, true}, {"UpperLetter", 0x101, false},
		{"TitleLetter", 0x1C5, true}, {"TitleLetter", 'A', false},
		{"Whitespace", 0xB, true}, {"Whitespace", 0xC, true},
		{"Whitespace", 0x85, true}, {"Whitespace", 0x1680, true},
		{"Whitespace", 0x200B, false}, {"CombiningMark", 0x93E, true},
		{"Punctuation", '$', false}, {"Symbol", '$', true},
		{"Punctuation", '.', true}, {"Symbol", '.', false},
		{"Noncharacter", 0xFDD0, true}, {"Noncharacter", 0x1FFFE, true},
		{"Noncharacter", 0x10FFFF, true}, {"Noncharacter", 0x1F600, false},
		{"CategoryCo", 0xE000, true}, {"CategoryCn", 0x378, true},
		{"CategoryCs", 0xD800, false}, {"CategoryC", 0xDFFF, false},
		{"UnicodeHexDigit", 0xFF21, true}, {"AsciiHexDigit", 0xFF21, false},
		{"UnicodeDigit", 0x661, true}, {"IDStart", 'A', true},
		{"Number", 0x2165, true}, {"UnicodeDigit", 0x2165, false},
		{"IDStart", '0', false}, {"IDContinue", '0', true},
	} {
		ranges, ok := properties[tc.name]
		if !ok {
			t.Fatalf("missing property %s", tc.name)
		}
		if got := tableContains(ranges, tc.value); got != tc.want {
			t.Errorf("%s(%U)=%t, want %t", tc.name, tc.value, got, tc.want)
		}
	}
}

// Newly added official property sources are pinned just like the original
// casing inputs; a modified file must not generate plausible stale tables.
// Rules: rules/library/core-library.md — §11.3.
func TestPropertySourcesRejectTampering(t *testing.T) {
	for _, name := range []string{"PropList.txt", "emoji/emoji-data.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for source := range ucdFileHashes {
				data, err := os.ReadFile(filepath.Join(repositoryUCD, source))
				if err != nil {
					t.Fatal(err)
				}
				if source == name {
					data = append(data, []byte("\n0041 ; Emoji # tampered\n")...)
				}
				dest := filepath.Join(dir, source)
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := loadUCD(dir, unicodeVersion); err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "does not match the pinned") {
				t.Fatalf("tampered source accepted: %v", err)
			}
		})
	}
}

// Every category is checked over the entire scalar domain; binary sources must
// all survive generation and no output can contain a surrogate rune literal.
// Rules: rules/library/core-library.md — §11.3(1–2).
func TestUnicodePropertyCoverage(t *testing.T) {
	db := loadRepositoryUCD(t)
	properties := unicodeProperties(db)
	seen := map[string]bool{}
	for _, p := range properties {
		if seen[p.name] {
			t.Fatalf("duplicate public spelling %s", p.name)
		}
		seen[p.name] = true
		for i, r := range p.ranges {
			if r.start > r.end || r.start < 0 || r.end > maxCodePoint || r.start <= 0xDFFF && r.end >= 0xD800 || i > 0 && p.ranges[i-1].end+1 >= r.start {
				t.Fatalf("invalid normalized scalar ranges for %s: %v", p.name, p.ranges)
			}
		}
		if category, ok := strings.CutPrefix(p.source, "General_Category="); ok && len(category) == 2 && category != "LC" {
			for ch := rune(0); ch <= maxCodePoint; ch++ {
				want := db.category(ch) == category && (ch < 0xD800 || ch > 0xDFFF)
				if tableContains(p.ranges, ch) != want {
					t.Fatalf("%s differs at %U", category, ch)
				}
			}
		}
	}
	if len(properties) != 111 || len(db.binaryProperties) != 63 {
		t.Fatalf("unexpected catalog: %d properties, %d binary", len(properties), len(db.binaryProperties))
	}
	for name := range db.binaryProperties {
		found := false
		for _, p := range properties {
			found = found || p.source == name
		}
		if !found {
			t.Fatalf("binary property %s omitted", name)
		}
	}
}

// Invalid headers, ranges, property fields and enumerated values fail closed;
// only the supported non-binary InCB records are deliberately not booleans.
// Rules: rules/library/core-library.md — §11.3(2).
func TestParseBinaryPropertyValidation(t *testing.T) {
	for _, data := range []string{
		"# PropList-17.0.0.txt\n0041; White_Space\n",
		"# PropList-18.0.0.txt\n0042..0041; White_Space\n",
		"# PropList-18.0.0.txt\n110000; White_Space\n",
		"# PropList-18.0.0.txt\n0041;\n",
		"# PropList-18.0.0.txt\n0041; White_Space; Yes\n",
	} {
		if err := (&ucdDatabase{}).parseBinaryProperties(strings.NewReader(data), "PropList", unicodeVersion); err == nil {
			t.Fatalf("accepted malformed data %q", data)
		}
	}
	for _, version := range []string{"", "17.0.0"} {
		data := "# emoji-data.txt\n# Version: " + version + "\n0041; Emoji\n"
		if err := (&ucdDatabase{}).parseBinaryProperties(strings.NewReader(data), "emoji-data", unicodeVersion); err == nil {
			t.Fatal("accepted unversioned/wrong emoji data")
		}
	}
	db := &ucdDatabase{}
	if err := db.parseDerivedCoreProperties(strings.NewReader("# DerivedCoreProperties-18.0.0.txt\n0915; InCB; Consonant\n0041; Alphabetic\n"), unicodeVersion); err != nil {
		t.Fatal(err)
	}
	if len(db.binaryProperties) != 1 || !tableContains(db.binaryProperties["Alphabetic"], 'A') {
		t.Fatal(db.binaryProperties)
	}
}

// Custom output directories apply to the complete family; check mode reports
// every stale/missing file and preserves both modified and missing artifacts.
// Rules: rules/library/core-library.md — §11.3.
func TestUnicodeGenerationCheckAndOutputIsolation(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "unicode", "unicode.sec")
	core := filepath.Join(root, "core")
	if err := generateUnicode(repositoryUCD, output, core, false); err != nil {
		t.Fatal(err)
	}
	if err := generateUnicode(repositoryUCD, output, core, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(core, "unicode_properties_generated.sec")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	err := generateUnicode(repositoryUCD, output, core, true)
	if err == nil || !strings.Contains(err.Error(), output) || !strings.Contains(err.Error(), missing) {
		t.Fatalf("incomplete stale report: %v", err)
	}
	if data, err := os.ReadFile(output); err != nil || string(data) != "modified" {
		t.Fatal("check mode wrote output")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("check mode recreated a file")
	}
}
