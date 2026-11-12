package project

import (
	"fmt"
	"sort"
	"strconv"
)

// Parameter is one typed compile-time project parameter (projects.md § 24):
// Kind is "bool", "integer", "float" or "string"; Text is its canonical
// value.
type Parameter struct {
	Kind string
	Bool bool
	Text string
}

// ParseManifestParameters reads the project `[parameters]` table and the
// selected Target's `[target.<name>.parameters]` overrides. The project
// declaration establishes each parameter's type; an override must keep that
// type and must not introduce a parameter (projects.md § 24(7), (11), (12)).
// Arrays, tables and date/time values are not Sec 0.1 parameters (§ 24(6)).
//
// Rules:
//   - rules/projects/projects.md — § 24 "Compile-time parameters"
func ParseManifestParameters(text string, target string) (map[string]Parameter, []error) {
	doc, err := ParseManifestDocument(text)
	if err != nil {
		return nil, []error{err}
	}
	parameters := map[string]Parameter{}
	problems := []error{}
	declared, err := doc.Table("parameters")
	if err == nil {
		for _, name := range sortedKeys(declared) {
			parameter, ok := parameterValue(declared[name])
			if !ok {
				problems = append(problems, fmt.Errorf("parameter %s: %T is not a Sec 0.1 compile-time parameter value (bool, integer, float or string)", name, declared[name]))
				continue
			}
			parameters[name] = parameter
		}
	}
	if target == "" {
		return parameters, problems
	}
	overrides, err := doc.Table("target", target, "parameters")
	if err != nil {
		return parameters, problems
	}
	for _, name := range sortedKeys(overrides) {
		base, known := parameters[name]
		if !known {
			problems = append(problems, fmt.Errorf("target %s overrides parameter %s, which [parameters] does not declare", target, name))
			continue
		}
		parameter, ok := parameterValue(overrides[name])
		if !ok || parameter.Kind != base.Kind {
			problems = append(problems, fmt.Errorf("target %s overrides %s parameter %s with a different type", target, base.Kind, name))
			continue
		}
		parameters[name] = parameter
	}
	return parameters, problems
}

func parameterValue(value any) (Parameter, bool) {
	switch value := value.(type) {
	case bool:
		return Parameter{Kind: "bool", Bool: value, Text: strconv.FormatBool(value)}, true
	case int64:
		return Parameter{Kind: "integer", Text: strconv.FormatInt(value, 10)}, true
	case int:
		return Parameter{Kind: "integer", Text: strconv.Itoa(value)}, true
	case float64:
		return Parameter{Kind: "float", Text: strconv.FormatFloat(value, 'g', -1, 64)}, true
	case string:
		return Parameter{Kind: "string", Text: value}, true
	}
	return Parameter{}, false
}

func sortedKeys(table map[string]any) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
