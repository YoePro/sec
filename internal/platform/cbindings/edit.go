package cbindings

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// OperationKind names one typed metadata mutation (correction § 4(3)).
type OperationKind string

const (
	OpPackageSet        OperationKind = "package.set"
	OpEnvironmentAdd    OperationKind = "environment.add"
	OpEnvironmentSet    OperationKind = "environment.set"
	OpEnvironmentRemove OperationKind = "environment.remove"
	OpModuleAdd         OperationKind = "module.add"
	OpModuleRemove      OperationKind = "module.remove"
)

// Operation is a typed edit request. Nil pointer fields are not changed.
// Environment names the environment of an environment operation and the
// module scope of a module operation unless Common is set.
type Operation struct {
	Kind                    OperationKind
	ID, Version, SourceRoot *string
	Environment             string
	OS, Arch, ABI, CRuntime *string
	CRuntimeVersions        *string
	ClearCRuntimeVersions   bool
	Common                  bool
	Module                  string
}

// Reasons explain a successful edit that changed nothing (correction § 5(4)).
const (
	ReasonAlreadyPresent = "already-present"
	ReasonNotPresent     = "not-present"
	ReasonUnchanged      = "unchanged"
)

// EditResult is the outcome of Apply. Text is the proposed file content;
// it equals the input when Changed is false or the edit is rejected.
type EditResult struct {
	Text        string
	Changed     bool
	Rejected    bool
	Reason      string
	Diagnostics []Diagnostic
}

// Apply performs one mutation as a minimal, comment-preserving text edit.
// The logical change is first applied to the parsed model; the text edit is
// accepted only when the edited file parses without diagnostics into exactly
// that expected model, so an edit can never change another element.
// Rules: MD-021 correction §§ 4(3)-(4), 5(3)-(4), 5(8); rules/platform/ffi.md — § 56;
// rules/projects/projects.md — § 5(4)-(6).
func Apply(text string, op Operation) EditResult {
	original := EditResult{Text: text}
	pkg, diagnostics := Parse(text)
	if pkg == nil {
		original.Rejected, original.Diagnostics = true, diagnostics
		return original
	}
	expected := pkg.clone()
	reason, rejection := expected.apply(op)
	if rejection != nil {
		original.Rejected, original.Diagnostics = true, []Diagnostic{*rejection}
		return original
	}
	if reason != "" {
		original.Reason = reason
		return original
	}
	if diagnostics := expected.Validate(); len(diagnostics) != 0 {
		original.Rejected, original.Diagnostics = true, diagnostics
		return original
	}
	edited, err := editText(text, op, pkg)
	if err != nil {
		original.Rejected = true
		original.Diagnostics = []Diagnostic{diagnostic("c-bindings.unsupported-edit", "%v; edit this element by hand or rewrite it as an explicit table", err)}
		return original
	}
	got, diagnostics := Parse(edited)
	if len(diagnostics) != 0 {
		original.Rejected, original.Diagnostics = true, diagnostics
		return original
	}
	if !got.equal(expected) {
		original.Rejected = true
		original.Diagnostics = []Diagnostic{diagnostic("c-bindings.unsupported-edit", "the file layout does not allow a minimal %s edit", op.Kind)}
		return original
	}
	return EditResult{Text: edited, Changed: true}
}

