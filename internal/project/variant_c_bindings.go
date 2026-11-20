package project

import (
	"fmt"
	"sort"
)

// VariantCBindings are the target and C-binding selectors of one
// `[variant.<name>]` table (projects.md § 19(16)).
type VariantCBindings struct {
	Variant         string
	OS              string
	Arch            string
	ABI             string
	CRuntime        string
	CRuntimeVersion string
	// BindingsID, BindingsVersion and BindingsEnvironment come from the
	// optional c_bindings table; Selected reports whether it is present.
	Selected            bool
	BindingsID          string
	BindingsVersion     string
	BindingsEnvironment string
}

// ReadVariantCBindings reads one Variant's target and C-binding selectors.
// An unknown Variant, non-string values and unknown c_bindings keys are
// configuration errors; c_bindings requires id.
// Rules: rules/projects/projects.md — §§ 19(1)-(3), 19(16); MD-021 correction § 8.3.
func ReadVariantCBindings(text, variant string) (VariantCBindings, []error) {
	result := VariantCBindings{Variant: variant}
	doc, err := ParseManifestDocument(text)
	if err != nil {
		return result, []error{err}
	}
	table, err := doc.Table("variant", variant)
	if err != nil {
		return result, []error{err}
	}
	if table == nil {
		return result, []error{fmt.Errorf("variant %q is not declared in the project manifest", variant)}
	}
	var errs []error
	text2 := func(key string) string {
		value, found := table[key]
		if !found {
			return ""
		}
		spelled, ok := value.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("[variant.%s].%s must be a string", variant, key))
		}
		return spelled
	}
	result.OS, result.Arch, result.ABI = text2("os"), text2("arch"), text2("abi")
	result.CRuntime, result.CRuntimeVersion = text2("c_runtime"), text2("c_runtime_version")
	if value, found := table["c_bindings"]; found {
		selector, ok := value.(map[string]any)
		if !ok {
			return result, append(errs, fmt.Errorf("[variant.%s].c_bindings must be a table", variant))
		}
		result.Selected = true
		keys := make([]string, 0, len(selector))
		for key := range selector {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			spelled, ok := selector[key].(string)
			if !ok {
				errs = append(errs, fmt.Errorf("[variant.%s].c_bindings.%s must be a string", variant, key))
				continue
			}
			switch key {
			case "id":
				result.BindingsID = spelled
			case "version":
				result.BindingsVersion = spelled
			case "environment":
				result.BindingsEnvironment = spelled
			default:
				errs = append(errs, fmt.Errorf("[variant.%s].c_bindings has unknown key %q", variant, key))
			}
		}
		if result.BindingsID == "" {
			errs = append(errs, fmt.Errorf("[variant.%s].c_bindings.id is required", variant))
		}
	}
	return result, errs
}
