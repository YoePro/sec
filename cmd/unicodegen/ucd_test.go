package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestParseUnicodeDataCategoriesRangesAndSimpleMappings(t *testing.T) {
	const data = `0041;LATIN CAPITAL LETTER A;Lu;0;L;;;;;N;;;;0061;
0061;LATIN SMALL LETTER A;Ll;0;L;;;;;N;;;0041;;0041
01C4;LATIN CAPITAL LETTER DZ WITH CARON;Lu;0;L;<compat> 0044 017D;;;;N;;;;01C6;01C5
01C5;LATIN CAPITAL LETTER D WITH SMALL LETTER Z WITH CARON;Lt;0;L;<compat> 0044 017E;;;;N;;;01C4;01C6;01C5
00DF;LATIN SMALL LETTER SHARP S;Ll;0;L;;;;;N;;;;;
3400;<CJK Ideograph Extension A, First>;Lo;0;L;;;;;N;;;;;
4DBF;<CJK Ideograph Extension A, Last>;Lo;0;L;;;;;N;;;;;
`
	db := &ucdDatabase{}
	if err := db.parseUnicodeData(strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	for value, want := range map[rune]string{'A': "Lu", 'a': "Ll", 0x01C5: "Lt", 0x3400: "Lo", 0x4000: "Lo", 0x4DBF: "Lo", 0x4DC0: "Cn", '0': "Cn"} {
		if got := db.category(value); got != want {
			t.Errorf("category(%U) = %s, want %s", value, got, want)
		}
	}
	if got := db.simpleLowercase('A'); got != 'a' {
		t.Errorf("simpleLowercase(A) = %U", got)
	}
	if got := db.simpleUppercase('a'); got != 'A' {
		t.Errorf("simpleUppercase(a) = %U", got)
	}
	// An explicit titlecase mapping wins; an absent one falls back to the
	// uppercase mapping, and an absent mapping maps to the code point itself.
	if got := db.simpleTitlecase(0x01C4); got != 0x01C5 {
		t.Errorf("simpleTitlecase(01C4) = %U", got)
	}
	if got := db.simpleTitlecase(0x01C6); got != 0x01C6 {
		t.Errorf("simpleTitlecase(01C6) = %U", got)
	}
	if got := db.simpleTitlecase(0x00DF); got != 0x00DF {
		t.Errorf("simpleTitlecase(00DF) = %U", got)
	}
}

func TestParseUnicodeDataRejectsMalformedInput(t *testing.T) {
	for name, data := range map[string]string{
		"field count":    "0041;LATIN CAPITAL LETTER A;Lu;0;L\n",
		"bad code point": "XYZ;BAD;Lu;0;L;;;;;N;;;;;\n",
		"too large":      "110000;BAD;Lu;0;L;;;;;N;;;;;\n",
		"unpaired first": "3400;<CJK, First>;Lo;0;L;;;;;N;;;;;\n",
		"interrupted":    "3400;<CJK, First>;Lo;0;L;;;;;N;;;;;\n0041;A;Lu;0;L;;;;;N;;;;0061;\n",
		"last without":   "4DBF;<CJK, Last>;Lo;0;L;;;;;N;;;;;\n",
		"bad mapping":    "0041;A;Lu;0;L;;;;;N;;;;ZZ;\n",
		"bad category":   "0041;A;L;0;L;;;;;N;;;;;\n",
	} {
		db := &ucdDatabase{}
		if err := db.parseUnicodeData(strings.NewReader(data)); err == nil {
			t.Errorf("%s: parse succeeded", name)
		}
	}
}

func TestParseSpecialCasingFullMappingsAndConditions(t *testing.T) {
	const data = `# SpecialCasing-18.0.0.txt
# Date: test

00DF; 00DF; 0053 0073; 0053 0053; # LATIN SMALL LETTER SHARP S
03A3; 03C2; 03A3; 03A3; Final_Sigma; # GREEK CAPITAL LETTER SIGMA
# 03C3; 03C2; 03A3; 03A3; Final_Sigma; # commented out
0307; ; 0307; 0307; tr After_I; # COMBINING DOT ABOVE
`
	db := &ucdDatabase{}
	if err := db.parseSpecialCasing(strings.NewReader(data), "18.0.0"); err != nil {
		t.Fatal(err)
	}
	want := []specialCasing{
		{codePoint: 0x00DF, lower: []rune{0x00DF}, title: []rune{0x0053, 0x0073}, upper: []rune{0x0053, 0x0053}},
		{codePoint: 0x03A3, lower: []rune{0x03C2}, title: []rune{0x03A3}, upper: []rune{0x03A3}, conditions: []string{"Final_Sigma"}},
		{codePoint: 0x0307, lower: []rune{}, title: []rune{0x0307}, upper: []rune{0x0307}, conditions: []string{"tr", "After_I"}},
	}
	if !reflect.DeepEqual(db.specialCasing, want) {
		t.Fatalf("specialCasing = %+v, want %+v", db.specialCasing, want)
	}
	if err := (&ucdDatabase{}).parseSpecialCasing(strings.NewReader(data), "17.0.0"); err == nil || !strings.Contains(err.Error(), "version header") {
		t.Fatalf("wrong version accepted: %v", err)
	}
	if err := (&ucdDatabase{}).parseSpecialCasing(strings.NewReader("# SpecialCasing-18.0.0.txt\n00DF; 00DF;\n"), "18.0.0"); err == nil {
		t.Fatal("short entry accepted")
	}
}

func TestParseDerivedCorePropertiesCasedAndCaseIgnorable(t *testing.T) {
	const data = `# DerivedCoreProperties-18.0.0.txt
0041..005A    ; Cased # L&  [26]
0061..007A    ; Cased # L&  [26]
005B          ; Cased # synthetic adjacency to merge
0027          ; Case_Ignorable # Po
0300..036F    ; Case_Ignorable # Mn [112]
0030..0039    ; Alphabetic # ignored property
0915          ; InCB; Consonant # ignored property with a value
`
	db := &ucdDatabase{}
	if err := db.parseDerivedCoreProperties(strings.NewReader(data), "18.0.0"); err != nil {
		t.Fatal(err)
	}
	if want := []codePointRange{{0x41, 0x5B}, {0x61, 0x7A}}; !reflect.DeepEqual(db.cased, want) {
		t.Fatalf("cased = %v, want %v", db.cased, want)
	}
	if want := []codePointRange{{0x27, 0x27}, {0x300, 0x36F}}; !reflect.DeepEqual(db.caseIgnorable, want) {
		t.Fatalf("caseIgnorable = %v, want %v", db.caseIgnorable, want)
	}
	if err := (&ucdDatabase{}).parseDerivedCoreProperties(strings.NewReader("# DerivedCoreProperties-18.0.0.txt\n007A..0061 ; Cased\n"), "18.0.0"); err == nil {
		t.Fatal("descending range accepted")
	}
}

func TestParseCaseFoldingStatuses(t *testing.T) {
	const data = `# CaseFolding-18.0.0.txt
0041; C; 0061; # LATIN CAPITAL LETTER A
00DF; F; 0073 0073; # LATIN SMALL LETTER SHARP S
1E9E; F; 0073 0073; # LATIN CAPITAL LETTER SHARP S
1E9E; S; 00DF; # LATIN CAPITAL LETTER SHARP S
0049; T; 0131; # LATIN CAPITAL LETTER I
`
	db := &ucdDatabase{}
	if err := db.parseCaseFolding(strings.NewReader(data), "18.0.0"); err != nil {
		t.Fatal(err)
	}
	if want := map[rune]rune{0x41: 0x61, 0x1E9E: 0xDF}; !reflect.DeepEqual(db.simpleFolding, want) {
		t.Errorf("simpleFolding = %v", db.simpleFolding)
	}
	if want := map[rune][]rune{0x41: {0x61}, 0xDF: {0x73, 0x73}, 0x1E9E: {0x73, 0x73}}; !reflect.DeepEqual(db.fullFolding, want) {
		t.Errorf("fullFolding = %v", db.fullFolding)
	}
	if want := map[rune]rune{0x49: 0x131}; !reflect.DeepEqual(db.turkicFolding, want) {
		t.Errorf("turkicFolding = %v", db.turkicFolding)
	}
	for name, entry := range map[string]string{
		"unknown status":  "0041; X; 0061;",
		"multi simple":    "0041; S; 0061 0062;",
		"multi common":    "0041; C; 0061 0062;",
		"empty mapping":   "0041; F; ;",
		"missing mapping": "0041; C;",
	} {
		if err := (&ucdDatabase{}).parseCaseFolding(strings.NewReader("# CaseFolding-18.0.0.txt\n"+entry+"\n"), "18.0.0"); err == nil {
			t.Errorf("%s: parse succeeded", name)
		}
	}
}

// The repository UCD is the pinned Unicode 18.0.0 data: representative facts
// from each file, including a letter Go's older unicode package lacks.
func TestRepositoryUCDFacts(t *testing.T) {
	db := loadRepositoryUCD(t)
	if db.category(0x1C89) != "Lu" {
		t.Errorf("U+1C89 CYRILLIC CAPITAL LETTER TJE (Unicode 16) category = %s", db.category(0x1C89))
	}
	if db.simpleTitlecase(0x01C4) != 0x01C5 || db.simpleLowercase('A') != 'a' || db.simpleUppercase(0x00DF) != 0x00DF {
		t.Error("simple case mappings differ from UnicodeData.txt")
	}
	foundSharpS, foundFinalSigma := false, false
	for _, entry := range db.specialCasing {
		if entry.codePoint == 0x00DF && len(entry.conditions) == 0 && reflect.DeepEqual(entry.upper, []rune{'S', 'S'}) {
			foundSharpS = true
		}
		if entry.codePoint == 0x03A3 && reflect.DeepEqual(entry.conditions, []string{"Final_Sigma"}) {
			foundFinalSigma = true
		}
	}
	if !foundSharpS || !foundFinalSigma {
		t.Errorf("SpecialCasing: sharp s %t, final sigma %t", foundSharpS, foundFinalSigma)
	}
	if !tableContains(db.cased, 'A') || tableContains(db.cased, '1') || !tableContains(db.caseIgnorable, '\'') || !tableContains(db.caseIgnorable, 0x0301) {
		t.Error("Cased/Case_Ignorable membership differs from DerivedCoreProperties.txt")
	}
	if db.simpleFolding['A'] != 'a' || !reflect.DeepEqual(db.fullFolding[0xDF], []rune{'s', 's'}) || db.turkicFolding['I'] != 0x0131 {
		t.Error("case folding differs from CaseFolding.txt")
	}
	if len(db.turkicFolding) != 2 || len(db.simpleFolding) < len(db.turkicFolding) {
		t.Errorf("unexpected folding sizes: simple %d, turkic %d", len(db.simpleFolding), len(db.turkicFolding))
	}
}

// Letters of the older Unicode version in Go's unicode package stay letters
// in the pinned data, which only adds letters (checked while Go is older).
func TestPinnedLettersExtendGoUnicodeLetters(t *testing.T) {
	if !versionOlder(unicode.Version, unicodeVersion) {
		t.Skipf("Go unicode %s is not older than %s", unicode.Version, unicodeVersion)
	}
	ranges := unicodeLetterRanges(loadRepositoryUCD(t))
	for value := rune(0); value <= unicode.MaxRune; value++ {
		if unicode.IsLetter(value) && !tableContains(ranges, value) {
			t.Fatalf("%U is a letter in Go unicode %s but not in Unicode %s", value, unicode.Version, unicodeVersion)
		}
	}
}

func versionOlder(left string, right string) bool {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		l, _ := strconv.Atoi(leftParts[index])
		r, _ := strconv.Atoi(rightParts[index])
		if l != r {
			return l < r
		}
	}
	return len(leftParts) < len(rightParts)
}

// Any file that is not the pinned Unicode 18.0.0 file is rejected, even when
// it carries the right version header.
func TestLoadUCDRejectsUnpinnedFiles(t *testing.T) {
	dir := t.TempDir()
	for name := range ucdFileHashes {
		data, err := os.ReadFile(filepath.Join(repositoryUCD, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "CaseFolding.txt" {
			data = append(data, []byte("0041; C; 0062; # tampered\n")...)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := loadUCD(dir, unicodeVersion); err == nil || !strings.Contains(err.Error(), "does not match the pinned") {
		t.Fatalf("tampered CaseFolding.txt accepted: %v", err)
	}
	if _, err := loadUCD(repositoryUCD, "17.0.0"); err == nil {
		t.Fatal("unpinned version accepted")
	}
	if _, err := loadUCD(filepath.Join(dir, "missing"), unicodeVersion); err == nil {
		t.Fatal("missing directory accepted")
	}
}
