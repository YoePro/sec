package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const maxCodePoint = 0x10FFFF

// ucdFileHashes pins the exact Unicode Character Database files the generator
// accepts for unicodeVersion. A different file, even one with a matching
// version header, is rejected so generated tables never drift silently.
var ucdFileHashes = map[string]string{
	"UnicodeData.txt":           "0736451de439ae7baf1425136617da495e09ee5afbe6e394374db7009ea08950",
	"SpecialCasing.txt":         "8538dea57c184f1ef3783885ea79677b10f6efa06423717157e63712f14d1ad2",
	"DerivedCoreProperties.txt": "09c928886a178fcafd93c29e4bd59073a058e5a100b716d425cb563ab50f68c9",
	"CaseFolding.txt":           "a004797658a457bec4dc11683e39f69249ea3b595b752dbea6721c4c9f587b0d",
}

// codePointRange is an inclusive range of code points.
type codePointRange struct {
	start rune
	end   rune
}

// specialCasing is one SpecialCasing.txt entry: full (possibly multi-code-point)
// lower, title and upper mappings, applied only when every condition holds.
// Conditions are language identifiers (tr, lt, az) and casing contexts
// (Final_Sigma, After_I, ...). An entry without conditions is unconditional.
type specialCasing struct {
	codePoint  rune
	lower      []rune
	title      []rune
	upper      []rune
	conditions []string
}

// ucdDatabase holds the UCD facts the generator consumes for one exact
// Unicode version.
type ucdDatabase struct {
	version string

	// generalCategory maps every assigned code point to its two-letter
	// General_Category value. Unlisted code points are Cn.
	generalCategory map[rune]string
	// simpleUpper, simpleLower and simpleTitle hold the explicit
	// UnicodeData.txt single-code-point mappings (fields 12, 13 and 14).
	simpleUpper map[rune]rune
	simpleLower map[rune]rune
	simpleTitle map[rune]rune

	specialCasing []specialCasing

	cased         []codePointRange
	caseIgnorable []codePointRange

	// simpleFolding is the C+S case folding, fullFolding the C+F case
	// folding, and turkicFolding the T entries (CaseFolding.txt status).
	simpleFolding map[rune]rune
	fullFolding   map[rune][]rune
	turkicFolding map[rune]rune
}

