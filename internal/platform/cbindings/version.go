package cbindings

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a MAJOR.MINOR.PATCH number; a runtime version with two parts
// normalizes PATCH to zero (correction § 8.4(2)).
type Version struct{ Major, Minor, Patch uint64 }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Compare orders versions componentwise.
// Rules: MD-021 correction § 8.4(1).
func (v Version) Compare(other Version) int {
	for _, pair := range [][2]uint64{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// ParsePackageVersion reads a binding-package version: exactly
// MAJOR.MINOR.PATCH decimal numbers.
// Rules: MD-021 correction § 8.4(1).
func ParsePackageVersion(text string) (Version, error) { return parseVersion(text, false) }

// ParseRuntimeVersion reads a C-runtime version: MAJOR.MINOR or
// MAJOR.MINOR.PATCH.
// Rules: MD-021 correction § 8.4(2).
func ParseRuntimeVersion(text string) (Version, error) { return parseVersion(text, true) }

// parseVersion reads two or three decimal components.
// Rules: MD-021 correction § 8.4(1)-(2).
func parseVersion(text string, allowTwoParts bool) (Version, error) {
	parts := strings.Split(text, ".")
	if len(parts) != 3 && !(allowTwoParts && len(parts) == 2) {
		if allowTwoParts {
			return Version{}, fmt.Errorf("version %q is not MAJOR.MINOR[.PATCH]", text)
		}
		return Version{}, fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", text)
	}
	numbers := make([]uint64, 3)
	for index, part := range parts {
		if part == "" || strings.TrimLeft(part, "0123456789") != "" {
			return Version{}, fmt.Errorf("version %q has a non-decimal component %q", text, part)
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("version %q: %v", text, err)
		}
		numbers[index] = value
	}
	return Version{numbers[0], numbers[1], numbers[2]}, nil
}

// Comparator is one `op version` term of a constraint.
type Comparator struct {
	Op      string
	Version Version
}

// Constraint is a conjunction of comparators; no OR, wildcard, caret,
// tilde, prerelease or implicit latest exists in schema v1.
type Constraint []Comparator

// ParseConstraint reads a comma-separated conjunction of `>`, `>=`, `<`,
// `<=`, `=` comparisons. packageVersions selects MAJOR.MINOR.PATCH package
// operands; otherwise operands are MAJOR.MINOR[.PATCH] runtime versions.
// Rules: MD-021 correction § 8.4(1)-(2).
func ParseConstraint(text string, packageVersions bool) (Constraint, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty version constraint")
	}
	var constraint Constraint
	for _, term := range strings.Split(text, ",") {
		term = strings.TrimSpace(term)
		op := ""
		for _, candidate := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(term, candidate) {
				op = candidate
				break
			}
		}
		if op == "" {
			return nil, fmt.Errorf("constraint term %q needs one of >, >=, <, <=, =", term)
		}
		operand := strings.TrimSpace(strings.TrimPrefix(term, op))
		var version Version
		var err error
		if packageVersions {
			version, err = ParsePackageVersion(operand)
		} else {
			version, err = ParseRuntimeVersion(operand)
		}
		if err != nil {
			return nil, err
		}
		constraint = append(constraint, Comparator{op, version})
	}
	return constraint, nil
}

// Allows reports whether version satisfies every comparator.
// Rules: MD-021 correction § 8.4(1).
func (c Constraint) Allows(version Version) bool {
	for _, comparator := range c {
		order := version.Compare(comparator.Version)
		ok := false
		switch comparator.Op {
		case ">":
			ok = order > 0
		case ">=":
			ok = order >= 0
		case "<":
			ok = order < 0
		case "<=":
			ok = order <= 0
		case "=":
			ok = order == 0
		}
		if !ok {
			return false
		}
	}
	return true
}
