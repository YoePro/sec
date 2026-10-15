package diagnostics

import (
	"bufio"
	"strconv"
	"strings"
)

// RulePolicy is the rule-specific diagnostic policy of a project: stable IDs
// or symbolic names mapped to off, info, warning or error
// (rules/tooling/diagnostics.md § 18(3)-(4)). Only configurable definitions
// are affected; mandatory diagnostics can be neither disabled nor demoted.
type RulePolicy map[string]string

// ParseRulePolicy reads the `[diagnostics.rules]` section of a sec.toml
// manifest. Keys may be quoted; values are quoted or bare policy states;
// trailing comments are ignored. Unknown states are dropped rather than
// guessed.
func ParseRulePolicy(manifest string) RulePolicy {
	policy := RulePolicy{}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(manifest))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "diagnostics.rules" || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if unquoted, err := strconv.Unquote(key); err == nil {
			key = unquoted
		}
		value = strings.TrimSpace(value)
		if comment := strings.Index(value, "#"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		switch value {
		case "off", "info", "warning", "error":
			policy[key] = value
		}
	}
	return policy
}

// Effective returns the effective severity of a configurable diagnostic and
// whether it is reported at all. A rule keyed by stable ID takes precedence
// over one keyed by symbolic name. Mandatory and unknown definitions keep
// their registered default.
func (policy RulePolicy) Effective(id string) (Severity, bool) {
	definition, known := registry[id]
	if !known {
		return "", true
	}
	if definition.Mandatory || len(policy) == 0 {
		return definition.DefaultSeverity, true
	}
	state, ok := policy[id]
	if !ok {
		state, ok = policy[definition.Name]
	}
	if !ok {
		return definition.DefaultSeverity, true
	}
	switch state {
	case "off":
		return "", false
	case "info":
		return SeverityInformation, true
	case "warning":
		return SeverityWarning, true
	case "error":
		return SeverityError, true
	}
	return definition.DefaultSeverity, true
}