// loadUCD reads the four UCD files from dir, verifies that each one is the
// pinned file for version, and parses them.
func loadUCD(dir string, version string) (*ucdDatabase, error) {
	db := &ucdDatabase{version: version}
	steps := []struct {
		name  string
		parse func(io.Reader) error
	}{
		{"UnicodeData.txt", db.parseUnicodeData},
		{"SpecialCasing.txt", func(r io.Reader) error { return db.parseSpecialCasing(r, version) }},
		{"DerivedCoreProperties.txt", func(r io.Reader) error { return db.parseDerivedCoreProperties(r, version) }},
		{"CaseFolding.txt", func(r io.Reader) error { return db.parseCaseFolding(r, version) }},
	}
	for _, step := range steps {
		path := filepath.Join(dir, step.name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if want := ucdFileHashes[step.name]; version != unicodeVersion || want == "" {
			return nil, fmt.Errorf("%s: no pinned file for Unicode %s", path, version)
		} else {
			sum := sha256.Sum256(data)
			if got := hex.EncodeToString(sum[:]); got != want {
				return nil, fmt.Errorf("%s: sha256 %s does not match the pinned Unicode %s file %s", path, got, version, want)
			}
		}
		if err := step.parse(strings.NewReader(string(data))); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return db, nil
}

// ucdLines yields the data part of each non-empty, non-comment line with its
// 1-based line number. The first line is returned separately so callers can
// check the version header.
func ucdLines(r io.Reader, visit func(line int, fields []string) error) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	header := ""
	number := 0
	for scanner.Scan() {
		number++
		text := scanner.Text()
		if number == 1 {
			header = text
		}
		if hash := strings.IndexByte(text, '#'); hash >= 0 {
			text = text[:hash]
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, ";")
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		if err := visit(number, fields); err != nil {
			return header, fmt.Errorf("line %d: %w", number, err)
		}
	}
	return header, scanner.Err()
}

// checkVersionHeader requires the "# Name-<version>.txt" first line that
// versioned UCD files carry.
func checkVersionHeader(header string, name string, version string) error {
	want := fmt.Sprintf("# %s-%s.txt", name, version)
	if strings.TrimSpace(header) != want {
		return fmt.Errorf("version header %q, want %q", header, want)
	}
	return nil
}

func parseCodePoint(text string) (rune, error) {
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil || value > maxCodePoint {
		return 0, fmt.Errorf("invalid code point %q", text)
	}
	return rune(value), nil
}

func parseCodePoints(text string) ([]rune, error) {
	fields := strings.Fields(text)
	values := make([]rune, 0, len(fields))
	for _, field := range fields {
		value, err := parseCodePoint(field)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func parseCodePointRange(text string) (codePointRange, error) {
	startText, endText, isRange := strings.Cut(text, "..")
	start, err := parseCodePoint(startText)
	if err != nil {
		return codePointRange{}, err
	}
	end := start
	if isRange {
		if end, err = parseCodePoint(endText); err != nil {
			return codePointRange{}, err
		}
	}
	if end < start {
		return codePointRange{}, fmt.Errorf("descending range %q", text)
	}
	return codePointRange{start: start, end: end}, nil
}

// parseUnicodeData reads General_Category and the simple case mappings.
// UnicodeData.txt carries no version header; its version is pinned by hash.
// "<..., First>" / "<..., Last>" pairs assign one category to the whole range.
func (db *ucdDatabase) parseUnicodeData(r io.Reader) error {
	db.generalCategory = map[rune]string{}
	db.simpleUpper = map[rune]rune{}
	db.simpleLower = map[rune]rune{}
	db.simpleTitle = map[rune]rune{}
	var rangeStart rune = -1
	_, err := ucdLines(r, func(_ int, fields []string) error {
		if len(fields) != 15 {
			return fmt.Errorf("want 15 fields, got %d", len(fields))
		}
		codePoint, err := parseCodePoint(fields[0])
		if err != nil {
			return err
		}
		name, category := fields[1], fields[2]
		if len(category) != 2 {
			return fmt.Errorf("invalid General_Category %q", category)
		}
		switch {
		case strings.HasSuffix(name, ", First>"):
			rangeStart = codePoint
			return nil
		case strings.HasSuffix(name, ", Last>"):
			if rangeStart < 0 || rangeStart > codePoint {
				return fmt.Errorf("range end %04X without a matching First entry", codePoint)
			}
			for value := rangeStart; value <= codePoint; value++ {
				db.generalCategory[value] = category
			}
			rangeStart = -1
			return nil
		case rangeStart >= 0:
			return fmt.Errorf("range First entry %04X is not followed by its Last entry", rangeStart)
		}
		db.generalCategory[codePoint] = category
		for index, mapping := range []map[rune]rune{db.simpleUpper, db.simpleLower, db.simpleTitle} {
			text := fields[12+index]
			if text == "" {
				continue
			}
			value, err := parseCodePoint(text)
			if err != nil {
				return err
			}
			mapping[codePoint] = value
		}
		return nil
	})
	if err == nil && rangeStart >= 0 {
		err = fmt.Errorf("range First entry %04X has no Last entry", rangeStart)
	}
	return err
}

// parseSpecialCasing reads "code; lower; title; upper; [conditions;]" entries.
// Commented-out entries are ignored with every other comment.
func (db *ucdDatabase) parseSpecialCasing(r io.Reader, version string) error {
	header, err := ucdLines(r, func(_ int, fields []string) error {
		// A trailing ";" before the comment leaves one empty final field.
		if len(fields) > 0 && fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
		if len(fields) != 4 && len(fields) != 5 {
			return fmt.Errorf("want 4 or 5 fields, got %d", len(fields))
		}
		codePoint, err := parseCodePoint(fields[0])
		if err != nil {
			return err
		}
		entry := specialCasing{codePoint: codePoint}
		for index, target := range []*[]rune{&entry.lower, &entry.title, &entry.upper} {
			if *target, err = parseCodePoints(fields[1+index]); err != nil {
				return err
			}
		}
		if len(fields) == 5 {
			entry.conditions = strings.Fields(fields[4])
		}
		db.specialCasing = append(db.specialCasing, entry)
		return nil
	})
	if err != nil {
		return err
	}
	return checkVersionHeader(header, "SpecialCasing", version)
}

// parseDerivedCoreProperties collects the Cased and Case_Ignorable ranges.
func (db *ucdDatabase) parseDerivedCoreProperties(r io.Reader, version string) error {
	db.cased, db.caseIgnorable = nil, nil
	header, err := ucdLines(r, func(_ int, fields []string) error {
		if len(fields) < 2 {
			return fmt.Errorf("want at least 2 fields, got %d", len(fields))
		}
		var target *[]codePointRange
		switch fields[1] {
		case "Cased":
			target = &db.cased
		case "Case_Ignorable":
			target = &db.caseIgnorable
		default:
			return nil
		}
		value, err := parseCodePointRange(fields[0])
		if err != nil {
			return err
		}
		*target = append(*target, value)
		return nil
	})
	if err != nil {
		return err
	}
	db.cased = normalizeRanges(db.cased)
	db.caseIgnorable = normalizeRanges(db.caseIgnorable)
	return checkVersionHeader(header, "DerivedCoreProperties", version)
}

// parseCaseFolding reads "code; status; mapping;" entries. C entries belong to
// both simple and full folding, S only to simple, F only to full, and T are
// the Turkic special cases.
func (db *ucdDatabase) parseCaseFolding(r io.Reader, version string) error {
	db.simpleFolding = map[rune]rune{}
	db.fullFolding = map[rune][]rune{}
	db.turkicFolding = map[rune]rune{}
	header, err := ucdLines(r, func(_ int, fields []string) error {
		if len(fields) > 0 && fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
		if len(fields) != 3 {
			return fmt.Errorf("want 3 fields, got %d", len(fields))
		}
		codePoint, err := parseCodePoint(fields[0])
		if err != nil {
			return err
		}
		mapping, err := parseCodePoints(fields[2])
		if err != nil {
			return err
		}
		if len(mapping) == 0 {
			return fmt.Errorf("empty folding for %04X", codePoint)
		}
		single := func() (rune, error) {
			if len(mapping) != 1 {
				return 0, fmt.Errorf("status %s folding for %04X must be one code point", fields[1], codePoint)
			}
			return mapping[0], nil
		}
		switch fields[1] {
		case "C":
			value, err := single()
			if err != nil {
				return err
			}
			db.simpleFolding[codePoint] = value
			db.fullFolding[codePoint] = mapping
		case "S":
			value, err := single()
			if err != nil {
				return err
			}
			db.simpleFolding[codePoint] = value
		case "F":
			db.fullFolding[codePoint] = mapping
		case "T":
			value, err := single()
			if err != nil {
				return err
			}
			db.turkicFolding[codePoint] = value
		default:
			return fmt.Errorf("unknown folding status %q", fields[1])
		}
		return nil
	})
	if err != nil {
		return err
	}
	return checkVersionHeader(header, "CaseFolding", version)
}

// category returns the General_Category of value; unassigned code points
// are Cn (UAX #44 default).
func (db *ucdDatabase) category(value rune) string {
	if category, ok := db.generalCategory[value]; ok {
		return category
	}
	return "Cn"
}

// simpleUppercase, simpleLowercase and simpleTitlecase apply the UAX #44
// defaults: an absent mapping maps a code point to itself, and an absent
// titlecase mapping falls back to the uppercase mapping.
func (db *ucdDatabase) simpleUppercase(value rune) rune {
	if mapped, ok := db.simpleUpper[value]; ok {
		return mapped
	}
	return value
}

func (db *ucdDatabase) simpleLowercase(value rune) rune {
	if mapped, ok := db.simpleLower[value]; ok {
		return mapped
	}
	return value
}

func (db *ucdDatabase) simpleTitlecase(value rune) rune {
	if mapped, ok := db.simpleTitle[value]; ok {
		return mapped
	}
	return db.simpleUppercase(value)
}

// rangesWhere collects maximal ranges of code points satisfying include.
func rangesWhere(include func(rune) bool) []codePointRange {
	var ranges []codePointRange
	for value := rune(0); value <= maxCodePoint; value++ {
		if !include(value) {
			continue
		}
		if count := len(ranges); count > 0 && ranges[count-1].end == value-1 {
			ranges[count-1].end = value
			continue
		}
		ranges = append(ranges, codePointRange{start: value, end: value})
	}
	return ranges
}

// normalizeRanges sorts ranges and merges overlapping or adjacent ones.
func normalizeRanges(ranges []codePointRange) []codePointRange {
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	merged := ranges[:0]
	for _, value := range ranges {
		if count := len(merged); count > 0 && value.start <= merged[count-1].end+1 {
			if value.end > merged[count-1].end {
				merged[count-1].end = value.end
			}
			continue
		}
		merged = append(merged, value)
	}
	return merged
}