// apply changes the model and returns a no-op reason or a rejection.
// Rules: MD-021 correction §§ 4(4), 5(4).
func (pkg *Package) apply(op Operation) (string, *Diagnostic) {
	reject := func(id, format string, args ...any) (string, *Diagnostic) {
		d := diagnostic(id, format, args...)
		return "", &d
	}
	switch op.Kind {
	case OpPackageSet:
		before := pkg.clone()
		for field, value := range map[*string]*string{&pkg.ID: op.ID, &pkg.Version: op.Version, &pkg.SourceRoot: op.SourceRoot} {
			if value != nil {
				*field = *value
			}
		}
		if pkg.equal(before) {
			return ReasonUnchanged, nil
		}
		return "", nil
	case OpEnvironmentAdd:
		if !validEnvironmentName(op.Environment) {
			return reject("c-bindings.invalid-environment-name", "environment name %q must start with a lowercase letter and contain only a-z, 0-9, '-' and '_'", op.Environment)
		}
		added := Environment{Name: op.Environment, Modules: []string{}}
		for field, value := range map[*string]*string{&added.OS: op.OS, &added.Arch: op.Arch, &added.ABI: op.ABI, &added.CRuntime: op.CRuntime, &added.CRuntimeVersions: op.CRuntimeVersions} {
			if value != nil {
				*field = *value
			}
		}
		if existing, found := pkg.Environments[op.Environment]; found {
			if existing.OS == added.OS && existing.Arch == added.Arch && existing.ABI == added.ABI && existing.CRuntime == added.CRuntime && existing.CRuntimeVersions == added.CRuntimeVersions {
				return ReasonAlreadyPresent, nil
			}
			return reject("c-bindings.environment-conflict", "environment %q already exists with different selectors; use environment set to change it", op.Environment)
		}
		pkg.Environments[op.Environment] = added
		return "", nil
	case OpEnvironmentSet:
		environment, found := pkg.Environments[op.Environment]
		if !found {
			return reject("c-bindings.unknown-environment", "environment %q does not exist; use environment add", op.Environment)
		}
		before := environment
		for field, value := range map[*string]*string{&environment.OS: op.OS, &environment.Arch: op.Arch, &environment.ABI: op.ABI, &environment.CRuntime: op.CRuntime, &environment.CRuntimeVersions: op.CRuntimeVersions} {
			if value != nil {
				*field = *value
			}
		}
		if op.ClearCRuntimeVersions {
			environment.CRuntimeVersions = ""
		}
		if reflect.DeepEqual(before, environment) {
			return ReasonUnchanged, nil
		}
		pkg.Environments[op.Environment] = environment
		return "", nil
	case OpEnvironmentRemove:
		if _, found := pkg.Environments[op.Environment]; !found {
			return ReasonNotPresent, nil
		}
		delete(pkg.Environments, op.Environment)
		return "", nil
	case OpModuleAdd, OpModuleRemove:
		if err := checkRelativePath(op.Module); err != nil {
			return reject("c-bindings.unsafe-path", "module %q: %v", op.Module, err)
		}
		modules := &pkg.CommonModules
		var environment Environment
		if !op.Common {
			var found bool
			if environment, found = pkg.Environments[op.Environment]; !found {
				return reject("c-bindings.unknown-environment", "environment %q does not exist", op.Environment)
			}
			modules = &environment.Modules
		}
		index := indexOf(*modules, op.Module)
		switch {
		case op.Kind == OpModuleAdd && index >= 0:
			return ReasonAlreadyPresent, nil
		case op.Kind == OpModuleRemove && index < 0:
			return ReasonNotPresent, nil
		case op.Kind == OpModuleAdd:
			*modules = append(append([]string{}, *modules...), op.Module)
		default:
			*modules = append(append([]string{}, (*modules)[:index]...), (*modules)[index+1:]...)
		}
		if !op.Common {
			pkg.Environments[op.Environment] = environment
		}
		return "", nil
	}
	return reject("c-bindings.unknown-operation", "unknown operation %q", op.Kind)
}

func indexOf(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return -1
}

// validEnvironmentName is the bare-key grammar new environments receive, so
// their tables never need quoted keys.
// Rules: MD-021 correction § 2(3); rules/projects/projects.md — § 6(4)-(6).
func validEnvironmentName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func (pkg *Package) clone() *Package {
	out := *pkg
	out.CommonModules = append([]string{}, pkg.CommonModules...)
	out.Environments = map[string]Environment{}
	for name, environment := range pkg.Environments {
		environment.Modules = append([]string{}, environment.Modules...)
		out.Environments[name] = environment
	}
	return &out
}

// equal compares the semantic model; nil and empty module lists are equal.
func (pkg *Package) equal(other *Package) bool {
	if pkg.SchemaVersion != other.SchemaVersion || pkg.ID != other.ID || pkg.Version != other.Version ||
		pkg.SourceRoot != other.SourceRoot || strings.Join(pkg.CommonModules, "\x00") != strings.Join(other.CommonModules, "\x00") ||
		len(pkg.CommonModules) != len(other.CommonModules) || len(pkg.Environments) != len(other.Environments) {
		return false
	}
	for name, environment := range pkg.Environments {
		theirs, found := other.Environments[name]
		if !found || len(environment.Modules) != len(theirs.Modules) {
			return false
		}
		if environment.Name != theirs.Name || environment.OS != theirs.OS || environment.Arch != theirs.Arch || environment.ABI != theirs.ABI ||
			environment.CRuntime != theirs.CRuntime || environment.CRuntimeVersions != theirs.CRuntimeVersions {
			return false
		}
		for index, module := range pkg.Environments[name].Modules {
			if other.Environments[name].Modules[index] != module {
				return false
			}
		}
	}
	return true
}

// editText performs the textual edit for an operation already validated on
// the model.
// Rules: MD-021 correction § 5(3).
func editText(text string, op Operation, pkg *Package) (string, error) {
	doc, err := scanDocument(text)
	if err != nil {
		return "", err
	}
	switch op.Kind {
	case OpPackageSet:
		for _, field := range []struct {
			key   string
			value *string
		}{{"id", op.ID}, {"version", op.Version}, {"source_root", op.SourceRoot}} {
			if field.value == nil {
				continue
			}
			if text, err = doc.setValue([]string{"bindings"}, field.key, tomlString(*field.value)); err != nil {
				return "", err
			}
			if doc, err = scanDocument(text); err != nil {
				return "", err
			}
		}
		return text, nil
	case OpEnvironmentAdd:
		lines := []string{}
		for _, field := range []struct {
			key   string
			value *string
		}{{"os", op.OS}, {"arch", op.Arch}, {"abi", op.ABI}, {"c_runtime", op.CRuntime}, {"c_runtime_versions", op.CRuntimeVersions}} {
			if field.value != nil && *field.value != "" {
				lines = append(lines, field.key+" = "+tomlString(*field.value))
			}
		}
		lines = append(lines, "modules = []")
		return doc.appendTable([]string{"environments", op.Environment}, lines), nil
	case OpEnvironmentSet:
		table := []string{"environments", op.Environment}
		for _, field := range []struct {
			key   string
			value *string
		}{{"os", op.OS}, {"arch", op.Arch}, {"abi", op.ABI}, {"c_runtime", op.CRuntime}, {"c_runtime_versions", op.CRuntimeVersions}} {
			if field.value == nil {
				continue
			}
			if text, err = doc.setValue(table, field.key, tomlString(*field.value)); err != nil {
				return "", err
			}
			if doc, err = scanDocument(text); err != nil {
				return "", err
			}
		}
		if op.ClearCRuntimeVersions && pkg.Environments[op.Environment].CRuntimeVersions != "" {
			return doc.removeKey(table, "c_runtime_versions")
		}
		return text, nil
	case OpEnvironmentRemove:
		return doc.removeTable([]string{"environments", op.Environment})
	case OpModuleAdd, OpModuleRemove:
		table, key := []string{"environments", op.Environment}, "modules"
		if op.Common {
			table, key = []string{"modules"}, "common"
		}
		if op.Kind == OpModuleAdd {
			return doc.addArrayString(table, key, op.Module)
		}
		return doc.removeArrayString(table, key, op.Module)
	}
	return "", fmt.Errorf("unknown operation %q", op.Kind)
}

// tomlString renders a TOML basic string, escaping only what TOML requires.
// Rules: rules/projects/projects.md — § 5(2).
func tomlString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, char := range value {
		switch {
		case char == '"' || char == '\\':
			out.WriteByte('\\')
			out.WriteRune(char)
		case char == '\n':
			out.WriteString(`\n`)
		case char == '\t':
			out.WriteString(`\t`)
		case char < 0x20 || char == 0x7f || char == utf8.RuneError:
			fmt.Fprintf(&out, `\u%04X`, char)
		default:
			out.WriteRune(char)
		}
	}
	out.WriteByte('"')
	return out.String()
}
